// Package summarize توضیح جاب را برای نمایش در پیام کوتاه می‌کند.
// فاز ۱: کوتاه‌سازی ساده. فاز ۲: پیاده‌سازی مبتنی بر Claude API با همین رابط.
package summarize

import (
	"strings"
	"unicode"
)

// Summarizer رابطی است که در فاز ۲ می‌توان پیاده‌سازی هوشمند را جایگزین کرد.
type Summarizer interface {
	Summarize(description string) string
}

// Simple یک خلاصه‌ساز بدون AI است که فاصله‌ها را جمع و متن را کوتاه می‌کند.
type Simple struct {
	MaxRunes int
}

// NewSimple یک خلاصه‌ساز ساده با حداکثر تعداد کاراکتر می‌سازد.
func NewSimple(maxRunes int) *Simple {
	if maxRunes <= 0 {
		maxRunes = 300
	}
	return &Simple{MaxRunes: maxRunes}
}

// Summarize فاصله‌های اضافی را جمع می‌کند و در صورت طولانی‌بودن، متن را می‌برد.
func (s *Simple) Summarize(description string) string {
	fields := strings.FieldsFunc(description, func(r rune) bool {
		return unicode.IsSpace(r)
	})
	clean := strings.Join(fields, " ")
	runes := []rune(clean)
	if len(runes) <= s.MaxRunes {
		return clean
	}
	return string(runes[:s.MaxRunes]) + "..."
}
