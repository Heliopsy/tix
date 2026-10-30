// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
	"testing"
	"time"
)

func TestParseRecurrenceCanonical(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"every day", "every day"},
		{"daily", "every day"},
		{"Every 1 Day", "every day"},
		{"every 2 days", "every 2 days"},
		{"every 2 day", "every 2 days"},
		{"weekly", "every week"},
		{"every week on thu,mon", "every week on mon,thu"},
		{"weekly on thu, mon", "every week on mon,thu"},
		{"every 2 weeks on Monday , Thursday", "every 2 weeks on mon,thu"},
		{"every week on sun,sat,mon,mon", "every week on mon,sat,sun"},
		{"monthly", "every month"},
		{"every month on day 31", "every month on day 31"},
		{"every 3 months", "every 3 months"},
		{"yearly", "every year"},
		{"every 2 years", "every 2 years"},
		{"3 days after done", "every 3 days after done"},
		{"every 3 days after done", "every 3 days after done"},
		{"daily after done", "every day after done"},
		{"every week until 2027-01-31", "every week until 2027-01-31"},
		{"every day for 5 times", "every day for 5 times"},
		{"every day for 1 times", "every day for 1 time"},
		{"every day for 1 time", "every day for 1 time"},
		{"for 3 times every day", ""},
		{"every 2 weeks on mon after done for 3 times", "every 2 weeks on mon after done for 3 times"},
		{"every month for 2 times after done", "every month after done for 2 times"},
		{"  every   week  ", "every week"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			r, err := ParseRecurrence(c.in)
			if c.want == "" {
				if err == nil {
					t.Fatalf("ParseRecurrence(%q) = %q, want an error", c.in, r)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRecurrence(%q) error = %v", c.in, err)
			}
			if got := r.String(); got != c.want {
				t.Fatalf("ParseRecurrence(%q).String() = %q, want %q", c.in, got, c.want)
			}
			again, err := ParseRecurrence(c.want)
			if err != nil || again.String() != c.want {
				t.Fatalf("canonical %q does not round trip: %q, %v", c.want, again.String(), err)
			}
		})
	}
}

func TestParseRecurrenceRefuses(t *testing.T) {
	cases := []struct {
		in, says string
	}{
		{"", "empty"},
		{"   ", "empty"},
		{"every", "repeats every"},
		{"every fortnight", "unknown unit"},
		{"every 0 days", "between 1 and"},
		{"every 1001 days", "between 1 and"},
		{"every -2 days", "between 1 and"},
		{"2 weeks", `starts with "every"`},
		{"week", "unknown word"},
		{"every 2", "repeats every"},
		{"every month on mon", "only to a weekly rule"},
		{"every day on mon", "only to a weekly rule"},
		{"every week on day 3", "only to a monthly rule"},
		{"every month on day 32", "between 1 and 31"},
		{"every month on day 0", "between 1 and 31"},
		{"every month on day", "between 1 and 31"},
		{"every week on", `"on" must name`},
		{"every week on funday", "unknown weekday"},
		{"every week on mon,,thu", "unknown weekday"},
		{"every week on mon on tue", "given twice"},
		{"every day after", `followed by "done"`},
		{"every day after lunch", `followed by "done"`},
		{"every day until tomorrow", "YYYY-MM-DD"},
		{"every day until 2027-02-30", "YYYY-MM-DD"},
		{"every day for 0 times", "between 1 and"},
		{"every day for 3", `followed by "times"`},
		{"every day for three times", "count between"},
		{"every day until 2027-01-01 for 3 times", "one end"},
		{"every day until 2027-01-01 until 2027-02-01", "given twice"},
		{"every day please", "unknown word"},
		{"every day and night", "unknown word"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			r, err := ParseRecurrence(c.in)
			if err == nil {
				t.Fatalf("ParseRecurrence(%q) = %q, want an error", c.in, r)
			}
			if !IsKind(err, KindInvalid) {
				t.Fatalf("ParseRecurrence(%q) error kind = %v, want invalid", c.in, err)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Fatalf("ParseRecurrence(%q) error = %q, want it to mention %q", c.in, err, c.says)
			}
		})
	}
}

