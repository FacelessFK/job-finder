// Package config تنظیمات سرویس را از متغیرهای محیطی و مقادیر پیش‌فرض می‌خواند.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config تنظیمات کامل اجرا است.
type Config struct {
	RapidAPIKey    string
	TelegramToken  string
	TelegramChatID string

	Keywords     []string // کلمات کلیدی جست‌وجو
	DatePosted   string   // all|today|3days|week|month
	MaxPerRun    int      // سقف پیام در هر اجرا
	DelaySeconds int      // فاصله بین پیام‌ها
	SummaryRunes int      // حداکثر طول توضیح
	SeenPath     string   // مسیر فایل وضعیت
	MaxSeen      int      // حداکثر آیدی نگه‌داشته‌شده
}

// DefaultKeywords عنوان‌های رایج برنامه‌نویسی برای جست‌وجو.
var DefaultKeywords = []string{
	"software developer",
	"backend developer",
	"frontend developer",
	"full stack developer",
	"golang developer",
}

// Load تنظیمات را می‌خواند. سه متغیر توکن الزامی‌اند.
func Load() (*Config, error) {
	cfg := &Config{
		RapidAPIKey:    os.Getenv("RAPIDAPI_KEY"),
		TelegramToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID: os.Getenv("TELEGRAM_CHAT_ID"),
		Keywords:       DefaultKeywords,
		DatePosted:     getEnvDefault("DATE_POSTED", "today"),
		MaxPerRun:      getEnvInt("MAX_PER_RUN", 20),
		DelaySeconds:   getEnvInt("DELAY_SECONDS", 4),
		SummaryRunes:   getEnvInt("SUMMARY_RUNES", 300),
		SeenPath:       getEnvDefault("SEEN_PATH", "data/seen_jobs.json"),
		MaxSeen:        getEnvInt("MAX_SEEN", 5000),
	}

	if kw := os.Getenv("KEYWORDS"); kw != "" {
		parts := strings.Split(kw, ",")
		var cleaned []string
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				cleaned = append(cleaned, t)
			}
		}
		if len(cleaned) > 0 {
			cfg.Keywords = cleaned
		}
	}

	var missing []string
	if cfg.RapidAPIKey == "" {
		missing = append(missing, "RAPIDAPI_KEY")
	}
	if cfg.TelegramToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if cfg.TelegramChatID == "" {
		missing = append(missing, "TELEGRAM_CHAT_ID")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
