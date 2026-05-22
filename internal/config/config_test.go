package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Log.Level != "info" {
		t.Errorf("expected log.level=info, got %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("expected log.format=text, got %q", cfg.Log.Format)
	}
	if cfg.Provider.LRCLIB.BaseURL != "https://lrclib.net" {
		t.Errorf("expected lrclib base_url=https://lrclib.net, got %q", cfg.Provider.LRCLIB.BaseURL)
	}
	if cfg.Provider.LRCLIB.Timeout != 5*time.Second {
		t.Errorf("expected lrclib timeout=5s, got %v", cfg.Provider.LRCLIB.Timeout)
	}
	if cfg.Lyrics.DurationToleranceMS != 2000 {
		t.Errorf("expected duration_tolerance_ms=2000, got %d", cfg.Lyrics.DurationToleranceMS)
	}
	if !cfg.Lyrics.PreferISRC {
		t.Error("expected prefer_isrc=true")
	}
	if !cfg.Lyrics.RequireSynced {
		t.Error("expected require_synced=true")
	}
	if !cfg.Cache.Enabled {
		t.Error("expected cache.enabled=true")
	}
}

func TestDefaultCacheDir(t *testing.T) {
	// Without env override, should live under ~/.config/mtmr-lyrx/cache.
	t.Setenv("MTMR_LYRX_CACHE_DIR", "")
	dir := DefaultCacheDir()
	if filepath.Base(dir) != "cache" {
		t.Errorf("expected cache dir to end in cache, got %q", dir)
	}
}

func TestDefaultCacheDirEnvOverride(t *testing.T) {
	t.Setenv("MTMR_LYRX_CACHE_DIR", "/tmp/test-cache")
	dir := DefaultCacheDir()
	if dir != "/tmp/test-cache" {
		t.Errorf("expected /tmp/test-cache, got %q", dir)
	}
}

func TestDefaultConfigPathEnvOverride(t *testing.T) {
	t.Setenv("MTMR_LYRX_CONFIG", "/tmp/test-config.yaml")
	p := DefaultConfigPath()
	if p != "/tmp/test-config.yaml" {
		t.Errorf("expected /tmp/test-config.yaml, got %q", p)
	}
}

func TestLoadPartialConfig(t *testing.T) {
	f := writeTempConfig(t, `
log:
  level: debug
`)
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log.level=debug, got %q", cfg.Log.Level)
	}
	// defaults preserved
	if cfg.Provider.LRCLIB.BaseURL != "https://lrclib.net" {
		t.Errorf("expected default base_url preserved, got %q", cfg.Provider.LRCLIB.BaseURL)
	}
}

func TestLoadUnknownKey(t *testing.T) {
	f := writeTempConfig(t, `
unknown_key: value
`)
	_, err := Load(f)
	if err == nil {
		t.Error("expected error for unknown YAML key, got nil")
	}
}

func TestValidateBadLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Log.Level = "verbose"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for bad log level")
	}
}

func TestValidateBadLogFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Log.Format = "xml"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for bad log format")
	}
}

func TestValidateBadTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider.LRCLIB.Timeout = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for zero timeout")
	}
}

func TestValidateBadBaseURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider.LRCLIB.BaseURL = "not-a-url"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for bad base URL")
	}
}

func TestValidateBadDurationTolerance(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Lyrics.DurationToleranceMS = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative duration tolerance")
	}
}

func TestValidateDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid, got: %v", err)
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	f.Close()
	return f.Name()
}
