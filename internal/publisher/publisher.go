// Package publisher رابط مقصد انتشار جاب‌ها.
package publisher

import (
	"context"

	"github.com/aghaie/job-finder/internal/core"
)

// Publisher یک مقصد انتشار (تلگرام و ...).
type Publisher interface {
	Publish(ctx context.Context, job core.Job) error
	// PublishSummary گزارش پایان اجرا را می‌فرستد تا اجرای بی‌نتیجه
	// از اجرای انجام‌نشده یا خطاخورده قابل تشخیص باشد.
	PublishSummary(ctx context.Context, s core.RunSummary) error
}
