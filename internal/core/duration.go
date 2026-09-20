package core

import (
	"encoding/json"
	"time"
)

// Duration is a time.Duration that marshals as a human string such as "15m"
// rather than as a nanosecond count. Lease TTLs and retention windows appear in
// config files, JSON payloads and CLI output, where a bare integer would be
// both unreadable and easy to misread by a factor of a billion.
type Duration time.Duration

// D converts to a standard duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

// String renders the duration in Go's duration syntax.
func (d Duration) String() string { return time.Duration(d).String() }

// MarshalJSON renders the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts either a duration string ("15m") or a number of
// nanoseconds, so payloads produced by other tooling still load.
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

// UnmarshalYAML accepts a duration string such as "15m", or a bare number of
// nanoseconds. YAML coerces a scalar number into a string, so a string that
// does not parse as a duration falls back to the numeric form before failing.
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
