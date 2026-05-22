package lrclib

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, 5*time.Second, "mtmr-lyrx/test")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

func TestGet200(t *testing.T) {
	synced := "[00:01.00]Hello"
	plain := "Hello"
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/get") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Lyrics{
			ID:           1,
			TrackName:    "Test",
			ArtistName:   "Artist",
			SyncedLyrics: &synced,
			PlainLyrics:  &plain,
		})
	})

	result, err := c.Get(context.Background(), Query{TrackName: "Test", ArtistName: "Artist"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.TrackName != "Test" {
		t.Errorf("expected TrackName=Test, got %q", result.TrackName)
	}
	if result.SyncedLyrics == nil || *result.SyncedLyrics != synced {
		t.Errorf("unexpected SyncedLyrics: %v", result.SyncedLyrics)
	}
}

func TestGet404(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.Get(context.Background(), Query{TrackName: "X", ArtistName: "Y"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGet429(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := c.Get(context.Background(), Query{TrackName: "X", ArtistName: "Y"})
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestGet500(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := c.Get(context.Background(), Query{TrackName: "X", ArtistName: "Y"})
	if !errors.Is(err, ErrProvider) {
		t.Errorf("expected ErrProvider, got %v", err)
	}
}

func TestGetInvalidJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	})
	_, err := c.Get(context.Background(), Query{TrackName: "X", ArtistName: "Y"})
	if !errors.Is(err, ErrProvider) {
		t.Errorf("expected ErrProvider for invalid JSON, got %v", err)
	}
}

func TestGetNilSyncedLyrics(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Lyrics{
			ID:           2,
			TrackName:    "Test",
			ArtistName:   "Artist",
			SyncedLyrics: nil,
		})
	})
	result, err := c.Get(context.Background(), Query{TrackName: "Test", ArtistName: "Artist"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.SyncedLyrics != nil {
		t.Errorf("expected nil SyncedLyrics")
	}
}

func TestGetQueryParams(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("track_name") != "My Song" {
			t.Errorf("expected track_name=My Song, got %q", q.Get("track_name"))
		}
		if q.Get("artist_name") != "My Artist" {
			t.Errorf("expected artist_name=My Artist, got %q", q.Get("artist_name"))
		}
		if q.Get("album_name") != "My Album" {
			t.Errorf("expected album_name=My Album, got %q", q.Get("album_name"))
		}
		if q.Get("duration") != "211" {
			t.Errorf("expected duration=211, got %q", q.Get("duration"))
		}
		json.NewEncoder(w).Encode(Lyrics{ID: 1, TrackName: "My Song", ArtistName: "My Artist"})
	})
	c.Get(context.Background(), Query{
		TrackName:  "My Song",
		ArtistName: "My Artist",
		AlbumName:  "My Album",
		Duration:   211,
	})
}

func TestGetUserAgent(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "mtmr-lyrx/test" {
			t.Errorf("expected User-Agent=mtmr-lyrx/test, got %q", r.Header.Get("User-Agent"))
		}
		json.NewEncoder(w).Encode(Lyrics{ID: 1})
	})
	c.Get(context.Background(), Query{TrackName: "X", ArtistName: "Y"})
}

func TestGetContextCancellation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// slow response
		select {
		case <-r.Context().Done():
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Get(ctx, Query{TrackName: "X", ArtistName: "Y"})
	if err == nil {
		t.Error("expected error from context cancellation")
	}
}

func TestSearch200(t *testing.T) {
	synced := "[00:01.00]Hello"
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/search") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]Lyrics{
			{ID: 1, TrackName: "Test", ArtistName: "Artist", SyncedLyrics: &synced},
		})
	})
	results, err := c.Search(context.Background(), Query{TrackName: "Test", ArtistName: "Artist"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestSearch404(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.Search(context.Background(), Query{TrackName: "X", ArtistName: "Y"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
