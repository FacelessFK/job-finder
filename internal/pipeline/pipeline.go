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
	"github.com/aghaie/job-finder/internal/store"
)

// Stats خلاصه‌ی یک اجرا.
type Stats struct {
	Fetched    int
	AfterDedup int
	AfterSeen  int
	Published  int
}

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
}

// New یک Pipeline می‌سازد.
func New(provs []providers.Provider, chain *filter.Chain, st store.SeenStore, pub publisher.Publisher, log *slog.Logger, cfg core.Config) *Pipeline {
	return &Pipeline{
		providers: provs,
		chain:     chain,
		store:     st,
		pub:       pub,
		log:       log,
		filters:   cfg.Filters,
		maxPerRun: cfg.MaxPerRun,
		delay:     time.Duration(cfg.DelaySeconds) * time.Second,
	}
}

// Run یک چرخه‌ی کامل را اجرا می‌کند.
func (p *Pipeline) Run(ctx context.Context) (Stats, error) {
	var stats Stats

	// ۱) گرفتن موازی از همه Providerها (Error Isolation)
	all := p.fetchAll(ctx)
	stats.Fetched = len(all)

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
	p.log.Info("run complete",
		"fetched", stats.Fetched, "afterDedup", stats.AfterDedup,
		"afterSeen", stats.AfterSeen, "published", stats.Published)
	return stats, nil
}

func (p *Pipeline) fetchAll(ctx context.Context) []core.Job {
	var (
		mu  sync.Mutex
		all []core.Job
		wg  sync.WaitGroup
	)
	for _, prov := range p.providers {
		wg.Add(1)
		go func(pr providers.Provider) {
			defer wg.Done()
			jobs, err := pr.SearchJobs(ctx, p.filters)
			if err != nil {
				p.log.Warn("provider fetch failed", "provider", pr.Name(), "err", err)
			}
			if len(jobs) > 0 {
				mu.Lock()
				all = append(all, jobs...)
				mu.Unlock()
			}
			p.log.Info("provider fetched", "provider", pr.Name(), "count", len(jobs))
		}(prov)
	}
	wg.Wait()
	return all
}
