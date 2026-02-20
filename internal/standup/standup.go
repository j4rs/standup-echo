package standup

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/j4rs/standup-echo/internal/store"
	"github.com/slack-go/slack"
)

// Service handles standup thread detection, reply retrieval, and DM sending.
type Service struct {
	client           *slack.Client
	channelID        string
	threadIdentifier string
	subscribers      *store.Subscribers
	logger           *slog.Logger
}

// NewService creates a new standup Service.
func NewService(client *slack.Client, channelID, threadIdentifier string, subscribers *store.Subscribers, logger *slog.Logger) *Service {
	return &Service{
		client:           client,
		channelID:        channelID,
		threadIdentifier: threadIdentifier,
		subscribers:      subscribers,
		logger:           logger,
	}
}

// FindPreviousStandupThread scans channel history for the most recent parent
// message matching the thread identifier, excluding the message at currentTS.
func (s *Service) FindPreviousStandupThread(currentTS string) (string, error) {
	cursor := ""
	for {
		params := &slack.GetConversationHistoryParameters{
			ChannelID: s.channelID,
			Limit:     100,
			Cursor:    cursor,
		}
		history, err := s.client.GetConversationHistory(params)
		if err != nil {
			return "", fmt.Errorf("fetching channel history: %w", err)
		}

		for _, msg := range history.Messages {
			if msg.Timestamp == currentTS {
				continue
			}
			if strings.Contains(msg.Text, s.threadIdentifier) {
				s.logger.Info("found previous standup thread", "ts", msg.Timestamp)
				return msg.Timestamp, nil
			}
		}

		if !history.HasMore {
			break
		}
		cursor = history.ResponseMetaData.NextCursor
	}

	return "", fmt.Errorf("no previous standup thread found")
}

// GetThreadReplies fetches all replies for a thread, deduplicates by user
// (keeping the latest reply), and skips the parent message.
func (s *Service) GetThreadReplies(threadTS string) (map[string]string, error) {
	params := &slack.GetConversationRepliesParameters{
		ChannelID: s.channelID,
		Timestamp: threadTS,
	}
	msgs, _, _, err := s.client.GetConversationReplies(params)
	if err != nil {
		return nil, fmt.Errorf("fetching thread replies: %w", err)
	}

	replies := make(map[string]string)
	for _, msg := range msgs {
		// Skip the parent message.
		if msg.Timestamp == threadTS {
			continue
		}
		// Keep the latest reply per user (messages come in chronological order).
		replies[msg.User] = msg.Text
	}

	s.logger.Info("collected thread replies", "thread_ts", threadTS, "users", len(replies))
	return replies, nil
}

// SendDMs sends each user their previous standup reply via direct message.
func (s *Service) SendDMs(threadDate time.Time, replies map[string]string) error {
	dateStr := threadDate.Format("Monday, January 2")

	for userID, reply := range replies {
		if !s.subscribers.IsSubscribed(userID) {
			s.logger.Debug("skipping non-subscribed user", "user", userID)
			continue
		}
		today := time.Now().Format("Monday, January 2")
		text := fmt.Sprintf(
			"📋 %s\n---\n%s\n\n✏️ %s\n---\n",
			dateStr, reply, today,
		)
		_, _, err := s.client.PostMessage(userID, slack.MsgOptionText(text, false))
		if err != nil {
			s.logger.Error("failed to DM user", "user", userID, "error", err)
			continue
		}
		s.logger.Info("sent DM", "user", userID)
	}
	return nil
}

// FindStandupThread scans channel history for a standup thread matching the
// thread identifier. If date is non-zero, it finds the thread posted on that
// date. Otherwise it returns the most recent one.
func (s *Service) FindStandupThread(date time.Time) (string, error) {
	filterByDate := !date.IsZero()
	cursor := ""
	for {
		params := &slack.GetConversationHistoryParameters{
			ChannelID: s.channelID,
			Limit:     100,
			Cursor:    cursor,
		}
		history, err := s.client.GetConversationHistory(params)
		if err != nil {
			return "", fmt.Errorf("fetching channel history: %w", err)
		}

		for _, msg := range history.Messages {
			if !strings.Contains(msg.Text, s.threadIdentifier) {
				continue
			}
			if filterByDate {
				msgDate := tsToTime(msg.Timestamp)
				y1, m1, d1 := msgDate.Date()
				y2, m2, d2 := date.Date()
				if y1 != y2 || m1 != m2 || d1 != d2 {
					continue
				}
			}
			s.logger.Info("found standup thread", "ts", msg.Timestamp)
			return msg.Timestamp, nil
		}

		if !history.HasMore {
			break
		}
		cursor = history.ResponseMetaData.NextCursor
	}

	return "", fmt.Errorf("no standup thread found")
}

// ProcessLatestStandup finds a standup thread, gets replies, and sends DMs.
// If onlyUser is non-empty, only that user will receive a DM.
// If date is non-zero, it finds the thread for that specific date.
func (s *Service) ProcessLatestStandup(onlyUser string, date time.Time) {
	s.logger.Info("processing standup thread", "only_user", onlyUser, "date", date)

	threadTS, err := s.FindStandupThread(date)
	if err != nil {
		s.logger.Error("no standup thread found", "error", err)
		return
	}

	replies, err := s.GetThreadReplies(threadTS)
	if err != nil {
		s.logger.Error("failed to get thread replies", "error", err)
		return
	}

	if onlyUser != "" {
		if reply, ok := replies[onlyUser]; ok {
			replies = map[string]string{onlyUser: reply}
		} else {
			s.logger.Info("user has no reply in latest thread", "user", onlyUser)
			return
		}
	}
	if len(replies) == 0 {
		s.logger.Info("no replies in standup thread")
		return
	}

	threadDate := tsToTime(threadTS)
	if err := s.SendDMs(threadDate, replies); err != nil {
		s.logger.Error("failed to send DMs", "error", err)
	}
}

// ProcessNewStandup is the orchestrator: find previous thread, get replies, send DMs.
func (s *Service) ProcessNewStandup(newThreadTS string) {
	s.logger.Info("processing new standup thread", "ts", newThreadTS)

	prevTS, err := s.FindPreviousStandupThread(newThreadTS)
	if err != nil {
		s.logger.Warn("no previous standup thread found", "error", err)
		return
	}

	replies, err := s.GetThreadReplies(prevTS)
	if err != nil {
		s.logger.Error("failed to get thread replies", "error", err)
		return
	}
	if len(replies) == 0 {
		s.logger.Info("no replies in previous standup thread")
		return
	}

	threadDate := tsToTime(prevTS)
	if err := s.SendDMs(threadDate, replies); err != nil {
		s.logger.Error("failed to send DMs", "error", err)
	}
}

// tsToTime converts a Slack timestamp (e.g. "1708300000.000000") to time.Time.
func tsToTime(ts string) time.Time {
	parts := strings.SplitN(ts, ".", 2)
	if len(parts) == 0 {
		return time.Time{}
	}
	var sec int64
	for _, c := range parts[0] {
		sec = sec*10 + int64(c-'0')
	}
	return time.Unix(sec, 0)
}
