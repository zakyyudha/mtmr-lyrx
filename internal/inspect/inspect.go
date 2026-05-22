package inspect

import "time"

// Record holds the metadata from the last lyrics lookup.
// Full lyric text is not stored here — only metadata for inspection.
type Record struct {
	Provider   string      `json:"provider"`
	Status     string      `json:"status"`
	Confidence int         `json:"confidence"`
	Reason     string      `json:"reason"`
	Request    interface{} `json:"request"`
	Match      interface{} `json:"match,omitempty"`
	FetchedAt  string      `json:"fetchedAt"`
}

// MatchMetadata holds the provider-side match info without lyric text.
type MatchMetadata struct {
	ID              int    `json:"id"`
	TrackName       string `json:"trackName"`
	ArtistName      string `json:"artistName"`
	AlbumName       string `json:"albumName"`
	Duration        int    `json:"duration"`
	Instrumental    bool   `json:"instrumental"`
	HasPlainLyrics  bool   `json:"hasPlainLyrics"`
	HasSyncedLyrics bool   `json:"hasSyncedLyrics"`
}

// NewRecord creates an inspect Record with the current timestamp.
func NewRecord(provider, status string, confidence int, reason string, request, match interface{}) Record {
	return Record{
		Provider:   provider,
		Status:     status,
		Confidence: confidence,
		Reason:     reason,
		Request:    request,
		Match:      match,
		FetchedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}
