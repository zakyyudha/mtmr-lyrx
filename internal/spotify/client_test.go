package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// fakeTokenSource returns a fixed token for tests.
type fakeTokenSource struct{ token *oauth2.Token }

func (f *fakeTokenSource) Token() (*oauth2.Token, error) {
	return f.token, nil
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(&fakeTokenSource{token: &oauth2.Token{AccessToken: "test-token"}})
	c.BaseURL = srv.URL
	return c, srv
}

func TestCurrentlyPlaying200(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %q", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(rawPlayback{
			IsPlaying:            true,
			ProgressMS:           42000,
			CurrentlyPlayingType: "track",
			Item: &rawItem{
				ID:         "spotify-id-1",
				Name:       "Test Song",
				DurationMS: 200000,
				Artists:    []rawArtist{{Name: "Public Domain"}},
				Album:      &rawAlbum{Name: "Test Album"},
				ExternalIDs: map[string]string{"isrc": "USTEST123456"},
			},
		})
	})

	state, err := c.CurrentlyPlaying(context.Background())
	if err != nil {
		t.Fatalf("CurrentlyPlaying: %v", err)
	}
	if state.TrackID != "spotify-id-1" {
		t.Errorf("expected spotify-id-1, got %q", state.TrackID)
	}
	if state.TrackName != "Test Song" {
		t.Errorf("expected Test Song, got %q", state.TrackName)
	}
	if state.ArtistName != "Public Domain" {
		t.Errorf("expected Public Domain, got %q", state.ArtistName)
	}
	if state.ISRC != "USTEST123456" {
		t.Errorf("expected USTEST123456, got %q", state.ISRC)
	}
	if !state.IsPlaying {
		t.Error("expected IsPlaying=true")
	}
	if state.ProgressMS != 42000 {
		t.Errorf("expected 42000, got %d", state.ProgressMS)
	}
}

func TestCurrentlyPlaying204(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	_, err := c.CurrentlyPlaying(context.Background())
	if !errors.Is(err, ErrNoContent) {
		t.Errorf("expected ErrNoContent, got %v", err)
	}
}

func TestCurrentlyPlaying401(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := c.CurrentlyPlaying(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestCurrentlyPlaying429(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := c.CurrentlyPlaying(context.Background())
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
	if n := RetryAfterSeconds(err); n != 30 {
		t.Errorf("expected RetryAfter=30, got %d", n)
	}
}

func TestCurrentlyPlaying500(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := c.CurrentlyPlaying(context.Background())
	if !errors.Is(err, ErrProvider) {
		t.Errorf("expected ErrProvider, got %v", err)
	}
}

func TestTrackMetadataMapping(t *testing.T) {
	state := PlaybackState{
		TrackID:    "id1",
		TrackName:  "Song",
		ArtistName: "Artist",
		AlbumName:  "Album",
		ISRC:       "US123",
		DurationMS: 180000,
	}
	meta := state.TrackMetadata()
	if meta.SpotifyID != "id1" {
		t.Errorf("expected SpotifyID=id1, got %q", meta.SpotifyID)
	}
	if meta.ISRC != "US123" {
		t.Errorf("expected ISRC=US123, got %q", meta.ISRC)
	}
	if meta.DurationMS != 180000 {
		t.Errorf("expected DurationMS=180000, got %d", meta.DurationMS)
	}
}

func TestNonTrackItemType(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(rawPlayback{
			IsPlaying:            true,
			CurrentlyPlayingType: "episode",
		})
	})
	state, err := c.CurrentlyPlaying(context.Background())
	if err != nil {
		t.Fatalf("CurrentlyPlaying: %v", err)
	}
	if state.IsMusicTrack() {
		t.Error("episode should not be a music track")
	}
	if state.ItemType != "episode" {
		t.Errorf("expected episode, got %q", state.ItemType)
	}
}

func TestFetchedAtIsRecent(t *testing.T) {
	before := time.Now()
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(rawPlayback{IsPlaying: true, CurrentlyPlayingType: "track"})
	})
	state, _ := c.CurrentlyPlaying(context.Background())
	if state.FetchedAt.Before(before) {
		t.Error("FetchedAt should be after test start")
	}
}
