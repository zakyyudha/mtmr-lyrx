package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearMetadataDryRun(t *testing.T) {
	dir := t.TempDir()
	// Rename dir to include "mtmr-lyrx" so guard passes
	cacheDir := filepath.Join(dir, "mtmr-lyrx")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a fake metadata file
	metaFile := filepath.Join(cacheDir, "lrclib-metadata.json")
	if err := os.WriteFile(metaFile, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ClearMetadata(cacheDir, ClearOptions{Provider: "lrclib", DryRun: true})
	if err != nil {
		t.Fatalf("ClearMetadata dry-run: %v", err)
	}
	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if len(result.Paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(result.Paths))
	}

	// File must still exist after dry-run
	if _, err := os.Stat(metaFile); os.IsNotExist(err) {
		t.Error("dry-run should not delete file")
	}
}

func TestClearMetadataActualDelete(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "mtmr-lyrx")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	metaFile := filepath.Join(cacheDir, "lrclib-metadata.json")
	if err := os.WriteFile(metaFile, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ClearMetadata(cacheDir, ClearOptions{Provider: "lrclib", DryRun: false})
	if err != nil {
		t.Fatalf("ClearMetadata: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Errorf("expected 1 path cleared, got %d", len(result.Paths))
	}

	// File must be gone
	if _, err := os.Stat(metaFile); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestClearMetadataMissingFilesOK(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "mtmr-lyrx")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	// No files created — should succeed with empty result
	result, err := ClearMetadata(cacheDir, ClearOptions{Provider: "lrclib", DryRun: false})
	if err != nil {
		t.Fatalf("ClearMetadata with no files: %v", err)
	}
	if len(result.Paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(result.Paths))
	}
}

func TestGuardSafeDeletePathEmpty(t *testing.T) {
	if err := GuardSafeDeletePath(""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestGuardSafeDeletePathRoot(t *testing.T) {
	if err := GuardSafeDeletePath("/"); err == nil {
		t.Error("expected error for root path")
	}
}

func TestGuardSafeDeletePathHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}
	if err := GuardSafeDeletePath(home); err == nil {
		t.Error("expected error for home directory")
	}
}

func TestGuardSafeDeletePathCwd(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Skip("cannot get cwd")
	}
	if err := GuardSafeDeletePath(cwd); err == nil {
		t.Error("expected error for current working directory")
	}
}

func TestGuardSafeDeletePathOutsideApp(t *testing.T) {
	if err := GuardSafeDeletePath("/tmp/some-other-app"); err == nil {
		t.Error("expected error for path outside mtmr-lyrx")
	}
}

func TestGuardSafeDeletePathValid(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "mtmr-lyrx")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := GuardSafeDeletePath(cacheDir); err != nil {
		t.Errorf("expected valid path to pass guard, got: %v", err)
	}
}
