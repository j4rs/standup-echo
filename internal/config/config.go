package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	SlackBotToken    string `yaml:"slack_bot_token"`
	SlackAppToken    string `yaml:"slack_app_token"`
	ChannelID        string `yaml:"channel_id"`
	ThreadIdentifier string `yaml:"thread_identifier"`
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
	return &cfg, nil
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
	if c.ChannelID == "" {
		return fmt.Errorf("channel_id is required")
	}
	if c.ThreadIdentifier == "" {
		return fmt.Errorf("thread_identifier is required")
	}
	return nil
}
