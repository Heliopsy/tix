// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/web"
)

// switchLink is one position of the view switch drawn on a task screen.
type switchLink struct {
	View    string
	Href    string
	Current bool
}

var switchLinkPattern = regexp.MustCompile(`<a class="segment[^"]*" href="([^"]*)" data-view="([^"]+)"([^>]*)>`)

// switchLinks reads the view switch's links, and only the switch's: the block
// is cut to the switch before any link is read.
func switchLinks(t *testing.T, page string) []switchLink {
	t.Helper()
	block := viewSwitch(t, page)
	var out []switchLink
	for _, m := range switchLinkPattern.FindAllStringSubmatch(block, -1) {
		out = append(out, switchLink{View: m[2], Href: html.UnescapeString(m[1]),
			Current: strings.Contains(m[3], `aria-current="page"`)})
	}
	if len(out) == 0 {
		t.Fatalf("the view switch holds no links:\n%s", block)
	}
	return out
}

// switchHrefs is where each position of the view switch leads.
func switchHrefs(t *testing.T, page string) []string {
	t.Helper()
	var out []string
	for _, l := range switchLinks(t, page) {
		out = append(out, l.Href)
	}
	return out
}

// drawnAs names the drawing a task screen's listing is in, from markup only
// that drawing writes.
func drawnAs(t *testing.T, page string) string {
	t.Helper()
	rows := listing(t, page)
	switch {
	case len(laneKeys(rows)) > 0 || strings.Contains(rows, `class="lanes`):
		return web.TaskViewLanes
	case strings.Contains(rows, `<div class="board`):
		return web.TaskViewBoard
	case strings.Contains(rows, `<ul class="tasklist`):
		return web.TaskViewList
	}
	return ""
}

