package spotify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RateLimitError carries Spotify Retry-After metadata.
type RateLimitError struct {
	RetryAfterSeconds int
}

func (e RateLimitError) Error() string {
	if e.RetryAfterSeconds > 0 {
		return fmt.Sprintf("%s: retry after %d seconds", ErrRateLimited, e.RetryAfterSeconds)
	}
	return ErrRateLimited.Error()
}

func (e RateLimitError) Unwrap() error { return ErrRateLimited }

// NewRateLimitError builds a typed rate-limit error from Retry-After header value.
func NewRateLimitError(retryAfter string) error {
	retryAfter = strings.TrimSpace(retryAfter)
	if retryAfter == "" {
		return RateLimitError{}
	}
	if n, err := strconv.Atoi(retryAfter); err == nil {
		return RateLimitError{RetryAfterSeconds: n}
	}
	if t, err := httpDate(retryAfter); err == nil {
		n := int(time.Until(t).Seconds())
		if n < 0 {
			n = 0
		}
		return RateLimitError{RetryAfterSeconds: n}
	}
	return RateLimitError{}
}

func httpDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC1123, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC1123Z, s); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("invalid HTTP date")
}

// RetryAfterSeconds extracts Retry-After seconds from a rate-limit error.
// Returns 0 if not available.
func RetryAfterSeconds(err error) int {
	var rle RateLimitError
	if errors.As(err, &rle) {
		return rle.RetryAfterSeconds
	}
	return 0
}

// RateLimitCachePath returns cross-process cache path for Spotify rate-limit state.
func RateLimitCachePath(cacheDir string) string {
	return filepath.Join(cacheDir, "spotify-rate-limit-until")
}

// CachedRateLimitUntil reads cached rate-limit deadline. Missing/invalid file means zero time.
func CachedRateLimitUntil(path string) time.Time {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(n, 0)
}

// SaveRateLimitUntil stores rate-limit deadline for other mtmr-lyrx processes.
func SaveRateLimitUntil(path string, until time.Time) error {
	if until.IsZero() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.FormatInt(until.Unix(), 10)), 0644)
}

// ClearRateLimit removes cached rate-limit state.
func ClearRateLimit(path string) {
	_ = os.Remove(path)
}

// CachedRateLimitError returns ErrRateLimited if cached deadline still active.
func CachedRateLimitError(path string, now time.Time) error {
	until := CachedRateLimitUntil(path)
	if until.IsZero() || !until.After(now) {
		ClearRateLimit(path)
		return nil
	}
	return RateLimitError{RetryAfterSeconds: int(time.Until(until).Seconds())}
}
