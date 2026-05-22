package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zakyyudha/mtmr-lyrx/internal/cache"
	"github.com/zakyyudha/mtmr-lyrx/internal/config"
)

func newCacheCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage provider cache and metadata",
	}
	cmd.AddCommand(newCacheClearCommand(opts))
	cmd.AddCommand(newCacheShowCommand(opts))
	return cmd
}

func newCacheClearCommand(opts *Options) *cobra.Command {
	var (
		provider string
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove LRCLIB lookup metadata (not lyric text) from cache",
		Long: `Removes LRCLIB lookup metadata files from the cache directory.
Only metadata is stored (provider, status, confidence, match info) — not lyric text.
Use --dry-run to preview what would be deleted without actually deleting.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}

			cacheDir := cfg.Cache.Dir
			if cacheDir == "" {
				cacheDir = config.DefaultCacheDir()
			}

			result, err := cache.ClearMetadata(cacheDir, cache.ClearOptions{
				Provider: provider,
				DryRun:   dryRun,
			})
			if err != nil {
				return fmt.Errorf("cache clear: %w", err)
			}

			if opts.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"provider": provider,
					"dry_run":  result.DryRun,
					"paths":    result.Paths,
				})
			}

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would clear %d file(s) for provider %q\n", len(result.Paths), provider)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "cleared %d file(s) for provider %q\n", len(result.Paths), provider)
			}
			for _, p := range result.Paths {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", p)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "lrclib", "provider to clear (lrclib, all)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print paths without deleting")

	return cmd
}

func newCacheShowCommand(opts *Options) *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show cache metadata files and their status",
		Long: `Lists known cache metadata files for the given provider.
Only metadata is stored (provider, status, confidence, match info) — not lyric text.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}

			cacheDir := cfg.Cache.Dir
			if cacheDir == "" {
				cacheDir = config.DefaultCacheDir()
			}

			files, err := cache.ShowMetadata(cacheDir, provider)
			if err != nil {
				return fmt.Errorf("cache show: %w", err)
			}

			if opts.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"cache_dir": cacheDir,
					"files":     files,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "cache_dir: %s\n", cacheDir)
			for _, f := range files {
				if f.Exists {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-40s  exists  %d bytes\n", f.Path, f.SizeBytes)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-40s  not found\n", f.Path)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "lrclib", "provider to show (lrclib, all)")
	return cmd
}
