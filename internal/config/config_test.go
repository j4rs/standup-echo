package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testChannels() []ChannelConfig {
	return []ChannelConfig{{Name: "m1", ChannelID: "C123", ThreadIdentifier: "Daily Check"}}
}

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
		SlackBotToken: "xoxb-test",
		SlackAppToken: "xapp-test",
		Channels:      testChannels(),
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

	if !reflect.DeepEqual(loaded, original) {
		t.Errorf("Load() = %+v, want %+v", loaded, original)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.yml")

	cfg := &Config{
		SlackBotToken: "xoxb-test",
		SlackAppToken: "xapp-test",
		Channels:      testChannels(),
	}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

// Configs written before multi-channel support use top-level channel_id and
// thread_identifier. Loading one must keep working without a manual edit.
func TestLoadMigratesLegacySingleChannel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	legacy := []byte("slack_bot_token: xoxb-test\n" +
		"slack_app_token: xapp-test\n" +
		"channel_id: C111\n" +
		"thread_identifier: Daily Standup\n")
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("migrated config should be valid: %v", err)
	}

	want := []ChannelConfig{{ChannelID: "C111", ThreadIdentifier: "Daily Standup"}}
	if !reflect.DeepEqual(cfg.Channels, want) {
		t.Errorf("Channels = %+v, want %+v", cfg.Channels, want)
	}
	if cfg.ChannelID != "" || cfg.ThreadIdentifier != "" {
		t.Errorf("legacy fields should be cleared, got %q / %q", cfg.ChannelID, cfg.ThreadIdentifier)
	}

	// Re-saving must not resurrect the legacy keys.
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); strings.Contains(got, "channel_id: C111\n") && !strings.Contains(got, "channels:") {
		t.Errorf("saved config kept the legacy shape:\n%s", got)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.Channels, want) {
		t.Errorf("round-tripped Channels = %+v, want %+v", reloaded.Channels, want)
	}
}

