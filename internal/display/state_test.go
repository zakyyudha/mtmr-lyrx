package display

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zakyyudha/mtmr-lyrx/internal/config"
)

func TestDefaultStateFile(t *testing.T) {
	t.Setenv("MTMR_LYRX_CACHE_DIR", "")
	p := DefaultStateFile()
	if filepath.Base(p) != "current.txt" {
		t.Errorf("expected current.txt, got %q", p)
	}
	if !strings.Contains(p, "mtmr-lyrx") {
		t.Errorf("expected path to contain mtmr-lyrx, got %q", p)
	}
}

func TestResolveStateFileCustom(t *testing.T) {
	cfg := config.DisplayConfig{StateFile: "/tmp/test-state.txt"}
	if got := ResolveStateFile(cfg); got != "/tmp/test-state.txt" {
		t.Errorf("expected /tmp/test-state.txt, got %q", got)
	}
}

func TestResolveStateFileDefault(t *testing.T) {
	t.Setenv("MTMR_LYRX_CACHE_DIR", "")
	cfg := config.DisplayConfig{StateFile: ""}
	got := ResolveStateFile(cfg)
	if filepath.Base(got) != "current.txt" {
		t.Errorf("expected current.txt, got %q", got)
	}
}

func TestSanitizeStateLineANSI(t *testing.T) {
	got := SanitizeStateLine("\x1b[31mRed\x1b[0m")
	if strings.Contains(got, "\x1b") {
		t.Errorf("expected ANSI stripped, got %q", got)
	}
	if got != "Red" {
		t.Errorf("expected Red, got %q", got)
	}
}

func TestSanitizeStateLineNewlines(t *testing.T) {
	got := SanitizeStateLine("Hello\nWorld\r\nFoo")
	if strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Errorf("expected no newlines, got %q", got)
	}
}

func TestWriteStateFileCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "current.txt")
	if err := WriteStateFile(path, "Hello"); err != nil {
		t.Fatalf("WriteStateFile: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to exist")
	}
}

func TestWriteStateFileNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "current.txt")
	if err := WriteStateFile(path, "Hello"); err != nil {
		t.Fatalf("WriteStateFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.HasSuffix(string(data), "\n") {
		t.Errorf("expected no trailing newline, got %q", string(data))
	}
	if string(data) != "Hello" {
		t.Errorf("expected Hello, got %q", string(data))
	}
}

func TestWriteStateFileAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "current.txt")

	if err := WriteStateFile(path, "First"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteStateFile(path, "Second"); err != nil {
		t.Fatalf("second write: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "Second" {
		t.Errorf("expected Second after replacement, got %q", string(data))
	}
}

func TestWriteStateFileSanitizesNewlines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "current.txt")
	if err := WriteStateFile(path, "Hello\nWorld"); err != nil {
		t.Fatalf("WriteStateFile: %v", err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "\n") {
		t.Errorf("expected no newlines in state file, got %q", string(data))
	}
}
