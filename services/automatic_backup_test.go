package services

import (
	"testing"
	"time"
)

func TestAutomaticBackupDue(t *testing.T) {
	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		last     time.Time
		interval uint
		want     bool
	}{
		{name: "first backup", interval: 24, want: true},
		{name: "interval not elapsed", last: now.Add(-23 * time.Hour), interval: 24, want: false},
		{name: "interval elapsed", last: now.Add(-24 * time.Hour), interval: 24, want: true},
		{name: "legacy interval defaults to one day", last: now.Add(-23 * time.Hour), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := automaticBackupDue(test.last, now, test.interval); got != test.want {
				t.Fatalf("automaticBackupDue() = %t, want %t", got, test.want)
			}
		})
	}
}
