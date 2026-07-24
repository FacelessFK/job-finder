package config

import "testing"

func TestLoadReadsEnv(t *testing.T) {
	t.Setenv("RAPIDAPI_KEY", "k")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	t.Setenv("TELEGRAM_CHAT_ID", "@ch")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RapidAPIKey != "k" || cfg.TelegramToken != "tok" || cfg.TelegramChatID != "@ch" {
		t.Errorf("unexpected cfg: %+v", cfg)
	}
	if len(cfg.Keywords) == 0 {
		t.Error("expected default keywords")
	}
	if cfg.DatePosted == "" {
		t.Error("expected default date_posted")
	}
}

func TestLoadFailsWithoutRequired(t *testing.T) {
	t.Setenv("RAPIDAPI_KEY", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when required env missing")
	}
}
