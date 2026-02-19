package cmd

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/j4rs/standup-echo/internal/bot"
	"github.com/j4rs/standup-echo/internal/config"
	"github.com/j4rs/standup-echo/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	serveCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")
	rootCmd.AddCommand(serveCmd)
}

var verbose bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the standup bot daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		logLevel := slog.LevelInfo
		if verbose {
			logLevel = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		dbPath := filepath.Join(filepath.Dir(cfgPath), "standup-echo.db")
		subscribers, err := store.NewSubscribers(dbPath)
		if err != nil {
			return err
		}
		defer subscribers.Close()

		logger.Info("starting standup-echo", "config", cfgPath, "channel", cfg.ChannelID)

		b, err := bot.New(cfg, subscribers, logger)
		if err != nil {
			return err
		}
		return b.Run()
	},
}
