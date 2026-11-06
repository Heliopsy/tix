// SPDX-License-Identifier: AGPL-3.0-or-later

package quickadd

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/output"
)

// A date written with digits is read in the order the reader writes dates,
// and one whose order is not known is refused rather than guessed.
func TestParseDueInReadsDigitsInTheReadersOrder(t *testing.T) {
	t.Parallel()
	nov6 := day(2026, 11, 6)
	jun11 := day(2026, 6, 11)
	cases := []struct {
		name  string
		value string
		order output.DateOrder
		want  *time.Time
	}{
		{"iso under every order", "2026-11-06", output.DateDayFirst, nov6},
		{"iso alone", "2026-11-06", output.DateISO, nov6},
		{"dotted day first", "06.11.2026", output.DateDayFirst, nov6},
		{"dotted without zeros", "6.11.2026", output.DateDayFirst, nov6},
		{"slashed day first", "06/11/2026", output.DateDayFirst, nov6},
		{"slashed month first", "06/11/2026", output.DateMonthFirst, jun11},
		{"month name", "6 Nov 2026", output.DateISO, nov6},
		{"long month name", "6 November 2026", output.DateMonthFirst, nov6},
		{"month name first", "Nov 6, 2026", output.DateDayFirst, nov6},
		{"slashes with no order", "06/11/2026", output.DateISO, nil},
		{"dots with no order", "06.11.2026", output.DateISO, nil},
		{"dots are never month first", "11.06.2026", output.DateMonthFirst, nil},
		{"a day that does not exist", "31.02.2026", output.DateDayFirst, nil},
		{"a two digit year", "06.11.26", output.DateDayFirst, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseDueIn(c.value, sunday, c.order)
			if c.want == nil {
				if err == nil {
					t.Fatalf("%q was read as %v, want a refusal", c.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q: %v", c.value, err)
			}
			if !got.Equal(*c.want) {
				t.Errorf("%q = %v, want %v", c.value, got, c.want)
			}
		})
	}
}

// A refusal shows a date written the way this reader would write it.
func TestADueRefusalShowsTheReadersOwnDate(t *testing.T) {
	t.Parallel()
	for order, want := range map[output.DateOrder]string{
		output.DateISO: "2026-11-20", output.DateDayFirst: "20.11.2026", output.DateMonthFirst: "11/20/2026",
	} {
		_, err := ParseDueIn("someday", sunday, order)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("order %d: refusal %v does not show %s", order, err, want)
		}
	}
}

// Every example a hint offers is a value the parser takes, in every style,
// both in a due field and after due: in a quick-add line.
func TestEveryDueExampleParses(t *testing.T) {
	t.Parallel()
	for _, format := range output.TimeFormats {
		style, err := output.NewTimeStyle(format, "utc")
		if err != nil {
			t.Fatal(err)
		}
		for _, ex := range DueExamples(style.Date(ExampleDay)) {
			if _, err := ParseDueIn(ex, sunday, style.DateOrder()); err != nil {
				t.Errorf("%s: the example %q is refused: %v", format, ex, err)
			}
		}
	}
	for _, ex := range DueExamples(ExampleDay.Format(time.DateOnly)) {
		r, err := Parse("x due:"+ex, sunday)
		if err != nil || r.Due == nil {
			t.Errorf("the quick-add example due:%s is refused: %v", ex, err)
		}
	}
}

// A quick-add line reads its due date in the reader's order too.
func TestParseInReadsTheDueDateInOrder(t *testing.T) {
	t.Parallel()
	r, err := ParseIn("pay rent due:06.11.2026", sunday, output.DateDayFirst)
	if err != nil {
		t.Fatal(err)
	}
	if r.Due == nil || !r.Due.Equal(*day(2026, 11, 6)) {
		t.Errorf("due = %v, want 2026-11-06", r.Due)
	}
	if _, err := Parse("pay rent due:06.11.2026", sunday); err == nil {
		t.Error("a line with no order read a dotted date")
	}
}
