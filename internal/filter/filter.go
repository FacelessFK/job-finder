// Package filter زنجیره‌ی فیلترهاست؛ جاب باید از همه رد شود تا منتشر گردد.
package filter

import (
	"context"
	"log/slog"

	"github.com/aghaie/job-finder/internal/core"
)

// Filter یک قاعده‌ی فیلترکردن (قانون‌محور یا AI).
type Filter interface {
	Name() string
	Evaluate(ctx context.Context, job core.Job) (core.Decision, error)
}

// Chain چند فیلتر را پشت سر هم اجرا می‌کند.
type Chain struct {
	filters []Filter
	log     *slog.Logger
}

// NewChain یک زنجیره می‌سازد.
func NewChain(log *slog.Logger, filters ...Filter) *Chain {
	return &Chain{filters: filters, log: log}
}

// Allow اگر جاب از همه‌ی فیلترها رد شود true برمی‌گرداند؛ در غیر این‌صورت دلیل را لاگ می‌کند.
func (c *Chain) Allow(ctx context.Context, job core.Job) bool {
	for _, f := range c.filters {
		d, err := f.Evaluate(ctx, job)
		if err != nil {
			c.log.Warn("filter error", "filter", f.Name(), "job", job.Title, "err", err)
			return false
		}
		if !d.Publish {
			c.log.Info("job rejected", "filter", f.Name(), "job", job.Title, "company", job.Company, "reason", d.Reason)
			return false
		}
	}
	return true
}
