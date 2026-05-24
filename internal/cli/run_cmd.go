package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/zakyyudha/mtmr-lyrx/internal/config"
	"github.com/zakyyudha/mtmr-lyrx/internal/display"
	"github.com/zakyyudha/mtmr-lyrx/internal/lrclib"
	"github.com/zakyyudha/mtmr-lyrx/internal/lyrics"
	"github.com/zakyyudha/mtmr-lyrx/internal/marquee"
	"github.com/zakyyudha/mtmr-lyrx/internal/spotify"
	lyricsync "github.com/zakyyudha/mtmr-lyrx/internal/sync"
)

// Fallback display strings for non-music states.
const (
	fallbackPaused     = "⏸"
	fallbackNoDevice   = "♪ no device"
	fallbackNoTrack    = "♪ no track"
	fallbackNotMusic   = "♪ not music"
	fallbackNoLyrics   = "♪ no lyrics"
	fallbackAuthNeeded = "♪ login needed"
)

func newRunCommand(opts *Options) *cobra.Command {
	var (
		mock       bool
		artist     string
		title      string
		album      string
		durationMS int
		isrc       string
		spotifyID  string
		baseURL    string
		offsetMS   int
		once       bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the lyric sync daemon and write to MTMR state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}

			stateFile := display.ResolveStateFile(cfg.Display)

			// Effective offset: flag overrides config
			effectiveOffsetMS := cfg.Lyrics.OffsetMS
			if offsetMS != 0 {
				effectiveOffsetMS = offsetMS
			}

			formatter := marquee.Formatter{
				Config: marquee.Config{
					Width:     cfg.Display.Width,
					Separator: cfg.Display.Separator,
				},
			}

			if mock {
				return runMockLoop(cmd, opts, cfg, stateFile, effectiveOffsetMS, formatter,
					artist, title, album, durationMS, isrc, spotifyID, baseURL, once)
			}

			return runSpotifyLoop(cmd, opts, cfg, stateFile, effectiveOffsetMS, formatter, once, offsetMS != 0)
		},
	}

	cmd.Flags().BoolVar(&mock, "mock", false, "use mock playback with manual track metadata")
	cmd.Flags().StringVar(&artist, "artist", "", "artist name (mock mode)")
	cmd.Flags().StringVar(&title, "title", "", "track title (mock mode)")
	cmd.Flags().StringVar(&album, "album", "", "album name (mock mode, optional)")
	cmd.Flags().IntVar(&durationMS, "duration-ms", 0, "track duration in milliseconds (mock mode)")
	cmd.Flags().StringVar(&isrc, "isrc", "", "ISRC identifier (mock mode, optional)")
	cmd.Flags().StringVar(&spotifyID, "spotify-id", "", "Spotify track ID (mock mode, optional)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "override LRCLIB base URL (for testing)")
	cmd.Flags().IntVar(&offsetMS, "offset-ms", 0, "timing offset in milliseconds (overrides config)")
	cmd.Flags().BoolVar(&once, "once", false, "write one frame and exit (for testing)")

	return cmd
}

