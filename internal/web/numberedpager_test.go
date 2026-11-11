// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// numberedFixture is a task list of twelve open tasks, row 01 to row 12, one
// to a page under limit=1 and sort=seq, so page N holds row N.
func numberedFixture(t *testing.T) (*fixture, *browser) {
	t.Helper()
	f := newFixture(t)
	b := f.as("alice")
	for i := 1; i <= 12; i++ {
		b.createTask("infra", fmt.Sprintf("row %02d", i))
	}
	return f, b
}

// pagerOf is the pager's own markup, so an assertion cannot be satisfied by
// a number or a word somewhere else on the screen.
func pagerOf(t *testing.T, page string) string {
	t.Helper()
	nav := between(t, page, `<nav class="pager"`, "</nav>")
	if nav == "" {
		t.Fatalf("no pager on the screen:\n%s", tail(page))
	}
	return nav
}

var pageEntry = regexp.MustCompile(`<li class="pager-gap"[^>]*>&hellip;</li>|<span class="pager-num" aria-current="page">(\d+)</span>|<a class="pager-num" href="[^"]*"[^>]*>(\d+)</a>`)

// numberedRun reads the numbered pages in order: a linked page as its number,
// the current page as [N], a gap as an ellipsis.
func numberedRun(t *testing.T, nav string) string {
	t.Helper()
	run := between(t, nav, `<ol class="pager-pages">`, "</ol>")
	out := []string{}
	for _, m := range pageEntry.FindAllStringSubmatch(run, -1) {
		switch {
		case m[1] != "":
			out = append(out, "["+m[1]+"]")
		case m[2] != "":
			out = append(out, m[2])
		default:
			out = append(out, "…")
		}
	}
	return strings.Join(out, " ")
}

// rowsOn is which of the fixture's rows the screen lists.
func rowsOn(page string) []string {
	out := []string{}
	for i := 1; i <= 12; i++ {
		if strings.Contains(page, fmt.Sprintf("row %02d", i)) {
			out = append(out, fmt.Sprintf("row %02d", i))
		}
	}
	return out
}

func TestThePagerSaysHowManyPagesAndLinksThemWithGaps(t *testing.T) {
	t.Parallel()
	_, b := numberedFixture(t)
	cases := []struct {
		page, run, at string
		first, last   bool
	}{
		{"1", "[1] 2 … 12", "Page 1 of 12", false, true},
		{"6", "1 … 5 [6] 7 … 12", "Page 6 of 12", true, true},
		{"3", "1 2 [3] 4 … 12", "Page 3 of 12", true, true},
		{"4", "1 2 3 [4] 5 … 12", "Page 4 of 12", true, true},
		{"9", "1 … 8 [9] 10 11 12", "Page 9 of 12", true, true},
		{"12", "1 … 11 [12]", "Page 12 of 12", true, false},
	}
	for _, tc := range cases {
		t.Run("page "+tc.page, func(t *testing.T) {
			nav := pagerOf(t, b.page("/tasks?limit=1&sort=seq&page="+tc.page))
			if !strings.Contains(nav, tc.at) || !strings.Contains(nav, "12 tasks") {
				t.Errorf("the indicator does not read %q and 12 tasks:\n%s", tc.at, nav)
			}
			if got := numberedRun(t, nav); got != tc.run {
				t.Errorf("numbered pages = %q, want %q", got, tc.run)
			}
			if got := strings.Contains(nav, "pager-first"); got != tc.first {
				t.Errorf("First offered = %v, want %v", got, tc.first)
			}
			if got := strings.Contains(nav, "pager-last"); got != tc.last {
				t.Errorf("Last offered = %v, want %v", got, tc.last)
			}
		})
	}
}

func TestAPageLinkIsANumberAndLandsOnThatPage(t *testing.T) {
	t.Parallel()
	_, b := numberedFixture(t)
	nav := pagerOf(t, b.page("/tasks?limit=1&sort=seq&page=6"))
	for class, want := range map[string]string{"pager-next": "7", "pager-prev": "5", "pager-last": "12"} {
		href := hrefWithClass(t, nav, class)
		u, err := url.Parse(href)
		if err != nil || u.Query().Get("page") != want || u.Query().Has("cursor") || u.Query().Get("sort") != "seq" {
			t.Errorf("%s = %q, want page=%s with the sort and no cursor", class, href, want)
		}
	}
	if href := hrefWithClass(t, nav, "pager-first"); strings.Contains(href, "page=") {
		t.Errorf("First = %q, want the bare listing", href)
	}
	next := b.page(hrefWithClass(t, nav, "pager-next"))
	if got := rowsOn(next); len(got) != 1 || got[0] != "row 07" {
		t.Errorf("following Next from page 6 lists %v, want row 07", got)
	}
}

