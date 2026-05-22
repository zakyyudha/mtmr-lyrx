package spotify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"

	"github.com/zakyyudha/mtmr-lyrx/internal/config"
)

// DefaultTokenPath returns the default path for the Spotify OAuth token file.
func DefaultTokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "spotify-auth.json")
	}
	return filepath.Join(home, ".config", "mtmr-lyrx", "spotify-auth.json")
}

// ResolveTokenPath returns cfg.TokenFile if non-empty, else DefaultTokenPath.
func ResolveTokenPath(cfg config.SpotifyConfig) string {
	if cfg.TokenFile != "" {
		return cfg.TokenFile
	}
	return DefaultTokenPath()
}

// SaveToken writes an OAuth2 token to path as JSON with mode 0600.
// Creates parent directories as needed.
func SaveToken(path string, tok *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("spotify: create token dir: %w", err)
	}
	data, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("spotify: marshal token: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("spotify: write token: %w", err)
	}
	return nil
}

// LoadToken reads an OAuth2 token from path.
func LoadToken(path string) (*oauth2.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spotify: read token: %w", err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("spotify: parse token: %w", err)
	}
	return &tok, nil
}

// Credentials resolves client ID and secret from env vars first, then config.
// Env vars: SPOTIFY_CLIENT_ID, SPOTIFY_CLIENT_SECRET.
func Credentials(cfg config.SpotifyConfig) (clientID, clientSecret string) {
	clientID = os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = cfg.ClientID
	}
	clientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = cfg.ClientSecret
	}
	return clientID, clientSecret
}

// OAuthConfig returns an oauth2.Config for Spotify Authorization Code flow.
func OAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"user-read-currently-playing", "user-read-playback-state"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
	}
}

// IsTokenValid returns true if the token is non-nil and not expired.
func IsTokenValid(tok *oauth2.Token) bool {
	if tok == nil {
		return false
	}
	if tok.AccessToken == "" {
		return false
	}
	if !tok.Expiry.IsZero() && tok.Expiry.Before(time.Now()) {
		return false
	}
	return true
}
