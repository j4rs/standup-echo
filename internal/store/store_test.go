package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Subscribers {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSubscribers(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAddAndIsSubscribed(t *testing.T) {
	s := newTestStore(t)

	if s.IsSubscribed("U1") {
		t.Error("new user should not be subscribed")
	}

	added, err := s.Add("U1")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Error("Add() should return true for new user")
	}
	if !s.IsSubscribed("U1") {
		t.Error("user should be subscribed after Add()")
	}
}

func TestAddDuplicate(t *testing.T) {
	s := newTestStore(t)

	s.Add("U1")
	added, err := s.Add("U1")
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Error("Add() should return false for duplicate user")
	}
}

func TestRemove(t *testing.T) {
	s := newTestStore(t)

	s.Add("U1")
	removed, err := s.Remove("U1")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("Remove() should return true for existing user")
	}
	if s.IsSubscribed("U1") {
		t.Error("user should not be subscribed after Remove()")
	}
}

func TestRemoveNonexistent(t *testing.T) {
	s := newTestStore(t)

	removed, err := s.Remove("U999")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("Remove() should return false for nonexistent user")
	}
}

func TestMultipleUsers(t *testing.T) {
	s := newTestStore(t)

	s.Add("U1")
	s.Add("U2")
	s.Add("U3")

	if !s.IsSubscribed("U1") || !s.IsSubscribed("U2") || !s.IsSubscribed("U3") {
		t.Error("all added users should be subscribed")
	}

	s.Remove("U2")

	if !s.IsSubscribed("U1") {
		t.Error("U1 should still be subscribed")
	}
	if s.IsSubscribed("U2") {
		t.Error("U2 should not be subscribed after removal")
	}
	if !s.IsSubscribed("U3") {
		t.Error("U3 should still be subscribed")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s1, err := NewSubscribers(path)
	if err != nil {
		t.Fatal(err)
	}
	s1.Add("U1")
	s1.Add("U2")
	s1.Close()

	s2, err := NewSubscribers(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	if !s2.IsSubscribed("U1") || !s2.IsSubscribed("U2") {
		t.Error("subscribers should persist across restarts")
	}
}

func TestCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deep", "test.db")
	s, err := NewSubscribers(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}
