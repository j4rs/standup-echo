package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/j4rs/standup-echo/internal/config"
	"github.com/j4rs/standup-echo/internal/standup"
	"github.com/j4rs/standup-echo/internal/store"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var (
	triggerUser     string
	triggerDate     string
	triggerChannel  string
	triggerReminder bool
)

func init() {
	triggerCmd.Flags().StringVar(&triggerUser, "user", "", "only DM this Slack user ID")
	triggerCmd.Flags().StringVar(&triggerDate, "date", "", "find thread for this date (YYYY-MM-DD)")
	triggerCmd.Flags().StringVar(&triggerChannel, "channel", "", "channel ID or name to trigger (required when multiple are configured)")
	triggerCmd.Flags().BoolVar(&triggerReminder, "reminder", false, "send the mid-day \"you haven't posted yet\" reminder instead of the echo")
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

		channel, err := resolveChannel(cfg, triggerChannel)
		if err != nil {
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
		svc := standup.NewService(
			api, channel.ChannelID, channel.ThreadIdentifier, cfg.MissedStandupGrace(), subscribers,
			logger.With("channel", channel.Label()),
		)

		// --reminder rehearses the scheduled nudge without waiting for the
		// timer, and without consulting the sent-reminder table, so it can be
		// run repeatedly. Pair it with --user to keep a test to yourself.
		if triggerReminder {
			threadTS, err := svc.FindStandupThread(date)
			if err != nil {
				return err
			}
			svc.RemindUnposted(threadTS, triggerUser)
			return nil
		}

		svc.ProcessLatestStandup(triggerUser, date)
		return nil
	},
}

// resolveChannel picks which configured channel to act on. It defaults to the
// only channel when just one is configured, and otherwise requires --channel.
func resolveChannel(cfg *config.Config, want string) (*config.ChannelConfig, error) {
	if want != "" {
		ch := cfg.FindChannel(want)
		if ch == nil {
			return nil, fmt.Errorf("channel %q not found in config; configured: %s", want, channelList(cfg))
		}
		return ch, nil
	}
	if len(cfg.Channels) == 1 {
		return &cfg.Channels[0], nil
	}
	return nil, fmt.Errorf("--channel is required when multiple channels are configured; configured: %s", channelList(cfg))
}

func channelList(cfg *config.Config) string {
	labels := make([]string, 0, len(cfg.Channels))
	for _, ch := range cfg.Channels {
		labels = append(labels, ch.Label())
	}
	return strings.Join(labels, ", ")
}
