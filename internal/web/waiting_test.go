// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// waitingPair creates a task and a second one depending on it.
func waitingPair(t *testing.T, b *browser) (first, waiting string) {
	t.Helper()
	first = b.createTask("infra", "lay the cable")
	waiting = b.createTask("infra", "light the room")
	resp := b.post("/tasks/"+waiting+"/deps", url.Values{"depends_on": {first}})
	if resp.StatusCode >= http.StatusBadRequest {
		t.Fatalf("adding a dependency answered %d", resp.StatusCode)
	}
	return first, waiting
}

// summaryOf is the summary paragraph of the task screen, nothing else.
func summaryLine(t *testing.T, page string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `<p class="lede summary">`)
	if !ok {
		t.Fatalf("no summary on the page")
	}
	line, _, _ := strings.Cut(rest, "</p>")
	return line
}

func TestTheHeadingSaysWaitingAndLinksToTheFilter(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	waitingPair(t, b)
	line := summaryLine(t, b.page("/tasks"))
	if strings.Contains(line, "blocked") {
		t.Errorf("the heading still calls a dependency wait blocked: %s", line)
	}
	if !strings.Contains(line, "1 waiting on another task") {
		t.Errorf("the heading does not count the waiting task: %s", line)
	}
	if !strings.Contains(line, `href="/tasks?q=is%3Awaiting-on"`) {
		t.Errorf("the waiting count does not link to is:waiting-on: %s", line)
	}
}

func TestAWaitingTaskIsMarkedInEveryView(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	first, waiting := waitingPair(t, b)
	for _, view := range []string{"list", "board", "lanes"} {
		t.Run(view, func(t *testing.T) {
			page := b.page("/tasks?view=" + view)
			_, item, ok := strings.Cut(page, `id="t-`+waiting+`"`)
			if !ok {
				t.Fatalf("no element for %s", waiting)
			}
			for _, end := range []string{"</li>", "</article>"} {
				if before, _, ok := strings.Cut(item, end); ok {
					item = before
				}
			}
			mark := strings.ToLower(`class="waitmark" href="/tasks/` + first + `"`)
			low := strings.ToLower(item)
			if !strings.Contains(low, mark) || !strings.Contains(low, strings.ToLower("waiting on "+first)) {
				t.Errorf("%s carries no waiting mark naming %s:\n%s", waiting, first, item)
			}
			_, firstItem, _ := strings.Cut(page, `id="t-`+first+`"`)
			if before, _, ok := strings.Cut(firstItem, `id="t-`); ok {
				firstItem = before
			}
			if strings.Contains(firstItem, "waitmark") {
				t.Errorf("%s waits on nothing but is marked", first)
			}
		})
	}
}

func TestTheSummaryCountsEveryPage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 12 {
		b.createTask("infra", fmt.Sprintf("task %d", i))
	}
	line := summaryLine(t, b.page("/tasks?limit=10"))
	if !strings.Contains(line, "<strong>12</strong>") {
		t.Errorf("the heading counts the page rather than the selection: %s", line)
	}
}

func TestBoardColumnsAreNotPagedTogether(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 52 {
		b.createTask("infra", fmt.Sprintf("todo %d", i))
	}
	page := b.page("/projects/infra?view=board&size=10&page=2")
	_, col, ok := strings.Cut(page, `data-state="todo"`)
	if !ok {
		t.Fatalf("no todo column")
	}
	col, _, _ = strings.Cut(col, "</section>")
	if !strings.Contains(col, "(52)") {
		t.Errorf("the todo heading does not count the whole state:\n%.400s", col)
	}
	if n := strings.Count(col, "<article id="); n != 50 {
		t.Errorf("the todo column draws %d cards, want the first 50 whatever the page", n)
	}
	if !strings.Contains(col, "Show 2 more") || !strings.Contains(col, "more=todo") {
		t.Errorf("the todo column offers no Show more for the rest")
	}
	more := b.page("/projects/infra?view=board&more=todo")
	_, col, _ = strings.Cut(more, `data-state="todo"`)
	col, _, _ = strings.Cut(col, "</section>")
	if n := strings.Count(col, "<article id="); n != 52 {
		t.Errorf("Show more drew %d cards, want all 52", n)
	}
}
