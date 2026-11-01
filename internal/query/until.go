// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// Snooze presets, named once so the CLI help, the terminal prompt and the
// browser's menu offer the same words.
const (
	UntilLater    = "later"
	UntilTomorrow = "tomorrow"
	UntilNextWeek = "next-week"
)

// UntilPresets lists the named wake times ParseUntil accepts.
var UntilPresets = []string{UntilLater, UntilTomorrow, UntilNextWeek}

// MorningHour is when "tomorrow", "next week" and a bare date wake a task.
const MorningHour = 9

// laterGap is how far ahead "later today" lands before rounding up to the hour.
const laterGap = 3 * time.Hour

// ParseUntil reads when a snooze should end: a preset, a duration such as
// 90m, 2h, 3d or 1w, or a date or timestamp. A date alone wakes at
// MorningHour, and a timestamp without a zone is read in loc. It does not
// check that the result is in the future; the service does.
func ParseUntil(value string, now time.Time, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return time.Time{}, core.Invalid("say when the task should wake")
	}
	if t, ok := Preset(v, now, loc); ok {
		return t, nil
	}
	if d, ok := untilDuration(v); ok {
		return now.Add(d), nil
	}
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(value), loc); err == nil {
			return t, nil
		}
	}
	if d, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
		return d.Add(MorningHour * time.Hour), nil
	}
	return time.Time{}, core.Invalid("cannot read %q as a time to wake; use %s, a duration such as 2h or 3d, "+
		"a date (YYYY-MM-DD) or a timestamp (YYYY-MM-DD HH:MM)", value, strings.Join(UntilPresets, ", "))
}

// Preset resolves a named wake time in loc, reporting false for any other word.
func Preset(name string, now time.Time, loc *time.Location) (time.Time, bool) {
	local := now.In(loc)
	morning := func(days int) time.Time {
		y, m, d := local.Date()
		return time.Date(y, m, d+days, MorningHour, 0, 0, 0, loc)
	}
	switch name {
	case UntilLater, "later-today":
		t := local.Add(laterGap)
		if t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
			return t, true
		}
		y, m, d := t.Date()
		return time.Date(y, m, d, t.Hour()+1, 0, 0, 0, loc), true
	case UntilTomorrow:
		return morning(1), true
	case UntilNextWeek, "nextweek", "next_week":
		days := (int(time.Monday) - int(local.Weekday()) + 7) % 7
		if days == 0 {
			days = 7
		}
		return morning(days), true
	}
	return time.Time{}, false
}

// untilDuration reads a duration, adding the day and week units time.Duration
// does not have.
func untilDuration(v string) (time.Duration, bool) {
	for suffix, unit := range map[string]time.Duration{"d": 24 * time.Hour, "w": 7 * 24 * time.Hour} {
		if n, ok := strings.CutSuffix(v, suffix); ok {
			count, err := strconv.Atoi(n)
			if err != nil || count <= 0 {
				return 0, false
			}
			return time.Duration(count) * unit, true
		}
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}
