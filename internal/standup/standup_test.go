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
