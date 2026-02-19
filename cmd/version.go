package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print build version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("standup-echo %s (commit: %s, built: %s)\n", version, commit, date)
	},
}
