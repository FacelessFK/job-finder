// Package config تنظیمات را از فایل JSON (فیلترها) و env (رازها) می‌خواند.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aghaie/job-finder/internal/core"
)

type fileConfig struct {
	Providers           []string         `json:"providers"`
	Filters             core.Filters     `json:"filters"`
	MaxPerRun           int              `json:"maxPerRun"`
	DelaySeconds        int              `json:"delaySeconds"`
	AllowInternship     bool             `json:"allowInternship"`
	MinDescriptionRunes int              `json:"minDescriptionRunes"`
	NumPages            int              `json:"numPages"`
	Rotation            core.Rotation    `json:"rotation"`
	Summary             core.SummaryMode `json:"summary"`
}

// Load فایل config و رازهای env را می‌خواند و اعتبارسنجی می‌کند.
func Load(path string) (core.Config, error) {
	if path == "" {
		path = "config.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return core.Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return core.Config{}, fmt.Errorf("parse config: %w", err)
	}

	cfg := core.Config{
		Providers:           fc.Providers,
		Filters:             fc.Filters,
		MaxPerRun:           orDefault(fc.MaxPerRun, 20),
		DelaySeconds:        orDefault(fc.DelaySeconds, 4),
		AllowInternship:     fc.AllowInternship,
		MinDescriptionRunes: orDefault(fc.MinDescriptionRunes, 200),
		NumPages:            orDefault(fc.NumPages, 1),
		Rotation:            fc.Rotation,
		Summary:             orDefaultMode(fc.Summary, core.SummaryOnChange),
		SeenPath:            getEnv("SEEN_PATH", "data/seen_jobs.json"),
		MaxSeen:             orDefault(atoiEnv("MAX_SEEN"), 5000),
		Secrets: core.Secrets{
			RapidAPIKeys:   splitKeys(os.Getenv("RAPIDAPI_KEYS"), os.Getenv("RAPIDAPI_KEY")),
			LinkedInKey:    os.Getenv("LINKEDIN_RAPIDAPI_KEY"),
			LinkedInHost:   os.Getenv("LINKEDIN_RAPIDAPI_HOST"),
			LinkedInPath:   getEnv("LINKEDIN_RAPIDAPI_PATH", "/search"),
			TelegramToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
			TelegramChatID: os.Getenv("TELEGRAM_CHAT_ID"),
		},
	}
	if len(cfg.Providers) == 0 {
		return cfg, fmt.Errorf("config: providers list is empty")
	}
	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validate(cfg core.Config) error {
	var missing []string
	if cfg.Secrets.TelegramToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if cfg.Secrets.TelegramChatID == "" {
		missing = append(missing, "TELEGRAM_CHAT_ID")
	}
	for _, p := range cfg.Providers {
		switch p {
		case "jsearch":
			if len(cfg.Secrets.RapidAPIKeys) == 0 {
				missing = append(missing, "RAPIDAPI_KEYS or RAPIDAPI_KEY (for jsearch)")
			}
		case "linkedin":
			if cfg.Secrets.LinkedInKey == "" {
				missing = append(missing, "LINKEDIN_RAPIDAPI_KEY (for linkedin)")
			}
			if cfg.Secrets.LinkedInHost == "" {
				missing = append(missing, "LINKEDIN_RAPIDAPI_HOST (for linkedin)")
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

func orDefaultMode(v, def core.SummaryMode) core.SummaryMode {
	switch v {
	case core.SummaryAlways, core.SummaryOnChange, core.SummaryNever:
		return v
	default:
		return def
	}
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiEnv(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// splitKeys کلیدهای RapidAPI را از env می‌خواند. RAPIDAPI_KEYS فهرستی
// جداشده با کاما است؛ RAPIDAPI_KEY تک‌کلیدی و برای سازگاری با قبل.
// تکراری‌ها حذف می‌شوند تا یک کلیدِ سوخته دوبار امتحان نشود.
func splitKeys(multi, single string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range append(strings.Split(multi, ","), single) {
		k := strings.TrimSpace(raw)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}
