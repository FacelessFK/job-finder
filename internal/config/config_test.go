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

func TestLoadReadsCountriesNumPagesAndValueFlag(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{
		"providers":["jsearch"],
		"filters":{
			"countries":["US","CA","AU"],
			"seniority":["mid","senior"],
			"requireRemoteOrRelocation":true
		},
		"numPages":2
	}`), 0o644)

	t.Setenv("RAPIDAPI_KEY", "rk")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Filters.Countries) != 3 || cfg.Filters.Countries[2] != "AU" {
		t.Errorf("countries: %+v", cfg.Filters.Countries)
	}
	if len(cfg.Filters.Seniority) != 2 {
		t.Errorf("seniority: %+v", cfg.Filters.Seniority)
	}
	if !cfg.Filters.RequireRemoteOrRelocation {
		t.Error("requireRemoteOrRelocation should be true")
	}
	if cfg.NumPages != 2 {
		t.Errorf("NumPages = %d, want 2", cfg.NumPages)
	}
}

func TestLoadDefaultsNumPagesToOne(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"providers":["jsearch"],"filters":{}}`), 0o644)
	t.Setenv("RAPIDAPI_KEY", "rk")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.NumPages != 1 {
		t.Errorf("NumPages = %d, want default 1", cfg.NumPages)
	}
}
