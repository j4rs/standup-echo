package bot

import (
	"testing"

	"github.com/j4rs/standup-echo/internal/config"
)

// Raw text as Slack delivers m2's workflow post: bold uses *…* markers and
// emoji arrive as :shortcodes:. Note that "Daily Standup" — the workflow's
// display name — appears nowhere in the body.
const m2Text = "Hey team, it's time for our *daily* async standup check-in! " +
	":wave: :smiley: :pencil2:\n\nPlease drop a quick *thread* :thread:\n" +
	"• _What you wrapped up *yesterday*_ :clock1:\n" +
	"• _What you're focusing on *today*_ :rocket:\n" +
	"• _Any *blockers* or *support* needed_ :no_entry_sign:"

const m1Text = "Check in thread for Fleet Maintenance"

// testBot builds a Bot with only the routing state populated. routeStandup must
// not touch the Slack client, so the per-channel services stay nil.
func testBot() *Bot {
	return &Bot{watchers: map[string]*watcher{
		"C0AK1NJRN3Z": {cfg: config.ChannelConfig{
			Name: "m1", ChannelID: "C0AK1NJRN3Z",
			ThreadIdentifier: "Check in thread for Fleet Maintenance",
		}},
		"C0AM05GS9GU": {cfg: config.ChannelConfig{
			Name: "m2", ChannelID: "C0AM05GS9GU",
			ThreadIdentifier: "async standup check-in",
		}},
	}}
}

func TestRouteStandup(t *testing.T) {
	b := testBot()

	tests := []struct {
		name      string
		channelID string
		subType   string
		threadTS  string
		text      string
		want      string // expected channel name, "" for no match
	}{
		{
			name:      "m1 post routes to m1",
			channelID: "C0AK1NJRN3Z",
			text:      m1Text,
			want:      "m1",
		},
		{
			name:      "m2 workflow post routes to m2",
			channelID: "C0AM05GS9GU",
			subType:   "bot_message",
			text:      m2Text,
			want:      "m2",
		},
		{
			name:      "m2 identifier survives surrounding bold markup",
			channelID: "C0AM05GS9GU",
			subType:   "bot_message",
			text:      "it's time for our *daily* async standup check-in!",
			want:      "m2",
		},
		// The whole point of per-channel identifiers: each channel matches only
		// its own wording, so one team's phrasing can't fire in another's channel.
		{
			name:      "m1 wording in m2's channel does not match",
			channelID: "C0AM05GS9GU",
			text:      m1Text,
			want:      "",
		},
		{
			name:      "m2 wording in m1's channel does not match",
			channelID: "C0AK1NJRN3Z",
			subType:   "bot_message",
			text:      m2Text,
			want:      "",
		},
		// The workflow's display name is not in the message body, so configuring
		// it as the identifier must not match — this is the trap for m2.
		{
			name:      "workflow display name is not matched",
			channelID: "C0AM05GS9GU",
			subType:   "bot_message",
			text:      "Daily Standup",
			want:      "",
		},
		{
			name:      "unwatched channel is ignored",
			channelID: "C0UNWATCHED",
			text:      m1Text,
			want:      "",
		},
		{
			name:      "thread reply is not a new standup post",
			channelID: "C0AM05GS9GU",
			subType:   "bot_message",
			threadTS:  "1786713302.936859",
			text:      m2Text,
			want:      "",
		},
		{
			name:      "unrelated subtypes are ignored",
			channelID: "C0AM05GS9GU",
			subType:   "channel_join",
			text:      m2Text,
			want:      "",
		},
		{
			name:      "message_changed is ignored",
			channelID: "C0AM05GS9GU",
			subType:   "message_changed",
			text:      m2Text,
			want:      "",
		},
		{
			name:      "chatter in a watched channel is ignored",
			channelID: "C0AM05GS9GU",
			text:      "anyone seen the deploy go through?",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.routeStandup(tt.channelID, tt.subType, tt.threadTS, tt.text)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("routeStandup routed to %q, want no match", got.cfg.Label())
				}
				return
			}
			if got == nil {
				t.Fatalf("routeStandup returned no match, want %q", tt.want)
			}
			if got.cfg.Label() != tt.want {
				t.Errorf("routeStandup routed to %q, want %q", got.cfg.Label(), tt.want)
			}
		})
	}
}

// A single-channel config must behave exactly as it did before multi-channel
// support, since that is what deployed instances load after migration.
func TestRouteStandupSingleChannel(t *testing.T) {
	b := &Bot{watchers: map[string]*watcher{
		"C0AK1NJRN3Z": {cfg: config.ChannelConfig{
			ChannelID:        "C0AK1NJRN3Z",
			ThreadIdentifier: "Check in thread for Fleet Maintenance",
		}},
	}}

	if got := b.routeStandup("C0AK1NJRN3Z", "", "", m1Text); got == nil {
		t.Error("single-channel config should still match its own standup post")
	}
	if got := b.routeStandup("C0AM05GS9GU", "bot_message", "", m2Text); got != nil {
		t.Errorf("unconfigured channel matched %q", got.cfg.Label())
	}
}
