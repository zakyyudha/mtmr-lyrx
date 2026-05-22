package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpers

func makeTarGz(t *testing.T, dir string, binaryContent []byte) string {
	t.Helper()
	path := filepath.Join(dir, "mtmr-lyrx_darwin_arm64.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Name:     "mtmr-lyrx",
		Mode:     0755,
		Size:     int64(len(binaryContent)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binaryContent); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	return path
}

func makeZip(t *testing.T, dir string, binaryContent []byte) string {
	t.Helper()
	path := filepath.Join(dir, "mtmr-lyrx_darwin_arm64.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create("mtmr-lyrx")
	if err != nil {
		t.Fatal(err)
	}
	w.Write(binaryContent)
	zw.Close()
	return path
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ---- VerifySHA256 ----

func TestVerifySHA256_Match(t *testing.T) {
	dir := t.TempDir()
	content := []byte("fake binary content")
	path := filepath.Join(dir, "bin")
	os.WriteFile(path, content, 0644)
	expected := sha256Hex(content)
	if err := VerifySHA256(path, expected); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestVerifySHA256_Mismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	os.WriteFile(path, []byte("content"), 0644)
	err := VerifySHA256(path, strings.Repeat("a", 64))
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error %q should contain 'checksum mismatch'", err.Error())
	}
}

// ---- ExtractBinary (tar.gz) ----

func TestExtractBinary_TarGz(t *testing.T) {
	dir := t.TempDir()
	content := []byte("#!/bin/sh\necho hello")
	archivePath := makeTarGz(t, dir, content)
	dstDir := t.TempDir()
	binPath, err := ExtractBinary(archivePath, dstDir)
	if err != nil {
		t.Fatalf("ExtractBinary: %v", err)
	}
	got, _ := os.ReadFile(binPath)
	if string(got) != string(content) {
		t.Errorf("extracted content mismatch")
	}
	info, _ := os.Stat(binPath)
	if info.Mode()&0111 == 0 {
		t.Error("extracted binary should be executable")
	}
}

// ---- ExtractBinary (zip) ----

func TestExtractBinary_Zip(t *testing.T) {
	dir := t.TempDir()
	content := []byte("#!/bin/sh\necho hello")
	archivePath := makeZip(t, dir, content)
	dstDir := t.TempDir()
	binPath, err := ExtractBinary(archivePath, dstDir)
	if err != nil {
		t.Fatalf("ExtractBinary zip: %v", err)
	}
	got, _ := os.ReadFile(binPath)
	if string(got) != string(content) {
		t.Errorf("extracted zip content mismatch")
	}
}

// ---- ExtractBinary path traversal ----

func TestExtractBinary_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evil.tar.gz")
	f, _ := os.Create(path)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	tw.WriteHeader(&tar.Header{
		Name:     "../evil",
		Mode:     0755,
		Size:     4,
		Typeflag: tar.TypeReg,
	})
	tw.Write([]byte("evil"))
	tw.Close()
	gw.Close()
	f.Close()

	_, err := ExtractBinary(path, t.TempDir())
	if err == nil {
		t.Fatal("expected unsafe archive path error")
	}
	if !strings.Contains(err.Error(), "unsafe archive path") {
		t.Errorf("error %q should contain 'unsafe archive path'", err.Error())
	}
}

// ---- ReplaceBinary ----

func TestReplaceBinary_Success(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mtmr-lyrx")
	os.WriteFile(target, []byte("old binary"), 0755)

	newBin := filepath.Join(dir, "mtmr-lyrx-new")
	os.WriteFile(newBin, []byte("new binary"), 0755)

	backup, err := ReplaceBinary(newBin, target)
	if err != nil {
		t.Fatalf("ReplaceBinary: %v", err)
	}
	if backup != target+".bak" {
		t.Errorf("backup path = %q, want %q", backup, target+".bak")
	}
	// backup should contain old content
	bak, _ := os.ReadFile(backup)
	if string(bak) != "old binary" {
		t.Errorf("backup content = %q, want 'old binary'", string(bak))
	}
	// target should contain new content
	got, _ := os.ReadFile(target)
	if string(got) != "new binary" {
		t.Errorf("target content = %q, want 'new binary'", string(got))
	}
}

// ---- Install dry-run ----

func TestInstall_DryRun(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mtmr-lyrx")
	os.WriteFile(target, []byte("original"), 0755)

	binaryContent := []byte("new binary")
	archivePath := makeTarGz(t, dir, binaryContent)
	archiveData, _ := os.ReadFile(archivePath)
	hash := sha256Hex(archiveData)

	// serve archive + checksums
	checksumData := hash + "  mtmr-lyrx_darwin_arm64.tar.gz\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "checksums.txt") {
			w.Write([]byte(checksumData))
			return
		}
		http.ServeFile(w, r, archivePath)
	}))
	defer srv.Close()

	c := Client{HTTPClient: srv.Client(), GOOS: "darwin", GOARCH: "arm64"}
	res, err := c.Install(context.Background(), "", InstallOptions{
		AssetURL:    srv.URL + "/mtmr-lyrx_darwin_arm64.tar.gz",
		ChecksumURL: srv.URL + "/checksums.txt",
		TargetPath:  target,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Install dry-run: %v", err)
	}
	if !res.DryRun {
		t.Error("expected dry_run=true")
	}
	if res.Installed {
		t.Error("expected installed=false in dry-run")
	}
	// original target must be unchanged
	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Errorf("dry-run must not modify target, got %q", string(got))
	}
	if !strings.Contains(res.Message, "dry run") {
		t.Errorf("message %q should contain 'dry run'", res.Message)
	}
}

