package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zakyyudha/mtmr-lyrx/internal/config"
	"github.com/zakyyudha/mtmr-lyrx/internal/lrclib"
	"github.com/zakyyudha/mtmr-lyrx/internal/lyrics"
)

func newLookupCommand(opts *Options) *cobra.Command {
	var (
		artist     string
		title      string
		album      string
		durationMS int
		isrc       string
		spotifyID  string
		baseURL    string
	)

	cmd := &cobra.Command{
		Use:   "lookup",
		Short: "Look up synced lyrics for a track via LRCLIB",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}

			effectiveBaseURL := cfg.Provider.LRCLIB.BaseURL
			if baseURL != "" {
				effectiveBaseURL = baseURL
			}

			client, err := lrclib.NewClient(effectiveBaseURL, cfg.Provider.LRCLIB.Timeout, "mtmr-lyrx/dev")
			if err != nil {
				return fmt.Errorf("create LRCLIB client: %w", err)
			}

			meta := lyrics.TrackMetadata{
				SpotifyID:  spotifyID,
				ISRC:       isrc,
				ArtistName: artist,
				TrackName:  title,
				AlbumName:  album,
				DurationMS: durationMS,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result := lyrics.Lookup(ctx, client, meta, cfg.Lyrics.DurationToleranceMS)

			output := map[string]interface{}{
				"provider":   result.Provider,
				"status":     string(result.Status),
				"confidence": result.Confidence,
				"reason":     result.Reason,
			}
			if result.Match != nil {
				output["match"] = result.Match
			}
			if result.Lyrics != nil {
				output["lines"] = len(result.Lyrics.Lines)
			}

			if opts.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
			}

			explanation := statusExplanation(result.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "status:     %s (%s)\n", result.Status, explanation)
			fmt.Fprintf(cmd.OutOrStdout(), "confidence: %d\n", result.Confidence)
			fmt.Fprintf(cmd.OutOrStdout(), "reason:     %s\n", result.Reason)
			if result.Match != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "match:      %s — %s\n", result.Match.ArtistName, result.Match.TrackName)
			}
			if result.Lyrics != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "lines:      %d\n", len(result.Lyrics.Lines))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&artist, "artist", "", "artist name (required)")
	cmd.Flags().StringVar(&title, "title", "", "track title (required)")
	cmd.Flags().StringVar(&album, "album", "", "album name (optional)")
	cmd.Flags().IntVar(&durationMS, "duration-ms", 0, "track duration in milliseconds")
	cmd.Flags().StringVar(&isrc, "isrc", "", "ISRC identifier (optional)")
	cmd.Flags().StringVar(&spotifyID, "spotify-id", "", "Spotify track ID (optional)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "override LRCLIB base URL (for testing)")

	return cmd
}

// statusExplanation returns a human-readable explanation for a lookup status.
func statusExplanation(status lyrics.LookupStatus) string {
	switch status {
	case lyrics.LookupMatched:
		return "synced lyrics found"
	case lyrics.LookupNoMatch:
		return "no matching track in LRCLIB"
	case lyrics.LookupNoSyncedLyrics:
		return "track found but no synced lyrics available"
	case lyrics.LookupMalformedLyrics:
		return "synced lyrics returned but could not be parsed"
	case lyrics.LookupProviderError:
		return "LRCLIB returned an error — try again"
	case lyrics.LookupRateLimited:
		return "LRCLIB rate limited — wait a moment and retry"
	case lyrics.LookupInvalidMetadata:
		return "artist and title are required"
	default:
		return string(status)
	}
}

// loadConfig loads config from the path in opts, falling back to defaults.
func loadConfig(opts *Options) (config.Config, error) {
	p := opts.ConfigPath
	if p == "" {
		p = config.DefaultConfigPath()
	}
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return config.DefaultConfig(), nil
	}
	cfg, err := config.Load(p)
	if err != nil {
		return config.DefaultConfig(), fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return config.DefaultConfig(), fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}
