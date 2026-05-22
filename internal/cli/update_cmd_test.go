package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zakyyudha/mtmr-lyrx/internal/updater"
)

// helpers

func makeTestTarGz(t *testing.T, dir string, content []byte) (path string, hash string) {
	t.Helper()
	path = filepath.Join(dir, "mtmr-lyrx_darwin_arm64.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	tw.WriteHeader(&tar.Header{Name: "mtmr-lyrx", Mode: 0755, Size: int64(len(content)), Typeflag: tar.TypeReg})
	tw.Write(content)
	tw.Close()
	gw.Close()
	f.Close()
	data, _ := os.ReadFile(path)
	h := sha256.Sum256(data)
	hash = hex.EncodeToString(h[:])
	return
}

func releaseJSON(srvURL string) updater.Release {
	return updater.Release{
		TagName: "v1.2.3",
		HTMLURL: srvURL + "/releases/tag/v1.2.3",
		Assets: []updater.Asset{
			{Name: "mtmr-lyrx_darwin_arm64.tar.gz", BrowserDownloadURL: srvURL + "/asset.tar.gz"},
			{Name: "checksums.txt", BrowserDownloadURL: srvURL + "/checksums.txt"},
		},
	}
}

// ---- update check ----

func TestUpdateCheckCommand_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := releaseJSON("http://" + r.Host)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--json", "update", "check",
		"--api-base-url", srv.URL,
		"--repo", "zakyyudha/mtmr-lyrx",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("update check: %v", err)
	}
	out := buf.String()
	var res map[string]interface{}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if res["update_available"] != true {
		t.Errorf("expected update_available=true, got: %v", res["update_available"])
	}
	if res["latest_version"] != "v1.2.3" {
		t.Errorf("expected latest_version=v1.2.3, got: %v", res["latest_version"])
	}
}

func TestUpdateCheckCommand_Human(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := releaseJSON("http://" + r.Host)
		json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"update", "check",
		"--api-base-url", srv.URL,
		"--repo", "zakyyudha/mtmr-lyrx",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("update check human: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Current version:") {
		t.Errorf("expected 'Current version:' in output, got: %s", out)
	}
	if !strings.Contains(out, "Update available:") {
		t.Errorf("expected 'Update available:' in output, got: %s", out)
	}
}

func TestUpdateCheckCommand_MissingAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := updater.Release{
			TagName: "v1.2.3",
			Assets:  []updater.Asset{{Name: "mtmr-lyrx_linux_amd64.tar.gz"}},
		}
		json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	root := NewRootCommand()
	errBuf := &bytes.Buffer{}
	root.SetErr(errBuf)
	root.SetArgs([]string{"update", "check",
		"--api-base-url", srv.URL,
		"--repo", "zakyyudha/mtmr-lyrx",
	})
	err := root.Execute()
	if err == nil {
		t.Error("expected error for missing darwin asset")
	}
	combined := errBuf.String() + err.Error()
	if !strings.Contains(combined, "no update asset found") {
		t.Errorf("expected 'no update asset found' in error, got: %s / %v", errBuf.String(), err)
	}
}

func TestUpdateCheckCommand_NetworkError(t *testing.T) {
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"update", "check",
		"--api-base-url", "http://127.0.0.1:1",
		"--repo", "zakyyudha/mtmr-lyrx",
	})
	err := root.Execute()
	if err == nil {
		t.Error("expected network error")
	}
}

// ---- update install dry-run ----

func TestUpdateInstallCommand_DryRun(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mtmr-lyrx")
	os.WriteFile(target, []byte("original"), 0755)

	archivePath, hash := makeTestTarGz(t, dir, []byte("new binary"))
	checksumData := hash + "  mtmr-lyrx_darwin_arm64.tar.gz\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "checksums.txt") {
			w.Write([]byte(checksumData))
			return
		}
		http.ServeFile(w, r, archivePath)
	}))
	defer srv.Close()

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--json", "update", "install",
		"--asset-url", srv.URL + "/asset.tar.gz",
		"--checksum-url", srv.URL + "/checksums.txt",
		"--target", target,
		"--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("update install dry-run: %v", err)
	}
	out := buf.String()
	var dryRes map[string]interface{}
	if err := json.Unmarshal([]byte(out), &dryRes); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if dryRes["dry_run"] != true {
		t.Errorf("expected dry_run=true, got: %v", dryRes["dry_run"])
	}
	if dryRes["installed"] != false {
		t.Errorf("expected installed=false, got: %v", dryRes["installed"])
	}
	// target must be unchanged
	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Errorf("dry-run must not modify target, got %q", string(got))
	}
}

// ---- update install success ----

func TestUpdateInstallCommand_Success(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mtmr-lyrx")
	os.WriteFile(target, []byte("old binary"), 0755)

	archivePath, hash := makeTestTarGz(t, dir, []byte("new binary content"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, archivePath)
	}))
	defer srv.Close()

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--json", "update", "install",
		"--asset-url", srv.URL + "/asset.tar.gz",
		"--checksum", hash,
		"--target", target,
		"--yes",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("update install: %v", err)
	}
	out := buf.String()
	var instRes map[string]interface{}
	if err := json.Unmarshal([]byte(out), &instRes); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if instRes["installed"] != true {
		t.Errorf("expected installed=true, got: %v", instRes["installed"])
	}
	// target should have new content
	got, _ := os.ReadFile(target)
	if string(got) != "new binary content" {
		t.Errorf("target content = %q, want 'new binary content'", string(got))
	}
}