// filterForm is the filter bar's form and nothing else.
func filterForm(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<form method="get"`, `</form>`)
}

func TestTheProjectPageListsItsTasksUnderTheFilter(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ops := seedProject(t, f.store, f.tenantA.ID, "ops")
	tagged := b.createTask("infra", "tagged infra work")
	untagged := b.createTask("infra", "untagged infra work")
	elsewhere := b.createTask("ops", "tagged ops work")
	for _, ref := range []string{tagged, elsewhere} {
		resp := b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"ops"}})
		_ = resp.Body.Close()
	}

	page := b.page("/projects/infra?view=list&q=" + url.QueryEscape("tag:ops"))
	if got := drawnAs(t, page); got != web.TaskViewList {
		t.Fatalf("the project page asked for its list drew %q", got)
	}
	rows := listing(t, page)
	if !strings.Contains(rows, `id="t-`+tagged+`"`) {
		t.Errorf("the project list left out %s, which the filter selects", tagged)
	}
	for _, ref := range []string{untagged, elsewhere} {
		if strings.Contains(rows, `id="t-`+ref+`"`) {
			t.Errorf("the project list drew %s, which the filter or the project leaves out", ref)
		}
	}
	form := filterForm(t, page)
	if !strings.Contains(form, `action="/projects/infra"`) {
		t.Errorf("the filter bar does not submit to the project page:\n%s", form)
	}
	if !strings.Contains(form, `name="q" value="tag:ops"`) {
		t.Errorf("the filter box lost the filter:\n%s", form)
	}
	if strings.Contains(form, `<select name="project"`) {
		t.Error("the project page offers a project select, a choice its path already made")
	}
	if !strings.Contains(form, `name="view" value="list"`) {
		t.Errorf("the filter bar does not keep the view the address named:\n%s", form)
	}
	if clear := between(t, form, `class="secondary clear" href="`, `"`); html.UnescapeString(clear) != "/projects/infra?view=list" {
		t.Errorf("Clear leads to %q, want the project page with its view", clear)
	}
	head := between(t, page, `<div class="pagehead-title">`, `<div id="tasks-summary">`)
	if !strings.Contains(head, "INFRA") || !strings.Contains(head, "Workflow ") {
		t.Errorf("the project page lost its project header:\n%s", head)
	}
	if !strings.Contains(page, `href="/projects/infra/settings"`) {
		t.Error("the project page lost its Settings button")
	}

	live := liveAttr(t, page, "data-live")
	if !strings.HasPrefix(live, "/projects/infra/live?") || !strings.Contains(live, "q=tag%3Aops") {
		t.Errorf("the project page refreshes from %q, not its own fragment under its filter", live)
	}
	infra := f.project
	if got := liveFilterOf(t, page).ProjectIDs; !slices.Equal(got, []string{infra.ID}) {
		t.Errorf("the project page subscribes to %v, want only %s (not %s)", got, infra.ID, ops.ID)
	}
	fragment := b.page(live)
	if !strings.Contains(fragment, `id="t-`+tagged+`"`) || strings.Contains(fragment, `id="t-`+untagged+`"`) {
		t.Errorf("the live fragment does not answer the page's filter:\n%s", fragment)
	}
	if strings.Contains(fragment, `class="filterbar"`) {
		t.Error("the live fragment carries the filter bar, which a refresh would swap under the reader")
	}

	board := b.page("/projects/infra?view=board&q=" + url.QueryEscape("tag:ops"))
	if len(columnStates(board)) == 0 {
		t.Fatalf("the project board under a filter drew no columns:\n%s", listing(t, board))
	}
	if cards := listing(t, board); !strings.Contains(cards, `data-task="`+tagged+`"`) || strings.Contains(cards, `data-task="`+untagged+`"`) {
		t.Errorf("the project board ignores the filter:\n%s", cards)
	}

	if got := drawnAs(t, b.page("/projects/infra")); got != web.TaskViewBoard {
		t.Errorf("a project page naming no view drew %q, want the board it has always opened on", got)
	}

	foreign := listing(t, b.page("/projects/infra?view=list&project=ops"))
	if !strings.Contains(foreign, `id="t-`+untagged+`"`) || strings.Contains(foreign, `id="t-`+elsewhere+`"`) {
		t.Errorf("a project parameter carried in from the task list changed which project the page shows:\n%s", foreign)
	}
}

// Paging and Show finished on the project page stay on the project page.
func TestTheProjectPageWritesItsOwnAddressIntoItsLinks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for range 3 {
		b.createTask("infra", "paged work")
	}
	page := b.page("/projects/infra?view=list&limit=2&sort=title")
	next := html.UnescapeString(between(t, page, `class="pager-step pager-next" href="`, `"`))
	u, err := url.Parse(next)
	if err != nil || u.Path != "/projects/infra" || u.Query().Get("sort") != "title" || u.Query().Get("view") != "list" {
		t.Errorf("Next leads to %q, want the project page keeping its sort and view", next)
	}
	all := html.UnescapeString(between(t, page, `open work only</span> &middot; <a href="`, `"`))
	if !strings.HasPrefix(all, "/projects/infra?") || !strings.Contains(all, "all=1") {
		t.Errorf("show finished too leads to %q, off the project page", all)
	}
}

// Every position of the view switch, on the task list and on a project page,
// in every view, carries the query the screen was drawn under with its own
// view, and following it without scripting draws that view with the filter.
func TestEveryViewSwitchLinkKeepsTheQuery(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "kept under the filter")
	resp := b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"ops"}})
	_ = resp.Body.Close()

	asked := url.Values{"q": {"tag:ops"}, "due": {""}, "sort": {"title"}, "project": {"infra"},
		"limit": {"25"}, "flash": {"done"}}
	for _, screen := range []string{"/tasks", "/projects/infra"} {
		for _, view := range web.TaskViews {
			t.Run(screen+" "+view, func(t *testing.T) {
				q := url.Values{}
				for k, v := range asked {
					q[k] = v
				}
				q.Set("view", view)
				page := b.page(screen + "?" + q.Encode())
				links := switchLinks(t, page)
				var views []string
				for _, l := range links {
					views = append(views, l.View)
					u, err := url.Parse(l.Href)
					if err != nil {
						t.Fatalf("unparseable switch link %q: %v", l.Href, err)
					}
					if u.Path != screen {
						t.Errorf("the %s link leads to %s, off %s", l.View, u.Path, screen)
					}
					got := u.Query()
					for _, k := range []string{"q", "sort", "project", "limit"} {
						if got.Get(k) != asked.Get(k) {
							t.Errorf("the %s link carries %s=%q, want %q", l.View, k, got.Get(k), asked.Get(k))
						}
					}
					if got.Get("view") != l.View {
						t.Errorf("the %s link names view %q", l.View, got.Get("view"))
					}
					if got.Has("flash") {
						t.Errorf("the %s link carries the flash: %s", l.View, l.Href)
					}
					if l.Current != (l.View == view) {
						t.Errorf("the %s link current = %v on the %s", l.View, l.Current, view)
					}
				}
				if !slices.Equal(views, web.TaskViews) {
					t.Fatalf("the switch offers %v, want %v", views, web.TaskViews)
				}
				for _, l := range links {
					there := b.page(l.Href)
					if got := drawnAs(t, there); got != l.View {
						t.Errorf("following %s drew %q", l.Href, got)
					}
					if !strings.Contains(filterForm(t, there), `name="q" value="tag:ops"`) {
						t.Errorf("following %s lost the filter", l.Href)
					}
					if !strings.Contains(listing(t, there), ref) {
						t.Errorf("following %s lost the task the filter selects", l.Href)
					}
					back := switchLinks(t, there)
					if back[slices.IndexFunc(back, func(s switchLink) bool { return s.View == view })].Href !=
						links[slices.IndexFunc(links, func(s switchLink) bool { return s.View == view })].Href {
						t.Errorf("switching back from %s does not restore the address it left", l.View)
					}
				}
			})
		}
	}
}

// The switch is links, so a browser without scripting follows it like any
// other; and the filter form carries the view, so Apply keeps it.
func TestTheViewSwitchWorksWithoutScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "anything")
	for _, screen := range []string{"/tasks", "/projects/infra"} {
		page := b.page(screen + "?view=lanes&q=" + url.QueryEscape("priority:normal"))
		block := viewSwitch(t, page)
		for _, absent := range []string{"<form", "<button", "data-needs-js", " hidden"} {
			if strings.Contains(block, absent) {
				t.Errorf("%s: the view switch carries %q, so it needs more than a link:\n%s", screen, absent, block)
			}
		}
		form := filterForm(t, page)
		if !strings.Contains(form, `type="hidden" name="view" value="lanes"`) {
			t.Errorf("%s: Apply would drop the view:\n%s", screen, form)
		}
		submitted := screen + "?" + url.Values{"view": {"lanes"}, "q": {"priority:high"}}.Encode()
		if got := drawnAs(t, b.page(submitted)); got != web.TaskViewLanes {
			t.Errorf("%s: applying a filter drew %q, not the lanes it was on", screen, got)
		}
	}
}

// The view in the address wins over the remembered one. On the task list it
// is also remembered; the project page leaves the task list's choice alone.
func TestTheViewInTheAddressWins(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "anything")
	b.setCookie(web.TaskViewCookie, web.TaskViewLanes)

	if got := drawnAs(t, b.page("/tasks?view=board")); got != web.TaskViewBoard {
		t.Fatalf("/tasks?view=board drew %q", got)
	}
	if got := b.cookie(web.TaskViewCookie); got != web.TaskViewBoard {
		t.Errorf("following the board on the task list remembered %q", got)
	}
	if got := drawnAs(t, b.page("/tasks")); got != web.TaskViewBoard {
		t.Errorf("the next bare visit drew %q, not the view last chosen", got)
	}
	if got := drawnAs(t, b.page("/projects/infra?view=list")); got != web.TaskViewList {
		t.Errorf("/projects/infra?view=list drew %q", got)
	}
	if got := b.cookie(web.TaskViewCookie); got != web.TaskViewBoard {
		t.Errorf("the project page changed the task list's remembered view to %q", got)
	}
	if got := drawnAs(t, b.page("/tasks?view=swimlanes")); got != web.TaskViewBoard {
		t.Errorf("an unknown view drew %q, not the remembered one", got)
	}
	if got := b.cookie(web.TaskViewCookie); got != web.TaskViewBoard {
		t.Errorf("an unknown view was remembered as %q", got)
	}
}

// With scripting, the filter bar narrows the listing in place; the switch is
// redrawn from the same answer, so switching afterwards keeps the new filter.
func TestSwitchingAfterAnInPlaceFilterKeepsItInABrowser(t *testing.T) {
	chrome := chromeOrSkip(t)
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "the one to find")
	b.createTask("infra", "the other one")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	page := launchChrome(ctx, t, chrome)
	page.call("Network.enable", nil)
	page.call("Page.enable", nil)
	page.call("Network.setExtraHTTPHeaders", map[string]any{"headers": map[string]string{actorHeader: "alice"}})

	for _, screen := range []string{"/tasks?view=list", "/projects/infra?view=list"} {
		page.openLive(t, f.server.URL+screen, "ul.tasklist li.task")
		page.eval(t, `(() => {
			const box = document.querySelector('form.filterbar input[name="q"]');
			box.value = 'find';
			box.dispatchEvent(new Event('input', {bubbles: true}));
			return 0;
		})()`, nil)
		page.waitUntil(t, `document.querySelector('#tasks-views a[data-view="board"]').href.includes('q=find')`,
			screen+": the switch was not redrawn with the filter typed in place", 30*time.Second)
		page.eval(t, `(() => { document.querySelector('#tasks-views a[data-view="board"]').click(); return 0; })()`, nil)
		page.waitUntil(t, `location.search.includes('view=board') && location.search.includes('q=find') &&
			document.querySelector('#tasks-listing .board') && document.querySelector('form.filterbar input[name="q"]').value === 'find'`,
			screen+": following Board after an in-place filter lost the filter", 30*time.Second)
	}
}
