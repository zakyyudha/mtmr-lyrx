package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
)

const (
	DefaultRepo       = "zakyyudha/mtmr-lyrx"
	DefaultAPIBaseURL = "https://api.github.com"
)

// Release represents a GitHub release.
type Release struct {
	TagName    string  `json:"tag_name"`
	HTMLURL    string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset represents a single release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckResult is the output of a version check.
type CheckResult struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url"`
	AssetName       string `json:"asset_name"`
	AssetURL        string `json:"asset_url"`
	ChecksumURL     string `json:"checksum_url"`
	Error           string `json:"error,omitempty"`
}

// CheckOptions controls the check behaviour.
type CheckOptions struct {
	// Version pins a specific release tag instead of fetching latest.
	Version string
}

// Client performs update checks and installs.
type Client struct {
	HTTPClient *http.Client
	APIBaseURL string
	Repo       string
	GOOS       string
	GOARCH     string
}

// DefaultClient returns a Client with sensible defaults.
func DefaultClient() Client {
	return Client{
		HTTPClient: http.DefaultClient,
		APIBaseURL: DefaultAPIBaseURL,
		Repo:       DefaultRepo,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
	}
}

// NormalizeVersion strips a leading "v" and trims whitespace.
func NormalizeVersion(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "v")
}

// CompareVersions compares two semver-ish version strings.
// Returns -1 if current < latest, 0 if equal, +1 if current > latest.
// Non-numeric / dev versions are treated as older than any tagged release.
func CompareVersions(current, latest string) (int, error) {
	cn := NormalizeVersion(current)
	ln := NormalizeVersion(latest)

	// "dev" or empty is always older
	if cn == "" || cn == "dev" {
		if ln == "" || ln == "dev" {
			return 0, nil
		}
		return -1, nil
	}
	if ln == "" || ln == "dev" {
		return 1, nil
	}

	cp, err := parseParts(cn)
	if err != nil {
		return -1, fmt.Errorf("invalid current version %q: %w", current, err)
	}
	lp, err := parseParts(ln)
	if err != nil {
		return -1, fmt.Errorf("invalid latest version %q: %w", latest, err)
	}

	for i := 0; i < 3; i++ {
		if cp[i] < lp[i] {
			return -1, nil
		}
		if cp[i] > lp[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func parseParts(v string) ([3]int, error) {
	// strip pre-release suffix (e.g. "1.2.3-beta")
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.SplitN(v, ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	var out [3]int
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return out, fmt.Errorf("non-numeric segment %q", parts[i])
		}
		out[i] = n
	}
	return out, nil
}

// Check fetches the latest (or pinned) release and returns a CheckResult.
func (c Client) Check(ctx context.Context, currentVersion string, opts CheckOptions) (CheckResult, error) {
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	base := c.APIBaseURL
	if base == "" {
		base = DefaultAPIBaseURL
	}
	repo := c.Repo
	if repo == "" {
		repo = DefaultRepo
	}

	var url string
	if opts.Version != "" {
		tag := opts.Version
		if !strings.HasPrefix(tag, "v") {
			tag = "v" + tag
		}
		url = fmt.Sprintf("%s/repos/%s/releases/tags/%s", base, repo, tag)
	} else {
		url = fmt.Sprintf("%s/repos/%s/releases/latest", base, repo)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return CheckResult{CurrentVersion: currentVersion, Error: err.Error()}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "mtmr-lyrx-updater")

	resp, err := hc.Do(req)
	if err != nil {
		return CheckResult{CurrentVersion: currentVersion, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		msg := fmt.Sprintf("release not found (HTTP 404): %s", url)
		return CheckResult{CurrentVersion: currentVersion, Error: msg}, fmt.Errorf("%s", msg)
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("GitHub API returned HTTP %d", resp.StatusCode)
		return CheckResult{CurrentVersion: currentVersion, Error: msg}, fmt.Errorf("%s", msg)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CheckResult{CurrentVersion: currentVersion, Error: err.Error()}, err
	}

	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return CheckResult{CurrentVersion: currentVersion, Error: err.Error()}, err
	}

	if rel.Draft || (rel.Prerelease && opts.Version == "") {
		msg := "latest release is a draft or prerelease"
		return CheckResult{CurrentVersion: currentVersion, Error: msg}, fmt.Errorf("%s", msg)
	}

	goos := c.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := c.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	asset, err := SelectAsset(rel, goos, goarch)
	if err != nil {
		return CheckResult{
			CurrentVersion: currentVersion,
			LatestVersion:  rel.TagName,
			ReleaseURL:     rel.HTMLURL,
			Error:          err.Error(),
		}, err
	}

	csAsset, _ := SelectChecksumAsset(rel)

	cmp, _ := CompareVersions(currentVersion, rel.TagName)
	return CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   rel.TagName,
		UpdateAvailable: cmp < 0,
		ReleaseURL:      rel.HTMLURL,
		AssetName:       asset.Name,
		AssetURL:        asset.BrowserDownloadURL,
		ChecksumURL:     csAsset.BrowserDownloadURL,
	}, nil
}

// SelectAsset picks the release asset matching goos/goarch.
func SelectAsset(rel Release, goos, goarch string) (Asset, error) {
	// Preferred patterns in order
	patterns := []string{
		fmt.Sprintf("mtmr-lyrx_%s_%s.tar.gz", goos, goarch),
		fmt.Sprintf("mtmr-lyrx_%s_%s.tgz", goos, goarch),
		fmt.Sprintf("mtmr-lyrx_%s_%s.zip", goos, goarch),
	}
	for _, pat := range patterns {
		for _, a := range rel.Assets {
			if strings.EqualFold(a.Name, pat) {
				return a, nil
			}
		}
	}
	// Fallback: any asset containing goos and goarch
	for _, a := range rel.Assets {
		lower := strings.ToLower(a.Name)
		if strings.Contains(lower, goos) && strings.Contains(lower, goarch) &&
			(strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".zip")) {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("no update asset found for %s/%s", goos, goarch)
}

// SelectChecksumAsset returns the checksums.txt asset if present.
func SelectChecksumAsset(rel Release) (Asset, error) {
	for _, a := range rel.Assets {
		if strings.EqualFold(a.Name, "checksums.txt") {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("no checksums.txt asset found")
}

// ParseChecksums parses a checksums file into a map[filename]sha256hex.
// Supports two formats:
//
//	<sha256>  <filename>
//	SHA256 (<filename>) = <sha256>
func ParseChecksums(data []byte) (map[string]string, error) {
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// BSD format: SHA256 (filename) = hash
		if strings.HasPrefix(line, "SHA256 (") {
			rest := strings.TrimPrefix(line, "SHA256 (")
			idx := strings.Index(rest, ") = ")
			if idx < 0 {
				continue
			}
			filename := rest[:idx]
			hash := strings.TrimSpace(rest[idx+4:])
			if len(hash) == 64 {
				result[filename] = strings.ToLower(hash)
			}
			continue
		}
		// GNU format: hash  filename
		parts := strings.Fields(line)
		if len(parts) == 2 && len(parts[0]) == 64 {
			result[parts[1]] = strings.ToLower(parts[0])
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid checksum entries found")
	}
	return result, nil
}
