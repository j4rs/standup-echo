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
	t.Run("includes spacing and thread link", func(t *testing.T) {
		got := buildDMText(
			"Monday, February 24",
			"Wednesday, February 25",
			"(a) Completed: finished ticket",
			"https://example.slack.com/archives/C123/p1740000000000100",
		)

		want := "*Monday, February 24*\n---\n\n(a) Completed: finished ticket\n\n---\n*Wednesday, February 25*\n\n<https://example.slack.com/archives/C123/p1740000000000100|Open today's standup thread>\n"
		if got != want {
			t.Fatalf("buildDMText() mismatch\nwant:\n%q\ngot:\n%q", want, got)
		}
	})

	t.Run("omits thread link when empty", func(t *testing.T) {
		got := buildDMText(
			"Monday, February 24",
			"Wednesday, February 25",
			"(a) Completed: finished ticket",
			"",
		)

		want := "*Monday, February 24*\n---\n\n(a) Completed: finished ticket\n\n---\n*Wednesday, February 25*\n"
		if got != want {
			t.Fatalf("buildDMText() mismatch\nwant:\n%q\ngot:\n%q", want, got)
		}
	})
}
