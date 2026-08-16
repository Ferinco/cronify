package scheduler

import (
	"testing"
	"time"
)

func TestNextComputesNextOccurrence(t *testing.T) {
	from := time.Date(2026, 8, 16, 8, 30, 0, 0, time.UTC)

	cases := []struct {
		name     string
		schedule string
		want     time.Time
	}{
		{"daily at 9am, before", "0 9 * * *", time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)},
		{"every 15 minutes", "*/15 * * * *", time.Date(2026, 8, 16, 8, 45, 0, 0, time.UTC)},
		{"hourly, past this hour's slot", "0 * * * *", time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Next(tc.schedule, from)
			if err != nil {
				t.Fatalf("Next(%q): %v", tc.schedule, err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("Next(%q) from %v = %v, want %v", tc.schedule, from, got, tc.want)
			}
		})
	}
}

func TestNextRejectsInvalidSchedule(t *testing.T) {
	if _, err := Next("not a cron schedule", time.Now()); err == nil {
		t.Fatalf("expected an error for an invalid cron schedule")
	}
}
