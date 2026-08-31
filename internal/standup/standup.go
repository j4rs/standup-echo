package standup

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
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
	maxMissed        int
	subscribers      *store.Subscribers
	logger           *slog.Logger
}

// NewService creates a new standup Service. maxMissed is how many consecutive
// standups a subscriber may miss and still be nudged; see lookbackThreads.
func NewService(client *slack.Client, channelID, threadIdentifier string, maxMissed int, subscribers *store.Subscribers, logger *slog.Logger) *Service {
	return &Service{
		client:           client,
		channelID:        channelID,
		threadIdentifier: threadIdentifier,
		maxMissed:        maxMissed,
		subscribers:      subscribers,
		logger:           logger,
	}
}

// lookbackThreads converts a missed-standup allowance into the number of
// preceding threads to scan: the one just before this standup, plus one more
// for each standup a subscriber is allowed to have missed.
//
// The allowance exists because the DM is what prompts the next update, so
// keying delivery solely off the immediately-preceding thread makes a single
// missed day self-perpetuating: no reply Wednesday means no nudge Thursday,
// which means no reply Thursday. Scanning back further breaks that loop while
// still letting a genuine absence fall out of the window and go quiet.
func lookbackThreads(maxMissed int) int {
	if maxMissed < 1 {
		return 1
	}
	return maxMissed + 1
}

// historyLookback bounds how far back a history scan will page. Without a
// bound, a misconfigured thread identifier walks a channel's entire history and
// burns through the conversations.history rate limit before failing.
const historyLookback = 21 * 24 * time.Hour

// previousThreadWindow bounds a search for the standup thread preceding
// currentTS: strictly older than currentTS, and no further back than
// historyLookback. Bounding the upper end matters when currentTS is not the
// newest thread in the channel — otherwise the scan would return a *newer*
// thread as the "previous" one.
func previousThreadWindow(currentTS string, now time.Time) (oldest, latest string) {
	ref := tsToTime(currentTS)
	if ref.Unix() <= 0 {
		// Unparseable timestamp: fall back to a window ending now.
		ref = now
	}
	return timeToTS(ref.Add(-historyLookback)), currentTS
}

// FindPreviousStandupThreads scans channel history for up to limit parent
// messages matching the thread identifier that are older than currentTS,
// returned newest first. It reports an error only when the channel yields no
// preceding thread at all; finding fewer than limit is normal in a young
// channel or after a holiday break.
func (s *Service) FindPreviousStandupThreads(currentTS string, limit int) ([]string, error) {
	if limit < 1 {
		limit = 1
	}
	oldest, latest := previousThreadWindow(currentTS, time.Now())
	params := &slack.GetConversationHistoryParameters{
		ChannelID: s.channelID,
		Limit:     100,
		Oldest:    oldest,
		// Slack treats latest as exclusive unless Inclusive is set, so this
		// drops currentTS itself as well as anything newer.
		Latest: latest,
	}

	// conversations.history returns newest first, so appending in scan order
	// keeps found[0] as the immediately-preceding thread.
	var found []string
	for len(found) < limit {
		history, err := s.client.GetConversationHistory(params)
		if err != nil {
			return nil, fmt.Errorf("fetching channel history: %w", err)
		}

		for _, msg := range history.Messages {
			if msg.Timestamp == currentTS {
				continue
			}
			if !strings.Contains(msg.Text, s.threadIdentifier) {
				continue
			}
			found = append(found, msg.Timestamp)
			if len(found) == limit {
				break
			}
		}

		if !history.HasMore {
			break
		}
		params.Cursor = history.ResponseMetaData.NextCursor
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("no previous standup thread found in the last %d days", int(historyLookback.Hours()/24))
	}
	s.logger.Info("found previous standup threads", "count", len(found), "newest", found[0])
	return found, nil
}

