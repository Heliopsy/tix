// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/quickadd"
)

// dateValue fills a due date field with the day t falls on, written the way
// the reader types a date, so saving the field unchanged keeps the day.
func dateValue(style output.TimeStyle, t *time.Time) string {
	if t == nil {
		return ""
	}
	return style.Date(t.UTC())
}

// dueExamples are the values a due date field's tip offers, the date among
// them written in the reader's own format.
func dueExamples(style output.TimeStyle) []string {
	return quickadd.DueExamples(style.Date(quickadd.ExampleDay))
}

// quickDueExamples are the due: forms the quick-add tip offers. A line splits
// on spaces, so a format whose date has spaces in it offers YYYY-MM-DD there.
func quickDueExamples(style output.TimeStyle) []string {
	date := style.Date(quickadd.ExampleDay)
	if strings.ContainsRune(date, ' ') {
		date = quickadd.ExampleDay.Format(time.DateOnly)
	}
	ex := quickadd.DueExamples(date)
	out := make([]string, len(ex))
	for i, v := range ex {
		out[i] = "due:" + v
	}
	return out
}
