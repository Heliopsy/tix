// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Duration is a time.Duration that marshals as a human string such as "15m"
type Duration time.Duration

// D converts to a standard duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

// String renders the duration in Go's duration syntax.
func (d Duration) String() string { return time.Duration(d).String() }

// MarshalJSON renders the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts a duration string such as "15m" or "30d", or nanoseconds.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		parsed, err := ParseDuration(s)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	}

	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return Invalid("duration must be a string such as \"15m\" or a number of nanoseconds")
	}
	*d = Duration(n)
	return nil
}

// MarshalYAML renders the duration as a string.
func (d Duration) MarshalYAML() (any, error) { return time.Duration(d).String(), nil }

// UnmarshalYAML accepts a duration string such as "15m" or "30d", or nanoseconds.
func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		parsed, perr := ParseDuration(s)
		if perr == nil {
			*d = parsed
			return nil
		}
		var n int64
		if unmarshal(&n) == nil {
			*d = Duration(n)
			return nil
		}
		return perr
	}

	var n int64
	if err := unmarshal(&n); err != nil {
		return Invalid("duration must be a string such as \"15m\" or a number of nanoseconds")
	}
	*d = Duration(n)
	return nil
}

// Human renders a duration the way a reader wants it rather than the way Go
// prints it, so an eighteen day lead time reads "16d 1h" and not
// "384h50m23.606839092s".
//
// It lives here because three surfaces show the same figures. The browser had
// a private copy of this and the command line and terminal had none, so the
// statistics screen that was readable in a browser printed nanoseconds
// everywhere else. A measure the product presents in three places has one
// rendering, for the same reason the leaderboard's caption does.
//
// The precision drops as the magnitude rises, which is the point: nobody
// reading "how long does work sit here" needs the seconds on a figure counted
// in weeks.
func (d Duration) Human() string {
	t := d.D()
	if t < 0 {
		t = -t
	}
	switch {
	case t < time.Minute:
		return fmt.Sprintf("%ds", int(t.Seconds()))
	case t < time.Hour:
		return fmt.Sprintf("%dm", int(t.Minutes()))
	case t < 24*time.Hour:
		if m := int(t.Minutes()) % 60; m > 0 {
			return fmt.Sprintf("%dh %dm", int(t.Hours()), m)
		}
		return fmt.Sprintf("%dh", int(t.Hours()))
	default:
		days := int(t.Hours()) / 24
		if h := int(t.Hours()) % 24; h > 0 {
			return fmt.Sprintf("%dd %dh", days, h)
		}
		return fmt.Sprintf("%dd", days)
	}
}

// ParseDuration reads the vocabulary the product prints. It accepts everything
// time.ParseDuration accepts, and additionally a "d" unit and whitespace
// between terms, so that Human output such as "16d 1h" is valid input.
//
// The day exists because Human renders one and the command line used to reject
// it: a retention window shown as "30d" that had to be typed as "720h" is the
// product contradicting itself. There is no week unit, because nothing renders
// one, and a unit the product accepts but never prints is the same asymmetry
// facing the other way.
//
// A day here is exactly 24 hours. Nothing in this type does calendar
// arithmetic, so no value of it can straddle a daylight saving transition.
func ParseDuration(s string) (Duration, error) {
	trimmed := strings.TrimSpace(s)
	if d, err := time.ParseDuration(trimmed); err == nil {
		return Duration(d), nil
	}
	d, ok := parseTerms(trimmed)
	if !ok {
		return 0, Invalid("duration %q must be a duration such as 15m, 2h30m or 30d", s)
	}
	return Duration(d), nil
}

// parseTerms reads a whitespace-separated or adjacent run of number-unit terms.
func parseTerms(s string) (time.Duration, bool) {
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	var total time.Duration
	terms := 0
	for i := 0; i < len(s); {
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}
		start := i
		for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
			i++
		}
		number := s[start:i]
		start = i
		for i < len(s) && (s[i] < '0' || s[i] > '9') && s[i] != '.' && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		unit := s[start:i]
		term, ok := parseTerm(number, unit)
		if !ok {
			return 0, false
		}
		if total+term < total {
			return 0, false
		}
		total += term
		terms++
	}
	if terms == 0 {
		return 0, false
	}
	if neg {
		total = -total
	}
	return total, true
}

// parseTerm converts one number and unit, handling the day Go does not know.
func parseTerm(number, unit string) (time.Duration, bool) {
	if number == "" || unit == "" {
		return 0, false
	}
	if unit != "d" {
		term, err := time.ParseDuration(number + unit)
		if err != nil || term < 0 {
			return 0, false
		}
		return term, true
	}
	if !strings.Contains(number, ".") {
		days, err := strconv.ParseInt(number, 10, 64)
		if err != nil || days > maxDurationDays {
			return 0, false
		}
		return time.Duration(days) * 24 * time.Hour, true
	}
	days, err := strconv.ParseFloat(number, 64)
	if err != nil || days < 0 || days > float64(maxDurationDays) {
		return 0, false
	}
	return time.Duration(days * 24 * float64(time.Hour)), true
}

// maxDurationDays is the largest whole day count a time.Duration can hold.
const maxDurationDays = int64(1<<63-1) / (24 * int64(time.Hour))
