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
	if cfg.MaxPerRun != 5 || len(cfg.Secrets.RapidAPIKeys) != 1 {
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

func TestLoadReadsMultipleRapidAPIKeys(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"providers":["jsearch"],"filters":{}}`), 0o644)
	t.Setenv("RAPIDAPI_KEYS", "a, b ,c,,a")
	t.Setenv("RAPIDAPI_KEY", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := []string{"a", "b", "c"}
	if len(cfg.Secrets.RapidAPIKeys) != 3 {
		t.Fatalf("keys = %v, want %v", cfg.Secrets.RapidAPIKeys, want)
	}
	for i, k := range want {
		if cfg.Secrets.RapidAPIKeys[i] != k {
			t.Errorf("key %d = %q, want %q", i, cfg.Secrets.RapidAPIKeys[i], k)
		}
	}
}

func TestLoadFallsBackToSingleRapidAPIKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"providers":["jsearch"],"filters":{}}`), 0o644)
	t.Setenv("RAPIDAPI_KEYS", "")
	t.Setenv("RAPIDAPI_KEY", "solo")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Secrets.RapidAPIKeys) != 1 || cfg.Secrets.RapidAPIKeys[0] != "solo" {
		t.Errorf("keys = %v, want [solo]", cfg.Secrets.RapidAPIKeys)
	}
}

func TestLoadReadsRotationAndSummary(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{
		"providers":["jsearch"],"filters":{},
		"rotation":{"slotHours":4,"countriesPerRun":1},
		"summary":"onChange"
	}`), 0o644)
	t.Setenv("RAPIDAPI_KEY", "rk")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tk")
	t.Setenv("TELEGRAM_CHAT_ID", "@c")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Rotation.SlotHours != 4 || cfg.Rotation.CountriesPerRun != 1 {
		t.Errorf("rotation = %+v", cfg.Rotation)
	}
	if cfg.Summary != "onChange" {
		t.Errorf("summary = %q", cfg.Summary)
	}
}