func TestTheGoToFormWorksWithoutScripting(t *testing.T) {
	t.Parallel()
	_, b := numberedFixture(t)
	nav := pagerOf(t, b.page("/tasks?limit=1&sort=seq&page=2"))
	form := between(t, nav, `<form class="pager-jump" method="get" action="/tasks">`, "</form>")
	if form == "" {
		t.Fatalf("no go-to GET form in the pager:\n%s", nav)
	}
	for _, field := range []string{
		`<input type="hidden" name="limit" value="1">`,
		`<input type="hidden" name="sort" value="seq">`,
		`name="page" min="1" max="12" value="2"`,
	} {
		if !strings.Contains(form, field) {
			t.Errorf("the go-to form lacks %s:\n%s", field, form)
		}
	}
	if strings.Contains(form, "cursor") {
		t.Errorf("the go-to form carries a cursor:\n%s", form)
	}
	submitted := b.page("/tasks?" + url.Values{"limit": {"1"}, "sort": {"seq"}, "page": {"9"}}.Encode())
	if got := rowsOn(submitted); len(got) != 1 || got[0] != "row 09" {
		t.Errorf("going to page 9 lists %v, want row 09", got)
	}
	past := pagerOf(t, b.page("/tasks?limit=1&sort=seq&page=99"))
	if !strings.Contains(past, "Page 12 of 12") {
		t.Errorf("page 99 of 12 is not drawn as the last page:\n%s", past)
	}
}

var boardSwitch = regexp.MustCompile(`href="([^"]*)" data-view="board"`)

func TestThePageSurvivesTheViewSwitchAndAFilterResetsIt(t *testing.T) {
	t.Parallel()
	_, b := numberedFixture(t)
	page := b.page("/tasks?limit=1&sort=seq&page=3")
	m := boardSwitch.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no switch to the board")
	}
	board := html.UnescapeString(m[1])
	u, _ := url.Parse(board)
	if u.Query().Get("page") != "3" || u.Query().Has("cursor") {
		t.Errorf("the board switch = %q, want page=3 and no cursor", board)
	}
	drawn := b.page(board)
	if strings.Contains(drawn, "Page 3 of 12") || strings.Contains(drawn, `rel="next"`) {
		t.Errorf("the board is paged, so its columns are cut at one shared row")
	}
	back := regexp.MustCompile(`href="([^"]*)" data-view="list"`).FindStringSubmatch(drawn)
	if back == nil || !strings.Contains(html.UnescapeString(back[1]), "page=3") {
		t.Errorf("the switch back to the list loses page 3: %v", back)
	}
	filter := between(t, page, `class="filterbar"`, "</form>")
	if filter == "" || strings.Contains(filter, `name="page"`) {
		t.Errorf("the filter form is missing or carries the page, so Apply would not reset it:\n%s", filter)
	}
}

func TestALiveRefreshKeepsTheReaderOnTheirPage(t *testing.T) {
	t.Parallel()
	_, b := numberedFixture(t)
	b.createTask("infra", "row 13")
	nav := pagerOf(t, b.page("/tasks/live?limit=1&sort=seq&page=3"))
	if !strings.Contains(nav, "Page 3 of 13") || !strings.Contains(nav, "13 tasks") {
		t.Errorf("the refreshed fragment is not page 3 of the new total:\n%s", nav)
	}
}

func TestACursorLinkStillOpensThePageItNames(t *testing.T) {
	t.Parallel()
	f, b := numberedFixture(t)
	first, err := f.svc.ListTasks(f.ctx(), core.TaskFilter{Page: core.Page{Limit: 4, Sort: core.SortSeq}})
	if err != nil || first.NextCursor == "" {
		t.Fatalf("listing for a cursor: %v", err)
	}
	page := b.page("/tasks?" + url.Values{"limit": {"1"}, "sort": {"seq"}, "cursor": {first.NextCursor}}.Encode())
	if got := rowsOn(page); len(got) != 1 || got[0] != "row 05" {
		t.Errorf("the cursor after row 04 lists %v, want row 05", got)
	}
}
