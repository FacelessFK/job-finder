// Package rotate کشورها را بین اجراهای پیاپی تقسیم می‌کند تا سهمیه‌ی API
// در طول شبانه‌روز پخش شود به‌جای اینکه یک‌جا مصرف گردد.
package rotate

import "time"

// Slice کشورهای فعالِ این اجرا را برمی‌گرداند.
//
// انتخاب فقط تابعِ زمان است، نه وضعیت ذخیره‌شده؛ پس هر اجرا مستقل و
// تکرارپذیر است و از دست رفتن یک اجرا چرخه را به‌هم نمی‌ریزد.
//
// slotHours طول هر بازه و perSlot تعداد کشور در هر بازه است. اگر هرکدام
// صفر یا نامعتبر باشند، چرخش خاموش می‌شود و همه‌ی کشورها برمی‌گردند.
func Slice(countries []string, now time.Time, slotHours, perSlot int) []string {
	n := len(countries)
	if n == 0 {
		return nil
	}
	if slotHours <= 0 || perSlot <= 0 || perSlot >= n {
		return countries
	}

	slot := now.Unix() / int64(slotHours) / 3600
	start := int((slot * int64(perSlot)) % int64(n))

	out := make([]string, 0, perSlot)
	for i := 0; i < perSlot; i++ {
		out = append(out, countries[(start+i)%n])
	}
	return out
}