// ---- Install success ----

func TestInstall_Success(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mtmr-lyrx")
	os.WriteFile(target, []byte("old"), 0755)

	binaryContent := []byte("new binary v2")
	archivePath := makeTarGz(t, dir, binaryContent)
	archiveData, _ := os.ReadFile(archivePath)
	hash := sha256Hex(archiveData)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, archivePath)
	}))
	defer srv.Close()

	c := Client{HTTPClient: srv.Client(), GOOS: "darwin", GOARCH: "arm64"}
	res, err := c.Install(context.Background(), "v2.0.0", InstallOptions{
		AssetURL:         srv.URL + "/mtmr-lyrx_darwin_arm64.tar.gz",
		ExpectedChecksum: hash,
		TargetPath:       target,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !res.Installed {
		t.Error("expected installed=true")
	}
	if res.Backup != target+".bak" {
		t.Errorf("backup=%q", res.Backup)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new binary v2" {
		t.Errorf("target content = %q", string(got))
	}
	bak, _ := os.ReadFile(target + ".bak")
	if string(bak) != "old" {
		t.Errorf("backup content = %q", string(bak))
	}
}

// ---- Install checksum mismatch ----

func TestInstall_ChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mtmr-lyrx")
	os.WriteFile(target, []byte("old"), 0755)

	binaryContent := []byte("new binary")
	archivePath := makeTarGz(t, dir, binaryContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, archivePath)
	}))
	defer srv.Close()

	c := Client{HTTPClient: srv.Client(), GOOS: "darwin", GOARCH: "arm64"}
	_, err := c.Install(context.Background(), "", InstallOptions{
		AssetURL:         srv.URL + "/mtmr-lyrx_darwin_arm64.tar.gz",
		ExpectedChecksum: strings.Repeat("a", 64), // wrong
		TargetPath:       target,
	})
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error %q should contain 'checksum mismatch'", err.Error())
	}
	// target must be unchanged
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Errorf("target should be unchanged after checksum failure, got %q", string(got))
	}
}

// ---- Install via release API ----

func TestInstall_ViaReleaseAPI(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mtmr-lyrx")
	os.WriteFile(target, []byte("old"), 0755)

	binaryContent := []byte("release binary")
	archivePath := makeTarGz(t, dir, binaryContent)
	archiveData, _ := os.ReadFile(archivePath)
	hash := sha256Hex(archiveData)
	checksumData := hash + "  mtmr-lyrx_darwin_arm64.tar.gz\n"

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "releases/latest"):
			rel := Release{
				TagName: "v1.5.0",
				HTMLURL: srvURL + "/releases/tag/v1.5.0",
				Assets: []Asset{
					{Name: "mtmr-lyrx_darwin_arm64.tar.gz", BrowserDownloadURL: srvURL + "/asset.tar.gz"},
					{Name: "checksums.txt", BrowserDownloadURL: srvURL + "/checksums.txt"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rel)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Write([]byte(checksumData))
		default:
			http.ServeFile(w, r, archivePath)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	c := Client{HTTPClient: srv.Client(), APIBaseURL: srv.URL, Repo: "zakyyudha/mtmr-lyrx", GOOS: "darwin", GOARCH: "arm64"}
	res, err := c.Install(context.Background(), "", InstallOptions{TargetPath: target})
	if err != nil {
		t.Fatalf("Install via API: %v", err)
	}
	if !res.Installed {
		t.Error("expected installed=true")
	}
	if res.Version != "v1.5.0" {
		t.Errorf("version=%q, want v1.5.0", res.Version)
	}
}
