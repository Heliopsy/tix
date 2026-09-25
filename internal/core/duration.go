// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"encoding/json"
	"fmt"
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

// UnmarshalJSON accepts a duration string such as "15m", or nanoseconds.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return Invalid("parsing duration %q: %v", s, err)
		}
		*d = Duration(parsed)
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

// UnmarshalYAML accepts a duration string such as "15m", or nanoseconds.
func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		parsed, perr := time.ParseDuration(s)
		if perr == nil {
			*d = Duration(parsed)
			return nil
		}
		var n int64
		if unmarshal(&n) == nil {
			*d = Duration(n)
			return nil
		}
		return Invalid("parsing duration %q: %v", s, perr)
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
