package cmd

import (
	"fmt"
	"os"

	"github.com/j4rs/standup-echo/internal/config"
	"github.com/spf13/cobra"
)

var cfgPath string

var rootCmd = &cobra.Command{
	Use:   "standup-echo",
	Short: "A Slack bot that DMs your previous standup update when a new thread appears",
}

func init() {
	defaultPath, _ := config.DefaultPath()
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", defaultPath, "path to config file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
