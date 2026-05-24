package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zakyyudha/mtmr-lyrx/internal/updater"
)

func newUpdateCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install mtmr-lyrx updates from GitHub Releases",
	}
	cmd.AddCommand(newUpdateCheckCommand(opts))
	cmd.AddCommand(newUpdateInstallCommand(opts))
	return cmd
}

func newUpdateCheckCommand(opts *Options) *cobra.Command {
	var (
		repo       string
		apiBaseURL string
		version    string
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check whether a newer mtmr-lyrx release is available",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := updater.Client{
				APIBaseURL: apiBaseURL,
				Repo:       repo,
			}
			currentVersion := version
			if currentVersion == "" {
				currentVersion = cmd.Root().Version
			}
			res, err := c.Check(cmd.Context(), currentVersion, updater.CheckOptions{Version: version})
			if err != nil && res.LatestVersion == "" {
				// hard network/parse error — still emit JSON if requested
				if opts.JSON {
					out, _ := json.MarshalIndent(res, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "update check failed: %v\n", err)
				}
				return err
			}

			if opts.JSON {
				out, _ := json.MarshalIndent(res, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Current version:  %s\n", res.CurrentVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "Latest version:   %s\n", res.LatestVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "Update available: %v\n", res.UpdateAvailable)
			if res.ReleaseURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Release:          %s\n", res.ReleaseURL)
			}
			if res.AssetName != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Asset:            %s\n", res.AssetName)
			}
			if res.Error != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", res.Error)
				return fmt.Errorf("%s", res.Error)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", updater.DefaultRepo, "GitHub repository (owner/repo)")
	cmd.Flags().StringVar(&apiBaseURL, "api-base-url", updater.DefaultAPIBaseURL, "GitHub API base URL (for testing)")
	cmd.Flags().StringVar(&version, "version", "", "check a specific release tag instead of latest")
	return cmd
}

func newUpdateInstallCommand(opts *Options) *cobra.Command {
	var (
		repo        string
		apiBaseURL  string
		version     string
		assetURL    string
		checksumURL string
		checksum    string
		target      string
		dryRun      bool
		yes         bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Download and install the latest mtmr-lyrx CLI binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := updater.Client{
				APIBaseURL: apiBaseURL,
				Repo:       repo,
			}

			iopts := updater.InstallOptions{
				TargetPath:       target,
				AssetURL:         assetURL,
				ChecksumURL:      checksumURL,
				ExpectedChecksum: checksum,
				DryRun:           dryRun,
				Yes:              yes,
			}

			res, err := c.Install(cmd.Context(), version, iopts)
			if opts.JSON {
				out, _ := json.MarshalIndent(res, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				if err != nil {
					return err
				}
				return nil
			}

			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "update install failed: %v\n", err)
				return err
			}

			if res.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run:   true\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Target:    %s\n", res.Target)
				fmt.Fprintf(cmd.OutOrStdout(), "Asset:     %s\n", res.AssetName)
				if res.ChecksumURL != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Checksum:  %s\n", res.ChecksumURL)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", res.Message)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed: %v\n", res.Installed)
			fmt.Fprintf(cmd.OutOrStdout(), "Target:    %s\n", res.Target)
			if res.Backup != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Backup:    %s\n", res.Backup)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", res.Message)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", updater.DefaultRepo, "GitHub repository (owner/repo)")
	cmd.Flags().StringVar(&apiBaseURL, "api-base-url", updater.DefaultAPIBaseURL, "GitHub API base URL (for testing)")
	cmd.Flags().StringVar(&version, "version", "", "install a specific release tag")
	cmd.Flags().StringVar(&assetURL, "asset-url", "", "explicit asset download URL (for testing)")
	cmd.Flags().StringVar(&checksumURL, "checksum-url", "", "explicit checksum file URL (for testing)")
	cmd.Flags().StringVar(&checksum, "checksum", "", "explicit expected SHA-256 hex (for testing)")
	cmd.Flags().StringVar(&target, "target", "", "override binary replacement path")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "resolve and download plan without replacing binary")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip interactive confirmation (for non-interactive callers)")
	return cmd
}
