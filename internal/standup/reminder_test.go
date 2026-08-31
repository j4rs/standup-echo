package standup

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// The reminder set is the roster minus whoever already posted. Getting this
// backwards would nag exactly the people who did their standup.
func TestUnpostedFrom(t *testing.T) {
	tests := []struct {
		name   string
		roster map[string]string
		posted map[string]string
		want   []string
	}{
		{
			name:   "nobody has posted yet",
			roster: map[string]string{"U2": "y", "U1": "y"},
			posted: map[string]string{},
			want:   []string{"U1", "U2"},
		},
		{
			name:   "everyone has posted",
			roster: map[string]string{"U1": "y", "U2": "y"},
			posted: map[string]string{"U1": "today", "U2": "today"},
			want:   nil,
		},
		{
			name:   "only the stragglers",
			roster: map[string]string{"U1": "y", "U2": "y", "U3": "y"},
			posted: map[string]string{"U2": "today"},
			want:   []string{"U1", "U3"},
		},
		{
			name:   "someone new posted today but is not on the roster",
			roster: map[string]string{"U1": "y"},
			posted: map[string]string{"U1": "today", "U9": "today"},
			want:   nil,
		},
		{
			name:   "an empty reply today still counts as posted",
			roster: map[string]string{"U1": "y"},
			posted: map[string]string{"U1": ""},
			want:   nil,
		},
		{
			name:   "empty roster reminds nobody",
			roster: map[string]string{},
			posted: map[string]string{"U1": "today"},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unpostedFrom(tt.roster, tt.posted); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unpostedFrom() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Sorted output keeps run-to-run logs comparable, since Go map iteration order
// is randomised.
func TestUnpostedFromIsSorted(t *testing.T) {
	roster := map[string]string{"UZ": "y", "UA": "y", "UM": "y", "UB": "y"}
	want := []string{"UA", "UB", "UM", "UZ"}
	for i := 0; i < 20; i++ {
		if got := unpostedFrom(roster, nil); !reflect.DeepEqual(got, want) {
			t.Fatalf("unpostedFrom() = %v, want %v", got, want)
		}
	}
}

func TestBuildReminderText(t *testing.T) {
	withLink := buildReminderText("https://slack.example/thread")
	if !strings.Contains(withLink, "https://slack.example/thread") {
		t.Errorf("reminder = %q, want it to contain the thread link", withLink)
	}
	if !strings.Contains(withLink, "haven't posted") {
		t.Errorf("reminder = %q, want it to say what is missing", withLink)
	}

	// A permalink lookup can fail; the reminder must still be sendable rather
	// than shipping an empty <|...> link.
	noLink := buildReminderText("")
	if strings.Contains(noLink, "<") || strings.Contains(noLink, ">") {
		t.Errorf("reminder = %q, want no link markup when the permalink is missing", noLink)
	}
	if noLink == "" {
		t.Error("reminder with no link is empty, want the bare nudge text")
	}
}

// The scheduler computes fire time from the thread's own timestamp, so this
// conversion is what decides whether a reminder lands at the right hour.
func TestThreadTime(t *testing.T) {
	if got := ThreadTime("1787750104.185779"); !got.Equal(time.Unix(1787750104, 0)) {
		t.Errorf("ThreadTime() = %v, want %v", got, time.Unix(1787750104, 0))
	}
	// Garbage must not become a plausible date that schedules a bogus reminder.
	if got := ThreadTime("not-a-ts"); !got.Equal(time.Unix(0, 0)) {
		t.Errorf("ThreadTime(garbage) = %v, want the epoch", got)
	}
}

// A thread timestamp plus the delay must land in the future for a same-morning
// post, and in the past for yesterday's, which is what the scheduler branches on.
func TestReminderWindowDirection(t *testing.T) {
	const fourHours = 4 * time.Hour
	now := time.Unix(1787922902, 0) // the Aug 28 standup post

	postedJustNow := ThreadTime("1787922902.523579").Add(fourHours)
	if !postedJustNow.After(now) {
		t.Errorf("a thread posted now should remind in the future, got %v", postedJustNow)
	}

	postedYesterday := ThreadTime("1787836502.502509").Add(fourHours)
	if !postedYesterday.Before(now) {
		t.Errorf("yesterday's thread should already be past its window, got %v", postedYesterday)
	}
}
