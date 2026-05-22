package lyrics

import (
	"context"
	"errors"
	"testing"

	"github.com/zakyyudha/mtmr-lyrx/internal/lrclib"
)

// fakeClient implements LRCLIBClient for tests.
type fakeClient struct {
	getResult    *lrclib.Lyrics
	getErr       error
	searchResult []lrclib.Lyrics
	searchErr    error
}

func (f *fakeClient) Get(ctx context.Context, q lrclib.Query) (*lrclib.Lyrics, error) {
	return f.getResult, f.getErr
}

func (f *fakeClient) Search(ctx context.Context, q lrclib.Query) ([]lrclib.Lyrics, error) {
	return f.searchResult, f.searchErr
}

func strPtr(s string) *string { return &s }

const fakeSynced = "[00:01.00]First line\n[00:02.00]Second line\n"

func TestLookupMatched(t *testing.T) {
	client := &fakeClient{
		getResult: &lrclib.Lyrics{
			ID:           1,
			TrackName:    "Test Song",
			ArtistName:   "Public Domain",
			AlbumName:    "Test Album",
			Duration:     9,
			SyncedLyrics: strPtr(fakeSynced),
		},
	}
	meta := TrackMetadata{
		TrackName:  "Test Song",
		ArtistName: "Public Domain",
		AlbumName:  "Test Album",
		DurationMS: 9000,
	}
	result := Lookup(context.Background(), client, meta, 2000)
	if result.Status != LookupMatched {
		t.Errorf("expected matched, got %s: %s", result.Status, result.Reason)
	}
	if result.Lyrics == nil {
		t.Error("expected non-nil Lyrics")
	}
	if result.Confidence < 70 {
		t.Errorf("expected confidence >= 70, got %d", result.Confidence)
	}
}

func TestLookupSearchFallback(t *testing.T) {
	client := &fakeClient{
		getErr: lrclib.ErrNotFound,
		searchResult: []lrclib.Lyrics{
			{
				ID:           2,
				TrackName:    "Test Song",
				ArtistName:   "Public Domain",
				Duration:     9,
				SyncedLyrics: strPtr(fakeSynced),
			},
		},
	}
	meta := TrackMetadata{
		TrackName:  "Test Song",
		ArtistName: "Public Domain",
		DurationMS: 9000,
	}
	result := Lookup(context.Background(), client, meta, 2000)
	if result.Status != LookupMatched {
		t.Errorf("expected matched via search fallback, got %s: %s", result.Status, result.Reason)
	}
}

func TestLookupNoMatch(t *testing.T) {
	client := &fakeClient{
		getErr:       lrclib.ErrNotFound,
		searchResult: []lrclib.Lyrics{},
	}
	meta := TrackMetadata{TrackName: "Unknown", ArtistName: "Nobody"}
	result := Lookup(context.Background(), client, meta, 2000)
	if result.Status != LookupNoMatch {
		t.Errorf("expected no_match, got %s", result.Status)
	}
}

func TestLookupNoSyncedLyrics(t *testing.T) {
	client := &fakeClient{
		getResult: &lrclib.Lyrics{
			ID:           3,
			TrackName:    "Test Song",
			ArtistName:   "Public Domain",
			Duration:     9,
			SyncedLyrics: nil,
			PlainLyrics:  strPtr("some plain lyrics"),
		},
	}
	meta := TrackMetadata{
		TrackName:  "Test Song",
		ArtistName: "Public Domain",
		DurationMS: 9000,
	}
	result := Lookup(context.Background(), client, meta, 2000)
	if result.Status != LookupNoSyncedLyrics {
		t.Errorf("expected no_synced_lyrics, got %s", result.Status)
	}
}

func TestLookupMalformedSyncedLyrics(t *testing.T) {
	client := &fakeClient{
		getResult: &lrclib.Lyrics{
			ID:           4,
			TrackName:    "Test Song",
			ArtistName:   "Public Domain",
			Duration:     9,
			SyncedLyrics: strPtr("[ar:only tags no timestamps]"),
		},
	}
	meta := TrackMetadata{
		TrackName:  "Test Song",
		ArtistName: "Public Domain",
		DurationMS: 9000,
	}
	result := Lookup(context.Background(), client, meta, 2000)
	if result.Status != LookupMalformedLyrics {
		t.Errorf("expected malformed_lyrics, got %s", result.Status)
	}
}

func TestLookupInvalidMetadata(t *testing.T) {
	client := &fakeClient{}
	result := Lookup(context.Background(), client, TrackMetadata{TrackName: "", ArtistName: ""}, 2000)
	if result.Status != LookupInvalidMetadata {
		t.Errorf("expected invalid_metadata, got %s", result.Status)
	}
}

