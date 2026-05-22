package spotify

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/zakyyudha/mtmr-lyrx/internal/config"
)

func TestDefaultTokenPath(t *testing.T) {
	p := DefaultTokenPath()
	if filepath.Base(p) != "spotify-auth.json" {
		t.Errorf("expected spotify-auth.json, got %q", p)
	}
	if filepath.Base(filepath.Dir(p)) != "mtmr-lyrx" {
		t.Errorf("expected parent dir mtmr-lyrx, got %q", filepath.Dir(p))
	}
}

func TestResolveTokenPathCustom(t *testing.T) {
	cfg := config.SpotifyConfig{TokenFile: "/tmp/test-token.json"}
	if got := ResolveTokenPath(cfg); got != "/tmp/test-token.json" {
		t.Errorf("expected /tmp/test-token.json, got %q", got)
	}
}

func TestResolveTokenPathDefault(t *testing.T) {
	cfg := config.SpotifyConfig{TokenFile: ""}
	got := ResolveTokenPath(cfg)
	if filepath.Base(got) != "spotify-auth.json" {
		t.Errorf("expected spotify-auth.json, got %q", got)
	}
}

func TestSaveAndLoadToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spotify-auth.json")

	tok := &oauth2.Token{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	if err := SaveToken(path, tok); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	// Check file mode
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected mode 0600, got %o", info.Mode().Perm())
	}

	loaded, err := LoadToken(path)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if loaded.AccessToken != "test-access" {
		t.Errorf("expected test-access, got %q", loaded.AccessToken)
	}
	if loaded.RefreshToken != "test-refresh" {
		t.Errorf("expected test-refresh, got %q", loaded.RefreshToken)
	}
}

func TestSaveTokenCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "spotify-auth.json")
	tok := &oauth2.Token{AccessToken: "test"}
	if err := SaveToken(path, tok); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected token file to exist")
	}
}

func TestLoadTokenMissing(t *testing.T) {
	_, err := LoadToken("/nonexistent/path/token.json")
	if err == nil {
		t.Error("expected error for missing token file")
	}
}

func TestCredentialsEnvOverride(t *testing.T) {
	t.Setenv("SPOTIFY_CLIENT_ID", "env-id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "env-secret")
	cfg := config.SpotifyConfig{ClientID: "cfg-id", ClientSecret: "cfg-secret"}
	id, secret := Credentials(cfg)
	if id != "env-id" {
		t.Errorf("expected env-id, got %q", id)
	}
	if secret != "env-secret" {
		t.Errorf("expected env-secret, got %q", secret)
	}
}

func TestCredentialsFallbackToConfig(t *testing.T) {
	t.Setenv("SPOTIFY_CLIENT_ID", "")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "")
	cfg := config.SpotifyConfig{ClientID: "cfg-id", ClientSecret: "cfg-secret"}
	id, secret := Credentials(cfg)
	if id != "cfg-id" {
		t.Errorf("expected cfg-id, got %q", id)
	}
	if secret != "cfg-secret" {
		t.Errorf("expected cfg-secret, got %q", secret)
	}
}

func TestIsTokenValid(t *testing.T) {
	if IsTokenValid(nil) {
		t.Error("nil token should be invalid")
	}
	if IsTokenValid(&oauth2.Token{}) {
		t.Error("empty token should be invalid")
	}
	expired := &oauth2.Token{
		AccessToken: "x",
		Expiry:      time.Now().Add(-time.Hour),
	}
	if IsTokenValid(expired) {
		t.Error("expired token should be invalid")
	}
	valid := &oauth2.Token{
		AccessToken: "x",
		Expiry:      time.Now().Add(time.Hour),
	}
	if !IsTokenValid(valid) {
		t.Error("valid token should be valid")
	}
}
