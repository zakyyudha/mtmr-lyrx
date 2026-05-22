package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"

	"github.com/zakyyudha/mtmr-lyrx/internal/lyrics"
)

// Sentinel errors for Spotify API status mapping.
var (
	ErrNoContent    = errors.New("spotify: no current track (204)")
	ErrUnauthorized = errors.New("spotify: unauthorized (401) — re-login required")
	ErrForbidden    = errors.New("spotify: forbidden (403)")
	ErrRateLimited  = errors.New("spotify: rate limited (429)")
	ErrProvider     = errors.New("spotify: provider error")
)

// PlaybackState holds the current Spotify playback information.
type PlaybackState struct {
	TrackID      string
	TrackName    string
	ArtistName   string
	AlbumName    string
	ISRC         string
	DurationMS   int
	ProgressMS   int
	IsPlaying    bool
	ItemType     string // "track", "episode", "ad", ""
	DeviceID     string
	DeviceName   string
	DeviceActive bool
	FetchedAt    time.Time
}

// IsEmpty returns true when there is no active track.
func (s PlaybackState) IsEmpty() bool {
	return s.TrackID == "" && s.ItemType == ""
}

// IsMusicTrack returns true when the item is a regular music track.
func (s PlaybackState) IsMusicTrack() bool {
	return s.ItemType == "track"
}

// TrackMetadata maps PlaybackState to lyrics.TrackMetadata for LRCLIB lookup.
func (s PlaybackState) TrackMetadata() lyrics.TrackMetadata {
	return lyrics.TrackMetadata{
		SpotifyID:  s.TrackID,
		ISRC:       s.ISRC,
		ArtistName: s.ArtistName,
		TrackName:  s.TrackName,
		AlbumName:  s.AlbumName,
		DurationMS: s.DurationMS,
	}
}

// Client is a Spotify Web API client.
type Client struct {
	BaseURL     string
	HTTPClient  *http.Client
	TokenSource oauth2.TokenSource
}

// NewClient creates a Spotify client with the given token source.
func NewClient(tokenSource oauth2.TokenSource) *Client {
	return &Client{
		BaseURL:     "https://api.spotify.com",
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
		TokenSource: tokenSource,
	}
}

// CurrentlyPlaying fetches the currently playing track.
// Returns ErrNoContent when nothing is playing.
func (c *Client) CurrentlyPlaying(ctx context.Context) (PlaybackState, error) {
	return c.fetchPlayback(ctx, c.BaseURL+"/v1/me/player/currently-playing")
}

// Playback fetches full playback state including device info.
func (c *Client) Playback(ctx context.Context) (PlaybackState, error) {
	return c.fetchPlayback(ctx, c.BaseURL+"/v1/me/player")
}

func (c *Client) fetchPlayback(ctx context.Context, url string) (PlaybackState, error) {
	tok, err := c.TokenSource.Token()
	if err != nil {
		return PlaybackState{}, fmt.Errorf("%w: get token: %v", ErrUnauthorized, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return PlaybackState{}, fmt.Errorf("spotify: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return PlaybackState{}, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return PlaybackState{}, ErrNoContent
	case http.StatusUnauthorized:
		return PlaybackState{}, ErrUnauthorized
	case http.StatusForbidden:
		return PlaybackState{}, ErrForbidden
	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			return PlaybackState{}, fmt.Errorf("%w: retry after %s seconds", ErrRateLimited, retryAfter)
		}
		return PlaybackState{}, ErrRateLimited
	}
	if resp.StatusCode >= 500 {
		return PlaybackState{}, fmt.Errorf("%w: HTTP %d", ErrProvider, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return PlaybackState{}, fmt.Errorf("%w: unexpected HTTP %d", ErrProvider, resp.StatusCode)
	}

	var raw rawPlayback
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return PlaybackState{}, fmt.Errorf("%w: decode response: %v", ErrProvider, err)
	}

	return raw.toPlaybackState(), nil
}

// rawPlayback is the JSON shape returned by Spotify playback endpoints.
type rawPlayback struct {
	IsPlaying            bool        `json:"is_playing"`
	ProgressMS           int         `json:"progress_ms"`
	CurrentlyPlayingType string      `json:"currently_playing_type"`
	Item                 *rawItem    `json:"item"`
	Device               *rawDevice  `json:"device"`
}

type rawItem struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	DurationMS int         `json:"duration_ms"`
	Artists    []rawArtist `json:"artists"`
	Album      *rawAlbum   `json:"album"`
	ExternalIDs map[string]string `json:"external_ids"`
}

type rawArtist struct {
	Name string `json:"name"`
}

type rawAlbum struct {
	Name string `json:"name"`
}

type rawDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

func (r rawPlayback) toPlaybackState() PlaybackState {
	s := PlaybackState{
		IsPlaying:    r.IsPlaying,
		ProgressMS:   r.ProgressMS,
		ItemType:     r.CurrentlyPlayingType,
		FetchedAt:    time.Now(),
	}

	if r.Device != nil {
		s.DeviceID = r.Device.ID
		s.DeviceName = r.Device.Name
		s.DeviceActive = r.Device.IsActive
	}

	if r.Item != nil {
		s.TrackID = r.Item.ID
		s.TrackName = r.Item.Name
		s.DurationMS = r.Item.DurationMS
		if len(r.Item.Artists) > 0 {
			s.ArtistName = r.Item.Artists[0].Name
		}
		if r.Item.Album != nil {
			s.AlbumName = r.Item.Album.Name
		}
		if r.Item.ExternalIDs != nil {
			s.ISRC = r.Item.ExternalIDs["isrc"]
		}
	}

	return s
}

// RetryAfterSeconds parses the Retry-After value from a rate-limit error message.
// Returns 0 if not parseable.
func RetryAfterSeconds(err error) int {
	if err == nil {
		return 0
	}
	msg := err.Error()
	// Look for "retry after N seconds"
	for i := len(msg) - 1; i >= 0; i-- {
		if msg[i] >= '0' && msg[i] <= '9' {
			j := i
			for j > 0 && msg[j-1] >= '0' && msg[j-1] <= '9' {
				j--
			}
			n, parseErr := strconv.Atoi(msg[j : i+1])
			if parseErr == nil {
				return n
			}
		}
	}
	return 0
}
