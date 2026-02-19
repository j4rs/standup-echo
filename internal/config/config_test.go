package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "standup-echo", "config.yml")
	if path != want {
		t.Errorf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	original := &Config{
		SlackBotToken:    "xoxb-test",
		SlackAppToken:    "xapp-test",
		ChannelID:        "C123",
		ThreadIdentifier: "Daily Check",
	}

	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}

	// Verify file permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if *loaded != *original {
		t.Errorf("Load() = %+v, want %+v", loaded, original)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.yml")

	cfg := &Config{
		SlackBotToken:    "xoxb-test",
		SlackAppToken:    "xapp-test",
		ChannelID:        "C123",
		ThreadIdentifier: "Daily Check",
	}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestValidate(t *testing.T) {
	valid := &Config{
		SlackBotToken:    "xoxb-test",
		SlackAppToken:    "xapp-test",
		ChannelID:        "C123",
		ThreadIdentifier: "Daily Check",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() returned error for valid config: %v", err)
	}

	tests := []struct {
		name  string
		cfg   Config
		field string
	}{
		{"missing bot token", Config{SlackAppToken: "x", ChannelID: "x", ThreadIdentifier: "x"}, "slack_bot_token"},
		{"missing app token", Config{SlackBotToken: "x", ChannelID: "x", ThreadIdentifier: "x"}, "slack_app_token"},
		{"missing channel", Config{SlackBotToken: "x", SlackAppToken: "x", ThreadIdentifier: "x"}, "channel_id"},
		{"missing identifier", Config{SlackBotToken: "x", SlackAppToken: "x", ChannelID: "x"}, "thread_identifier"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Errorf("Validate() should return error when %s is missing", tt.field)
			}
		})
	}
}
