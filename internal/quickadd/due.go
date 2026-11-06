// SPDX-License-Identifier: AGPL-3.0-or-later

package quickadd

import (
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/query"
)

// DueWords are the calendar words due: accepts besides weekday names, offsets
// and dates, spelled out once so the help, the docs and the refusal agree.
var DueWords = []string{"today", "tomorrow"}

// ExampleDay is the day every hint writes its example date with. It falls
// after the 12th, so the example shows which of its numbers is the day.
var ExampleDay = time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC)

// DueExamples are the values a due date hint offers, ending with date, the
// example day written the way the reader writes dates. Every one is a value
// ParseDueIn accepts, which a test holds each surface's hint to.
func DueExamples(date string) []string {
	return []string{"tomorrow", "fri", "+3d", "+2w", date}
}

// monthNameLayouts are the dates every order reads, since a month written as a
// word cannot be mistaken for the day.
var monthNameLayouts = []string{"2 Jan 2006", "2 January 2006", "Jan 2 2006", "January 2 2006"}

// orderedLayouts are the dates written with digits alone that an order reads
// beyond YYYY-MM-DD.
var orderedLayouts = map[output.DateOrder][]string{
	output.DateDayFirst:   {"2.1.2006", "2/1/2006"},
	output.DateMonthFirst: {"1/2/2006"},
}

// orderedExample is how a refusal writes the example day for an order.
var orderedExample = map[output.DateOrder]string{
	output.DateISO:        "2026-11-20",
	output.DateDayFirst:   "20.11.2026",
	output.DateMonthFirst: "11/20/2026",
}

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
	return ParseDueIn(value, now, output.DateISO)
}

// ParseDueIn is ParseDue for a reader whose dates are written in order. A date
// such as 06/11/2026 is read only under an order that says which number is
// the day, and refused otherwise rather than guessed.
func ParseDueIn(value string, now time.Time, order output.DateOrder) (*time.Time, error) {
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
	if at, ok := ParseDate(value, order); ok {
		return &at, nil
	}
	return nil, core.Invalid("due date %q must be today, tomorrow, a weekday such as fri, an offset such as +3d or +2w, or a date such as %s", value, orderedExample[order]).
		WithDetail(core.DetailField, "title")
}

// ParseDate reads a calendar date written with a month name, or with digits in
// the reader's order, as midnight UTC of that day. YYYY-MM-DD is query's.
func ParseDate(value string, order output.DateOrder) (time.Time, bool) {
	trimmed := strings.Join(strings.Fields(strings.ReplaceAll(value, ",", " ")), " ")
	for _, layouts := range [][]string{monthNameLayouts, orderedLayouts[order]} {
		for _, layout := range layouts {
			if at, err := time.Parse(layout, trimmed); err == nil {
				return at, true
			}
		}
	}
	return time.Time{}, false
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
