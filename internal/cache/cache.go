package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CacheFileInfo describes a known cache metadata file.
type CacheFileInfo struct {
	Path      string `json:"path"`
	Provider  string `json:"provider"`
	Exists    bool   `json:"exists"`
	SizeBytes int64  `json:"size_bytes"`
}

// ShowMetadata returns info about known cache metadata files for the given provider.
func ShowMetadata(cacheDir string, provider string) ([]CacheFileInfo, error) {
	var targets []string
	switch provider {
	case "lrclib":
		for _, f := range knownFiles["lrclib"] {
			targets = append(targets, filepath.Join(cacheDir, f))
		}
	case "all":
		for _, files := range knownFiles {
			for _, f := range files {
				targets = append(targets, filepath.Join(cacheDir, f))
			}
		}
	default:
		return nil, fmt.Errorf("cache: unknown provider %q, use lrclib or all", provider)
	}

	var results []CacheFileInfo
	for _, path := range targets {
		info := CacheFileInfo{Path: path, Provider: provider}
		if fi, err := os.Stat(path); err == nil {
			info.Exists = true
			info.SizeBytes = fi.Size()
		}
		results = append(results, info)
	}
	return results, nil
}

// ClearOptions controls what gets cleared.
type ClearOptions struct {
	Provider string // "lrclib" or "all"
	DryRun   bool
}

// ClearResult reports what was (or would be) cleared.
type ClearResult struct {
	Paths  []string
	DryRun bool
}

// knownFiles maps provider names to the metadata filenames they own.
var knownFiles = map[string][]string{
	"lrclib": {"lrclib-metadata.json", "last-lookup.json"},
}

// ClearMetadata removes provider cache metadata files from cacheDir.
// If opts.DryRun is true, it returns the paths without deleting.
func ClearMetadata(cacheDir string, opts ClearOptions) (ClearResult, error) {
	if err := GuardSafeDeletePath(cacheDir); err != nil {
		return ClearResult{}, fmt.Errorf("cache: unsafe cache dir: %w", err)
	}

	var targets []string
	switch opts.Provider {
	case "lrclib":
		for _, f := range knownFiles["lrclib"] {
			targets = append(targets, filepath.Join(cacheDir, f))
		}
	case "all":
		for _, files := range knownFiles {
			for _, f := range files {
				targets = append(targets, filepath.Join(cacheDir, f))
			}
		}
	default:
		return ClearResult{}, fmt.Errorf("cache: unknown provider %q, use lrclib or all", opts.Provider)
	}

	result := ClearResult{DryRun: opts.DryRun}

	for _, path := range targets {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue // skip missing files silently
		}
		result.Paths = append(result.Paths, path)
		if !opts.DryRun {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return result, fmt.Errorf("cache: remove %s: %w", path, err)
			}
		}
	}

	return result, nil
}

// GuardSafeDeletePath rejects paths that are unsafe to delete:
// empty string, filesystem root, user home dir, current working dir,
// or any path that doesn't look like an app-controlled cache path.
func GuardSafeDeletePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	// Reject filesystem root
	if abs == "/" || abs == filepath.VolumeName(abs)+string(filepath.Separator) {
		return fmt.Errorf("refusing to delete filesystem root %q", abs)
	}

	// Reject home directory
	home, err := os.UserHomeDir()
	if err == nil {
		homeAbs, _ := filepath.Abs(home)
		if abs == homeAbs {
			return fmt.Errorf("refusing to delete home directory %q", abs)
		}
	}

	// Reject current working directory
	cwd, err := os.Getwd()
	if err == nil {
		cwdAbs, _ := filepath.Abs(cwd)
		if abs == cwdAbs {
			return fmt.Errorf("refusing to delete current working directory %q", abs)
		}
	}

	// Reject paths that don't contain "mtmr-lyrx" — extra safety for app-controlled paths
	if !strings.Contains(abs, "mtmr-lyrx") {
		return fmt.Errorf("refusing to delete path outside mtmr-lyrx cache: %q", abs)
	}

	return nil
}
