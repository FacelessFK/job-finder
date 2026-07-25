// Package pipeline مراحل گرفتن، نرمال‌سازی، dedup، فیلتر و انتشار را هماهنگ می‌کند.
package pipeline

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/aghaie/job-finder/internal/core"
	"github.com/aghaie/job-finder/internal/dedup"
	"github.com/aghaie/job-finder/internal/filter"
	"github.com/aghaie/job-finder/internal/normalize"
	"github.com/aghaie/job-finder/internal/providers"
	"github.com/aghaie/job-finder/internal/publisher"
	"github.com/aghaie/job-finder/internal/rotate"
	"github.com/aghaie/job-finder/internal/store"
)

// Stats خلاصه‌ی یک اجرا.
type Stats = core.RunSummary

// Pipeline اجزای سرویس را نگه می‌دارد.
type Pipeline struct {
	providers []providers.Provider
	chain     *filter.Chain
	store     store.SeenStore
	pub       publisher.Publisher
	log       *slog.Logger
	filters   core.Filters
	maxPerRun int
	delay     time.Duration
	summary   core.SummaryMode
}

// New یک Pipeline می‌سازد.
func New(provs []providers.Provider, chain *filter.Chain, st store.SeenStore, pub publisher.Publisher, log *slog.Logger, cfg core.Config) *Pipeline {
	f := cfg.Filters
	// چرخش کشورها: هر اجرا فقط بخشی از فهرست را می‌گیرد تا سهمیه‌ی API
	// در طول شبانه‌روز پخش شود به‌جای مصرف یک‌جا.
	f.Countries = rotate.Slice(f.Countries, time.Now().UTC(),
		cfg.Rotation.SlotHours, cfg.Rotation.CountriesPerRun)

	return &Pipeline{
		providers: provs,
		chain:     chain,
		store:     st,
		pub:       pub,
		log:       log,
		filters:   f,
		maxPerRun: cfg.MaxPerRun,
		delay:     time.Duration(cfg.DelaySeconds) * time.Second,
		summary:   cfg.Summary,
	}
}

// Run یک چرخه‌ی کامل را اجرا می‌کند.
func (p *Pipeline) Run(ctx context.Context) (Stats, error) {
	var stats Stats

	// ۱) گرفتن موازی از همه Providerها (Error Isolation)
	all, fetchErrs := p.fetchAll(ctx)
	stats.Fetched = len(all)
	stats.Errors = fetchErrs

	// ۲) نرمال‌سازی
	for i := range all {
		all[i] = normalize.Job(all[i])
	}

	// ۳+۴) dedup داخل‌اجرا
	deduped := dedup.Dedupe(all)
	stats.AfterDedup = len(deduped)

	// ۵) حذف دیده‌شده‌ها
	var fresh []core.Job
	for _, j := range deduped {
		if !p.store.IsSeen(j.Fingerprint()) {
			fresh = append(fresh, j)
		}
	}
	stats.AfterSeen = len(fresh)

	// ۶+۷) فیلتر و انتشار
	for _, j := range fresh {
		if p.maxPerRun > 0 && stats.Published >= p.maxPerRun {
			p.log.Info("reached maxPerRun", "max", p.maxPerRun)
			break
		}
		if !p.chain.Allow(ctx, j) {
			continue
		}
		if err := p.pub.Publish(ctx, j); err != nil {
			p.log.Warn("publish failed", "job", j.Title, "err", err)
			continue // دیده‌شده علامت نمی‌زنیم تا دفعه‌ی بعد دوباره تلاش شود
		}
		p.store.MarkSeen(j.Fingerprint())
		stats.Published++
		if p.delay > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(p.delay):
			}
		}
	}

	// ۸) ذخیره
	if err := p.store.Save(ctx); err != nil {
		return stats, err
	}

	// ۹) گزارش پایان اجرا طبق حالت پیکربندی‌شده. با اجراهای پرتکرار،
	// حالت onChange فقط وقتی حرف می‌زند که چیزی منتشر شده یا خطایی داده.
	if p.summary.ShouldSend(stats) {
		if err := p.pub.PublishSummary(ctx, stats); err != nil {
			p.log.Warn("summary publish failed", "err", err)
		}
	}

	p.log.Info("run complete",
		"fetched", stats.Fetched, "afterDedup", stats.AfterDedup,
		"afterSeen", stats.AfterSeen, "published", stats.Published,
		"errors", len(stats.Errors))
	return stats, nil
}

func (p *Pipeline) fetchAll(ctx context.Context) ([]core.Job, []string) {
	var (
		mu   sync.Mutex
		all  []core.Job
		errs []string
		wg   sync.WaitGroup
	)
	for _, prov := range p.providers {
		wg.Add(1)
		go func(pr providers.Provider) {
			defer wg.Done()
			jobs, err := pr.SearchJobs(ctx, p.filters)
			mu.Lock()
			if err != nil {
				p.log.Warn("provider fetch failed", "provider", pr.Name(), "err", err)
				errs = append(errs, pr.Name()+": "+err.Error())
			}
			all = append(all, jobs...)
			mu.Unlock()
			p.log.Info("provider fetched", "provider", pr.Name(), "count", len(jobs))
		}(prov)
	}
	wg.Wait()
	return all, errs
}
