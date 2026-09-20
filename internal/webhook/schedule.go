package webhook

import "time"

// Schedule is the backoff before each retry, one entry per retry, in order.
type Schedule []time.Duration

// DefaultSchedule is the shipped backoff: seven retries spread over nine hours.
func DefaultSchedule() Schedule {
	return Schedule{
		time.Second,
		5 * time.Second,
		30 * time.Second,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		6 * time.Hour,
	}
}

// MaxAttempts returns how many attempts a delivery gets before it fails for good.
func (s Schedule) MaxAttempts() int { return len(s) + 1 }

// Next returns the backoff owed after the given number of failed attempts and
// whether another attempt is due at all.
func (s Schedule) Next(attempts int) (time.Duration, bool) {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > len(s) {
		return 0, false
	}
	return s[attempts-1], true
}

// NextAttemptAt returns when the attempt following a failure is due, and
// whether the delivery has any attempt left.
func (s Schedule) NextAttemptAt(attempts int, now time.Time) (time.Time, bool) {
	d, ok := s.Next(attempts)
	if !ok {
		return now.UTC(), false
	}
	return now.Add(d).UTC(), true
}
