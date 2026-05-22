package spotify

import (
	"testing"
	"time"
)

func TestProgressAnchorPlaying(t *testing.T) {
	now := time.Now()
	anchor := ProgressAnchor{
		ProgressMS: 10000,
		FetchedAt:  now.Add(-2 * time.Second),
		Playing:    true,
	}
	got := anchor.Current(now)
	// Should be ~12000ms (10000 + 2000)
	if got < 11900 || got > 12100 {
		t.Errorf("expected ~12000ms, got %d", got)
	}
}

func TestProgressAnchorPaused(t *testing.T) {
	now := time.Now()
	anchor := ProgressAnchor{
		ProgressMS: 5000,
		FetchedAt:  now.Add(-5 * time.Second),
		Playing:    false,
	}
	got := anchor.Current(now)
	if got != 5000 {
		t.Errorf("expected 5000 when paused, got %d", got)
	}
}

func TestProgressAnchorNegativeElapsed(t *testing.T) {
	now := time.Now()
	anchor := ProgressAnchor{
		ProgressMS: 1000,
		FetchedAt:  now.Add(time.Second), // future fetch time
		Playing:    true,
	}
	got := anchor.Current(now)
	if got < 1000 {
		t.Errorf("expected >= 1000 with negative elapsed clamped, got %d", got)
	}
}

func TestShouldResyncTrackChange(t *testing.T) {
	prev := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "track"}
	next := PlaybackState{TrackID: "b", IsPlaying: true, DeviceID: "d1", ItemType: "track", ProgressMS: 100}
	if !ShouldResync(prev, next, 100, 2000) {
		t.Error("expected resync on track change")
	}
}

func TestShouldResyncPauseResume(t *testing.T) {
	prev := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "track"}
	next := PlaybackState{TrackID: "a", IsPlaying: false, DeviceID: "d1", ItemType: "track", ProgressMS: 5000}
	if !ShouldResync(prev, next, 5000, 2000) {
		t.Error("expected resync on pause")
	}
}

func TestShouldResyncDeviceChange(t *testing.T) {
	prev := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "track"}
	next := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d2", ItemType: "track", ProgressMS: 100}
	if !ShouldResync(prev, next, 100, 2000) {
		t.Error("expected resync on device change")
	}
}

func TestShouldResyncItemTypeChange(t *testing.T) {
	prev := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "track"}
	next := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "episode", ProgressMS: 100}
	if !ShouldResync(prev, next, 100, 2000) {
		t.Error("expected resync on item type change")
	}
}

func TestShouldResyncSeekJump(t *testing.T) {
	prev := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "track"}
	// predicted 5000, actual 10000 — diff 5000 > threshold 2000
	next := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "track", ProgressMS: 10000}
	if !ShouldResync(prev, next, 5000, 2000) {
		t.Error("expected resync on seek jump")
	}
}

func TestShouldResyncNoResyncWithinThreshold(t *testing.T) {
	prev := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "track"}
	// predicted 5000, actual 5500 — diff 500 < threshold 2000
	next := PlaybackState{TrackID: "a", IsPlaying: true, DeviceID: "d1", ItemType: "track", ProgressMS: 5500}
	if ShouldResync(prev, next, 5000, 2000) {
		t.Error("expected no resync within threshold")
	}
}

func TestNewProgressAnchor(t *testing.T) {
	state := PlaybackState{
		ProgressMS: 3000,
		IsPlaying:  true,
		FetchedAt:  time.Now(),
	}
	anchor := NewProgressAnchor(state)
	if anchor.ProgressMS != 3000 {
		t.Errorf("expected 3000, got %d", anchor.ProgressMS)
	}
	if !anchor.Playing {
		t.Error("expected Playing=true")
	}
}
