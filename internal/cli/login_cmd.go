package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"github.com/zakyyudha/mtmr-lyrx/internal/spotify"
)

func newLoginCommand(opts *Options) *cobra.Command {
	var (
		clientID     string
		clientSecret string
		redirectURL  string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Spotify using OAuth",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}

			// Resolve credentials: flags > env > config
			id := clientID
			if id == "" {
				id, _ = spotify.Credentials(cfg.Spotify)
			}
			secret := clientSecret
			if secret == "" {
				_, secret = spotify.Credentials(cfg.Spotify)
			}

			// Interactive prompt fallback if TTY
			if id == "" {
				fmt.Fprint(cmd.OutOrStdout(), "Spotify Client ID: ")
				fmt.Fscan(os.Stdin, &id)
			}
			if secret == "" {
				fmt.Fprint(cmd.OutOrStdout(), "Spotify Client Secret: ")
				fmt.Fscan(os.Stdin, &secret)
			}

			if id == "" || secret == "" {
				return fmt.Errorf("client ID and secret are required; use --client-id/--client-secret or SPOTIFY_CLIENT_ID/SPOTIFY_CLIENT_SECRET env vars")
			}

			ru := redirectURL
			if ru == "" {
				ru = cfg.Spotify.RedirectURL
			}

			oauthCfg := spotify.OAuthConfig(id, secret, ru)

			// Generate random state
			stateBytes := make([]byte, 16)
			rand.Read(stateBytes)
			state := hex.EncodeToString(stateBytes)

			// Start local callback server
			codeCh := make(chan string, 1)
			errCh := make(chan error, 1)

			listener, err := net.Listen("tcp", "127.0.0.1:8888")
			if err != nil {
				return fmt.Errorf("start callback server: %w (is port 8888 in use?)", err)
			}

			mux := http.NewServeMux()
			srv := &http.Server{Handler: mux}

			mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("state") != state {
					errCh <- fmt.Errorf("state mismatch — possible CSRF")
					http.Error(w, "state mismatch", http.StatusBadRequest)
					return
				}
				code := r.URL.Query().Get("code")
				if code == "" {
					errCh <- fmt.Errorf("no code in callback: %s", r.URL.Query().Get("error"))
					http.Error(w, "no code", http.StatusBadRequest)
					return
				}
				fmt.Fprintln(w, "<html><body><h2>Login successful! You can close this tab.</h2></body></html>")
				codeCh <- code
			})

			go srv.Serve(listener)
			defer srv.Close()

			authURL := oauthCfg.AuthCodeURL(state)

			// Try to open browser
			fmt.Fprintf(cmd.OutOrStdout(), "\nOpening browser for Spotify login...\n")
			if err := exec.Command("open", authURL).Start(); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Could not open browser automatically.\n")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nIf the browser did not open, visit:\n%s\n\n", authURL)

			// Wait for callback
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			var code string
			select {
			case code = <-codeCh:
			case err := <-errCh:
				return fmt.Errorf("login callback error: %w", err)
			case <-ctx.Done():
				return fmt.Errorf("login timed out after 5 minutes")
			}

			// Exchange code for token
			tok, err := oauthCfg.Exchange(context.Background(), code)
			if err != nil {
				return fmt.Errorf("exchange code for token: %w", err)
			}

			// Save token
			tokenPath := spotify.ResolveTokenPath(cfg.Spotify)
			if err := spotify.SaveToken(tokenPath, tok); err != nil {
				return fmt.Errorf("save token: %w", err)
			}

			// Persist Spotify app credentials so status/run can refresh tokens later
			// without requiring one-shot environment variables.
			cfg.Spotify.ClientID = id
			cfg.Spotify.ClientSecret = secret
			cfg.Spotify.RedirectURL = ru
			configPath := configPathFromOpts(opts)
			data, err := marshalFullConfig(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			if err := os.MkdirAll(configDir(configPath), 0755); err != nil {
				return fmt.Errorf("create config dir: %w", err)
			}
			if err := os.WriteFile(configPath, data, 0600); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Login successful! Token saved to: %s\n", tokenPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Spotify credentials saved to: %s\n", configPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Run 'mtmr-lyrx status' to verify.\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&clientID, "client-id", "", "Spotify client ID (or set SPOTIFY_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Spotify client secret (or set SPOTIFY_CLIENT_SECRET)")
	cmd.Flags().StringVar(&redirectURL, "redirect-url", "", "OAuth redirect URL (default: http://127.0.0.1:8888/callback)")

	return cmd
}
