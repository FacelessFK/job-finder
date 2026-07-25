package core

// Secrets رازهایی که فقط از متغیرهای محیطی می‌آیند.
type Secrets struct {
	// RapidAPIKeys یک یا چند کلید JSearch. وقتی سهمیه‌ی یکی تمام شود
	// به‌صورت خودکار به بعدی سوییچ می‌شود.
	RapidAPIKeys   []string
	LinkedInKey    string
	LinkedInHost   string
	LinkedInPath   string
	TelegramToken  string
	TelegramChatID string
}

// Config تنظیمات کامل اجرا (فیلترها از فایل، رازها از env).
type Config struct {
	Providers           []string
	Filters             Filters
	MaxPerRun           int
	DelaySeconds        int
	AllowInternship     bool
	MinDescriptionRunes int
	NumPages            int // تعداد صفحه در هر جست‌وجو؛ هر صفحه یک درخواست API
	SeenPath            string
	MaxSeen             int
	Rotation            Rotation
	Summary             SummaryMode
	Secrets             Secrets
}

// Rotation تقسیم کشورها بین اجراهای پیاپی. با SlotHours صفر خاموش است و
// هر اجرا همه‌ی کشورها را می‌گیرد.
type Rotation struct {
	SlotHours       int `json:"slotHours"`
	CountriesPerRun int `json:"countriesPerRun"`
}

// SummaryMode تعیین می‌کند گزارش پایان اجرا چه وقت فرستاده شود.
type SummaryMode string

const (
	// SummaryAlways هر اجرا گزارش می‌دهد. با اجراهای پرتکرار پرسروصداست.
	SummaryAlways SummaryMode = "always"
	// SummaryOnChange فقط وقتی چیزی منتشر شده یا خطایی رخ داده.
	SummaryOnChange SummaryMode = "onChange"
	// SummaryNever هیچ‌وقت.
	SummaryNever SummaryMode = "never"
)

// ShouldSend می‌گوید با این خلاصه باید پیام فرستاده شود یا نه.
func (m SummaryMode) ShouldSend(s RunSummary) bool {
	switch m {
	case SummaryNever:
		return false
	case SummaryAlways:
		return true
	default: // onChange
		return s.Published > 0 || len(s.Errors) > 0
	}
}
