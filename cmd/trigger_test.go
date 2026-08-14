package cmd

import (
	"strings"
	"testing"

	"github.com/j4rs/standup-echo/internal/config"
)

func TestResolveChannel(t *testing.T) {
	single := &config.Config{Channels: []config.ChannelConfig{
		{Name: "m1", ChannelID: "C111", ThreadIdentifier: "Daily Standup"},
	}}
	multi := &config.Config{Channels: []config.ChannelConfig{
		{Name: "m1", ChannelID: "C111", ThreadIdentifier: "Daily Standup"},
		{Name: "m2", ChannelID: "C222", ThreadIdentifier: "async standup check-in"},
	}}

	t.Run("defaults to the only channel", func(t *testing.T) {
		got, err := resolveChannel(single, "")
		if err != nil {
			t.Fatal(err)
		}
		if got.ChannelID != "C111" {
			t.Errorf("got %s, want C111", got.ChannelID)
		}
	})

	t.Run("requires a flag when several are configured", func(t *testing.T) {
		_, err := resolveChannel(multi, "")
		if err == nil {
			t.Fatal("expected an error when --channel is omitted")
		}
		// The error should tell the user what they can pick from.
		if !strings.Contains(err.Error(), "m1") || !strings.Contains(err.Error(), "m2") {
			t.Errorf("error should list configured channels, got: %v", err)
		}
	})

	t.Run("resolves by name and by id", func(t *testing.T) {
		for _, want := range []string{"m2", "C222"} {
			got, err := resolveChannel(multi, want)
			if err != nil {
				t.Fatalf("resolveChannel(%q): %v", want, err)
			}
			if got.ChannelID != "C222" {
				t.Errorf("resolveChannel(%q) = %s, want C222", want, got.ChannelID)
			}
		}
	})

	t.Run("rejects an unknown channel", func(t *testing.T) {
		if _, err := resolveChannel(multi, "m3"); err == nil {
			t.Error("expected an error for an unconfigured channel")
		}
	})
}
