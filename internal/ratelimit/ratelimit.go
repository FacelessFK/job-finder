// Package ratelimit یک محدودکننده‌ی نرخِ ساده (حداقل فاصله بین فراخوانی‌ها).
package ratelimit

import (
	"context"
	"time"
)

// Limiter حداقل فاصله بین فراخوانی‌ها را تضمین می‌کند. امن برای استفاده‌ی هم‌زمان.
type Limiter struct {
	interval time.Duration
	tokens   chan struct{}
	last     time.Time
}

// New یک Limiter با فاصله‌ی مشخص می‌سازد.
func New(interval time.Duration) *Limiter {
	l := &Limiter{interval: interval, tokens: make(chan struct{}, 1)}
	l.tokens <- struct{}{}
	return l
}

// Wait تا زمان مجاز برای فراخوانی بعدی صبر می‌کند.
func (l *Limiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.tokens:
	}
	defer func() { l.tokens <- struct{}{} }()

	if l.interval > 0 {
		if wait := l.interval - time.Since(l.last); wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	l.last = time.Now()
	return nil
}
