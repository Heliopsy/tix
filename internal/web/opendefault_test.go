// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The open-work default is a default and not a filter, so everything that
// carries the reader from one view of the listing to the next has to carry it
// too. These cover the three places it was dropped.

// A cursor is a position in one ordered result set. The pager copied the
// filter parameters onto the next-page link and left the open-work default
// behind, so Next landed on a page narrowed back to the open states while the
// cursor still addressed the walk over everything: page two was both the
// wrong set and offset into another one, and the rows in between were skipped
// with nothing on screen saying so.
func TestTheNextPageKeepsTheFinishedWorkTheReaderAskedFor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 4 {
		ref := b.createTask("infra", fmt.Sprintf("finished %d", i))
		done := b.post("/tasks/"+ref+"/complete", url.Values{})
		_ = done.Body.Close()
		wantStatus(t, done, http.StatusSeeOther)
	}

	page := b.page("/tasks?all=1&limit=2")
	next := hrefWithClass(t, page, "pager-next")
	if next == "" {
		t.Fatalf("a listing of four tasks two at a time offers no next page:\n%s", page)
	}
	if !strings.Contains(next, "all=1") {
		t.Errorf("the next page drops the request for finished work: %q", next)
	}

	// The link is followed rather than only read: the parameter being on it
	// proves nothing if the page it leads to narrows anyway.
	second := b.page(next)
	summary := between(t, second, `class="lede summary"`, "</p>")
	if strings.Contains(summary, "open work only") {
		t.Errorf("page two of a listing of everything narrowed itself back:\n%s", summary)
	}
	if !strings.Contains(summary, "done") {
		t.Errorf("page two of a listing of everything counts no finished work:\n%s", summary)
	}
}

// The board's columns are the workflow, terminal states included, so a board
// narrowed to the open states has terminal columns no card can ever be in. A
// reader who finished a card watched it leave the board, and the only way
// back was a link in the list view's summary.
func TestTheBoardKeepsTheColumnsACardIsMovedInto(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "finished on the board")
	done := b.post("/tasks/"+ref+"/complete", url.Values{})
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)
	b.chooseBoard("/tasks")

	page := b.page("/tasks")
	column := boardColumn(t, page, "done")
	if !strings.Contains(column, `data-task="`+ref+`"`) {
		t.Errorf("a finished card is not in the Done column it was moved into:\n%s", column)
	}

	// The affordance belongs to the narrowed list. A board that was never
	// narrowed must not claim it was.
	summary := between(t, page, `class="lede summary"`, "</p>")
	if strings.Contains(summary, "open work only") {
		t.Errorf("the board says it is narrowed when it is not:\n%s", summary)
	}
}

// The way off the default was written out as /tasks?all=1, so following it
// from a sorted listing of a hundred rows returned a reader to the default
// sort and the default page size.
func TestShowingFinishedWorkKeepsTheListingTheReaderBuilt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "still open")

	page := b.page("/tasks?sort=created_at&limit=7")
	summary := between(t, page, `class="lede summary"`, "</p>")
	href := between(t, summary, `<a href="`, `"`)
	if href == "" {
		t.Fatalf("the narrowed summary offers no way off:\n%s", summary)
	}
	href = strings.ReplaceAll(href, "&amp;", "&")
	for _, want := range []string{"all=1", "sort=created_at", "limit=7"} {
		if !strings.Contains(href, want) {
			t.Errorf("showing finished work drops %q: %q", want, href)
		}
	}
}
