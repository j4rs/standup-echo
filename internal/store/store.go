package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Subscribers manages opted-in user IDs, persisted in a SQLite database.
type Subscribers struct {
	db *sql.DB
}

// NewSubscribers opens (or creates) the SQLite database at the given path.
func NewSubscribers(path string) (*Subscribers, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating store directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS subscribers (
		user_id TEXT PRIMARY KEY
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("creating table: %w", err)
	}
	return &Subscribers{db: db}, nil
}

// Close closes the database connection.
func (s *Subscribers) Close() error {
	return s.db.Close()
}

// Add subscribes a user. Returns true if they were newly added.
func (s *Subscribers) Add(userID string) (bool, error) {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO subscribers (user_id) VALUES (?)`, userID)
	if err != nil {
		return false, fmt.Errorf("adding subscriber: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Remove unsubscribes a user. Returns true if they were previously subscribed.
func (s *Subscribers) Remove(userID string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM subscribers WHERE user_id = ?`, userID)
	if err != nil {
		return false, fmt.Errorf("removing subscriber: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// IsSubscribed returns whether a user is opted in.
func (s *Subscribers) IsSubscribed(userID string) bool {
	var exists bool
	err := s.db.QueryRow(`SELECT 1 FROM subscribers WHERE user_id = ?`, userID).Scan(&exists)
	return err == nil && exists
}
