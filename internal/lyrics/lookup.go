package lyrics

import (
	"context"
	"errors"
	"math"
	"strings"
	"unicode"

	"github.com/zakyyudha/mtmr-lyrx/internal/inspect"
	"github.com/zakyyudha/mtmr-lyrx/internal/lrc"
	"github.com/zakyyudha/mtmr-lyrx/internal/lrclib"
)

// LookupStatus represents the outcome of a lyrics lookup.
type LookupStatus string

const (
	LookupMatched         LookupStatus = "matched"
	LookupNoMatch         LookupStatus = "no_match"
	LookupNoSyncedLyrics  LookupStatus = "no_synced_lyrics"
	LookupMalformedLyrics LookupStatus = "malformed_lyrics"
	LookupProviderError   LookupStatus = "provider_error"
	LookupRateLimited     LookupStatus = "rate_limited"
	LookupInvalidMetadata LookupStatus = "invalid_metadata"
)

// TrackMetadata holds the track info used for lyrics lookup.
type TrackMetadata struct {
	SpotifyID  string `json:"spotifyId,omitempty"`
	ISRC       string `json:"isrc,omitempty"`
	ArtistName string `json:"artistName"`
	TrackName  string `json:"trackName"`
	AlbumName  string `json:"albumName,omitempty"`
	DurationMS int    `json:"durationMs,omitempty"`
}

// MatchMetadata mirrors inspect.MatchMetadata for result embedding.
type MatchMetadata = inspect.MatchMetadata

// LookupResult holds the full outcome of a lookup.
type LookupResult struct {
	Provider   string
	Status     LookupStatus
	Confidence int
	Reason     string
	Request    TrackMetadata
	Match      *MatchMetadata
	Lyrics     *lrc.Document
}

// LRCLIBClient is the interface the lookup engine uses — allows fake in tests.
type LRCLIBClient interface {
	Get(ctx context.Context, q lrclib.Query) (*lrclib.Lyrics, error)
	Search(ctx context.Context, q lrclib.Query) ([]lrclib.Lyrics, error)
}

// Lookup fetches synced lyrics for the given track metadata.
// toleranceMS is the acceptable duration difference in milliseconds.
func Lookup(ctx context.Context, client LRCLIBClient, meta TrackMetadata, toleranceMS int) LookupResult {
	base := LookupResult{
		Provider: "lrclib",
		Request:  meta,
	}

	if strings.TrimSpace(meta.ArtistName) == "" || strings.TrimSpace(meta.TrackName) == "" {
		base.Status = LookupInvalidMetadata
		base.Reason = "artist and title are required"
		return base
	}

	durationSec := int(math.Round(float64(meta.DurationMS) / 1000))
	q := lrclib.Query{
		TrackName:  meta.TrackName,
		ArtistName: meta.ArtistName,
		AlbumName:  meta.AlbumName,
		Duration:   durationSec,
	}

	// Try exact get first
	candidate, err := client.Get(ctx, q)
	if err != nil {
		if errors.Is(err, lrclib.ErrNotFound) {
			// Fall through to search
			candidate = nil
		} else if errors.Is(err, lrclib.ErrRateLimited) {
			base.Status = LookupRateLimited
			base.Reason = "LRCLIB rate limited"
			return base
		} else {
			base.Status = LookupProviderError
			base.Reason = err.Error()
			return base
		}
	}

	// If exact get succeeded, score it
	if candidate != nil {
		conf, reason := Score(meta, *candidate, toleranceMS)
		if conf >= 70 {
			return buildResult(base, candidate, conf, reason)
		}
		// Low confidence — fall through to search
	}

	// Search fallback
	results, err := client.Search(ctx, q)
	if err != nil {
		if errors.Is(err, lrclib.ErrNotFound) {
			base.Status = LookupNoMatch
			base.Reason = "no results from LRCLIB"
			return base
		} else if errors.Is(err, lrclib.ErrRateLimited) {
			base.Status = LookupRateLimited
			base.Reason = "LRCLIB rate limited"
			return base
		}
		base.Status = LookupProviderError
		base.Reason = err.Error()
		return base
	}

	// Score all candidates, pick best
	bestConf := 0
	var bestCandidate *lrclib.Lyrics
	bestReason := ""
	for i := range results {
		conf, reason := Score(meta, results[i], toleranceMS)
		if conf > bestConf {
			bestConf = conf
			bestCandidate = &results[i]
			bestReason = reason
		}
	}

	if bestCandidate == nil || bestConf < 70 {
		base.Status = LookupNoMatch
		base.Reason = "no acceptable match found"
		return base
	}

	return buildResult(base, bestCandidate, bestConf, bestReason)
}

