package display

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zakyyudha/mtmr-lyrx/internal/config"
	"github.com/zakyyudha/mtmr-lyrx/internal/spotify"
)

// DefaultStatusFile returns the default daemon status JSON path.
func DefaultStatusFile() string {
	return filepath.Join(config.DefaultCacheDir(), "status.json")
}

// DaemonStatus is local playback/auth state written by the daemon for UI readers.
type DaemonStatus struct {
	LoggedIn       bool   `json:"logged_in"`
	TokenValid     bool   `json:"token_valid"`
	TrackID        string `json:"track_id,omitempty"`
	TrackName      string `json:"track_name,omitempty"`
	ArtistName     string `json:"artist_name,omitempty"`
	DurationMS     int    `json:"duration_ms,omitempty"`
	ProgressMS     int    `json:"progress_ms,omitempty"`
	IsPlaying      bool   `json:"is_playing"`
	FetchedAt      string `json:"fetched_at,omitempty"`
	RateLimitUntil string `json:"rate_limit_until,omitempty"`
	Error          string `json:"error,omitempty"`
	UpdatedAt      string `json:"updated_at"`
}

// NewDaemonStatus maps playback state to daemon status.
func NewDaemonStatus(state spotify.PlaybackState, loggedIn bool, tokenValid bool, rateLimitUntil time.Time, errText string) DaemonStatus {
	status := DaemonStatus{
		LoggedIn:   loggedIn,
		TokenValid: tokenValid,
		TrackID:    state.TrackID,
		TrackName:  state.TrackName,
		ArtistName: state.ArtistName,
		DurationMS: state.DurationMS,
		ProgressMS: state.ProgressMS,
		IsPlaying:  state.IsPlaying,
		Error:      errText,
		UpdatedAt:  time.Now().Format(time.RFC3339),
	}
	if !state.FetchedAt.IsZero() {
		status.FetchedAt = state.FetchedAt.Format(time.RFC3339)
	}
	if !rateLimitUntil.IsZero() {
		status.RateLimitUntil = rateLimitUntil.Format(time.RFC3339)
	}
	return status
}

// WriteStatusFile atomically writes daemon status JSON.
func WriteStatusFile(path string, status DaemonStatus) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("display: marshal status: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("display: create status dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".status-*.tmp")
	if err != nil {
		return fmt.Errorf("display: create status temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("display: write status temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("display: close status temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("display: rename status file: %w", err)
	}
	return nil
}