func TestNormalizeRecurrence(t *testing.T) {
	if got, err := NormalizeRecurrence("  "); err != nil || got != "" {
		t.Fatalf("NormalizeRecurrence(blank) = %q, %v; want empty", got, err)
	}
	if got, err := NormalizeRecurrence("weekly on fri,mon"); err != nil || got != "every week on mon,fri" {
		t.Fatalf("NormalizeRecurrence = %q, %v", got, err)
	}
	if _, err := NormalizeRecurrence("sometimes"); err == nil {
		t.Fatal("NormalizeRecurrence(sometimes) accepted")
	}
}

func day(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func TestRecurrenceNext(t *testing.T) {
	cases := []struct {
		name      string
		rule      string
		due, done string
		want      string
	}{
		{"daily on time", "every day", "2026-11-02T09:00:00Z", "2026-11-02T08:00:00Z", "2026-11-03T09:00:00Z"},
		{"weekly on the day", "every week", "2026-11-02T00:00:00Z", "2026-11-02T15:00:00Z", "2026-11-09T00:00:00Z"},
		{"finished early keeps the schedule", "every week", "2026-11-09T00:00:00Z", "2026-11-02T15:00:00Z", "2026-11-16T00:00:00Z"},
		{"finished late skips missed", "every week", "2026-10-01T00:00:00Z", "2026-10-20T10:00:00Z", "2026-10-22T00:00:00Z"},
		{"finished years late", "every day", "2020-01-01T09:00:00Z", "2026-10-20T10:00:00Z", "2026-10-21T09:00:00Z"},
		{"finished late exactly on an occurrence", "every week", "2026-10-01T00:00:00Z", "2026-10-15T00:00:00Z", "2026-10-22T00:00:00Z"},
		{"every 2 days", "every 2 days", "2026-11-02T00:00:00Z", "2026-11-02T00:00:00Z", "2026-11-04T00:00:00Z"},
		{"after done", "every 3 days after done", "2026-10-01T00:00:00Z", "2026-10-20T10:00:00Z", "2026-10-23T00:00:00Z"},
		{"after done keeps the due clock", "every day after done", "2026-10-01T09:30:00Z", "2026-10-20T22:00:00Z", "2026-10-21T09:30:00Z"},
		{"undated counts from completion", "every 2 weeks", "", "2026-10-20T10:00:00Z", "2026-11-03T00:00:00Z"},
		{"month end clamps", "every month", "2027-01-31T00:00:00Z", "2027-01-31T10:00:00Z", "2027-02-28T00:00:00Z"},
		{"month end clamps in a leap year", "every month", "2028-01-31T00:00:00Z", "2028-01-31T10:00:00Z", "2028-02-29T00:00:00Z"},
		{"month end late keeps the origin day", "every month", "2027-01-31T00:00:00Z", "2027-03-05T00:00:00Z", "2027-03-31T00:00:00Z"},
		{"30th into april", "every month", "2027-03-30T00:00:00Z", "2027-03-30T00:00:00Z", "2027-04-30T00:00:00Z"},
		{"pinned day 31 after a clamp", "every month on day 31", "2027-02-28T00:00:00Z", "2027-02-28T00:00:00Z", "2027-03-31T00:00:00Z"},
		{"pinned day later this month", "every month on day 20", "2027-03-05T00:00:00Z", "2027-03-05T00:00:00Z", "2027-03-20T00:00:00Z"},
		{"pinned day after done", "every month on day 15 after done", "", "2027-03-20T08:00:00Z", "2027-04-15T00:00:00Z"},
		{"every 3 months", "every 3 months", "2026-11-30T00:00:00Z", "2026-11-30T00:00:00Z", "2027-02-28T00:00:00Z"},
		{"month across a year", "every month", "2026-12-15T00:00:00Z", "2026-12-15T00:00:00Z", "2027-01-15T00:00:00Z"},
		{"yearly", "every year", "2026-11-02T00:00:00Z", "2026-11-02T00:00:00Z", "2027-11-02T00:00:00Z"},
		{"leap day clamps", "every year", "2028-02-29T00:00:00Z", "2028-02-29T00:00:00Z", "2029-02-28T00:00:00Z"},
		{"leap day every 4 years", "every 4 years", "2028-02-29T00:00:00Z", "2028-02-29T00:00:00Z", "2032-02-29T00:00:00Z"},
		{"weekday later this week", "every 2 weeks on mon,thu", "2026-11-02T00:00:00Z", "2026-11-02T00:00:00Z", "2026-11-05T00:00:00Z"},
		{"weekday jumps the gap", "every 2 weeks on mon,thu", "2026-11-05T00:00:00Z", "2026-11-05T00:00:00Z", "2026-11-16T00:00:00Z"},
		{"weekday from a sunday", "every week on mon", "2026-11-08T00:00:00Z", "2026-11-08T00:00:00Z", "2026-11-09T00:00:00Z"},
		{"weekday sunday is the week's end", "every 2 weeks on sun", "2026-11-02T00:00:00Z", "2026-11-02T00:00:00Z", "2026-11-08T00:00:00Z"},
		{"weekday late", "every week on wed", "2026-10-07T00:00:00Z", "2026-10-29T12:00:00Z", "2026-11-04T00:00:00Z"},
		{"weekday after done", "every week on fri after done", "2026-10-01T00:00:00Z", "2026-10-28T12:00:00Z", "2026-10-30T00:00:00Z"},
		{"until reached on the day", "every week until 2026-11-09", "2026-11-02T10:00:00Z", "2026-11-02T10:00:00Z", "2026-11-09T10:00:00Z"},
		{"until passed", "every week until 2027-07-05", "2027-06-30T00:00:00Z", "2027-06-30T00:00:00Z", ""},
		{"count left", "every day for 2 times", "2026-11-02T00:00:00Z", "2026-11-02T00:00:00Z", "2026-11-03T00:00:00Z"},
		{"count run out", "every day for 1 time", "2026-11-02T00:00:00Z", "2026-11-02T00:00:00Z", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := ParseRecurrence(c.rule)
			if err != nil {
				t.Fatal(err)
			}
			var due *time.Time
			if c.due != "" {
				d := day(c.due)
				due = &d
			}
			got, ok := r.Next(due, day(c.done))
			if c.want == "" {
				if ok {
					t.Fatalf("Next = %s, want the rule to have run out", got)
				}
				return
			}
			if !ok {
				t.Fatalf("Next ran out, want %s", c.want)
			}
			if !got.Equal(day(c.want)) {
				t.Fatalf("Next = %s, want %s", got.Format(time.RFC3339), c.want)
			}
		})
	}
}