// collectRecentReplies unions the replies across threadTSs, which must be
// ordered newest first, keeping each user's most recent update. A user who
// replied two standups ago is therefore still echoed their own last words
// rather than an empty prompt.
func (s *Service) collectRecentReplies(threadTSs []string) (map[string]string, error) {
	perThread := make([]map[string]string, 0, len(threadTSs))
	for i, ts := range threadTSs {
		replies, err := s.GetThreadReplies(ts)
		if err != nil {
			// The immediately-preceding thread is the one that matters; losing
			// it is a real failure. Older threads only widen the grace window,
			// so a failure there degrades the nudge rather than blocking it.
			if i == 0 {
				return nil, err
			}
			s.logger.Warn("skipping older standup thread", "thread_ts", ts, "error", err)
			continue
		}
		perThread = append(perThread, replies)
	}
	return mergeReplies(perThread), nil
}

// mergeReplies folds per-thread reply maps, newest first, so the first entry
// seen for a user wins.
func mergeReplies(perThread []map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, replies := range perThread {
		for user, reply := range replies {
			if _, seen := merged[user]; !seen {
				merged[user] = reply
			}
		}
	}
	return merged
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

// SendDMs sends each user their previous standup reply via direct message,
// skipping anyone who has not opted in. The per-run summary is logged at info
// so a run that delivers nothing says why rather than looking like a failure.
func (s *Service) SendDMs(replies map[string]string, newThreadTS string) {
	today := time.Now().Format("Monday, January 2")
	newThreadLink := s.getThreadPermalink(newThreadTS)

	var sent, skipped, failed int
	for userID, reply := range replies {
		if !s.subscribers.IsSubscribed(userID) {
			s.logger.Debug("skipping non-subscribed user", "user", userID)
			skipped++
			continue
		}
		text := buildDMText(today, reply, newThreadLink)
		_, _, err := s.client.PostMessage(userID, slack.MsgOptionText(text, false))
		if err != nil {
			s.logger.Error("failed to DM user", "user", userID, "error", err)
			failed++
			continue
		}
		s.logger.Info("sent DM", "user", userID)
		sent++
	}

	s.logger.Info("finished sending DMs",
		"sent", sent, "skipped_not_subscribed", skipped, "failed", failed)
}

func (s *Service) getThreadPermalink(threadTS string) string {
	if threadTS == "" {
		return ""
	}

	link, err := s.client.GetPermalink(&slack.PermalinkParameters{
		Channel: s.channelID,
		Ts:      threadTS,
	})
	if err != nil {
		s.logger.Warn("failed to get thread permalink", "thread_ts", threadTS, "error", err)
		return ""
	}

	return link
}

func buildDMText(todayDate, reply, threadLink string) string {
	trimmedReply := strings.TrimSpace(reply)
	formattedReply := boldDateLines(trimmedReply)
	todayPrompt := fmt.Sprintf("*%s*\nWhat are you up to today?", todayDate)

	text := todayPrompt
	if formattedReply != "" {
		text = fmt.Sprintf("%s\n\n%s", formattedReply, todayPrompt)
	}

	if threadLink == "" {
		return text
	}

	return fmt.Sprintf("%s\n\n<%s|Open today's standup thread>", text, threadLink)
}

var dateLineRegex = regexp.MustCompile(`^(Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday), (January|February|March|April|May|June|July|August|September|October|November|December) [0-9]{1,2}$`)

func boldDateLines(text string) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if isBoldDateLine(line) {
			continue
		}
		if dateLineRegex.MatchString(line) {
			lines[i] = fmt.Sprintf("*%s*", line)
		}
	}

	return strings.Join(lines, "\n")
}

func isBoldDateLine(line string) bool {
	if !strings.HasPrefix(line, "*") || !strings.HasSuffix(line, "*") {
		return false
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(line, "*"), "*")
	return dateLineRegex.MatchString(inner)
}

