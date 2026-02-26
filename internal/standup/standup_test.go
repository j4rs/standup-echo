package standup

import (
	"testing"
	"time"
)

func TestTsToTime(t *testing.T) {
	tests := []struct {
		name string
		ts   string
		want time.Time
	}{
		{
			name: "standard timestamp",
			ts:   "1708300000.000000",
			want: time.Unix(1708300000, 0),
		},
		{
			name: "no fractional part",
			ts:   "1708300000",
			want: time.Unix(1708300000, 0),
		},
		{
			name: "different timestamp",
			ts:   "1700000000.123456",
			want: time.Unix(1700000000, 0),
		},
		{
			name: "empty string",
			ts:   "",
			want: time.Unix(0, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsToTime(tt.ts)
			if !got.Equal(tt.want) {
				t.Errorf("tsToTime(%q) = %v, want %v", tt.ts, got, tt.want)
			}
		})
	}
}

func TestBuildDMText(t *testing.T) {
	t.Run("keeps previous entries and appends today's prompt with thread link", func(t *testing.T) {
		got := buildDMText(
			"Wednesday, February 25",
			"Tuesday, February 24\n:construction: Continue with ticket\n\nWednesday, February 25\n:blocker: Review PR",
			"https://example.slack.com/archives/C123/p1740000000000100",
		)

		want := "Tuesday, February 24\n:construction: Continue with ticket\n\nWednesday, February 25\n:blocker: Review PR\n\nWednesday, February 25\nWhat are you up to today?\n\n<https://example.slack.com/archives/C123/p1740000000000100|Open today's standup thread>"
		if got != want {
			t.Fatalf("buildDMText() mismatch\nwant:\n%q\ngot:\n%q", want, got)
		}
	})

	t.Run("omits thread link when empty", func(t *testing.T) {
		got := buildDMText(
			"Wednesday, February 25",
			"(a) Completed: finished ticket",
			"",
		)

		want := "(a) Completed: finished ticket\n\nWednesday, February 25\nWhat are you up to today?"
		if got != want {
			t.Fatalf("buildDMText() mismatch\nwant:\n%q\ngot:\n%q", want, got)
		}
	})

	t.Run("handles empty previous reply", func(t *testing.T) {
		got := buildDMText(
			"Wednesday, February 25",
			"",
			"",
		)

		want := "Wednesday, February 25\nWhat are you up to today?"
		if got != want {
			t.Fatalf("buildDMText() mismatch\nwant:\n%q\ngot:\n%q", want, got)
		}
	})
}