// runMockLoop runs the mock playback loop using a real LRCLIB lookup + local ticker.
func runMockLoop(cmd *cobra.Command, opts *Options, cfg config.Config, stateFile string, offsetMS int,
	formatter marquee.Formatter, artist, title, album string, durationMS int, isrc, spotifyID, baseURL string, once bool) error {

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

	lookupCtx, lookupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer lookupCancel()

	result := lyrics.Lookup(lookupCtx, client, meta, cfg.Lyrics.DurationToleranceMS)

	if result.Status != lyrics.LookupMatched || result.Lyrics == nil {
		if opts.Debug {
			fmt.Fprintf(os.Stderr, "debug: lookup status=%s reason=%s\n", result.Status, result.Reason)
		}
		_ = display.WriteStateFile(stateFile, cfg.Display.Placeholder)
		fmt.Fprintf(cmd.OutOrStdout(), "status: %s — %s\n", result.Status, result.Reason)
		return nil
	}

	doc := result.Lyrics

	if once {
		text := lyricsync.ActiveText(*doc, 0, offsetMS, cfg.Display.Placeholder)
		frame := formatter.Frame(text, 0)
		return display.WriteStateFile(stateFile, frame)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	start := time.Now()
	ticker := time.NewTicker(time.Duration(cfg.Display.ScrollSpeedMS) * time.Millisecond)
	defer ticker.Stop()

	tick := 0
	for {
		select {
		case <-ctx.Done():
			_ = display.WriteStateFile(stateFile, cfg.Display.Placeholder)
			return nil
		case <-ticker.C:
			elapsedMS := int(time.Since(start).Milliseconds())
			text := lyricsync.ActiveText(*doc, elapsedMS, offsetMS, cfg.Display.Placeholder)
			frame := formatter.Frame(text, tick)
			if err := display.WriteStateFile(stateFile, frame); err != nil && opts.Debug {
				fmt.Fprintf(os.Stderr, "debug: write state file: %v\n", err)
			}
			if opts.Debug {
				fmt.Fprintf(os.Stderr, "debug: t=%dms line=%q frame=%q\n", elapsedMS, text, frame)
			}
			tick++
		}
	}
}

// runSpotifyLoop runs the real Spotify playback loop.
func runSpotifyLoop(cmd *cobra.Command, opts *Options, cfg config.Config, stateFile string, offsetMS int,
	formatter marquee.Formatter, once bool, offsetOverride bool) error {

	tokenPath := spotify.ResolveTokenPath(cfg.Spotify)
	tok, err := spotify.LoadToken(tokenPath)
	if err != nil {
		_ = display.WriteStateFile(stateFile, fallbackAuthNeeded)
		return fmt.Errorf("not logged in — run 'mtmr-lyrx login' first: %w", err)
	}

	clientID, clientSecret := spotify.Credentials(cfg.Spotify)
	oauthCfg := spotify.OAuthConfig(clientID, clientSecret, cfg.Spotify.RedirectURL)
	tokenSource := oauthCfg.TokenSource(context.Background(), tok)

	// Save refreshed token on exit
	defer func() {
		if newTok, err := tokenSource.Token(); err == nil {
			_ = spotify.SaveToken(tokenPath, newTok)
		}
	}()

	spotifyClient := spotify.NewClient(tokenSource)

	lrclibClient, err := lrclib.NewClient(cfg.Provider.LRCLIB.BaseURL, cfg.Provider.LRCLIB.Timeout, "mtmr-lyrx/dev")
	if err != nil {
		return fmt.Errorf("create LRCLIB client: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	configPath := configPathFromOpts(opts)
	configModTime := fileModTime(configPath)
	cacheDir := cfg.Cache.Dir
	if cacheDir == "" {
		cacheDir = config.DefaultCacheDir()
	}
	rateLimitPath := spotify.RateLimitCachePath(cacheDir)
	placeholder := cfg.Display.Placeholder
	displaySpeedMS := cfg.Display.ScrollSpeedMS

	statusPath := display.DefaultStatusFile()

	var (
		currentTrackID string
		currentResult  *lyrics.LookupResult
		anchor         spotify.ProgressAnchor
		prevState      spotify.PlaybackState
		nextPollAt     time.Time
	)

	displayTicker := time.NewTicker(time.Duration(displaySpeedMS) * time.Millisecond)
	defer displayTicker.Stop()

	tick := 0

	writeStatus := func(state spotify.PlaybackState, errText string) {
		rateLimitUntil := spotify.CachedRateLimitUntil(rateLimitPath)
		_ = display.WriteStatusFile(statusPath, display.NewDaemonStatus(state, true, spotify.IsTokenValid(tok), rateLimitUntil, errText))
	}

	scheduleNextPoll := func(state spotify.PlaybackState) {
		now := time.Now()
		if state.IsEmpty() {
			nextPollAt = now.Add(60 * time.Second)
			return
		}
		if !state.IsPlaying {
			nextPollAt = now.Add(45 * time.Second)
			return
		}
		currentMS := spotify.NewProgressAnchor(state).Current(now)
		remaining := state.DurationMS - currentMS
		if state.DurationMS > 0 && remaining > 0 && remaining < 30_000 {
			nextPollAt = now.Add(time.Duration(remaining+1000) * time.Millisecond)
			return
		}
		nextPollAt = now.Add(20 * time.Second)
	}

	doSpotifyPoll := func() {
		if err := spotify.CachedRateLimitError(rateLimitPath, time.Now()); err != nil {
			writeStatus(prevState, err.Error())
			if retryAfter := spotify.RetryAfterSeconds(err); retryAfter > 0 {
				nextPollAt = time.Now().Add(time.Duration(retryAfter) * time.Second)
			} else {
				nextPollAt = time.Now().Add(60 * time.Second)
			}
			if opts.Debug {
				fmt.Fprintf(os.Stderr, "debug: spotify poll error: %v\n", err)
			}
			return
		}

		state, err := spotifyClient.CurrentlyPlaying(ctx)
		if err != nil {
			switch {
			case errors.Is(err, spotify.ErrNoContent):
				prevState = spotify.PlaybackState{}
				anchor = spotify.ProgressAnchor{}
				currentResult = nil
				currentTrackID = ""
				_ = display.WriteStateFile(stateFile, fallbackNoTrack)
				writeStatus(prevState, "no current track")
				nextPollAt = time.Now().Add(60 * time.Second)
			case errors.Is(err, spotify.ErrUnauthorized):
				_ = display.WriteStateFile(stateFile, fallbackAuthNeeded)
				writeStatus(prevState, "unauthorized — run 'mtmr-lyrx login' to re-authenticate")
				nextPollAt = time.Now().Add(60 * time.Second)
			case errors.Is(err, spotify.ErrRateLimited):
				if retryAfter := spotify.RetryAfterSeconds(err); retryAfter > 0 {
					until := time.Now().Add(time.Duration(retryAfter) * time.Second)
					_ = spotify.SaveRateLimitUntil(rateLimitPath, until)
					nextPollAt = until
				} else {
					nextPollAt = time.Now().Add(60 * time.Second)
				}
				writeStatus(prevState, err.Error())
				// keep current display
			default:
				_ = display.WriteStateFile(stateFile, fallbackNoDevice)
				writeStatus(prevState, err.Error())
				nextPollAt = time.Now().Add(60 * time.Second)
			}
			if opts.Debug {
				fmt.Fprintf(os.Stderr, "debug: spotify poll error: %v\n", err)
			}
			return
		}

		spotify.ClearRateLimit(rateLimitPath)

		predicted := anchor.Current(time.Now())
		needResync := spotify.ShouldResync(prevState, state, predicted, cfg.Spotify.SeekResyncThresholdMS)

		if needResync || state.TrackID != currentTrackID {
			currentTrackID = state.TrackID
			anchor = spotify.NewProgressAnchor(state)
			prevState = state
			writeStatus(prevState, "")
			scheduleNextPoll(prevState)

			if !state.IsMusicTrack() {
				_ = display.WriteStateFile(stateFile, fallbackNotMusic)
				currentResult = nil
				return
			}

			// Show track identity immediately while lyrics lookup happens and before
			// the first synced lyric line starts.
			_ = display.WriteStateFile(stateFile, trackDisplayText(state))
			lookupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result := lyrics.Lookup(lookupCtx, lrclibClient, state.TrackMetadata(), cfg.Lyrics.DurationToleranceMS)
			currentResult = &result
			if opts.Debug {
				fmt.Fprintf(os.Stderr, "debug: track=%q lookup=%s\n", state.TrackName, result.Status)
			}
		} else {
			anchor = spotify.NewProgressAnchor(state)
			prevState = state
			writeStatus(prevState, "")
			scheduleNextPoll(prevState)
		}
	}

	// Initial poll
	doSpotifyPoll()

	if once {
		posMS := anchor.Current(time.Now())
		text := getDisplayText(currentResult, posMS, offsetMS, cfg.Display.Placeholder, prevState)
		frame := formatter.Frame(text, 0)
		return display.WriteStateFile(stateFile, frame)
	}

	for {
		select {
		case <-ctx.Done():
			_ = display.WriteStateFile(stateFile, placeholder)
			return nil
		case <-displayTicker.C:
			if !nextPollAt.IsZero() && !time.Now().Before(nextPollAt) {
				doSpotifyPoll()
			}
			if mt := fileModTime(configPath); !mt.IsZero() && mt.After(configModTime) {
				if updated, err := config.Load(configPath); err == nil {
					configModTime = mt
					if !offsetOverride {
						offsetMS = updated.Lyrics.OffsetMS
					}
					placeholder = updated.Display.Placeholder
					formatter.Config.Width = updated.Display.Width
					formatter.Config.Separator = updated.Display.Separator
					if updated.Display.ScrollSpeedMS > 0 && updated.Display.ScrollSpeedMS != displaySpeedMS {
						displaySpeedMS = updated.Display.ScrollSpeedMS
						displayTicker.Reset(time.Duration(displaySpeedMS) * time.Millisecond)
					}
					if opts.Debug {
						fmt.Fprintf(os.Stderr, "debug: reloaded display config offset_ms=%d width=%d scroll_speed_ms=%d\n", offsetMS, formatter.Config.Width, displaySpeedMS)
					}
				} else if opts.Debug {
					fmt.Fprintf(os.Stderr, "debug: reload config failed: %v\n", err)
				}
			}
			posMS := anchor.Current(time.Now())
			text := getDisplayText(currentResult, posMS, offsetMS, placeholder, prevState)
			frame := formatter.Frame(text, tick)
			if err := display.WriteStateFile(stateFile, frame); err != nil && opts.Debug {
				fmt.Fprintf(os.Stderr, "debug: write state file: %v\n", err)
			}
			if opts.Debug {
				fmt.Fprintf(os.Stderr, "debug: pos=%dms text=%q\n", posMS, text)
			}
			tick++
		}
	}
}

// getDisplayText returns the appropriate display text given current playback state.
func getDisplayText(result *lyrics.LookupResult, posMS, offsetMS int, placeholder string, state spotify.PlaybackState) string {
	if state.IsEmpty() {
		return fallbackNoTrack
	}
	if !state.IsMusicTrack() {
		return fallbackNotMusic
	}
	if !state.IsPlaying {
		return fallbackPaused
	}
	if result == nil {
		return trackDisplayText(state)
	}
	if result.Status != lyrics.LookupMatched || result.Lyrics == nil {
		return fallbackNoLyrics
	}
	text := lyricsync.ActiveText(*result.Lyrics, posMS, offsetMS, placeholder)
	if text == placeholder {
		return trackDisplayText(state)
	}
	return text
}

func trackDisplayText(state spotify.PlaybackState) string {
	if state.ArtistName != "" && state.TrackName != "" {
		return state.ArtistName + " — " + state.TrackName
	}
	if state.TrackName != "" {
		return state.TrackName
	}
	return fallbackNoTrack
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// Ensure oauth2 import is used.
var _ oauth2.TokenSource = nil