// FindStandupThread scans channel history for a standup thread matching the
// thread identifier. If date is non-zero, it finds the thread posted on that
// date. Otherwise it returns the most recent one.
func (s *Service) FindStandupThread(date time.Time) (string, error) {
	filterByDate := !date.IsZero()

	params := &slack.GetConversationHistoryParameters{
		ChannelID: s.channelID,
		Limit:     100,
	}
	if filterByDate {
		// Bound the scan to the requested day so we don't page the whole channel.
		start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		params.Oldest = timeToTS(start)
		params.Latest = timeToTS(start.AddDate(0, 0, 1))
	} else {
		params.Oldest = timeToTS(time.Now().Add(-historyLookback))
	}

	for {
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
		params.Cursor = history.ResponseMetaData.NextCursor
	}

	if filterByDate {
		return "", fmt.Errorf("no standup thread found on %s", date.Format("2006-01-02"))
	}
	return "", fmt.Errorf("no standup thread found in the last %d days", int(historyLookback.Hours()/24))
}

// ProcessLatestStandup rehearses the live flow against an existing thread: it
// locates a standup thread, then echoes the *preceding* thread's replies into
// it, exactly as ProcessNewStandup does when a thread is posted for real.
// If onlyUser is non-empty, only that user receives a DM.
// If date is non-zero, it targets the thread posted on that date.
func (s *Service) ProcessLatestStandup(onlyUser string, date time.Time) {
	s.logger.Info("processing standup thread", "only_user", onlyUser, "date", date)

	threadTS, err := s.FindStandupThread(date)
	if err != nil {
		s.logger.Error("no standup thread found", "error", err)
		return
	}

	s.echoPreviousInto(threadTS, onlyUser)
}

// ProcessNewStandup is the orchestrator: find previous thread, get replies, send DMs.
func (s *Service) ProcessNewStandup(newThreadTS string) {
	s.logger.Info("processing new standup thread", "ts", newThreadTS)
	s.echoPreviousInto(newThreadTS, "")
}

// echoPreviousInto DMs each subscriber their most recent update drawn from the
// standup threads preceding newThreadTS, linking newThreadTS. It looks back
// past the immediately-preceding thread by the configured grace allowance, so
// missing one standup does not silence the nudge that prompts the next one.
// When onlyUser is non-empty, delivery is limited to that user. Both the live
// path and the manual trigger go through here so a trigger run is a faithful
// rehearsal rather than an approximation.
func (s *Service) echoPreviousInto(newThreadTS, onlyUser string) {
	prevTSs, err := s.FindPreviousStandupThreads(newThreadTS, lookbackThreads(s.maxMissed))
	if err != nil {
		s.logger.Warn("no previous standup thread found", "error", err)
		return
	}

	replies, err := s.collectRecentReplies(prevTSs)
	if err != nil {
		s.logger.Error("failed to get thread replies", "error", err)
		return
	}

	if onlyUser != "" {
		reply, ok := replies[onlyUser]
		if !ok {
			s.logger.Info("user has no reply in the recent standup threads", "user", onlyUser, "threads", len(prevTSs))
			return
		}
		replies = map[string]string{onlyUser: reply}
	}
	if len(replies) == 0 {
		s.logger.Info("no replies in the recent standup threads", "threads", len(prevTSs))
		return
	}

	s.SendDMs(replies, newThreadTS)
}

// tsToTime converts a Slack timestamp (e.g. "1708300000.000000") to time.Time.
// A malformed timestamp yields the Unix epoch rather than a garbage time.
func tsToTime(ts string) time.Time {
	secPart, _, _ := strings.Cut(ts, ".")
	sec, err := strconv.ParseInt(secPart, 10, 64)
	if err != nil {
		return time.Unix(0, 0)
	}
	return time.Unix(sec, 0)
}

// timeToTS converts a time.Time to a Slack timestamp string, for use as an
// oldest/latest bound on history queries.
func timeToTS(t time.Time) string {
	return fmt.Sprintf("%d.000000", t.Unix())
}
