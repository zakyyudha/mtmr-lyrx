package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- NormalizeVersion ----

func TestNormalizeVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"dev", "dev"},
		{"", ""},
		{" v1.0.0 ", "1.0.0"},
	}
	for _, c := range cases {
		got := NormalizeVersion(c.in)
		if got != c.want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---- CompareVersions ----

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		cur, lat string
		want     int
	}{
		{"dev", "v1.0.0", -1},
		{"", "v1.0.0", -1},
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1.0", "v1.0.9", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"dev", "dev", 0},
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.4", "v1.2.3", 1},
	}
	for _, c := range cases {
		got, err := CompareVersions(c.cur, c.lat)
		if err != nil {
			t.Errorf("CompareVersions(%q,%q) unexpected error: %v", c.cur, c.lat, err)
			continue
		}
		if got != c.want {
			t.Errorf("CompareVersions(%q,%q) = %d, want %d", c.cur, c.lat, got, c.want)
		}
	}
}

// ---- SelectAsset ----

func TestSelectAsset(t *testing.T) {
	rel := Release{
		Assets: []Asset{
			{Name: "mtmr-lyrx_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/arm64.tar.gz"},
			{Name: "mtmr-lyrx_darwin_amd64.tar.gz", BrowserDownloadURL: "https://example.com/amd64.tar.gz"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
		},
	}

	a, err := SelectAsset(rel, "darwin", "arm64")
	if err != nil {
		t.Fatalf("SelectAsset arm64: %v", err)
	}
	if a.Name != "mtmr-lyrx_darwin_arm64.tar.gz" {
		t.Errorf("got %q, want arm64 asset", a.Name)
	}

	a, err = SelectAsset(rel, "darwin", "amd64")
	if err != nil {
		t.Fatalf("SelectAsset amd64: %v", err)
	}
	if a.Name != "mtmr-lyrx_darwin_amd64.tar.gz" {
		t.Errorf("got %q, want amd64 asset", a.Name)
	}

	_, err = SelectAsset(rel, "linux", "arm64")
	if err == nil {
		t.Error("expected error for linux/arm64, got nil")
	}
	if err != nil && !containsStr(err.Error(), "no update asset found") {
		t.Errorf("error %q should contain 'no update asset found'", err.Error())
	}
}

// ---- SelectChecksumAsset ----

func TestSelectChecksumAsset(t *testing.T) {
	rel := Release{
		Assets: []Asset{
			{Name: "mtmr-lyrx_darwin_arm64.tar.gz"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
		},
	}
	a, err := SelectChecksumAsset(rel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name != "checksums.txt" {
		t.Errorf("got %q, want checksums.txt", a.Name)
	}

	_, err = SelectChecksumAsset(Release{})
	if err == nil {
		t.Error("expected error for missing checksums.txt")
	}
}

// ---- ParseChecksums ----

func TestParseChecksums(t *testing.T) {
	gnuFormat := []byte(
		"abc123def456abc123def456abc123def456abc123def456abc123def456abc1  mtmr-lyrx_darwin_arm64.tar.gz\n" +
			"def456abc123def456abc123def456abc123def456abc123def456abc123def4  mtmr-lyrx_darwin_amd64.tar.gz\n",
	)
	m, err := ParseChecksums(gnuFormat)
	if err != nil {
		t.Fatalf("ParseChecksums GNU: %v", err)
	}
	if m["mtmr-lyrx_darwin_arm64.tar.gz"] != "abc123def456abc123def456abc123def456abc123def456abc123def456abc1" {
		t.Errorf("wrong hash for arm64: %q", m["mtmr-lyrx_darwin_arm64.tar.gz"])
	}

	bsdFormat := []byte(
		"SHA256 (mtmr-lyrx_darwin_arm64.tar.gz) = abc123def456abc123def456abc123def456abc123def456abc123def456abc1\n",
	)
	m2, err := ParseChecksums(bsdFormat)
	if err != nil {
		t.Fatalf("ParseChecksums BSD: %v", err)
	}
	if m2["mtmr-lyrx_darwin_arm64.tar.gz"] != "abc123def456abc123def456abc123def456abc123def456abc123def456abc1" {
		t.Errorf("wrong BSD hash: %q", m2["mtmr-lyrx_darwin_arm64.tar.gz"])
	}

	_, err = ParseChecksums([]byte("no valid lines here\n"))
	if err == nil {
		t.Error("expected error for empty checksum file")
	}
}

// ---- Client.Check via httptest ----

func TestClientCheck_UpdateAvailable(t *testing.T) {
	rel := Release{
		TagName: "v1.2.3",
		HTMLURL: "https://github.com/zakyyudha/mtmr-lyrx/releases/tag/v1.2.3",
		Assets: []Asset{
			{Name: "mtmr-lyrx_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/arm64.tar.gz"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	c := Client{
		HTTPClient: srv.Client(),
		APIBaseURL: srv.URL,
		Repo:       "zakyyudha/mtmr-lyrx",
		GOOS:       "darwin",
		GOARCH:     "arm64",
	}
	res, err := c.Check(context.Background(), "dev", CheckOptions{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !res.UpdateAvailable {
		t.Error("expected update_available=true")
	}
	if res.LatestVersion != "v1.2.3" {
		t.Errorf("latest_version=%q, want v1.2.3", res.LatestVersion)
	}
	if res.AssetName != "mtmr-lyrx_darwin_arm64.tar.gz" {
		t.Errorf("asset_name=%q", res.AssetName)
	}
}

func TestClientCheck_AlreadyLatest(t *testing.T) {
	rel := Release{
		TagName: "v1.0.0",
		HTMLURL: "https://github.com/zakyyudha/mtmr-lyrx/releases/tag/v1.0.0",
		Assets: []Asset{
			{Name: "mtmr-lyrx_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/arm64.tar.gz"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	c := Client{HTTPClient: srv.Client(), APIBaseURL: srv.URL, Repo: "zakyyudha/mtmr-lyrx", GOOS: "darwin", GOARCH: "arm64"}
	res, err := c.Check(context.Background(), "v1.0.0", CheckOptions{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.UpdateAvailable {
		t.Error("expected update_available=false when already at latest")
	}
}

func TestClientCheck_MissingAsset(t *testing.T) {
	rel := Release{
		TagName: "v1.2.3",
		Assets:  []Asset{{Name: "mtmr-lyrx_linux_amd64.tar.gz"}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	c := Client{HTTPClient: srv.Client(), APIBaseURL: srv.URL, Repo: "zakyyudha/mtmr-lyrx", GOOS: "darwin", GOARCH: "arm64"}
	_, err := c.Check(context.Background(), "dev", CheckOptions{})
	if err == nil {
		t.Error("expected error for missing darwin asset")
	}
	if !containsStr(err.Error(), "no update asset found") {
		t.Errorf("error %q should contain 'no update asset found'", err.Error())
	}
}

func TestClientCheck_NetworkError(t *testing.T) {
	c := Client{
		HTTPClient: http.DefaultClient,
		APIBaseURL: "http://127.0.0.1:1", // nothing listening
		Repo:       "zakyyudha/mtmr-lyrx",
		GOOS:       "darwin",
		GOARCH:     "arm64",
	}
	_, err := c.Check(context.Background(), "dev", CheckOptions{})
	if err == nil {
		t.Error("expected network error")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
