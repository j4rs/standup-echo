package bot

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/j4rs/standup-echo/internal/config"
	"github.com/j4rs/standup-echo/internal/standup"
	"github.com/j4rs/standup-echo/internal/store"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// watcher pairs a configured channel with the standup service scoped to it.
type watcher struct {
	cfg     config.ChannelConfig
	service *standup.Service
}

// Bot listens for new standup threads via Socket Mode and triggers DM delivery.
// One Bot serves every configured channel, so teams with different channels and
// different standup wording share a single process and Slack app.
type Bot struct {
	client      *slack.Client
	handler     *socketmode.SocketmodeHandler
	watchers    map[string]*watcher
	subscribers *store.Subscribers
	config      *config.Config
	logger      *slog.Logger
	botUserID   string
}

// New creates a Bot from the given config, subscriber store, and logger.
func New(cfg *config.Config, subscribers *store.Subscribers, logger *slog.Logger) (*Bot, error) {
	api := slack.New(
		cfg.SlackBotToken,
		slack.OptionAppLevelToken(cfg.SlackAppToken),
	)
	sm := socketmode.New(api)
	handler := socketmode.NewSocketmodeHandler(sm)

	watchers := make(map[string]*watcher, len(cfg.Channels))
	for _, ch := range cfg.Channels {
		watchers[ch.ChannelID] = &watcher{
			cfg: ch,
			service: standup.NewService(
				api, ch.ChannelID, ch.ThreadIdentifier, subscribers,
				logger.With("channel", ch.Label()),
			),
		}
	}

	b := &Bot{
		client:      api,
		handler:     handler,
		watchers:    watchers,
		subscribers: subscribers,
		config:      cfg,
		logger:      logger,
	}

	handler.HandleEvents(slackevents.Message, b.handleMessageEvent)

	return b, nil
}

// Run starts the Socket Mode event loop. It blocks until the connection is closed.
func (b *Bot) Run() error {
	auth, err := b.client.AuthTest()
	if err != nil {
		return fmt.Errorf("auth test failed: %w", err)
	}
	b.botUserID = auth.UserID
	b.logger.Info("authenticated", "bot_user_id", b.botUserID)

	for _, w := range b.watchers {
		b.logger.Info("watching channel",
			"channel", w.cfg.Label(),
			"channel_id", w.cfg.ChannelID,
			"thread_identifier", w.cfg.ThreadIdentifier)
	}

	b.logger.Info("starting socket mode connection")
	if err := b.handler.RunEventLoop(); err != nil {
		return fmt.Errorf("socket mode error: %w", err)
	}
	return nil
}

func (b *Bot) handleMessageEvent(evt *socketmode.Event, client *socketmode.Client) {
	client.Ack(*evt.Request)

	eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}
	ev, ok := eventsAPI.InnerEvent.Data.(*slackevents.MessageEvent)
	if !ok {
		return
	}

	b.logger.Debug("received message", "channel", ev.Channel, "channel_type", ev.ChannelType, "user", ev.User, "subtype", ev.SubType)

	// Ignore our own messages.
	if ev.User == b.botUserID {
		return
	}

	// DM messages: handle subscribe/unsubscribe commands (regular messages only).
	if ev.ChannelType == "im" {
		if ev.SubType != "" {
			return
		}
		b.handleDMCommand(ev)
		return
	}

	// Channel messages: detect new standup threads.
	w := b.routeStandup(ev.Channel, ev.SubType, ev.ThreadTimeStamp, ev.Text)
	if w == nil {
		return
	}

	b.logger.Info("detected new standup thread", "channel", w.cfg.Label(), "ts", ev.TimeStamp, "user", ev.User)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				b.logger.Error("panic in ProcessNewStandup", "channel", w.cfg.Label(), "recover", r)
			}
		}()
		w.service.ProcessNewStandup(ev.TimeStamp)
	}()
}

// routeStandup decides which watcher, if any, should process a channel message.
// It returns nil when the message is not a new standup post in a watched
// channel. Kept free of Slack client calls so the routing rules are testable.
func (b *Bot) routeStandup(channelID, subType, threadTS, text string) *watcher {
	// Allow regular messages and bot_message, which is how a Slack Workflow
	// posts a scheduled standup.
	if subType != "" && subType != "bot_message" {
		return nil
	}
	w, watched := b.watchers[channelID]
	if !watched {
		return nil
	}
	// Thread replies are not new standup posts.
	if threadTS != "" {
		return nil
	}
	// Each channel matches its own wording, against the message body — a
	// workflow's display name never appears in the text.
	if !strings.Contains(text, w.cfg.ThreadIdentifier) {
		return nil
	}
	return w
}

func (b *Bot) handleDMCommand(ev *slackevents.MessageEvent) {
	text := strings.TrimSpace(strings.ToLower(ev.Text))

	switch text {
	case "subscribe":
		added, err := b.subscribers.Add(ev.User)
		if err != nil {
			b.logger.Error("failed to add subscriber", "user", ev.User, "error", err)
			b.reply(ev.Channel, "Something went wrong. Please try again.")
			return
		}
		if added {
			b.reply(ev.Channel, "You're subscribed! I'll DM you your previous standup update when a new thread appears.\n\nSend `unsubscribe` to stop.")
		} else {
			b.reply(ev.Channel, "You're already subscribed. Send `unsubscribe` to stop.")
		}

	case "unsubscribe":
		removed, err := b.subscribers.Remove(ev.User)
		if err != nil {
			b.logger.Error("failed to remove subscriber", "user", ev.User, "error", err)
			b.reply(ev.Channel, "Something went wrong. Please try again.")
			return
		}
		if removed {
			b.reply(ev.Channel, "You've been unsubscribed. Send `subscribe` to opt back in.")
		} else {
			b.reply(ev.Channel, "You're not currently subscribed. Send `subscribe` to opt in.")
		}

	default:
		b.reply(ev.Channel, "Available commands:\n• `subscribe` — get DMs with your previous standup update\n• `unsubscribe` — stop getting DMs")
	}
}

func (b *Bot) reply(channel, text string) {
	_, _, err := b.client.PostMessage(channel, slack.MsgOptionText(text, false))
	if err != nil {
		b.logger.Error("failed to send reply", "channel", channel, "error", err)
	}
}
