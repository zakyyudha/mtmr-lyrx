package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zakyyudha/mtmr-lyrx/internal/spotify"
)

func newStatusCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Spotify auth and playback status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}

			tokenPath := spotify.ResolveTokenPath(cfg.Spotify)

			type statusOutput struct {
				LoggedIn     bool   `json:"logged_in"`
				TokenPath    string `json:"token_path"`
				TokenValid   bool   `json:"token_valid"`
				TrackID      string `json:"track_id,omitempty"`
				TrackName    string `json:"track_name,omitempty"`
				ArtistName   string `json:"artist_name,omitempty"`
				ProgressMS   int    `json:"progress_ms,omitempty"`
				DurationMS   int    `json:"duration_ms,omitempty"`
				IsPlaying    bool   `json:"is_playing"`
				ItemType     string `json:"item_type,omitempty"`
				DeviceName   string `json:"device_name,omitempty"`
				DeviceActive bool   `json:"device_active"`
				Error        string `json:"error,omitempty"`
			}

			out := statusOutput{TokenPath: tokenPath}

			tok, err := spotify.LoadToken(tokenPath)
			if err != nil {
				out.Error = "not logged in — run 'mtmr-lyrx login' first"
				return printStatus(cmd, opts, out)
			}
			out.LoggedIn = true
			out.TokenValid = spotify.IsTokenValid(tok)

			// Try to fetch current playback
			clientID, clientSecret := spotify.Credentials(cfg.Spotify)
			oauthCfg := spotify.OAuthConfig(clientID, clientSecret, cfg.Spotify.RedirectURL)
			tokenSource := oauthCfg.TokenSource(context.Background(), tok)
			spotifyClient := spotify.NewClient(tokenSource)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			state, err := spotifyClient.CurrentlyPlaying(ctx)
			if err != nil {
				if errors.Is(err, spotify.ErrNoContent) {
					out.Error = "no current track"
				} else if errors.Is(err, spotify.ErrUnauthorized) {
					out.Error = "unauthorized — run 'mtmr-lyrx login' to re-authenticate"
				} else {
					out.Error = err.Error()
				}
				return printStatus(cmd, opts, out)
			}

			out.TrackID = state.TrackID
			out.TrackName = state.TrackName
			out.ArtistName = state.ArtistName
			out.ProgressMS = state.ProgressMS
			out.DurationMS = state.DurationMS
			out.IsPlaying = state.IsPlaying
			out.ItemType = state.ItemType
			out.DeviceName = state.DeviceName
			out.DeviceActive = state.DeviceActive

			return printStatus(cmd, opts, out)
		},
	}
	return cmd
}

func printStatus(cmd *cobra.Command, opts *Options, out interface{}) error {
	if opts.JSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
	}
	type s struct {
		LoggedIn     bool   `json:"logged_in"`
		TokenPath    string `json:"token_path"`
		TokenValid   bool   `json:"token_valid"`
		TrackID      string `json:"track_id,omitempty"`
		TrackName    string `json:"track_name,omitempty"`
		ArtistName   string `json:"artist_name,omitempty"`
		ProgressMS   int    `json:"progress_ms,omitempty"`
		DurationMS   int    `json:"duration_ms,omitempty"`
		IsPlaying    bool   `json:"is_playing"`
		ItemType     string `json:"item_type,omitempty"`
		DeviceName   string `json:"device_name,omitempty"`
		DeviceActive bool   `json:"device_active"`
		Error        string `json:"error,omitempty"`
	}
	data, _ := json.Marshal(out)
	var v s
	json.Unmarshal(data, &v)

	fmt.Fprintf(cmd.OutOrStdout(), "logged_in:     %v\n", v.LoggedIn)
	fmt.Fprintf(cmd.OutOrStdout(), "token_valid:   %v\n", v.TokenValid)
	fmt.Fprintf(cmd.OutOrStdout(), "token_path:    %s\n", v.TokenPath)
	if v.Error != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "error:         %s\n", v.Error)
		return nil
	}
	if v.TrackName != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "track:         %s — %s\n", v.ArtistName, v.TrackName)
		fmt.Fprintf(cmd.OutOrStdout(), "progress:      %dms / %dms\n", v.ProgressMS, v.DurationMS)
		fmt.Fprintf(cmd.OutOrStdout(), "playing:       %v\n", v.IsPlaying)
		fmt.Fprintf(cmd.OutOrStdout(), "item_type:     %s\n", v.ItemType)
	}
	if v.DeviceName != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "device:        %s (active: %v)\n", v.DeviceName, v.DeviceActive)
	}
	return nil
}

func newOffsetCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "offset",
		Short: "Adjust or show the lyrics timing offset",
	}

	cmd.AddCommand(newOffsetShowCommand(opts))
	cmd.AddCommand(newOffsetSetCommand(opts))
	cmd.AddCommand(newOffsetAdjustCommand(opts, "+"))
	cmd.AddCommand(newOffsetAdjustCommand(opts, "-"))

	return cmd
}

func newOffsetShowCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current timing offset",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}
			if opts.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]int{"offset_ms": cfg.Lyrics.OffsetMS})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "offset_ms: %d\n", cfg.Lyrics.OffsetMS)
			return nil
		},
	}
}

func newOffsetSetCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "set <ms>",
		Short: "Set timing offset to exact value in milliseconds",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ms int
			if _, err := fmt.Sscanf(args[0], "%d", &ms); err != nil {
				return fmt.Errorf("invalid offset value %q: must be an integer", args[0])
			}
			return writeOffsetToConfig(opts, ms, cmd)
		},
	}
}

func newOffsetAdjustCommand(opts *Options, sign string) *cobra.Command {
	use := sign + "<ms>"
	short := "Increase timing offset by ms"
	if sign == "-" {
		short = "Decrease timing offset by ms"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var delta int
			if _, err := fmt.Sscanf(args[0], "%d", &delta); err != nil {
				return fmt.Errorf("invalid offset delta %q: must be an integer", args[0])
			}
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}
			newOffset := cfg.Lyrics.OffsetMS
			if sign == "+" {
				newOffset += delta
			} else {
				newOffset -= delta
			}
			return writeOffsetToConfig(opts, newOffset, cmd)
		},
	}
}

func writeOffsetToConfig(opts *Options, offsetMS int, cmd *cobra.Command) error {
	p := opts.ConfigPath
	if p == "" {
		p = configPathFromOpts(opts)
	}

	cfg, err := loadConfig(opts)
	if err != nil {
		return err
	}
	cfg.Lyrics.OffsetMS = offsetMS

	data, err := marshalFullConfig(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(configDir(p), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(p, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "offset_ms set to %d\n", offsetMS)
	return nil
}