// A legacy config that has already been migrated by hand must not end up with
// the same channel twice.
func TestLoadLegacyDoesNotDuplicateExistingChannel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	mixed := []byte("slack_bot_token: xoxb-test\n" +
		"slack_app_token: xapp-test\n" +
		"channel_id: C111\n" +
		"thread_identifier: Daily Standup\n" +
		"channels:\n" +
		"  - channel_id: C111\n" +
		"    thread_identifier: Daily Standup\n" +
		"  - channel_id: C222\n" +
		"    thread_identifier: async standup check-in\n")
	if err := os.WriteFile(path, mixed, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Channels) != 2 {
		t.Fatalf("got %d channels, want 2: %+v", len(cfg.Channels), cfg.Channels)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestFindChannel(t *testing.T) {
	cfg := &Config{Channels: []ChannelConfig{
		{Name: "m1", ChannelID: "C111", ThreadIdentifier: "Daily Standup"},
		{Name: "m2", ChannelID: "C222", ThreadIdentifier: "async standup check-in"},
	}}

	tests := []struct {
		query string
		want  string
	}{
		{"m2", "C222"},
		{"C111", "C111"},
		{"", ""},
		{"nope", ""},
	}
	for _, tt := range tests {
		got := cfg.FindChannel(tt.query)
		if tt.want == "" {
			if got != nil {
				t.Errorf("FindChannel(%q) = %+v, want nil", tt.query, got)
			}
			continue
		}
		if got == nil || got.ChannelID != tt.want {
			t.Errorf("FindChannel(%q) = %+v, want channel %s", tt.query, got, tt.want)
		}
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
		SlackBotToken: "xoxb-test",
		SlackAppToken: "xapp-test",
		Channels: []ChannelConfig{
			{Name: "m1", ChannelID: "C111", ThreadIdentifier: "Daily Standup"},
			{Name: "m2", ChannelID: "C222", ThreadIdentifier: "async standup check-in"},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() returned error for valid config: %v", err)
	}

	tests := []struct {
		name  string
		cfg   Config
		field string
	}{
		{"missing bot token", Config{SlackAppToken: "x", Channels: testChannels()}, "slack_bot_token"},
		{"missing app token", Config{SlackBotToken: "x", Channels: testChannels()}, "slack_app_token"},
		{"no channels", Config{SlackBotToken: "x", SlackAppToken: "x"}, "channels"},
		{
			"missing channel id",
			Config{SlackBotToken: "x", SlackAppToken: "x", Channels: []ChannelConfig{{ThreadIdentifier: "x"}}},
			"channel_id",
		},
		{
			"missing identifier",
			Config{SlackBotToken: "x", SlackAppToken: "x", Channels: []ChannelConfig{{ChannelID: "x"}}},
			"thread_identifier",
		},
		{
			"duplicate channel id",
			Config{SlackBotToken: "x", SlackAppToken: "x", Channels: []ChannelConfig{
				{ChannelID: "C111", ThreadIdentifier: "a"},
				{ChannelID: "C111", ThreadIdentifier: "b"},
			}},
			"duplicate channel_id",
		},
		{
			"duplicate name",
			Config{SlackBotToken: "x", SlackAppToken: "x", Channels: []ChannelConfig{
				{Name: "team", ChannelID: "C111", ThreadIdentifier: "a"},
				{Name: "team", ChannelID: "C222", ThreadIdentifier: "b"},
			}},
			"duplicate name",
		},
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

// An absent max_missed_standups must not read as zero grace, which would
// restore the bug where one missed standup silences the bot for good.
func TestMissedStandupGraceDefaultsWhenUnset(t *testing.T) {
	cfg := &Config{SlackBotToken: "x", SlackAppToken: "x", Channels: testChannels()}
	if got := cfg.MissedStandupGrace(); got != defaultMaxMissedStandups {
		t.Errorf("MissedStandupGrace() = %d, want the default %d", got, defaultMaxMissedStandups)
	}
	if defaultMaxMissedStandups < 1 {
		t.Errorf("defaultMaxMissedStandups = %d, want at least 1 so a missed day recovers", defaultMaxMissedStandups)
	}
}

// An explicit 0 is a deliberate opt-out of the grace window and must survive
// the round trip distinguishable from "unset".
func TestMissedStandupGraceHonoursExplicitZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	zero := 0
	original := &Config{
		SlackBotToken:     "xoxb-test",
		SlackAppToken:     "xapp-test",
		Channels:          testChannels(),
		MaxMissedStandups: &zero,
	}
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MaxMissedStandups == nil {
		t.Fatal("MaxMissedStandups = nil after round trip, want an explicit 0")
	}
	if got := loaded.MissedStandupGrace(); got != 0 {
		t.Errorf("MissedStandupGrace() = %d, want 0", got)
	}
}

func TestMissedStandupGraceHonoursExplicitValue(t *testing.T) {
	five := 5
	cfg := &Config{MaxMissedStandups: &five}
	if got := cfg.MissedStandupGrace(); got != 5 {
		t.Errorf("MissedStandupGrace() = %d, want 5", got)
	}
}

func TestValidateRejectsNegativeMaxMissedStandups(t *testing.T) {
	negative := -1
	cfg := &Config{
		SlackBotToken:     "x",
		SlackAppToken:     "x",
		Channels:          testChannels(),
		MaxMissedStandups: &negative,
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error for a negative allowance")
	}
	if !strings.Contains(err.Error(), "max_missed_standups") {
		t.Errorf("Validate() error = %q, want it to name max_missed_standups", err)
	}
}

// An unset reminder_after must enable reminders at the default delay, not
// silently disable the feature.
func TestReminderDelayDefaultsWhenUnset(t *testing.T) {
	cfg := &Config{SlackBotToken: "x", SlackAppToken: "x", Channels: testChannels()}
	got, enabled := cfg.ReminderDelay()
	if !enabled {
		t.Error("ReminderDelay() enabled = false, want true when unset")
	}
	if got != defaultReminderAfter {
		t.Errorf("ReminderDelay() = %v, want the default %v", got, defaultReminderAfter)
	}
}

func TestReminderDelayParsesAndDisables(t *testing.T) {
	tests := []struct {
		in          string
		want        time.Duration
		wantEnabled bool
	}{
		{"4h", 4 * time.Hour, true},
		{"90m", 90 * time.Minute, true},
		{"3h30m", 3*time.Hour + 30*time.Minute, true},
		{"0", 0, false},
		{"0s", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			cfg := &Config{ReminderAfter: tt.in}
			got, enabled := cfg.ReminderDelay()
			if enabled != tt.wantEnabled {
				t.Errorf("ReminderDelay(%q) enabled = %v, want %v", tt.in, enabled, tt.wantEnabled)
			}
			if enabled && got != tt.want {
				t.Errorf("ReminderDelay(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// A typo must fail loudly at load rather than quietly turning reminders off.
func TestValidateRejectsUnparseableReminderAfter(t *testing.T) {
	for _, bad := range []string{"4 hours", "later", "4", "-"} {
		cfg := &Config{
			SlackBotToken: "x",
			SlackAppToken: "x",
			Channels:      testChannels(),
			ReminderAfter: bad,
		}
		err := cfg.Validate()
		if err == nil {
			t.Errorf("Validate() with reminder_after %q = nil, want an error", bad)
			continue
		}
		if !strings.Contains(err.Error(), "reminder_after") {
			t.Errorf("Validate() error = %q, want it to name reminder_after", err)
		}
	}
}

func TestReminderAfterSurvivesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	original := &Config{
		SlackBotToken: "xoxb-test",
		SlackAppToken: "xapp-test",
		Channels:      testChannels(),
		ReminderAfter: "4h",
	}
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ReminderAfter != "4h" {
		t.Errorf("ReminderAfter = %q after round trip, want %q", loaded.ReminderAfter, "4h")
	}
}
