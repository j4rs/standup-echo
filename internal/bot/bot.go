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

// Bot listens for new standup threads via Socket Mode and triggers DM delivery.
type Bot struct {
	client      *slack.Client
	handler     *socketmode.SocketmodeHandler
	standup     *standup.Service
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

	svc := standup.NewService(api, cfg.ChannelID, cfg.ThreadIdentifier, subscribers, logger)

	b := &Bot{
		client:      api,
		handler:     handler,
		standup:     svc,
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
	// Allow regular messages and bot_message (workflow posts).
	if ev.SubType != "" && ev.SubType != "bot_message" {
		return
	}
	if ev.Channel != b.config.ChannelID {
		return
	}
	if ev.ThreadTimeStamp != "" {
		return
	}
	if !strings.Contains(ev.Text, b.config.ThreadIdentifier) {
		return
	}

	b.logger.Info("detected new standup thread", "ts", ev.TimeStamp, "user", ev.User)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				b.logger.Error("panic in ProcessNewStandup", "recover", r)
			}
		}()
		b.standup.ProcessNewStandup(ev.TimeStamp)
	}()
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