// buildResult converts a chosen candidate into a LookupResult.
func buildResult(base LookupResult, candidate *lrclib.Lyrics, conf int, reason string) LookupResult {
	base.Confidence = conf
	base.Reason = reason
	base.Match = &MatchMetadata{
		ID:              candidate.ID,
		TrackName:       candidate.TrackName,
		ArtistName:      candidate.ArtistName,
		AlbumName:       candidate.AlbumName,
		Duration:        int(math.Round(candidate.Duration)),
		Instrumental:    candidate.Instrumental,
		HasPlainLyrics:  candidate.PlainLyrics != nil && *candidate.PlainLyrics != "",
		HasSyncedLyrics: candidate.SyncedLyrics != nil && *candidate.SyncedLyrics != "",
	}

	// Instrumental — no lyrics expected
	if candidate.Instrumental {
		base.Status = LookupNoSyncedLyrics
		base.Reason = "track is instrumental"
		return base
	}

	if candidate.SyncedLyrics == nil || *candidate.SyncedLyrics == "" {
		base.Status = LookupNoSyncedLyrics
		base.Reason = "no synced lyrics available"
		return base
	}

	doc, err := lrc.Parse(*candidate.SyncedLyrics)
	if err != nil {
		base.Status = LookupMalformedLyrics
		base.Reason = "synced lyrics parse error: " + err.Error()
		return base
	}

	base.Status = LookupMatched
	base.Lyrics = &doc
	return base
}

// Score computes a confidence score (0-100) for a candidate against track metadata.
func Score(meta TrackMetadata, candidate lrclib.Lyrics, toleranceMS int) (int, string) {
	score := 0
	var reasons []string

	normMeta := NormalizeForMatch(meta.TrackName)
	normCand := NormalizeForMatch(candidate.TrackName)
	if normMeta != "" && normMeta == normCand {
		score += 35
		reasons = append(reasons, "title match")
	}

	normArtistMeta := NormalizeForMatch(meta.ArtistName)
	normArtistCand := NormalizeForMatch(candidate.ArtistName)
	if normArtistMeta != "" && normArtistMeta == normArtistCand {
		score += 30
		reasons = append(reasons, "artist match")
	}

	if meta.AlbumName != "" && candidate.AlbumName != "" {
		if NormalizeForMatch(meta.AlbumName) == NormalizeForMatch(candidate.AlbumName) {
			score += 15
			reasons = append(reasons, "album match")
		}
	}

	if meta.DurationMS > 0 && candidate.Duration > 0 {
		diffMS := meta.DurationMS - int(math.Round(candidate.Duration*1000))
		if diffMS < 0 {
			diffMS = -diffMS
		}
		if diffMS <= toleranceMS {
			score += 15
			reasons = append(reasons, "duration match")
		}
	}

	reason := strings.Join(reasons, ", ")
	if reason == "" {
		reason = "no matching fields"
	}
	return score, reason
}

// NormalizeForMatch normalizes a string for fuzzy matching:
// lowercases, removes punctuation, collapses spaces.
func NormalizeForMatch(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		} else if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if !prevSpace && b.Len() > 0 {
				b.WriteRune(' ')
				prevSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}
