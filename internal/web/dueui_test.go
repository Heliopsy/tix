// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// dueRow returns the one list row naming a task, so an assertion about that
// row cannot pass on the row beside it. The listing is one <li> per task.
func dueRow(t *testing.T, page, title string) string {
	t.Helper()
	_, after, ok := strings.Cut(page, `<ul class="tasklist">`)
	if !ok {
		t.Fatalf("the page carries no task list")
	}
	list, _, _ := strings.Cut(after, "</ul>")
	for _, row := range strings.Split(list, "<li ")[1:] {
		if strings.Contains(row, title) {
			return row
		}
	}
	t.Fatalf("no row names %q in:\n%s", title, list)
	return ""
}

// seedDeadlines creates three tasks whose deadlines are absolute rather than
// relative, so which of them a window selects does not depend on the day the
// suite runs.
func seedDeadlines(t *testing.T, f *fixture) {
	t.Helper()
	for _, tc := range []struct {
		title string
		due   *time.Time
	}{
		{"rotate the keys", ptr(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC))},
		{"replace the racks", ptr(time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))},
		{"tidy the cupboard", nil},
	} {
		if _, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{
			ProjectRef: "infra", Title: tc.title, DueAt: tc.due,
		}); err != nil {
			t.Fatalf("creating %q: %v", tc.title, err)
		}
	}
}

func ptr(t time.Time) *time.Time { return &t }

// TestTheListingMarksOnlyAPressingDeadline is the browser's half of the rule
// the board follows. A badge on every dated row marks nearly every row in a
// real backlog, which is why the terminal's own due marker was removed once.
func TestTheListingMarksOnlyAPressingDeadline(t *testing.T) {
	f := newFixture(t)
	seedDeadlines(t, f)
	page := f.as("alice").page("/tasks")

	if row := dueRow(t, page, "rotate the keys"); !strings.Contains(row, `class="duemark is-overdue"`) {
		t.Errorf("the overdue row carries no deadline badge:\n%s", row)
	}
	if row := dueRow(t, page, "replace the racks"); strings.Contains(row, "duemark") {
		t.Errorf("a deadline in 2099 is badged as pressing:\n%s", row)
	}
	if row := dueRow(t, page, "tidy the cupboard"); strings.Contains(row, "duemark") {
		t.Errorf("a row with no deadline carries a deadline badge:\n%s", row)
	}
	// The badge is a word, not a colour: a reader on a monochrome screen, or
	// one who cannot tell the two hues apart, still reads which it is.
	if row := dueRow(t, page, "rotate the keys"); !strings.Contains(row, ">overdue<") {
		t.Errorf("the badge says nothing without its colour:\n%s", row)
	}
}

// TestTheDeadlineControlNarrowsTheListing is the control the list had no way
// to offer: "what is overdue" was answerable over the API and by typing an
// expression, and by no control a reader could reach.
func TestTheDeadlineControlNarrowsTheListing(t *testing.T) {
	f := newFixture(t)
	seedDeadlines(t, f)
	b := f.as("alice")

	page := b.page("/tasks?" + url.Values{"due": {"overdue"}}.Encode())
	if !strings.Contains(page, "rotate the keys") {
		t.Fatalf("the overdue task is missing from an overdue listing:\n%s", tail(page))
	}
	for _, absent := range []string{"replace the racks", "tidy the cupboard"} {
		if strings.Contains(page, absent) {
			t.Errorf("an overdue listing still carries %q", absent)
		}
	}
	// The control's own state comes back selected, so a reader can see which
	// question the page in front of them answers.
	if !strings.Contains(page, `<option value="overdue" selected>`) {
		t.Errorf("the deadline control does not show its own selection:\n%s", tail(page))
	}

	// It is the same question the expression asks, answered by the same
	// parser: a control that set a bound of its own could drift from the
	// language, which is exactly how the web's second filter parser went
	// wrong before it was deleted.
	typed := b.page("/tasks?" + url.Values{"q": {"due:overdue"}}.Encode())
	if strings.Contains(typed, "replace the racks") || !strings.Contains(typed, "rotate the keys") {
		t.Errorf("due:overdue typed into the box selects something else:\n%s", tail(typed))
	}
}

// TestTheDeadlineControlRefusesAWindowItDoesNotHave keeps a bad value on the
// screen with its message, rather than answering a different question.
func TestTheDeadlineControlRefusesAWindowItDoesNotHave(t *testing.T) {
	f := newFixture(t)
	seedDeadlines(t, f)
	page := f.as("alice").page("/tasks?" + url.Values{"due": {"whenever"}}.Encode())
	if !strings.Contains(page, "filterfault") {
		t.Fatalf("an unknown deadline window was accepted silently:\n%s", tail(page))
	}
	if strings.Contains(page, "rotate the keys") {
		t.Errorf("a refused filter still listed rows:\n%s", tail(page))
	}
}

// TestTheTaskScreenNamesTheDeadlineState covers the detail, where the date was
// drawn bare and left the reader comparing it with a calendar.
func TestTheTaskScreenNamesTheDeadlineState(t *testing.T) {
	f := newFixture(t)
	seedDeadlines(t, f)
	b := f.as("alice")
	list := b.page("/tasks")
	ref := refOf(t, list, "rotate the keys")

	page := b.page("/tasks/" + ref)
	if !strings.Contains(page, `class="duemark is-overdue"`) {
		t.Errorf("the task screen does not say the deadline has passed:\n%s", tail(page))
	}
	other := b.page("/tasks/" + refOf(t, list, "replace the racks"))
	if strings.Contains(other, "duemark") {
		t.Errorf("a deadline in 2099 is marked pressing on the task screen:\n%s", tail(other))
	}
}

// refOf reads the reference out of the row naming a task.
func refOf(t *testing.T, page, title string) string {
	t.Helper()
	row := dueRow(t, page, title)
	_, after, ok := strings.Cut(row, `id="t-`)
	if !ok {
		t.Fatalf("the row carries no reference:\n%s", row)
	}
	ref, _, _ := strings.Cut(after, `"`)
	return ref
}
