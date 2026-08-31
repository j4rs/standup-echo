package store

import (
	"path/filepath"
	"testing"
)

func newTestReminders(t *testing.T) *Reminders {
	t.Helper()
	subs, err := NewSubscribers(filepath.Join(t.TempDir(), "standup-echo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { subs.Close() })
	r, err := NewReminders(subs)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Claim is what stops a restart or a mid-afternoon redeploy from nagging people
// twice for the same thread: the first call wins, every later one loses.
func TestClaimIsIdempotentPerThread(t *testing.T) {
	r := newTestReminders(t)

	claimed, err := r.Claim("C1", "1787922902.523579")
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("first Claim() = false, want true")
	}

	for i := 0; i < 3; i++ {
		claimed, err := r.Claim("C1", "1787922902.523579")
		if err != nil {
			t.Fatal(err)
		}
		if claimed {
			t.Errorf("repeat Claim() = true, want false")
		}
	}
}

// Channels are reminded independently, so the same thread timestamp in another
// channel — and another thread in the same channel — must still be claimable.
func TestClaimIsScopedToChannelAndThread(t *testing.T) {
	r := newTestReminders(t)

	if claimed, err := r.Claim("C1", "111.0"); err != nil || !claimed {
		t.Fatalf("Claim(C1, 111) = %v, %v; want true, nil", claimed, err)
	}
	if claimed, err := r.Claim("C2", "111.0"); err != nil || !claimed {
		t.Errorf("Claim(C2, 111) = %v, %v; want true, nil — channels are independent", claimed, err)
	}
	if claimed, err := r.Claim("C1", "222.0"); err != nil || !claimed {
		t.Errorf("Claim(C1, 222) = %v, %v; want true, nil — a new day is a new thread", claimed, err)
	}
}

// The reminders table must not disturb the subscribers table, whose primary key
// a per-channel opt-out would have to rebuild.
func TestNewRemindersLeavesSubscribersIntact(t *testing.T) {
	subs, err := NewSubscribers(filepath.Join(t.TempDir(), "standup-echo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer subs.Close()

	if _, err := subs.Add("U1"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewReminders(subs); err != nil {
		t.Fatal(err)
	}
	if !subs.IsSubscribed("U1") {
		t.Error("subscriber lost after creating the reminders table")
	}
}

// Creating the table twice is the normal path on every restart.
func TestNewRemindersIsRepeatable(t *testing.T) {
	subs, err := NewSubscribers(filepath.Join(t.TempDir(), "standup-echo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer subs.Close()

	r1, err := NewReminders(subs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r1.Claim("C1", "111.0"); err != nil {
		t.Fatal(err)
	}
	r2, err := NewReminders(subs)
	if err != nil {
		t.Fatalf("second NewReminders() = %v, want nil", err)
	}
	// The claim must survive, or a restart would re-nag.
	if claimed, err := r2.Claim("C1", "111.0"); err != nil || claimed {
		t.Errorf("Claim() after reopen = %v, %v; want false, nil", claimed, err)
	}
}
