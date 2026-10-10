// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// token encodes a cursor the way the store issues one, so the tests below
// exercise the validation against real values rather than against strings
// that merely look like them.
func token(id string) string {
	return core.Cursor{SortValue: id, ID: id, Sort: "created_at", Direction: core.Ascending}.Encode()
}

// request builds a GET for one listing URL.
func request(target string) *http.Request {
	return httptest.NewRequest(http.MethodGet, target, nil)
}

// The trail arrives from the address bar, so it is whatever anybody cares to
// type. Anything this build did not issue is dropped whole, which costs a
// reader a return to the first page and can cost them nothing worse.
func TestTrailAcceptsOnlyCursorsThisBuildIssued(t *testing.T) {
	t.Parallel()
	good := token("A") + "." + token("B")
	long := make([]string, maxTrailDepth+1)
	for i := range long {
		long[i] = token("x")
	}

	cases := []struct {
		name   string
		trail  string
		want   []string
		reason string
	}{
		{"absent", "", nil, "nothing to walk back through"},
		{"two cursors", good, []string{token("A"), token("B")}, "both were issued here"},
		{"not base64", "!!!!", nil, "no cursor decodes from that"},
		{"valid base64, not a cursor", "YWJj", nil, "decodes, but not to a cursor"},
		{"an empty entry", token("A") + ".." + token("B"), nil,
			"an empty entry would make Previous skip every page between here and the start"},
		{"one bad entry among good ones", good + ".!!!", nil, "a half-kept trail walks back to a page nobody was on"},
		{"deeper than the cap", strings.Join(long, trailSeparator), nil, "the cap bounds what a link can carry"},
		{"longer than the byte cap", strings.Repeat("A", maxTrailLength+1), nil, "the byte cap bounds it too"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := trailFrom(request("/tasks?" + url.Values{TrailParam: {c.trail}}.Encode()))
			if len(got) != len(c.want) {
				t.Fatalf("trailFrom(%q) = %v, want %v (%s)", c.trail, got, c.want, c.reason)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("trailFrom(%q)[%d] = %q, want %q", c.trail, i, got[i], c.want[i])
				}
			}
		})
	}
}

// Previous pops the trail rather than asking the store for a backwards
// keyset, and the first page is the one with no cursor at all.
func TestPagerWalksBackDownItsTrail(t *testing.T) {
	t.Parallel()
	a, b, c := token("A"), token("B"), token("C")

	third := newPager(request("/tasks?q=tag%3Aops&"+url.Values{
		CursorParam: {c}, TrailParam: {a + trailSeparator + b},
	}.Encode()), RouteTasks, token("D"), 50, "tasks", "q", "sort")

	if !third.HasPrev() || !third.HasNext() {
		t.Fatal("a page in the middle of a walk offers both directions")
	}
	if third.Page() != 4 {
		t.Errorf("Page() = %d, want 4", third.Page())
	}
	prev := third.PrevHref()
	if !strings.Contains(prev, url.QueryEscape(b)) {
		t.Errorf("Previous = %q, want it to move back to the last cursor of the trail", prev)
	}
	if strings.Contains(prev, url.QueryEscape(c)) {
		t.Errorf("Previous = %q, still carries the cursor of the page it is leaving", prev)
	}
	if !strings.Contains(prev, "q=tag%3Aops") {
		t.Errorf("Previous = %q, dropped the filter it was showing", prev)
	}

	next := third.NextHref()
	for _, want := range []string{url.QueryEscape(a), url.QueryEscape(b), url.QueryEscape(c)} {
		if !strings.Contains(next, want) {
			t.Errorf("Next = %q, want the current cursor pushed onto the trail", next)
		}
	}
}

// Previous is absent on the first page, not disabled: there is nowhere behind
// it, and a control that does nothing is worse than no control.
func TestFirstPageOffersNoPrevious(t *testing.T) {
	t.Parallel()
	first := newPager(request("/tasks"), RouteTasks, token("A"), 50, "tasks", "q", "sort")
	if first.HasPrev() {
		t.Error("the first page offers a way back")
	}
	if first.Page() != 1 {
		t.Errorf("Page() = %d, want 1", first.Page())
	}
	if !first.Show() {
		t.Error("a listing with a second page renders no pager")
	}
}

// A listing that fits on one page renders nothing at all rather than a pair
// of dead controls.
func TestASinglePageRendersNoPager(t *testing.T) {
	t.Parallel()
	only := newPager(request("/actors"), RouteActors, "", 3, "actors")
	if only.Show() {
		t.Error("a listing with one page still renders a pager")
	}
}

// Walking past the cap keeps working and only shortens the memory, from the
// oldest end, since Previous is walked from the newest one.
func TestTheTrailStopsGrowingAtTheCap(t *testing.T) {
	t.Parallel()
	full := make([]string, maxTrailDepth)
	for i := range full {
		full[i] = token("x")
	}
	at := newPager(request("/tasks?"+url.Values{
		CursorParam: {token("here")}, TrailParam: {strings.Join(full, trailSeparator)},
	}.Encode()), RouteTasks, token("next"), 50, "tasks")

	trail := at.NextHref()
	parsed, err := url.Parse(trail)
	if err != nil {
		t.Fatalf("parsing %q: %v", trail, err)
	}
	got := strings.Split(parsed.Query().Get(TrailParam), trailSeparator)
	if len(got) != maxTrailDepth {
		t.Fatalf("the trail grew to %d entries, want it capped at %d", len(got), maxTrailDepth)
	}
	if got[len(got)-1] != token("here") {
		t.Error("the page being left was not pushed onto the trail")
	}
}

