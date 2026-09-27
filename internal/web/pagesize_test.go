// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// taskRow matches one row of ul.tasklist and nothing else on the page. The
// whole document would not do: the titles also appear in the shortcut data and
// a count anywhere else on the screen would stand in for the rows themselves.
var taskRow = regexp.MustCompile(`<li id="t-[^"]+" class="task`)

// taskRows counts the rows the task list rendered, reading ul.tasklist alone.
// A missing or unclosed list fails rather than counting zero, which would read
// as a page size taking effect.
func taskRows(t *testing.T, page string) int {
	t.Helper()
	_, after, ok := strings.Cut(page, `<ul class="tasklist">`)
	if !ok {
		t.Fatalf("the page carries no task list:\n%s", tail(page))
	}
	list, _, ok := strings.Cut(after, "</ul>")
	if !ok {
		t.Fatalf("the task list is not closed")
	}
	return len(taskRow.FindAllString(list, -1))
}

// pageThrough renders one path through a handler built with custom options,
// signed in the way the rest of this package's tests sign in.
func (f *fixture) pageThrough(t *testing.T, handler http.Handler, path string) string {
	t.Helper()
	server := httptest.NewServer(f.authenticate(handler))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set(actorHeader, "alice")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("getting %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d", path, resp.StatusCode)
	}
	return readAll(t, resp)
}

// TestTheTaskListShowsTheConfiguredNumberOfRows is the guard for the number
// itself: rows counted inside ul.tasklist, through a handler configured with a
// page size no default could produce.
func TestTheTaskListShowsTheConfiguredNumberOfRows(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 9 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}

	page := f.pageThrough(t, web.Handler(f.svc, web.WithPageSize(4)), "/tasks")
	if got := taskRows(t, page); got != 4 {
		t.Errorf("the task list rendered %d rows with web.page_size 4, want 4", got)
	}
	// A short list has to be a page rather than the end of the board.
	if !strings.Contains(page, "pager-next") {
		t.Error("the task list offers no next page, so the short page was the whole listing")
	}
}

// TestTheBrowserDefaultsToTwentyFiveRows names the default the browser gets
// with nothing configured, which is the number this change exists for.
func TestTheBrowserDefaultsToTwentyFiveRows(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 30 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}

	if got := taskRows(t, b.page("/tasks")); got != 25 {
		t.Errorf("the task list rendered %d rows with nothing configured, want 25", got)
	}
}

// TestAReaderCanAskOnePageForADifferentSize is the affordance the browser had
// none of: configuration decided the number and nothing else could.
func TestAReaderCanAskOnePageForADifferentSize(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 9 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}

	if got := taskRows(t, b.page("/tasks?limit=7")); got != 7 {
		t.Errorf("/tasks?limit=7 rendered %d rows, want 7", got)
	}
	// The asked-for size has to survive the walk, or Next silently returns
	// the reader to the configured size.
	next := hrefWithClass(t, b.page("/tasks?limit=3"), "pager-next")
	if !strings.Contains(next, "limit=3") {
		t.Errorf("the next-page link dropped the requested size: %s", next)
	}
	if got := taskRows(t, b.page(next)); got != 3 {
		t.Errorf("page two of a limit=3 walk rendered %d rows, want 3", got)
	}
}

// TestAnUnusableRequestedSizeFallsBackToTheConfiguredOne covers the parameter
// as what it is: text from the address bar. It must cost the reader the size
// they asked for and never the screen.
func TestAnUnusableRequestedSizeFallsBackToTheConfiguredOne(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 9 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}
	handler := web.Handler(f.svc, web.WithPageSize(4))

	for _, raw := range []string{"0", "-3", "abc", strconv.Itoa(core.MaxPageLimit + 1), ""} {
		if got := taskRows(t, f.pageThrough(t, handler, "/tasks?limit="+raw)); got != 4 {
			t.Errorf("/tasks?limit=%q rendered %d rows, want the configured 4", raw, got)
		}
	}
}

// The range the parameter accepts is 1..MaxPageLimit, and the test above only
// names values outside it. Both ends have to be inside: refusing one more
// value at either end reads exactly the same there, and costs a reader the
// single-row page a phone asks for.
func TestTheEndsOfTheAcceptedPageSizeAreAccepted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 3 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}
	handler := web.Handler(f.svc, web.WithPageSize(2))

	if got := taskRows(t, f.pageThrough(t, handler, "/tasks?limit=1")); got != 1 {
		t.Errorf("/tasks?limit=1 rendered %d rows, want 1", got)
	}
	widest := strconv.Itoa(core.MaxPageLimit)
	if got := taskRows(t, f.pageThrough(t, handler, "/tasks?limit="+widest)); got != 3 {
		t.Errorf("/tasks?limit=%s rendered %d rows, want all 3", widest, got)
	}
}

// WithPageSize accepts the same range, and the test for the values it refuses
// names only values outside it. Both ends have to be kept, or configuring the
// range's own endpoint silently leaves the shipped default in place.
func TestTheEndsOfTheConfigurablePageSizeAreKept(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	// More rows than the shipped default carries, so refusing the widest
	// value is visible rather than hidden behind a default wide enough.
	rows := core.DefaultDisplayLimit + 1
	for i := range rows {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}

	one := web.Handler(f.svc, web.WithPageSize(1))
	if got := taskRows(t, f.pageThrough(t, one, "/tasks")); got != 1 {
		t.Errorf("a handler configured for 1 row rendered %d", got)
	}
	widest := web.Handler(f.svc, web.WithPageSize(core.MaxPageLimit))
	if got := taskRows(t, f.pageThrough(t, widest, "/tasks")); got != rows {
		t.Errorf("a handler configured for the widest page rendered %d of %d rows", got, rows)
	}
}

// TestAnUnusableConfiguredSizeLeavesTheShippedDefault keeps WithPageSize from
// putting a number the contract refuses into every query.
func TestAnUnusableConfiguredSizeLeavesTheShippedDefault(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 30 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}

	for _, rows := range []int{0, -1, core.MaxPageLimit + 1} {
		page := f.pageThrough(t, web.Handler(f.svc, web.WithPageSize(rows)), "/tasks")
		if got := taskRows(t, page); got != core.DefaultDisplayLimit {
			t.Errorf("WithPageSize(%d) rendered %d rows, want the shipped %d",
				rows, got, core.DefaultDisplayLimit)
		}
	}
}
