package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{
		"providers":["jsearch"],
		"filters":{"keywords":["go"],"remoteOnly":true,"postedWithinHours":48},
		"maxPerRun":5,"delaySeconds":2,"allowInternship":false,"minDescriptionRunes":100
	}`), 0o644)

	t.Setenv("RAPIDAPI_KEY", "rk")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0] != "jsearch" {
		t.Errorf("providers: %+v", cfg.Providers)
	}
	if !cfg.Filters.RemoteOnly || cfg.Filters.PostedWithinHours != 48 {
		t.Errorf("filters: %+v", cfg.Filters)
	}
	if cfg.MaxPerRun != 5 || cfg.Secrets.RapidAPIKey != "rk" {
		t.Errorf("cfg: %+v", cfg)
	}
}

func TestLoadFailsWhenProviderSecretMissing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"providers":["jsearch"],"filters":{}}`), 0o644)
	t.Setenv("RAPIDAPI_KEY", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected error for missing RAPIDAPI_KEY")
	}
}
