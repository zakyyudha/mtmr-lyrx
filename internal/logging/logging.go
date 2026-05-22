package logging

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/zakyyudha/mtmr-lyrx/internal/config"
)

// NewLogger creates a slog.Logger from config and flags.
// If debug is true, level is forced to debug regardless of cfg.Level.
// Output is written to w (typically os.Stderr).
func NewLogger(cfg config.LogConfig, debug bool, w io.Writer) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch cfg.Format {
	case "text":
		handler = slog.NewTextHandler(w, opts)
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		return nil, fmt.Errorf("logging: unknown format %q, must be text|json", cfg.Format)
	}

	return slog.New(handler), nil
}

func parseLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("logging: unknown level %q, must be debug|info|warn|error", s)
	}
}
