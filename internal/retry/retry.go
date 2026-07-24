// Package retry تلاش مجدد با backoff نمایی برای خطاهای موقت.
package retry

import (
	"context"
	"time"
)

// Do تابع fn را تا attempts بار اجرا می‌کند. اگر isRetryable(err) نادرست باشد بلافاصله
// برمی‌گردد. بین تلاش‌ها با backoff نمایی صبر می‌کند.
func Do(ctx context.Context, attempts int, base time.Duration, isRetryable func(error) bool, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var err error
	delay := base
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if isRetryable != nil && !isRetryable(err) {
			return err
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
	}
	return err
}
