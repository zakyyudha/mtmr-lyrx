package display

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/zakyyudha/mtmr-lyrx/internal/config"
	"github.com/zakyyudha/mtmr-lyrx/internal/marquee"
)

// DefaultStateFile returns the default path for the MTMR state file.
func DefaultStateFile() string {
	return filepath.Join(config.DefaultCacheDir(), "current.txt")
}

// ResolveStateFile returns cfg.StateFile if non-empty, else DefaultStateFile.
func ResolveStateFile(cfg config.DisplayConfig) string {
	if cfg.StateFile != "" {
		return cfg.StateFile
	}
	return DefaultStateFile()
}

// SanitizeStateLine strips ANSI, replaces CR/LF with spaces, and ensures
// valid UTF-8. Returns a single-line string safe for MTMR display.
func SanitizeStateLine(text string) string {
	text = marquee.StripANSI(text)
	text = marquee.SingleLine(text)

	// Replace invalid UTF-8 sequences with the replacement character
	if !utf8.ValidString(text) {
		var b strings.Builder
		for _, r := range text {
			b.WriteRune(r)
		}
		text = b.String()
	}

	return text
}

// WriteStateFile atomically writes text to path as a single line with no
// trailing newline. Creates parent directories as needed.
func WriteStateFile(path string, text string) error {
	text = SanitizeStateLine(text)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("display: create state dir %s: %w", dir, err)
	}

	// Write to a temp file in the same directory, then rename atomically.
	tmp, err := os.CreateTemp(dir, ".current-*.tmp")
	if err != nil {
		return fmt.Errorf("display: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("display: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("display: close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("display: rename state file: %w", err)
	}

	return nil
}
