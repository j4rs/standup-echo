package standup

import (
	"fmt"
	"sort"
	"time"

	"github.com/slack-go/slack"
)

// ThreadTime converts a Slack thread timestamp to a time.Time. Exported so the
// bot can schedule work relative to a thread's own post time rather than a wall
// clock, which keeps the schedule free of timezone and DST handling.
func ThreadTime(ts string) time.Time {
	return tsToTime(ts)
}

// unpostedFrom returns the users present in roster but absent from posted: the
// set that still owes an update. Sorted so logs and tests are deterministic.
func unpostedFrom(roster, posted map[string]string) []string {
	var owing []string
	for userID := range roster {
		if _, done := posted[userID]; done {
			continue
		}
		owing = append(owing, userID)
	}
	sort.Strings(owing)
	return owing
}

func buildReminderText(threadLink string) string {
	text := "Nudge: you haven't posted in today's standup thread yet."
	if threadLink == "" {
		return text
	}
	return fmt.Sprintf("%s\n\n<%s|Open today's standup thread>", text, threadLink)
}

// RemindUnposted DMs everyone who posted in the standup thread immediately
// preceding threadTS but has not yet posted in threadTS itself.
//
// The roster deliberately comes from the *immediately* preceding thread only,
// not the wider grace window used by the morning echo. The echo's grace exists
// to recover a loop that went silent, whereas a reminder is a nag and should
// decay fast: someone away for a week is dropped from the roster after one
// standup instead of being pinged for days.
//
// If onlyUser is non-empty, only that user is reminded, which makes
// `trigger --reminder --user <id>` a safe rehearsal.
func (s *Service) RemindUnposted(threadTS, onlyUser string) {
	s.logger.Info("building reminder roster", "ts", threadTS, "only_user", onlyUser)

	prevTSs, err := s.FindPreviousStandupThreads(threadTS, 1)
	if err != nil {
		s.logger.Warn("no previous standup thread to build a reminder roster from", "error", err)
		return
	}

	roster, err := s.GetThreadReplies(prevTSs[0])
	if err != nil {
		s.logger.Error("failed to get previous thread replies", "error", err)
		return
	}
	posted, err := s.GetThreadReplies(threadTS)
	if err != nil {
		s.logger.Error("failed to get today's thread replies", "error", err)
		return
	}

	owing := unpostedFrom(roster, posted)
	if len(owing) == 0 {
		s.logger.Info("everyone on the roster has posted", "roster", len(roster))
		return
	}

	threadLink := s.getThreadPermalink(threadTS)
	text := buildReminderText(threadLink)

	var sent, skipped, failed int
	for _, userID := range owing {
		if onlyUser != "" && userID != onlyUser {
			continue
		}
		if !s.subscribers.IsSubscribed(userID) {
			s.logger.Debug("skipping non-subscribed user", "user", userID)
			skipped++
			continue
		}
		if _, _, err := s.client.PostMessage(userID, slack.MsgOptionText(text, false)); err != nil {
			s.logger.Error("failed to DM reminder", "user", userID, "error", err)
			failed++
			continue
		}
		s.logger.Info("sent reminder", "user", userID)
		sent++
	}

	s.logger.Info("finished sending reminders",
		"sent", sent, "skipped_not_subscribed", skipped, "failed", failed,
		"roster", len(roster), "already_posted", len(roster)-len(owing))
}
