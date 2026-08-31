package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ChannelConfig describes one standup channel: where to watch and what text
// marks a standup post in that channel. Different teams schedule their standup
// with different wording, so the identifier is paired with the channel.
type ChannelConfig struct {
	// Name is an optional human label used in logs and the trigger command.
	Name             string `yaml:"name,omitempty"`
	ChannelID        string `yaml:"channel_id"`
	ThreadIdentifier string `yaml:"thread_identifier"`
}

// Label returns the channel's name if set, otherwise its ID.
func (c ChannelConfig) Label() string {
	if c.Name != "" {
		return c.Name
	}
	return c.ChannelID
}

// defaultMaxMissedStandups is the grace allowance applied when a config does
// not set one. Two keeps a forgotten day (and a two-day trip) recoverable while
// costing an absent subscriber only two nudges before the bot goes quiet.
const defaultMaxMissedStandups = 2

// defaultReminderAfter is how long after a standup thread is posted the
// "you haven't posted yet" reminder fires when the config does not say.
// Anchoring to the thread rather than a wall clock avoids a timezone field and
// follows the standup if its scheduled time ever moves.
const defaultReminderAfter = 4 * time.Hour

type Config struct {
	SlackBotToken string          `yaml:"slack_bot_token"`
	SlackAppToken string          `yaml:"slack_app_token"`
	Channels      []ChannelConfig `yaml:"channels"`

	// MaxMissedStandups is how many consecutive standups a subscriber may miss
	// and still receive a nudge. It is a pointer so that an explicit 0 (never
	// nudge someone who skipped the last standup) is distinguishable from the
	// key being absent, which takes defaultMaxMissedStandups. Global rather
	// than per-channel, matching the global subscriber opt-in.
	MaxMissedStandups *int `yaml:"max_missed_standups,omitempty"`

	// ReminderAfter is how long after a standup thread is posted to DM anyone
	// on the roster who has not replied to it yet, as a Go duration ("4h",
	// "90m"). Empty takes defaultReminderAfter; "0" disables reminders.
	ReminderAfter string `yaml:"reminder_after,omitempty"`

	// Deprecated: pre-multi-channel fields. Load folds these into Channels and
	// clears them, so configs written before multi-channel support keep working.
	ChannelID        string `yaml:"channel_id,omitempty"`
	ThreadIdentifier string `yaml:"thread_identifier,omitempty"`
}

// MissedStandupGrace returns how many consecutive standups a subscriber may
// miss and still be nudged, falling back to the default when unset.
func (c *Config) MissedStandupGrace() int {
	if c.MaxMissedStandups == nil {
		return defaultMaxMissedStandups
	}
	return *c.MaxMissedStandups
}

// ReminderDelay returns how long after a standup thread is posted the reminder
// should fire, and whether reminders are enabled at all. Validate has already
// rejected an unparseable value, so a parse failure here disables reminders
// rather than guessing a delay.
func (c *Config) ReminderDelay() (time.Duration, bool) {
	if c.ReminderAfter == "" {
		return defaultReminderAfter, true
	}
	d, err := time.ParseDuration(c.ReminderAfter)
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}

// DefaultPath returns ~/.config/standup-echo/config.yml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "standup-echo", "config.yml"), nil
}

// Load reads a Config from a YAML file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	cfg.migrateLegacyChannel()
	return &cfg, nil
}

// migrateLegacyChannel folds the deprecated top-level channel_id and
// thread_identifier into Channels, then clears them so Save writes only the
// current shape.
func (c *Config) migrateLegacyChannel() {
	if c.ChannelID == "" && c.ThreadIdentifier == "" {
		return
	}
	if c.FindChannel(c.ChannelID) == nil {
		c.Channels = append(c.Channels, ChannelConfig{
			ChannelID:        c.ChannelID,
			ThreadIdentifier: c.ThreadIdentifier,
		})
	}
	c.ChannelID = ""
	c.ThreadIdentifier = ""
}

// FindChannel returns the channel matching the given ID or name, or nil.
func (c *Config) FindChannel(idOrName string) *ChannelConfig {
	if idOrName == "" {
		return nil
	}
	for i := range c.Channels {
		if c.Channels[i].ChannelID == idOrName || c.Channels[i].Name == idOrName {
			return &c.Channels[i]
		}
	}
	return nil
}

// Save writes the Config as YAML to the given path with 0600 permissions.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// Validate checks that all required fields are set.
func (c *Config) Validate() error {
	if c.SlackBotToken == "" {
		return fmt.Errorf("slack_bot_token is required")
	}
	if c.SlackAppToken == "" {
		return fmt.Errorf("slack_app_token is required")
	}
	if len(c.Channels) == 0 {
		return fmt.Errorf("at least one entry under channels is required")
	}
	if c.MaxMissedStandups != nil && *c.MaxMissedStandups < 0 {
		return fmt.Errorf("max_missed_standups must not be negative, got %d", *c.MaxMissedStandups)
	}
	// A typo here would otherwise disable reminders silently.
	if c.ReminderAfter != "" {
		if _, err := time.ParseDuration(c.ReminderAfter); err != nil {
			return fmt.Errorf("reminder_after %q is not a valid duration (e.g. \"4h\", \"90m\", or \"0\" to disable): %w", c.ReminderAfter, err)
		}
	}

	seenID := make(map[string]bool, len(c.Channels))
	seenName := make(map[string]bool, len(c.Channels))
	for i, ch := range c.Channels {
		if ch.ChannelID == "" {
			return fmt.Errorf("channels[%d]: channel_id is required", i)
		}
		if ch.ThreadIdentifier == "" {
			return fmt.Errorf("channels[%d]: thread_identifier is required", i)
		}
		// Duplicates would silently shadow each other in the bot's channel
		// lookup, so reject them at load time instead.
		if seenID[ch.ChannelID] {
			return fmt.Errorf("channels[%d]: duplicate channel_id %q", i, ch.ChannelID)
		}
		seenID[ch.ChannelID] = true
		if ch.Name != "" {
			if seenName[ch.Name] {
				return fmt.Errorf("channels[%d]: duplicate name %q", i, ch.Name)
			}
			seenName[ch.Name] = true
		}
	}
	return nil
}
