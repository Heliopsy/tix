package webhook

import (
	"testing"
	"time"
)

func TestDefaultScheduleSteps(t *testing.T) {
	s := DefaultSchedule()
	want := []time.Duration{
		time.Second, 5 * time.Second, 30 * time.Second,
		5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour,
	}
	if len(s) != len(want) {
		t.Fatalf("schedule length = %d, want %d", len(s), len(want))
	}
	for i, d := range want {
		got, ok := s.Next(i + 1)
		if !ok || got != d {
			t.Fatalf("Next(%d) = %v, %v, want %v, true", i+1, got, ok, d)
		}
		if i > 0 && want[i] <= want[i-1] {
			t.Fatalf("step %d does not back off further than step %d", i, i-1)
		}
	}
	if _, ok := s.Next(len(want) + 1); ok {
		t.Fatal("an attempt was offered past the end of the schedule")
	}
	if s.MaxAttempts() != len(want)+1 {
		t.Fatalf("MaxAttempts = %d, want %d", s.MaxAttempts(), len(want)+1)
	}
}

func TestNextAttemptAt(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := DefaultSchedule()

	at, retry := s.NextAttemptAt(0, now)
	if !retry || !at.Equal(now.Add(time.Second)) {
		t.Fatalf("NextAttemptAt(0) = %v, %v", at, retry)
	}
	at, retry = s.NextAttemptAt(3, now)
	if !retry || !at.Equal(now.Add(30*time.Second)) {
		t.Fatalf("NextAttemptAt(3) = %v, %v", at, retry)
	}
	at, retry = s.NextAttemptAt(s.MaxAttempts(), now)
	if retry || !at.Equal(now) {
		t.Fatalf("NextAttemptAt past the schedule = %v, %v", at, retry)
	}
}
