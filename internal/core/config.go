package core

// Secrets رازهایی که فقط از متغیرهای محیطی می‌آیند.
type Secrets struct {
	RapidAPIKey    string // JSearch
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
	Secrets             Secrets
}
