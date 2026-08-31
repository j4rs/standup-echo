package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Reminders records which standup threads have already had mid-day reminders
// sent, so that a restart or a redeploy part-way through the day cannot nag
// people twice for the same thread.
//
// It shares the Subscribers connection rather than opening a second handle on
// the same SQLite file, and lives in its own table so the subscribers table —
// whose primary key a per-channel opt-out would have to rebuild — is untouched.
type Reminders struct {
	db *sql.DB
}

// NewReminders creates the reminders table on the Subscribers connection.
func NewReminders(s *Subscribers) (*Reminders, error) {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS reminders (
		channel_id TEXT NOT NULL,
		thread_ts  TEXT NOT NULL,
		sent_at    TEXT NOT NULL,
		PRIMARY KEY (channel_id, thread_ts)
	)`)
	if err != nil {
		return nil, fmt.Errorf("creating reminders table: %w", err)
	}
	return &Reminders{db: s.db}, nil
}

// Claim marks a channel's thread as reminded, returning true if this call was
// the one that claimed it. A false return means an earlier run already sent the
// reminder and this one must stay quiet.
//
// The insert is the claim, so two schedulers racing the same thread cannot both
// win. Rows accumulate at roughly one per channel per weekday, which is small
// enough not to need pruning.
func (r *Reminders) Claim(channelID, threadTS string) (bool, error) {
	res, err := r.db.Exec(
		`INSERT OR IGNORE INTO reminders (channel_id, thread_ts, sent_at) VALUES (?, ?, ?)`,
		channelID, threadTS, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return false, fmt.Errorf("claiming reminder: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