func TestRecurrenceNextKeepsWallClockAcrossDST(t *testing.T) {
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Skipf("no zone database: %v", err)
	}
	cases := []struct {
		name string
		rule string
		due  time.Time
		want time.Time
		gap  time.Duration
	}{
		{"spring forward", "every day", time.Date(2027, 3, 27, 9, 0, 0, 0, sofia), time.Date(2027, 3, 28, 9, 0, 0, 0, sofia), 23 * time.Hour},
		{"fall back", "every week", time.Date(2026, 10, 22, 9, 0, 0, 0, sofia), time.Date(2026, 10, 29, 9, 0, 0, 0, sofia), 169 * time.Hour},
		{"month across the change", "every month", time.Date(2027, 3, 15, 9, 0, 0, 0, sofia), time.Date(2027, 4, 15, 9, 0, 0, 0, sofia), 31*24*time.Hour - time.Hour},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := ParseRecurrence(c.rule)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := r.Next(&c.due, c.due)
			if !ok || !got.Equal(c.want) {
				t.Fatalf("Next = %s, %v; want %s", got, ok, c.want)
			}
			if h := got.In(sofia).Hour(); h != 9 {
				t.Fatalf("Next lands at %02d:00 local, want 09:00", h)
			}
			if gap := got.Sub(c.due); gap != c.gap {
				t.Fatalf("Next is %s after the due date, want %s across the offset change", gap, c.gap)
			}
		})
	}
}

func TestRecurrenceSuccessor(t *testing.T) {
	r, _ := ParseRecurrence("every day for 3 times")
	if got := r.Successor().String(); got != "every day for 2 times" {
		t.Fatalf("Successor = %q", got)
	}
	r, _ = ParseRecurrence("every day")
	if got := r.Successor().String(); got != "every day" {
		t.Fatalf("Successor = %q", got)
	}
	if (Recurrence{}).String() != "" {
		t.Fatal("the zero rule renders as text")
	}
	if _, ok := (Recurrence{}).Next(nil, time.Now()); ok {
		t.Fatal("the zero rule produced an occurrence")
	}
}
