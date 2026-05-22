package lrclib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Sentinel errors for HTTP status mapping.
var (
	ErrNotFound    = errors.New("lrclib: not found")
	ErrRateLimited = errors.New("lrclib: rate limited")
	ErrBadRequest  = errors.New("lrclib: bad request")
	ErrProvider    = errors.New("lrclib: provider error")
)

// Lyrics represents a single LRCLIB lyric record.
type Lyrics struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  *string `json:"plainLyrics"`
	SyncedLyrics *string `json:"syncedLyrics"`
}

// Query holds the metadata used to look up lyrics.
type Query struct {
	TrackName  string
	ArtistName string
	AlbumName  string
	Duration   int // seconds
}

// Client is an LRCLIB HTTP client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new LRCLIB client.
// baseURL should be "https://lrclib.net" for production.
// userAgent defaults to "mtmr-lyrx/dev" if empty.
func NewClient(baseURL string, timeout time.Duration, userAgent string) (*Client, error) {
	u, err := url.ParseRequestURI(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("lrclib: invalid base URL %q", baseURL)
	}
	if userAgent == "" {
		userAgent = "mtmr-lyrx/dev"
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		userAgent:  userAgent,
	}, nil
}

// Get calls /api/get with exact metadata. Returns ErrNotFound on 404.
func (c *Client) Get(ctx context.Context, q Query) (*Lyrics, error) {
	params := url.Values{}
	params.Set("track_name", q.TrackName)
	params.Set("artist_name", q.ArtistName)
	if q.AlbumName != "" {
		params.Set("album_name", q.AlbumName)
	}
	if q.Duration > 0 {
		params.Set("duration", strconv.Itoa(q.Duration))
	}

	endpoint := c.baseURL + "/api/get?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("lrclib: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lrclib: request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := mapStatus(resp.StatusCode); err != nil {
		return nil, err
	}

	var lyrics Lyrics
	if err := json.NewDecoder(resp.Body).Decode(&lyrics); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrProvider, err)
	}
	return &lyrics, nil
}

// Search calls /api/search and returns a list of candidates.
func (c *Client) Search(ctx context.Context, q Query) ([]Lyrics, error) {
	params := url.Values{}
	params.Set("track_name", q.TrackName)
	params.Set("artist_name", q.ArtistName)
	if q.AlbumName != "" {
		params.Set("album_name", q.AlbumName)
	}

	endpoint := c.baseURL + "/api/search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("lrclib: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lrclib: request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := mapStatus(resp.StatusCode); err != nil {
		return nil, err
	}

	var results []Lyrics
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrProvider, err)
	}
	return results, nil
}

// mapStatus converts HTTP status codes to sentinel errors.
func mapStatus(code int) error {
	switch {
	case code == http.StatusOK:
		return nil
	case code == http.StatusNotFound:
		return ErrNotFound
	case code == http.StatusTooManyRequests:
		return ErrRateLimited
	case code == http.StatusBadRequest:
		return ErrBadRequest
	case code >= 500:
		return fmt.Errorf("%w: HTTP %d", ErrProvider, code)
	default:
		return fmt.Errorf("%w: unexpected HTTP %d", ErrProvider, code)
	}
}
