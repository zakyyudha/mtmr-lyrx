package spotify

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRateLimitErrorNumericRetryAfter(t *testing.T) {
	err := NewRateLimitError("30")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if n := RetryAfterSeconds(err); n != 30 {
		t.Fatalf("expected 30, got %d", n)
	}
	if got := err.Error(); got != "spotify: rate limited (429): retry after 30 seconds" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestCachedRateLimitError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spotify-rate-limit-until")
	until := time.Now().Add(2 * time.Minute)
	if err := SaveRateLimitUntil(path, until); err != nil {
		t.Fatalf("SaveRateLimitUntil: %v", err)
	}
	err := CachedRateLimitError(path, time.Now())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if RetryAfterSeconds(err) <= 0 {
		t.Fatalf("expected positive retry-after, got %d", RetryAfterSeconds(err))
	}
}

func TestCachedRateLimitExpiredClearsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spotify-rate-limit-until")
	if err := SaveRateLimitUntil(path, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("SaveRateLimitUntil: %v", err)
	}
	if err := CachedRateLimitError(path, time.Now()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got %v", err)
	}
}
