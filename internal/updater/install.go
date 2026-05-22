package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// InstallOptions controls the install behaviour.
type InstallOptions struct {
	// TargetPath overrides the binary to replace (default: os.Executable()).
	TargetPath string
	// AssetURL skips release lookup and downloads this URL directly.
	AssetURL string
	// ChecksumURL skips release lookup for checksum file.
	ChecksumURL string
	// ExpectedChecksum is an explicit SHA-256 hex string (skips checksum file download).
	ExpectedChecksum string
	// DryRun resolves everything but does not replace the target.
	DryRun bool
	// Yes skips interactive confirmation (non-interactive callers).
	Yes bool
}

// InstallResult is the output of an install operation.
type InstallResult struct {
	Installed   bool   `json:"installed"`
	DryRun      bool   `json:"dry_run"`
	Version     string `json:"version"`
	Target      string `json:"target"`
	Backup      string `json:"backup,omitempty"`
	AssetName   string `json:"asset_name"`
	ChecksumURL string `json:"checksum_url,omitempty"`
	Message     string `json:"message"`
	Error       string `json:"error,omitempty"`
}

// Install fetches the release, verifies checksum, and replaces the target binary.
func (c Client) Install(ctx context.Context, versionHint string, opts InstallOptions) (InstallResult, error) {
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}

	res := InstallResult{DryRun: opts.DryRun}

	// Resolve target path
	target := opts.TargetPath
	if target == "" {
		exe, err := os.Executable()
		if err != nil {
			res.Error = err.Error()
			return res, fmt.Errorf("resolve executable: %w", err)
		}
		target = exe
	}
	res.Target = target

	// Resolve asset URL and checksum URL via release lookup if not provided
	assetURL := opts.AssetURL
	checksumURL := opts.ChecksumURL
	assetName := ""

	if assetURL == "" {
		checkRes, err := c.Check(ctx, versionHint, CheckOptions{Version: versionHint})
		if err != nil {
			res.Error = err.Error()
			return res, err
		}
		assetURL = checkRes.AssetURL
		checksumURL = checkRes.ChecksumURL
		assetName = checkRes.AssetName
		res.Version = checkRes.LatestVersion
	} else {
		// derive name from URL
		parts := strings.Split(assetURL, "/")
		assetName = parts[len(parts)-1]
		res.Version = versionHint
	}
	res.AssetName = assetName
	res.ChecksumURL = checksumURL

	if opts.DryRun {
		res.Message = fmt.Sprintf("dry run: would install %s to %s", assetName, target)
		return res, nil
	}

	// Download asset to temp file
	tmpDir, err := os.MkdirTemp("", "mtmr-lyrx-update-*")
	if err != nil {
		res.Error = err.Error()
		return res, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	if err := DownloadFile(ctx, hc, assetURL, archivePath); err != nil {
		res.Error = err.Error()
		return res, fmt.Errorf("download asset: %w", err)
	}

	// Verify checksum
	expectedHash := opts.ExpectedChecksum
	if expectedHash == "" && checksumURL != "" {
		csPath := filepath.Join(tmpDir, "checksums.txt")
		if err := DownloadFile(ctx, hc, checksumURL, csPath); err != nil {
			res.Error = err.Error()
			return res, fmt.Errorf("download checksums: %w", err)
		}
		csData, err := os.ReadFile(csPath)
		if err != nil {
			res.Error = err.Error()
			return res, fmt.Errorf("read checksums: %w", err)
		}
		csMap, err := ParseChecksums(csData)
		if err != nil {
			res.Error = err.Error()
			return res, fmt.Errorf("parse checksums: %w", err)
		}
		h, ok := csMap[assetName]
		if !ok {
			msg := fmt.Sprintf("checksum not found for %s in checksums.txt", assetName)
			res.Error = msg
			return res, fmt.Errorf("%s", msg)
		}
		expectedHash = h
	}

	if expectedHash != "" {
		if err := VerifySHA256(archivePath, expectedHash); err != nil {
			res.Error = err.Error()
			return res, err
		}
	}

	// Extract binary
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		res.Error = err.Error()
		return res, fmt.Errorf("create extract dir: %w", err)
	}
	binPath, err := ExtractBinary(archivePath, extractDir)
	if err != nil {
		res.Error = err.Error()
		return res, fmt.Errorf("extract binary: %w", err)
	}

	// Replace binary
	backup, err := ReplaceBinary(binPath, target)
	if err != nil {
		res.Error = err.Error()
		return res, fmt.Errorf("replace binary: %w", err)
	}

	res.Installed = true
	res.Backup = backup
	res.Message = fmt.Sprintf("installed %s; restart daemon/menu bar to use new binary", res.Version)
	return res, nil
}

// DownloadFile downloads url to dst path.
func DownloadFile(ctx context.Context, hc *http.Client, url, dst string) error {
	if hc == nil {
		hc = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "mtmr-lyrx-updater")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// VerifySHA256 computes the SHA-256 of path and compares to expected (hex string).
func VerifySHA256(path string, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	exp := strings.ToLower(strings.TrimSpace(expected))
	if got != exp {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, exp)
	}
	return nil
}

// ExtractBinary extracts the mtmr-lyrx binary from a .tar.gz, .tgz, or .zip archive.
// Returns the path to the extracted binary in dstDir.
func ExtractBinary(archivePath, dstDir string) (string, error) {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractTarGz(archivePath, dstDir)
	}
	if strings.HasSuffix(lower, ".zip") {
		return extractZip(archivePath, dstDir)
	}
	return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
}

func isSafePath(name string) bool {
	if filepath.IsAbs(name) {
		return false
	}
	cleaned := filepath.Clean(name)
	if strings.HasPrefix(cleaned, "..") {
		return false
	}
	return true
}

func isBinaryEntry(name string) bool {
	base := filepath.Base(name)
	return base == "mtmr-lyrx"
}

func extractTarGz(archivePath, dstDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip open: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read: %w", err)
		}
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			return "", fmt.Errorf("unsafe archive path: symlinks/hardlinks not allowed")
		}
		if !isSafePath(hdr.Name) {
			return "", fmt.Errorf("unsafe archive path: %q", hdr.Name)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !isBinaryEntry(hdr.Name) {
			continue
		}
		dst := filepath.Join(dstDir, "mtmr-lyrx")
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", err
		}
		out.Close()
		return dst, nil
	}
	return "", fmt.Errorf("mtmr-lyrx binary not found in archive")
}

func extractZip(archivePath, dstDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("unsafe archive path: symlinks not allowed")
		}
		if !isSafePath(f.Name) {
			return "", fmt.Errorf("unsafe archive path: %q", f.Name)
		}
		if !isBinaryEntry(f.Name) {
			continue
		}
		dst := filepath.Join(dstDir, "mtmr-lyrx")
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return "", copyErr
		}
		return dst, nil
	}
	return "", fmt.Errorf("mtmr-lyrx binary not found in archive")
}

// ReplaceBinary backs up target to target.bak and renames src to target.
// Returns the backup path on success.
func ReplaceBinary(src, target string) (string, error) {
	backup := target + ".bak"

	// Backup existing binary
	if _, err := os.Stat(target); err == nil {
		if err := copyFile(target, backup); err != nil {
			return "", fmt.Errorf("backup failed: %w", err)
		}
	}

	// Rename new binary into place
	if err := os.Rename(src, target); err != nil {
		// Try to restore backup
		if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
			_ = copyFile(backup, target)
		}
		return "", fmt.Errorf("replace binary: %w", err)
	}

	return backup, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
