package cli

import (
	"github.com/spf13/cobra"
)

var version = "dev"

// Options holds persistent flags shared across all commands.
type Options struct {
	ConfigPath string
	Debug      bool
	JSON       bool
}

// NewRootCommand creates and returns the root cobra command.
func NewRootCommand() *cobra.Command {
	opts := &Options{}

	root := &cobra.Command{
		Use:          "mtmr-lyrx",
		Short:        "Show synced lyrics on the MTMR Touch Bar",
		Version:      version,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "config file path (default: ~/.config/mtmr-lyrx/config.yaml)")
	root.PersistentFlags().BoolVar(&opts.Debug, "debug", false, "enable debug logging")
	root.PersistentFlags().BoolVar(&opts.JSON, "json", false, "output as JSON")

	root.AddCommand(newConfigCommand(opts))
	root.AddCommand(newLookupCommand(opts))
	root.AddCommand(newCacheCommand(opts))
	root.AddCommand(newRunCommand(opts))
	root.AddCommand(newMTMRConfigCommand(opts))
	root.AddCommand(newLoginCommand(opts))
	root.AddCommand(newStatusCommand(opts))
	root.AddCommand(newOffsetCommand(opts))
	root.AddCommand(newUpdateCommand(opts))

	return root
}
