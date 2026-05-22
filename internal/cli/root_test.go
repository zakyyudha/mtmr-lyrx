package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zakyyudha/mtmr-lyrx/internal/lrclib"
)

func TestConfigPathCommand(t *testing.T) {
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "path"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config path: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "mtmr-lyrx") {
		t.Errorf("expected config path to contain mtmr-lyrx, got %q", out)
	}
}

func TestCacheClearDryRunCommand(t *testing.T) {
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"cache", "clear", "--provider", "lrclib", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("cache clear dry-run: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "lrclib") {
		t.Errorf("expected output to mention lrclib, got %q", out)
	}
}

func TestLookupCommandMatchedJSON(t *testing.T) {
	synced := "[00:01.00]First line\n[00:02.00]Second line\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/get") {
			json.NewEncoder(w).Encode(lrclib.Lyrics{
				ID:           1,
				TrackName:    "Test Song",
				ArtistName:   "Public Domain",
				AlbumName:    "Test Album",
				Duration:     9,
				SyncedLyrics: &synced,
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"lookup",
		"--artist", "Public Domain",
		"--title", "Test Song",
		"--album", "Test Album",
		"--duration-ms", "9000",
		"--base-url", srv.URL,
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("lookup: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("parse JSON output: %v\noutput: %s", err, buf.String())
	}
	if result["status"] != "matched" {
		t.Errorf("expected status=matched, got %v", result["status"])
	}
}

func TestLookupCommandNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"lookup",
		"--artist", "Nobody",
		"--title", "Unknown Song",
		"--base-url", srv.URL,
		"--json",
	})
	// Execute may return nil even on no_match — it's not a fatal error
	root.Execute()

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("parse JSON output: %v\noutput: %s", err, buf.String())
	}
	if result["status"] == "matched" {
		t.Errorf("expected non-matched status, got matched")
	}
}

func TestRunMockOnceWritesStateFile(t *testing.T) {
	synced := "[00:00.00]First line\n[00:02.00]Second line\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/get") {
			json.NewEncoder(w).Encode(lrclib.Lyrics{
				ID:           1,
				TrackName:    "Test Song",
				ArtistName:   "Public Domain",
				Duration:     9,
				SyncedLyrics: &synced,
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Use a temp dir for the state file
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "current.txt")

	root := NewRootCommand()
	root.SetArgs([]string{
		"run",
		"--mock",
		"--once",
		"--artist", "Public Domain",
		"--title", "Test Song",
		"--duration-ms", "9000",
		"--base-url", srv.URL,
		"--config", filepath.Join(tmpDir, "config.yaml"), // non-existent → uses defaults
	})
	// Override state file via env
	t.Setenv("MTMR_LYRX_CACHE_DIR", tmpDir)

	if err := root.Execute(); err != nil {
		t.Fatalf("run --mock --once: %v", err)
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty state file")
	}
	if strings.HasSuffix(string(data), "\n") {
		t.Errorf("state file should not have trailing newline, got %q", string(data))
	}
}

func TestMTMRConfigJSON(t *testing.T) {
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"mtmr-config"})
	if err := root.Execute(); err != nil {
		t.Fatalf("mtmr-config: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("mtmr-config output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if result["type"] != "shellScriptTitledButton" {
		t.Errorf("expected type=shellScriptTitledButton, got %v", result["type"])
	}
	if _, ok := result["refreshInterval"]; !ok {
		t.Error("expected refreshInterval field")
	}
	if _, ok := result["source"]; !ok {
		t.Error("expected source field")
	}
	source, ok := result["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source object, got %T", result["source"])
	}
	inline, _ := source["inline"].(string)
	if inline == "" {
		t.Fatalf("expected source.inline command")
	}
	if strings.Contains(inline, "http") {
		t.Errorf("source.inline should not contain network calls, got %q", inline)
	}
}

func TestMTMRConfigContainsStateFile(t *testing.T) {
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"mtmr-config", "--state-file", "/tmp/test-current.txt"})
	if err := root.Execute(); err != nil {
		t.Fatalf("mtmr-config --state-file: %v", err)
	}
	if !strings.Contains(buf.String(), "test-current.txt") {
		t.Errorf("expected state file path in output, got %q", buf.String())
	}
}

func TestConfigSetDisplayWidth(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--config", cfgPath, "config", "set", "display.width", "55"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config set display.width: %v", err)
	}
	if !strings.Contains(buf.String(), "config set display.width=55") {
		t.Errorf("expected success message, got %q", buf.String())
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "width: 55") {
		t.Errorf("expected width: 55 in config, got:\n%s", string(data))
	}
}

func TestConfigSetNegativeOffset(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--config", cfgPath, "config", "set", "lyrics.offset_ms", "-500"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config set lyrics.offset_ms: %v", err)
	}
	if !strings.Contains(buf.String(), "config set lyrics.offset_ms=-500") {
		t.Errorf("expected success message, got %q", buf.String())
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "offset_ms: -500") {
		t.Errorf("expected offset_ms: -500 in config, got:\n%s", string(data))
	}
}

func TestConfigSetPreservesSpotifyCredentials(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	// Write initial config with Spotify credentials
	initial := `log:
  level: info
  format: text
provider:
  lrclib:
    base_url: https://lrclib.net
    timeout: 5s
lyrics:
  duration_tolerance_ms: 2000
  prefer_isrc: true
  require_synced: true
  offset_ms: 0
cache:
  enabled: true
  dir: ""
display:
  width: 30
  scroll_speed_ms: 200
  separator: " · "
  placeholder: "♪"
  state_file: ""
  mtmr_refresh_interval: 1
spotify:
  client_id: "test-client-id"
  client_secret: "test-client-secret"
  redirect_url: "http://127.0.0.1:8888/callback"
  token_file: ""
  poll_interval_ms: 2000
  seek_resync_threshold_ms: 2000
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	root := NewRootCommand()
	root.SetArgs([]string{"--config", cfgPath, "config", "set", "display.width", "45"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config set: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"test-client-id"`) {
		t.Errorf("client_id was lost after config set:\n%s", content)
	}
	if !strings.Contains(content, `"test-client-secret"`) {
		t.Errorf("client_secret was lost after config set:\n%s", content)
	}
}

func TestConfigSetInvalidWidth(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	root := NewRootCommand()
	root.SetArgs([]string{"--config", cfgPath, "config", "set", "display.width", "0"})
	if err := root.Execute(); err == nil {
		t.Error("expected error for display.width=0, got nil")
	}
}

func TestConfigSetUnknownKey(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	root := NewRootCommand()
	root.SetArgs([]string{"--config", cfgPath, "config", "set", "unknown.key", "value"})
	if err := root.Execute(); err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}
