package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/j4rs/standup-echo/internal/config"
	"github.com/j4rs/standup-echo/internal/standup"
	"github.com/j4rs/standup-echo/internal/store"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var (
	triggerUser string
	triggerDate string
)

func init() {
	triggerCmd.Flags().StringVar(&triggerUser, "user", "", "only DM this Slack user ID")
	triggerCmd.Flags().StringVar(&triggerDate, "date", "", "find thread for this date (YYYY-MM-DD)")
	rootCmd.AddCommand(triggerCmd)
}

var triggerCmd = &cobra.Command{
	Use:   "trigger",
	Short: "Manually find a standup thread and DM subscribers their replies",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		var date time.Time
		if triggerDate != "" {
			date, err = time.ParseInLocation("2006-01-02", triggerDate, time.Local)
			if err != nil {
				return fmt.Errorf("invalid date format, use YYYY-MM-DD: %w", err)
			}
		}

		dbPath := filepath.Join(filepath.Dir(cfgPath), "standup-echo.db")
		subscribers, err := store.NewSubscribers(dbPath)
		if err != nil {
			return err
		}
		defer subscribers.Close()

		api := slack.New(cfg.SlackBotToken)
		svc := standup.NewService(api, cfg.ChannelID, cfg.ThreadIdentifier, subscribers, logger)

		svc.ProcessLatestStandup(triggerUser, date)
		return nil
	},
}