func TestLookupRateLimited(t *testing.T) {
	client := &fakeClient{getErr: lrclib.ErrRateLimited}
	meta := TrackMetadata{TrackName: "X", ArtistName: "Y"}
	result := Lookup(context.Background(), client, meta, 2000)
	if result.Status != LookupRateLimited {
		t.Errorf("expected rate_limited, got %s", result.Status)
	}
}

func TestLookupProviderError(t *testing.T) {
	client := &fakeClient{getErr: lrclib.ErrProvider}
	meta := TrackMetadata{TrackName: "X", ArtistName: "Y"}
	result := Lookup(context.Background(), client, meta, 2000)
	if result.Status != LookupProviderError {
		t.Errorf("expected provider_error, got %s", result.Status)
	}
}

func TestLookupDurationOutsideTolerance(t *testing.T) {
	// Duration 300s vs 9s — way outside 2s tolerance, score should be low
	client := &fakeClient{
		getResult: &lrclib.Lyrics{
			ID:           5,
			TrackName:    "Test Song",
			ArtistName:   "Public Domain",
			Duration:     300,
			SyncedLyrics: strPtr(fakeSynced),
		},
		searchResult: []lrclib.Lyrics{},
	}
	meta := TrackMetadata{
		TrackName:  "Test Song",
		ArtistName: "Public Domain",
		DurationMS: 9000,
	}
	result := Lookup(context.Background(), client, meta, 2000)
	// Without duration match, score = 35+30 = 65 < 70 → no_match
	if result.Status != LookupNoMatch {
		t.Errorf("expected no_match for duration mismatch, got %s (confidence=%d)", result.Status, result.Confidence)
	}
}

func TestNormalizeForMatch(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello world"},
		{"Hello, World!", "hello world"},
		{"  spaces  ", "spaces"},
		{"Café", "café"},
		{"feat. Someone", "feat someone"},
		{"Rock & Roll", "rock roll"},
	}
	for _, c := range cases {
		got := NormalizeForMatch(c.input)
		if got != c.expected {
			t.Errorf("NormalizeForMatch(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestScoreExactMatch(t *testing.T) {
	meta := TrackMetadata{
		TrackName:  "Test Song",
		ArtistName: "Public Domain",
		AlbumName:  "Test Album",
		DurationMS: 9000,
	}
	candidate := lrclib.Lyrics{
		TrackName:  "Test Song",
		ArtistName: "Public Domain",
		AlbumName:  "Test Album",
		Duration:   9,
	}
	conf, _ := Score(meta, candidate, 2000)
	if conf != 95 {
		t.Errorf("expected 95 for exact match, got %d", conf)
	}
}

func TestScoreWrongArtist(t *testing.T) {
	meta := TrackMetadata{TrackName: "Test Song", ArtistName: "Public Domain", DurationMS: 9000}
	candidate := lrclib.Lyrics{TrackName: "Test Song", ArtistName: "Wrong Artist", Duration: 9}
	conf, _ := Score(meta, candidate, 2000)
	// title(35) + duration(15) = 50 < 70
	if conf >= 70 {
		t.Errorf("expected conf < 70 for wrong artist, got %d", conf)
	}
}

func TestScoreCaseInsensitive(t *testing.T) {
	meta := TrackMetadata{TrackName: "TEST SONG", ArtistName: "PUBLIC DOMAIN", DurationMS: 9000}
	candidate := lrclib.Lyrics{TrackName: "test song", ArtistName: "public domain", Duration: 9}
	conf, _ := Score(meta, candidate, 2000)
	if conf < 70 {
		t.Errorf("expected conf >= 70 for case-insensitive match, got %d", conf)
	}
}

func TestLookupAllStatusStrings(t *testing.T) {
	// Verify all status constants are defined
	statuses := []LookupStatus{
		LookupMatched,
		LookupNoMatch,
		LookupNoSyncedLyrics,
		LookupMalformedLyrics,
		LookupProviderError,
		LookupRateLimited,
		LookupInvalidMetadata,
	}
	expected := []string{
		"matched", "no_match", "no_synced_lyrics",
		"malformed_lyrics", "provider_error", "rate_limited", "invalid_metadata",
	}
	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("status[%d]: expected %q, got %q", i, expected[i], s)
		}
	}
}

func TestLookupSearchProviderError(t *testing.T) {
	client := &fakeClient{
		getErr:    lrclib.ErrNotFound,
		searchErr: errors.New("network error"),
	}
	meta := TrackMetadata{TrackName: "X", ArtistName: "Y"}
	result := Lookup(context.Background(), client, meta, 2000)
	if result.Status != LookupProviderError {
		t.Errorf("expected provider_error from search, got %s", result.Status)
	}
}
