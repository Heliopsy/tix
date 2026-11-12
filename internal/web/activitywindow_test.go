// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// activityFilterForm is the feed's filter form, and only it.
func activityFilterForm(t *testing.T, page string) string {
	t.Helper()
	form := between(t, page, `<form method="get" action="/activity" class="filterbar activityfilter">`, "</form>")
	if form == "" {
		t.Fatalf("the feed has no filter form:\n%s", page)
	}
	return form
}

// activityRows is the feed's row list, and only it.
func activityRows(t *testing.T, page string) string {
	t.Helper()
	rows := between(t, page, `id="live-feed"`, `<div class="rawslot">`)
	if rows == "" {
		t.Fatalf("the feed has no row list:\n%s", page)
	}
	return rows
}

// listed reports whether a feed's rows name a task.
func listed(rows, ref string) bool { return strings.Contains(rows, `href="/tasks/`+ref+`"`) }

// The audit log could be narrowed by everything but when and what was done,
// which the app's audit screen and `tix admin audit` both offer. The bounds
// are read in the reader's own zone, which here is nine hours off the clock's.
func TestTheActivityFeedNarrowsToATimeWindow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	zone, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("loading the reader's zone: %v", err)
	}
	set := b.post("/timezone", url.Values{"timezone": {"Asia/Tokyo"}, "next": {"/settings"}})
	_ = set.Body.Close()
	start := f.clock.Now().In(zone)
	early := b.createTask("infra", "made in the morning")
	f.clock.Advance(3 * time.Hour)
	late := b.createTask("infra", "made in the afternoon")

	at := func(d time.Duration) string { return start.Add(d).Format("2006-01-02T15:04") }
	for _, tc := range []struct {
		name, query   string
		kept, dropped string
	}{
		{"from", "from=" + at(time.Hour), late, early},
		{"to", "to=" + at(time.Hour), early, late},
		{"to is inclusive of its minute", "to=" + at(3*time.Hour), late, ""},
		{"a bare date keeps the whole day", "to=" + start.Format("2006-01-02"), late, ""},
		{"both", "from=" + at(-time.Hour) + "&to=" + at(time.Hour), early, late},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := activityRows(t, b.page("/activity?"+tc.query))
			if !listed(rows, tc.kept) {
				t.Errorf("%s dropped %s, which is inside the window:\n%s", tc.query, tc.kept, rows)
			}
			if tc.dropped != "" && listed(rows, tc.dropped) {
				t.Errorf("%s kept %s, which is outside the window:\n%s", tc.query, tc.dropped, rows)
			}
		})
	}
}

// What was done is the action an entry records, matched exactly, the way
// `tix admin audit --action` matches it.
func TestTheActivityFeedNarrowsToAnAction(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "created, never commented")

	created := activityRows(t, b.page("/activity?action=task.create"))
	if !listed(created, ref) {
		t.Errorf("the task.create action dropped a created task:\n%s", created)
	}
	other := activityRows(t, b.page("/activity?action=comment.create"))
	if listed(other, ref) {
		t.Errorf("the comment.create action kept a task that was only created:\n%s", other)
	}
	if !strings.Contains(activityFilterForm(t, b.page("/activity")), `<option value="task.create">`) {
		t.Error("the action box does not suggest the actions on the page")
	}
}

// Each new term is a control on the form, a chip of its own that clears only
// it, and part of every link the screen makes, so paging and a live refresh
// keep the window.
func TestTheActivityWindowAndActionAreShownAndKept(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "inside the window")

	query := "action=task.create&from=2025-12-31&to=2026-01-02T10:30"
	page := b.page("/activity?" + query)

	form := activityFilterForm(t, page)
	for _, want := range []string{
		`name="action" value="task.create"`,
		`type="datetime-local" name="from" value="2025-12-31T00:00"`,
		`type="datetime-local" name="to" value="2026-01-02T10:30"`,
	} {
		if !strings.Contains(form, want) {
			t.Errorf("the filter form does not hold %s:\n%s", want, form)
		}
	}

	chips := activeChips(t, page)
	for name, label := range map[string]string{
		"action": "action task.create",
		"from":   "on or after 2025-12-31",
		"to":     "on or before 2026-01-02 10:30",
	} {
		if !strings.Contains(chips, label) {
			t.Errorf("the %s term has no chip:\n%s", name, chips)
		}
		values, _ := url.ParseQuery(query)
		values.Del(name)
		if !strings.Contains(chips, `href="/activity?`+strings.ReplaceAll(values.Encode(), "&", "&amp;")+`"`) {
			t.Errorf("the %s chip does not clear only its own term:\n%s", name, chips)
		}
	}
	feed := between(t, page, `data-feed="`, `"`)
	for _, name := range []string{"action=", "from=", "to="} {
		if !strings.Contains(feed, name) {
			t.Errorf("the live refresh drops %s: %s", name, feed)
		}
	}
}

// A bound that is not a date is refused rather than quietly ignored, which
// would show the whole log under a filter the reader thinks is applied.
func TestAnUnreadableActivityWindowIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, query := range []string{"from=yesterday", "to=2026-13-01", "from=2026-01-02&to=2026-01-01"} {
		resp := b.get("/activity?" + query)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s answered %d, not a refusal", query, resp.StatusCode)
		}
	}
}
