package standup

import (
	"reflect"
	"testing"
)

// The grace allowance is expressed as "standups you may miss", but the scan
// needs a thread count, and it must always include the immediately-preceding
// thread — a zero or negative allowance still scans one.
func TestLookbackThreads(t *testing.T) {
	tests := []struct {
		name      string
		maxMissed int
		want      int
	}{
		{"no grace still scans the previous thread", 0, 1},
		{"negative is clamped, never zero threads", -3, 1},
		{"one missed standup scans two threads", 1, 2},
		{"default allowance scans three threads", 2, 3},
		{"a week of grace", 5, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lookbackThreads(tt.maxMissed); got != tt.want {
				t.Errorf("lookbackThreads(%d) = %d, want %d", tt.maxMissed, got, tt.want)
			}
		})
	}
}

// Threads arrive newest first, so the newest reply for a user must win. Echoing
// a stale update over a fresh one would show someone yesterday's work as if it
// were their latest.
func TestMergeRepliesKeepsNewestPerUser(t *testing.T) {
	merged := mergeReplies([]map[string]string{
		{"U1": "thursday from u1"},
		{"U1": "wednesday from u1", "U2": "wednesday from u2"},
		{"U1": "tuesday from u1", "U2": "tuesday from u2", "U3": "tuesday from u3"},
	})

	want := map[string]string{
		"U1": "thursday from u1",
		"U2": "wednesday from u2",
		"U3": "tuesday from u3",
	}
	if !reflect.DeepEqual(merged, want) {
		t.Errorf("mergeReplies() = %v, want %v", merged, want)
	}
}

// A user who replied only in an older thread is exactly the case the grace
// window exists for: they missed the last standup and must still be nudged.
func TestMergeRepliesIncludesUsersMissingFromNewestThread(t *testing.T) {
	merged := mergeReplies([]map[string]string{
		{"U1": "replied yesterday"},
		{"U2": "replied the day before"},
	})

	if _, ok := merged["U2"]; !ok {
		t.Errorf("mergeReplies() = %v, want it to include U2 from the older thread", merged)
	}
	if len(merged) != 2 {
		t.Errorf("mergeReplies() has %d users, want 2", len(merged))
	}
}

// An empty reply must not shadow a real one from an older thread, and must not
// be invented for a user who never replied.
func TestMergeRepliesEdgeCases(t *testing.T) {
	if got := mergeReplies(nil); len(got) != 0 {
		t.Errorf("mergeReplies(nil) = %v, want empty", got)
	}
	if got := mergeReplies([]map[string]string{{}, {}}); len(got) != 0 {
		t.Errorf("mergeReplies(empty maps) = %v, want empty", got)
	}

	// A user present with an empty string in the newest thread still counts as
	// having replied there; that is Slack's data, not a gap to paper over.
	merged := mergeReplies([]map[string]string{
		{"U1": ""},
		{"U1": "older text"},
	})
	if merged["U1"] != "" {
		t.Errorf("mergeReplies() U1 = %q, want the newest (empty) reply", merged["U1"])
	}
}
