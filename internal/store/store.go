// Package store وضعیت جاب‌های منتشرشده را برای جلوگیری از ارسال تکراری نگه می‌دارد.
package store

import "context"

// SeenStore رابط ذخیره‌سازی Fingerprintهای دیده‌شده.
type SeenStore interface {
	IsSeen(fingerprint string) bool
	MarkSeen(fingerprint string)
	Save(ctx context.Context) error
}
