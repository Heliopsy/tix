// SPDX-License-Identifier: AGPL-3.0-or-later

package quickadd

import (
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

// DueWords are the calendar words due: accepts besides weekday names, offsets
// and dates, spelled out once so the help, the docs and the refusal agree.
var DueWords = []string{"today", "tomorrow"}

// maxOffset bounds +Nd and +Nw, so an offset cannot overflow a date.
const maxOffset = 36500

// weekdays maps every accepted weekday spelling to its day.
var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// ParseDue reads a due: value against now. Calendar words and offsets are
// counted in now's location and land on midnight UTC of the day they name,
// the instant a bare YYYY-MM-DD gives everywhere else in tix. A weekday is the
// next such day after today, never today, which has a word of its own.
func ParseDue(value string, now time.Time) (*time.Time, error) {
	word := strings.ToLower(strings.TrimSpace(value))
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	switch word {
	case "today":
		return &today, nil
	case "tomorrow":
		at := today.AddDate(0, 0, 1)
		return &at, nil
	}
	if day, ok := weekdays[word]; ok {
		ahead := (int(day) - int(now.Weekday()) + 7) % 7
		if ahead == 0 {
			ahead = 7
		}
		at := today.AddDate(0, 0, ahead)
		return &at, nil
	}
	if days, ok := offset(word); ok {
		at := today.AddDate(0, 0, days)
		return &at, nil
	}
	if at, err := query.ParseTime(value); err == nil {
		return at, nil
	}
	return nil, core.Invalid("due date %q must be today, tomorrow, a weekday such as fri, an offset such as +3d or +2w, or YYYY-MM-DD", value).
		WithDetail(core.DetailField, "title")
}

// offset reads +Nd or +Nw as a number of days.
func offset(word string) (int, bool) {
	rest, ok := strings.CutPrefix(word, "+")
	if !ok || len(rest) < 2 {
		return 0, false
	}
	unit := rest[len(rest)-1]
	digits := rest[:len(rest)-1]
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	switch unit {
	case 'd':
	case 'w':
		n *= 7
	default:
		return 0, false
	}
	if n > maxOffset {
		return 0, false
	}
	return n, true
}

// FormatDue writes a due date the way a preview shows it: the day alone at
// midnight UTC, the full instant otherwise.
func FormatDue(t time.Time) string {
	u := t.UTC()
	if u.Hour() == 0 && u.Minute() == 0 && u.Second() == 0 && u.Nanosecond() == 0 {
		return u.Format(time.DateOnly)
	}
	return u.Format(time.RFC3339)
}