// The position indicator stalled past the trail's bound. Page() was
// len(Trail)+2 and the trail stops growing at maxTrailDepth, so page 42 and
// every page after it reported "Page 42": the one number on the control that
// says where the reader is froze while they kept walking.
//
// The walk is driven through the pager's own links rather than from
// constructed URLs, because the defect was that the links stopped carrying
// enough to count with. A test that built each page's query itself could not
// have seen it.
func TestThePageNumberKeepsCountingPastTheTrailsBound(t *testing.T) {
	t.Parallel()

	// Start on page 2, the first page with a cursor, and walk forward.
	target := RouteTasks + "?" + url.Values{CursorParam: {token("c2")}}.Encode()
	var at pager
	for n := 2; n <= maxTrailDepth+13; n++ {
		at = newPager(request(target), RouteTasks, token("c"+strconv.Itoa(n+1)), 50, "tasks")
		if got := at.Page(); got != n {
			t.Fatalf("walking forward reached page %d but the control says %d", n, got)
		}
		target = at.NextHref()
	}

	// And back down, which is the half a trimmed trail cannot answer on its
	// own: the oldest cursors were dropped, so only the carried number knows
	// where the walk is. target now addresses the page after the last one
	// walked forward to.
	//
	// Previous is followed until it runs out, which it does once the trail is
	// empty: the oldest cursors were trimmed away, so the walk back ends on
	// the cursorless first page rather than on page 2. That is the trimming's
	// own cost and is not what this guards; the number counting down by
	// exactly one for every page it does reach is.
	page := at.Page()
	if page != maxTrailDepth+13 {
		t.Fatalf("the forward walk ended on page %d, want %d", page, maxTrailDepth+13)
	}
	steps := 0
	for at.HasPrev() {
		back := newPager(request(at.PrevHref()), RouteTasks, token("onward"), 50, "tasks")
		want := page - 1
		if back.Page() == 1 && !back.HasPrev() {
			break
		}
		if got := back.Page(); got != want {
			t.Fatalf("Previous from page %d reached a control that says %d, want %d", page, got, want)
		}
		at, page = back, want
		steps++
	}
	if steps < maxTrailDepth-1 {
		t.Fatalf("Previous only walked back %d pages, want the %d the trail remembers", steps, maxTrailDepth-1)
	}
	home := newPager(request(at.PrevHref()), RouteTasks, token("onward"), 50, "tasks")
	if got := home.Page(); got != 1 || home.HasPrev() {
		t.Fatalf("the trail ran out and Previous landed on page %d, want the cursorless first page", got)
	}
}

// No page short of the bound grows a parameter for a number its trail already
// proves, and a number a reader typed in cannot drop the count below what the
// trail holds.
func TestThePageNumberIsWrittenOnlyWhereTheTrailCannotCount(t *testing.T) {
	t.Parallel()

	early := newPager(request(RouteTasks+"?"+url.Values{
		CursorParam: {token("c2")},
	}.Encode()), RouteTasks, token("c3"), 50, "tasks")
	for _, href := range []string{early.NextHref(), early.PrevHref()} {
		parsed, err := url.Parse(href)
		if err != nil {
			t.Fatalf("parsing %q: %v", href, err)
		}
		if got := parsed.Query().Get(PageParam); got != "" {
			t.Errorf("%q carries a page number (%q) its trail already counts", href, got)
		}
	}

	full := make([]string, maxTrailDepth)
	for i := range full {
		full[i] = token("x")
	}
	trail := strings.Join(full, trailSeparator)
	for _, typed := range []string{"", "1", "7", "-3", "not a number", strconv.Itoa(maxPage + 1)} {
		at := newPager(request(RouteTasks+"?"+url.Values{
			CursorParam: {token("here")}, TrailParam: {trail}, PageParam: {typed},
		}.Encode()), RouteTasks, token("next"), 50, "tasks")
		if got := at.Page(); got != maxTrailDepth+2 {
			t.Errorf("page=%q gave Page() = %d, want the trail's own count of %d",
				typed, got, maxTrailDepth+2)
		}
	}
}

// The pager writes its own position and carries nothing else, so the filter
// form's plain GET, which submits neither cursor nor trail, resets the walk.
// A trail is a record of positions in one ordered result set: change the
// filter or the sort and its cursors address a different set, or a different
// ordering of it, and following them back would land the reader on rows that
// were never on their screen.
func TestThePagerCarriesOnlyTheListingsOwnParameters(t *testing.T) {
	t.Parallel()
	at := newPager(request("/tasks?"+url.Values{
		"q": {"tag:ops"}, "sort": {"urgency"}, "flash": {"completed"},
		CursorParam: {token("A")},
	}.Encode()), RouteTasks, token("B"), 50, "tasks", "q", "sort")

	for _, href := range []string{at.NextHref(), at.PrevHref()} {
		parsed, err := url.Parse(href)
		if err != nil {
			t.Fatalf("parsing %q: %v", href, err)
		}
		if parsed.Query().Get("flash") != "" {
			t.Errorf("%q carries a flash message forward", href)
		}
		if parsed.Query().Get("q") != "tag:ops" || parsed.Query().Get("sort") != "urgency" {
			t.Errorf("%q dropped the filter or the sort", href)
		}
	}
}
