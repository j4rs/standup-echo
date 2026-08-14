package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/j4rs/standup-echo/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configureCmd)
}

var errInputClosed = errors.New("input closed")

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

		botToken, err := promptSecret(reader, "Slack Bot Token (xoxb-...)", existing.SlackBotToken)
		if err != nil {
			return err
		}
		appToken, err := promptSecret(reader, "Slack App Token (xapp-...)", existing.SlackAppToken)
		if err != nil {
			return err
		}
		channels, err := promptChannels(reader, existing.Channels)
		if err != nil {
			return err
		}

		cfg := &config.Config{
			SlackBotToken: botToken,
			SlackAppToken: appToken,
			Channels:      channels,
		}

		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := config.Save(cfgPath, cfg); err != nil {
			return err
		}
		fmt.Printf("\nConfiguration saved to %s (%d channel(s))\n", cfgPath, len(cfg.Channels))
		return nil
	},
}

// promptChannels walks the existing channels for edits, then offers to add more.
// Entering '-' for a channel ID drops that channel.
func promptChannels(reader *bufio.Reader, existing []config.ChannelConfig) ([]config.ChannelConfig, error) {
	var channels []config.ChannelConfig

	for i, ch := range existing {
		fmt.Printf("\n--- Channel %d: %s ---\n", i+1, ch.Label())
		fmt.Println("(enter '-' as the channel ID to remove it)")
		updated, keep, err := promptChannel(reader, ch)
		if err != nil {
			return nil, err
		}
		if keep {
			channels = append(channels, updated)
		} else {
			fmt.Printf("Removed %s\n", ch.Label())
		}
	}

	for {
		if len(channels) > 0 {
			more, err := confirm(reader, "\nAdd another channel?")
			if err != nil {
				return nil, err
			}
			if !more {
				break
			}
		}

		fmt.Printf("\n--- Channel %d: new ---\n", len(channels)+1)
		added, keep, err := promptChannel(reader, config.ChannelConfig{})
		if err != nil {
			return nil, err
		}
		if keep {
			channels = append(channels, added)
			continue
		}
		// Nothing entered and nothing configured — stop and let Validate report it
		// rather than re-prompting forever.
		if len(channels) == 0 {
			return nil, nil
		}
	}

	return channels, nil
}

// promptChannel collects one channel's fields. It returns keep=false when the
// channel should be dropped or was skipped.
func promptChannel(reader *bufio.Reader, existing config.ChannelConfig) (config.ChannelConfig, bool, error) {
	id, err := prompt(reader, "Channel ID", existing.ChannelID)
	if err != nil {
		return config.ChannelConfig{}, false, err
	}
	if id == "" || id == "-" {
		return config.ChannelConfig{}, false, nil
	}

	identifier, err := prompt(reader, "Thread identifier (text from the standup message body)", existing.ThreadIdentifier)
	if err != nil {
		return config.ChannelConfig{}, false, err
	}
	name, err := prompt(reader, "Short name for logs (optional)", existing.Name)
	if err != nil {
		return config.ChannelConfig{}, false, err
	}

	return config.ChannelConfig{
		ChannelID:        id,
		ThreadIdentifier: identifier,
		Name:             name,
	}, true, nil
}

func confirm(reader *bufio.Reader, label string) (bool, error) {
	fmt.Printf("%s [y/N]: ", label)
	input, err := readLine(reader)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(input) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// prompt reads a value, showing the current value as the default.
func prompt(reader *bufio.Reader, label, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	input, err := readLine(reader)
	if err != nil {
		return "", err
	}
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}

// promptSecret reads a value, showing the current value masked so a token isn't
// echoed in full.
func promptSecret(reader *bufio.Reader, label, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, maskToken(defaultVal))
	} else {
		fmt.Printf("%s: ", label)
	}
	input, err := readLine(reader)
	if err != nil {
		return "", err
	}
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}

func readLine(reader *bufio.Reader) (string, error) {
	input, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("reading input: %w", err)
	}
	// A bare EOF with no bytes means stdin closed; bail instead of looping.
	if errors.Is(err, io.EOF) && strings.TrimSpace(input) == "" {
		return "", errInputClosed
	}
	return strings.TrimSpace(input), nil
}

// maskToken shows the first 8 and last 4 characters of a token, masking the rest.
func maskToken(s string) string {
	if len(s) <= 16 {
		return "****"
	}
	return s[:8] + "..." + s[len(s)-4:]
}
