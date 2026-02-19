package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/j4rs/standup-echo/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configureCmd)
}

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Interactively set up the standup bot configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load existing config as defaults if it exists.
		existing, _ := config.Load(cfgPath)
		if existing == nil {
			existing = &config.Config{}
		}

		reader := bufio.NewReader(os.Stdin)

		cfg := &config.Config{
			SlackBotToken:    prompt(reader, "Slack Bot Token (xoxb-...)", existing.SlackBotToken),
			SlackAppToken:    prompt(reader, "Slack App Token (xapp-...)", existing.SlackAppToken),
			ChannelID:        prompt(reader, "Channel ID", existing.ChannelID),
			ThreadIdentifier: prompt(reader, "Thread identifier (text to match in standup posts)", existing.ThreadIdentifier),
		}

		if err := config.Save(cfgPath, cfg); err != nil {
			return err
		}
		fmt.Printf("Configuration saved to %s\n", cfgPath)
		return nil
	},
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, maskToken(defaultVal))
	} else {
		fmt.Printf("%s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

// maskToken shows the first 8 and last 4 characters of a token, masking the rest.
func maskToken(s string) string {
	if len(s) <= 16 {
		return "****"
	}
	return s[:8] + "..." + s[len(s)-4:]
}
