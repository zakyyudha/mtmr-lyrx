package spotify

import (
	"time"
)

// ProgressAnchor tracks playback position between Spotify API polls.
// It uses a monotonic wall clock to advance progress locally.
type ProgressAnchor struct {
	ProgressMS int
	FetchedAt  time.Time
	Playing    bool
}

// NewProgressAnchor creates an anchor from a PlaybackState.
func NewProgressAnchor(state PlaybackState) ProgressAnchor {
	return ProgressAnchor{
		ProgressMS: state.ProgressMS,
		FetchedAt:  state.FetchedAt,
		Playing:    state.IsPlaying,
	}
}

// Current returns the estimated current playback position in milliseconds.
// If not playing, returns the anchored position unchanged.
func (a ProgressAnchor) Current(now time.Time) int {
	if !a.Playing {
		return a.ProgressMS
	}
	elapsed := int(now.Sub(a.FetchedAt).Milliseconds())
	if elapsed < 0 {
		elapsed = 0
	}
	return a.ProgressMS + elapsed
}

// ShouldResync returns true when the new playback state requires a full resync.
// This happens when the track, play/pause state, device, or item type changes,
// or when the actual progress differs from the predicted progress by more than thresholdMS.
func ShouldResync(prev PlaybackState, next PlaybackState, predictedMS int, thresholdMS int) bool {
	// Track changed
	if prev.TrackID != next.TrackID {
		return true
	}
	// Play/pause changed
	if prev.IsPlaying != next.IsPlaying {
		return true
	}
	// Device changed
	if prev.DeviceID != next.DeviceID {
		return true
	}
	// Item type changed
	if prev.ItemType != next.ItemType {
		return true
	}
	// Seek jump: actual progress differs from predicted by more than threshold
	diff := next.ProgressMS - predictedMS
	if diff < 0 {
		diff = -diff
	}
	if diff > thresholdMS {
		return true
	}
	return false
}
