package standup

import (
	"testing"
	"time"
)

func TestTimeToTS(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{"epoch", time.Unix(0, 0), "0.000000"},
		{"standard", time.Unix(1708300000, 0), "1708300000.000000"},
		{"sub-second precision is dropped", time.Unix(1708300000, 999999999), "1708300000.000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := timeToTS(tt.in); got != tt.want {
				t.Errorf("timeToTS(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// timeToTS and tsToTime are used as a pair to bound history queries, so a value
// must survive the round trip at second granularity.
func TestTimestampRoundTrip(t *testing.T) {
	want := time.Unix(1786713302, 0)
	if got := tsToTime(timeToTS(want)); !got.Equal(want) {
		t.Errorf("round trip = %v, want %v", got, want)
	}
}

func TestTsToTimeRejectsMalformedInput(t *testing.T) {
	// Garbage must not silently become a plausible-looking date.
	for _, ts := range []string{"not-a-ts", "17083x0000.000000", "", "-", "1e9"} {
		if got := tsToTime(ts); !got.Equal(time.Unix(0, 0)) {
			t.Errorf("tsToTime(%q) = %v, want the epoch", ts, got)
		}
	}
}

// The lookback bound is what stops a wrong thread identifier from paging a
// channel's entire history and exhausting the conversations.history rate limit.
func TestHistoryLookbackIsBounded(t *testing.T) {
	if historyLookback <= 0 {
		t.Fatalf("historyLookback = %v, must be positive", historyLookback)
	}
	// Long enough to survive a holiday break, short enough to stay cheap.
	if historyLookback < 7*24*time.Hour || historyLookback > 90*24*time.Hour {
		t.Errorf("historyLookback = %v, want between 7 and 90 days", historyLookback)
	}

	now := time.Unix(1786713302, 0)
	oldest := tsToTime(timeToTS(now.Add(-historyLookback)))
	if !oldest.Before(now) {
		t.Errorf("oldest bound %v should precede now %v", oldest, now)
	}
	if got := now.Sub(oldest); got != historyLookback {
		t.Errorf("bound spans %v, want %v", got, historyLookback)
	}
}

// A date-filtered scan bounds to a single local day, so the requested date sits
// inside the window and the neighbouring days do not.
func TestDateBoundsCoverExactlyOneDay(t *testing.T) {
	date := time.Date(2026, 8, 13, 0, 0, 0, 0, time.Local)
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.AddDate(0, 0, 1)

	oldest := tsToTime(timeToTS(start))
	latest := tsToTime(timeToTS(end))

	if got := latest.Sub(oldest); got != 24*time.Hour {
		t.Errorf("window spans %v, want 24h", got)
	}

	midday := time.Date(2026, 8, 13, 12, 0, 0, 0, time.Local)
	if midday.Before(oldest) || !midday.Before(latest) {
		t.Errorf("%v should fall within [%v, %v)", midday, oldest, latest)
	}
	dayBefore := midday.AddDate(0, 0, -1)
	if !dayBefore.Before(oldest) {
		t.Errorf("%v should fall before the window start %v", dayBefore, oldest)
	}
	dayAfter := midday.AddDate(0, 0, 1)
	if dayAfter.Before(latest) {
		t.Errorf("%v should fall at or after the window end %v", dayAfter, latest)
	}
}
