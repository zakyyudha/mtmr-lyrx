package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zakyyudha/mtmr-lyrx/internal/config"
)

// configPathFromOpts returns the resolved config file path.
func configPathFromOpts(opts *Options) string {
	if opts.ConfigPath != "" {
		return opts.ConfigPath
	}
	return config.DefaultConfigPath()
}

// configDir returns the directory containing the config file.
func configDir(p string) string {
	return filepath.Dir(p)
}

// marshalFullConfig serializes a Config to YAML bytes with comments.
func marshalFullConfig(cfg config.Config) ([]byte, error) {
	content := fmt.Sprintf(`# mtmr-lyrx configuration

log:
  level: %s
  format: %s

provider:
  lrclib:
    base_url: %s
    timeout: %s

lyrics:
  duration_tolerance_ms: %d
  prefer_isrc: %v
  require_synced: %v
  offset_ms: %d

cache:
  enabled: %v
  dir: ""

display:
  width: %d
  scroll_speed_ms: %d
  separator: %q
  placeholder: %q
  state_file: ""
  mtmr_refresh_interval: %d

spotify:
  client_id: %q
  client_secret: %q
  redirect_url: %q
  token_file: ""
  poll_interval_ms: %d
  seek_resync_threshold_ms: %d
`,
		cfg.Log.Level,
		cfg.Log.Format,
		cfg.Provider.LRCLIB.BaseURL,
		cfg.Provider.LRCLIB.Timeout.String(),
		cfg.Lyrics.DurationToleranceMS,
		cfg.Lyrics.PreferISRC,
		cfg.Lyrics.RequireSynced,
		cfg.Lyrics.OffsetMS,
		cfg.Cache.Enabled,
		cfg.Display.Width,
		cfg.Display.ScrollSpeedMS,
		cfg.Display.Separator,
		cfg.Display.Placeholder,
		cfg.Display.MTMRRefreshInterval,
		cfg.Spotify.ClientID,
		cfg.Spotify.ClientSecret,
		cfg.Spotify.RedirectURL,
		cfg.Spotify.PollIntervalMS,
		cfg.Spotify.SeekResyncThresholdMS,
	)
	return []byte(content), nil
}

func newConfigCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	cmd.AddCommand(newConfigPathCommand(opts))
	cmd.AddCommand(newConfigInitCommand(opts))
	cmd.AddCommand(newConfigSetCommand(opts))
	return cmd
}

func newConfigPathCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := opts.ConfigPath
			if p == "" {
				p = config.DefaultConfigPath()
			}
			if opts.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": p})
			}
			fmt.Fprintln(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

func newConfigInitCommand(opts *Options) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Create default config file if missing",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := opts.ConfigPath
			if p == "" {
				p = config.DefaultConfigPath()
			}

			if _, err := os.Stat(p); err == nil && !force {
				fmt.Fprintf(cmd.OutOrStdout(), "config already exists: %s (use --force to overwrite)\n", p)
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				return fmt.Errorf("create config dir: %w", err)
			}

			cfg := config.DefaultConfig()
			data, err := marshalYAML(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}

			if err := os.WriteFile(p, data, 0644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "created config: %s\n", p)
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "overwrite existing config")
	return c
}

func newConfigSetCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a single config value",
		Long: `Set a single config value and persist it to the config file.

Supported keys:
  display.width               integer > 0
  display.scroll_speed_ms     integer > 0
  display.placeholder         non-empty string
  display.separator           non-empty string
  lyrics.offset_ms            integer (positive or negative)
  spotify.poll_interval_ms    integer > 0
  spotify.seek_resync_threshold_ms  integer > 0`,
		// DisableFlagParsing lets negative numbers like -250 pass through as args
		// instead of being interpreted as flags by cobra.
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// With DisableFlagParsing=true, cobra passes ALL tokens including
			// parent persistent flags (--config, --debug, --json) to this command.
			// Filter them out to get just [key, value].
			filtered := make([]string, 0, 2)
			skip := false
			for _, a := range args {
				if skip {
					// This token is the value for a flag we're consuming — capture --config value
					skip = false
					continue
				}
				if a == "--config" {
					// Next token is the config path — capture it
					skip = true
					// Find the value in the next iteration by peeking
					continue
				}
				if a == "--debug" || a == "--json" {
					continue
				}
				if strings.HasPrefix(a, "--config=") {
					// Extract and set config path from --config=<path> form
					opts.ConfigPath = strings.TrimPrefix(a, "--config=")
					continue
				}
				filtered = append(filtered, a)
			}
			// Second pass to capture --config <value> pairs
			for i, a := range args {
				if a == "--config" && i+1 < len(args) {
					opts.ConfigPath = args[i+1]
					break
				}
			}
			if len(filtered) != 2 {
				return fmt.Errorf("expected exactly 2 arguments: <key> <value>, got %d", len(filtered))
			}
			key := filtered[0]
			val := filtered[1]

			cfg, err := loadConfig(opts)
			if err != nil {
				// Config may not exist yet — start from defaults
				cfg = config.DefaultConfig()
			}

			switch key {
			case "display.width":
				n, err := parseInt(val)
				if err != nil || n <= 0 {
					return fmt.Errorf("display.width must be an integer > 0, got %q", val)
				}
				cfg.Display.Width = n
			case "display.scroll_speed_ms":
				n, err := parseInt(val)
				if err != nil || n <= 0 {
					return fmt.Errorf("display.scroll_speed_ms must be an integer > 0, got %q", val)
				}
				cfg.Display.ScrollSpeedMS = n
			case "display.placeholder":
				if val == "" {
					return fmt.Errorf("display.placeholder must not be empty")
				}
				cfg.Display.Placeholder = val
			case "display.separator":
				if val == "" {
					return fmt.Errorf("display.separator must not be empty")
				}
				cfg.Display.Separator = val
			case "lyrics.offset_ms":
				n, err := parseInt(val)
				if err != nil {
					return fmt.Errorf("lyrics.offset_ms must be an integer, got %q", val)
				}
				cfg.Lyrics.OffsetMS = n
			case "spotify.poll_interval_ms":
				n, err := parseInt(val)
				if err != nil || n <= 0 {
					return fmt.Errorf("spotify.poll_interval_ms must be an integer > 0, got %q", val)
				}
				cfg.Spotify.PollIntervalMS = n
			case "spotify.seek_resync_threshold_ms":
				n, err := parseInt(val)
				if err != nil || n <= 0 {
					return fmt.Errorf("spotify.seek_resync_threshold_ms must be an integer > 0, got %q", val)
				}
				cfg.Spotify.SeekResyncThresholdMS = n
			default:
				return fmt.Errorf("unknown config key %q; run 'mtmr-lyrx config set --help' for supported keys", key)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("config validation failed: %w", err)
			}

			p := configPathFromOpts(opts)
			data, err := marshalFullConfig(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			if err := os.MkdirAll(configDir(p), 0755); err != nil {
				return fmt.Errorf("create config dir: %w", err)
			}
			if err := os.WriteFile(p, data, 0600); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "config set %s=%s\n", key, val)
			return nil
		},
	}
}

// parseInt parses a decimal integer string, allowing negative values.
func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// marshalYAML produces a YAML representation of the default config with comments.
func marshalYAML(cfg config.Config) ([]byte, error) {
	content := fmt.Sprintf(`# mtmr-lyrx configuration
# Generated by: mtmr-lyrx config init

log:
  level: %s
  format: %s

provider:
  lrclib:
    base_url: %s
    timeout: %s

lyrics:
  duration_tolerance_ms: %d
  prefer_isrc: %v
  require_synced: %v

cache:
  enabled: %v
  dir: ""
`,
		cfg.Log.Level,
		cfg.Log.Format,
		cfg.Provider.LRCLIB.BaseURL,
		cfg.Provider.LRCLIB.Timeout.String(),
		cfg.Lyrics.DurationToleranceMS,
		cfg.Lyrics.PreferISRC,
		cfg.Lyrics.RequireSynced,
		cfg.Cache.Enabled,
	)
	return []byte(content), nil
}
