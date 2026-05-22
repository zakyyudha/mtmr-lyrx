package config

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure.
type Config struct {
	Log      LogConfig      `yaml:"log"`
	Provider ProviderConfig `yaml:"provider"`
	Lyrics   LyricsConfig   `yaml:"lyrics"`
	Cache    CacheConfig    `yaml:"cache"`
	Display  DisplayConfig  `yaml:"display"`
	Spotify  SpotifyConfig  `yaml:"spotify"`
}

// LogConfig controls logging behavior.
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// ProviderConfig holds provider-specific settings.
type ProviderConfig struct {
	LRCLIB LRCLIBConfig `yaml:"lrclib"`
}

// LRCLIBConfig holds LRCLIB HTTP client settings.
type LRCLIBConfig struct {
	BaseURL string        `yaml:"base_url"`
	Timeout time.Duration `yaml:"timeout"`
}

// LyricsConfig controls lyrics matching behavior.
type LyricsConfig struct {
	DurationToleranceMS int  `yaml:"duration_tolerance_ms"`
	PreferISRC          bool `yaml:"prefer_isrc"`
	RequireSynced       bool `yaml:"require_synced"`
	OffsetMS            int  `yaml:"offset_ms"`
}

// CacheConfig controls caching behavior.
type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
}

// DisplayConfig controls Touch Bar display and MTMR output behavior.
type DisplayConfig struct {
	Width               int    `yaml:"width"`
	ScrollSpeedMS       int    `yaml:"scroll_speed_ms"`
	Separator           string `yaml:"separator"`
	Placeholder         string `yaml:"placeholder"`
	StateFile           string `yaml:"state_file"`
	MTMRRefreshInterval int    `yaml:"mtmr_refresh_interval"`
}

// SpotifyConfig holds Spotify OAuth and polling settings.
type SpotifyConfig struct {
	ClientID              string `yaml:"client_id"`
	ClientSecret          string `yaml:"client_secret"`
	RedirectURL           string `yaml:"redirect_url"`
	TokenFile             string `yaml:"token_file"`
	PollIntervalMS        int    `yaml:"poll_interval_ms"`
	SeekResyncThresholdMS int    `yaml:"seek_resync_threshold_ms"`
}

// DefaultConfig returns a Config populated with sane defaults.
func DefaultConfig() Config {
	return Config{
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Provider: ProviderConfig{
			LRCLIB: LRCLIBConfig{
				BaseURL: "https://lrclib.net",
				Timeout: 5 * time.Second,
			},
		},
		Lyrics: LyricsConfig{
			DurationToleranceMS: 2000,
			PreferISRC:          true,
			RequireSynced:       true,
		},
		Cache: CacheConfig{
			Enabled: true,
			Dir:     DefaultCacheDir(),
		},
		Display: DisplayConfig{
			Width:               30,
			ScrollSpeedMS:       200,
			Separator:           " · ",
			Placeholder:         "♪",
			StateFile:           "",
			MTMRRefreshInterval: 1,
		},
		Spotify: SpotifyConfig{
			ClientID:              "",
			ClientSecret:          "",
			RedirectURL:           "http://127.0.0.1:8888/callback",
			TokenFile:             "",
			PollIntervalMS:        2000,
			SeekResyncThresholdMS: 2000,
		},
	}
}

// DefaultConfigPath returns the resolved config file path.
// Respects MTMR_LYRX_CONFIG env override.
func DefaultConfigPath() string {
	if v := os.Getenv("MTMR_LYRX_CONFIG"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "mtmr-lyrx", "config.yaml")
}

// DefaultCacheDir returns the resolved cache directory path.
// Respects MTMR_LYRX_CACHE_DIR env override.
func DefaultCacheDir() string {
	if v := os.Getenv("MTMR_LYRX_CACHE_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "mtmr-lyrx", "cache")
	}
	return filepath.Join(home, ".config", "mtmr-lyrx", "cache")
}

// Load reads a YAML config file and merges it over defaults.
// Unknown YAML keys are rejected.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("config: parse %s: %w", path, err)
	}

	return cfg, nil
}

// Validate checks that all config values are within acceptable ranges.
func (c Config) Validate() error {
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: log.level must be debug|info|warn|error, got %q", c.Log.Level)
	}

	switch c.Log.Format {
	case "text", "json":
	default:
		return fmt.Errorf("config: log.format must be text|json, got %q", c.Log.Format)
	}

	if c.Provider.LRCLIB.Timeout <= 0 {
		return fmt.Errorf("config: provider.lrclib.timeout must be > 0")
	}

	if c.Lyrics.DurationToleranceMS < 0 {
		return fmt.Errorf("config: lyrics.duration_tolerance_ms must be >= 0")
	}

	u, err := url.ParseRequestURI(c.Provider.LRCLIB.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("config: provider.lrclib.base_url must be an absolute URL, got %q", c.Provider.LRCLIB.BaseURL)
	}

	if c.Display.Width <= 0 {
		return fmt.Errorf("config: display.width must be > 0")
	}
	if c.Display.ScrollSpeedMS <= 0 {
		return fmt.Errorf("config: display.scroll_speed_ms must be > 0")
	}
	if c.Display.Separator == "" {
		return fmt.Errorf("config: display.separator must not be empty")
	}
	if c.Display.Placeholder == "" {
		return fmt.Errorf("config: display.placeholder must not be empty")
	}
	if c.Display.MTMRRefreshInterval <= 0 {
		return fmt.Errorf("config: display.mtmr_refresh_interval must be > 0")
	}

	if c.Spotify.PollIntervalMS <= 0 {
		return fmt.Errorf("config: spotify.poll_interval_ms must be > 0")
	}
	if c.Spotify.SeekResyncThresholdMS <= 0 {
		return fmt.Errorf("config: spotify.seek_resync_threshold_ms must be > 0")
	}
	if c.Spotify.RedirectURL != "" {
		ru, err := url.ParseRequestURI(c.Spotify.RedirectURL)
		if err != nil || ru.Scheme == "" || ru.Host == "" {
			return fmt.Errorf("config: spotify.redirect_url must be an absolute URL, got %q", c.Spotify.RedirectURL)
		}
	}

	return nil
}
