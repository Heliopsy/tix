// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

var (
	arrangedColumn = regexp.MustCompile(`<section class="column[^"]*" data-state="([^"]+)"`)
	arrangedLane   = regexp.MustCompile(`<section class="lane[^"]*" id="lane-([^"]+)"`)
	listGroupPat   = regexp.MustCompile(`<section class="listgroup[^"]*" data-group="([^"]+)"`)
)

// groupSection is the one section a view draws for a group: from its opening
// tag to the next section's, so an assertion reads that group and no other.
func groupSection(t *testing.T, page, marker string) string {
	t.Helper()
	at := strings.Index(page, marker)
	if at < 0 {
		t.Fatalf("the page draws no group marked %s", marker)
	}
	start := strings.LastIndex(page[:at], "<section")
	end := strings.Index(page[at:], "</section>")
	if start < 0 || end < 0 {
		t.Fatalf("the group marked %s is not a section", marker)
	}
	return page[start : at+end]
}

// arrange posts one heading menu form the way a browser without scripting does.
func arrange(t *testing.T, b interface {
	post(string, url.Values) *http.Response
}, route string, form url.Values) {
	t.Helper()
	resp := b.post(route, form)
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != form.Get("next") {
		t.Fatalf("%s returned to %q, want %q", route, got, form.Get("next"))
	}
}

func setGroup(view, group, set, value, next string) url.Values {
	return url.Values{"view": {view}, "group": {group}, "set": {set}, "value": {value}, "next": {next}}
}

func boardBrowser(f *fixture, who string) *browser {
	b := f.as(who)
	b.setCookie(web.TaskViewCookie, web.TaskViewBoard)
	return b
}

func TestEveryGroupHeadingCarriesAMenu(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "on the board")

	lanes := f.as("alice")
	lanes.setCookie(web.TaskViewCookie, web.TaskViewLanes)
	listed := f.as("alice")
	listed.setCookie(web.ListGroupCookie, web.ListGroupStatus)

	cases := []struct {
		name, page, marker, menu, view string
	}{
		{"the task screen's board", boardBrowser(f, "alice").page("/tasks"), `data-state="todo"`, "gm-status-todo", "board"},
		{"the lanes", lanes.page("/tasks"), `id="lane-infra"`, "gm-project-infra", "lanes"},
		{"the project board", b.page("/projects/infra"), `data-state="todo"`, "gm-status-todo", "project:infra"},
		{"the grouped list", listed.page("/tasks"), `data-group="todo"`, "gm-status-todo", "board"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sect := groupSection(t, tc.page, tc.marker)
			head := between(t, sect, `<div class="grouphead">`, `<div id="`+tc.menu+`"`)
			if !strings.Contains(head, `popovertarget="`+tc.menu+`"`) || !strings.Contains(head, "#more") {
				t.Fatalf("the heading carries no three dots opening %s:\n%s", tc.menu, head)
			}
			if strings.Contains(between(t, sect, "<h2>", "</h2>"), "groupmore") {
				t.Error("the dots are inside the h2, which live.js reads a column's name from")
			}
			menu := between(t, sect, `<div id="`+tc.menu+`"`, `popovertargetaction="hide"`)
			for _, want := range []string{
				`name="view" value="` + tc.view + `"`,
				`name="set" value="hidden"`, `name="set" value="starred"`, `name="set" value="collapsed"`,
				`hx-post="/view/group"`, `name="next" value="`,
			} {
				if !strings.Contains(menu, want) {
					t.Errorf("the menu lacks %s:\n%s", want, menu)
				}
			}
		})
	}
}

func TestAHiddenColumnFollowsTheReaderAndNobodyElse(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	carol := seedActor(t, f.store, f.tenantA.ID, "carol", core.RoleMember)
	alice := boardBrowser(f, "alice")
	alice.createTask("infra", "a card")
	if !strings.Contains(alice.page("/tasks"), `data-state="done"`) {
		t.Fatal("the board draws no done column to hide")
	}

	arrange(t, alice, "/view/group", setGroup("board", "done", "hidden", "1", "/tasks"))

	fresh := boardBrowser(f, "alice")
	page := fresh.page("/tasks")
	if strings.Contains(page, `data-state="done"`) {
		t.Error("a fresh browser for the same reader still draws the hidden column")
	}
	hidden := between(t, page, `<div class="grouphidden">`, `popovertargetaction="hide"`)
	if !strings.Contains(hidden, "Hidden: 1") || !strings.Contains(hidden, `name="group" value="done"`) ||
		!strings.Contains(hidden, `name="value" value="0"`) {
		t.Errorf("the Hidden control does not offer done back:\n%s", hidden)
	}
	viewGroups := between(t, page, `<section id="view-groups">`, "</section>")
	if !strings.Contains(viewGroups, `name="group" value="done"`) || !strings.Contains(viewGroups, `action="/view/reset"`) {
		t.Errorf("the View menu does not list the hidden column and a reset:\n%s", viewGroups)
	}
	if !strings.Contains(boardBrowser(f, "actor:"+carol.ID).page("/tasks"), `data-state="done"`) {
		t.Error("hiding a column hid it for another reader")
	}
	if !strings.Contains(fresh.page("/projects/infra"), `data-state="done"`) {
		t.Error("hiding a column on the task screen hid it on the project's board, a different view")
	}

	arrange(t, fresh, "/view/group", setGroup("board", "done", "hidden", "0", "/tasks"))
	if !strings.Contains(alice.page("/tasks"), `data-state="done"`) {
		t.Error("Show did not bring the column back")
	}
}

func TestStarCollapseAndMoveRearrangeTheLanes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "ops")
	b := f.as("alice")
	b.setCookie(web.TaskViewCookie, web.TaskViewLanes)
	b.createTask("infra", "infra work")
	b.createTask("ops", "ops work")

	order := func() []string { return refsIn(b.page("/tasks"), arrangedLane) }
	if got := order(); strings.Join(got, ",") != "infra,ops" {
		t.Fatalf("lanes before = %v", got)
	}

	arrange(t, b, "/view/group", setGroup("lanes", "ops", "starred", "1", "/tasks"))
	if got := order(); strings.Join(got, ",") != "ops,infra" {
		t.Errorf("after starring ops the lanes are %v, want ops first", got)
	}
	lane := groupSection(t, b.page("/tasks"), `id="lane-ops"`)
	if !strings.Contains(lane, "is-starred") || !strings.Contains(lane, `class="groupstar"`) {
		t.Error("a starred lane does not say so")
	}
	arrange(t, b, "/view/group", setGroup("lanes", "ops", "starred", "0", "/tasks"))

	page := b.page("/tasks")
	menu := between(t, groupSection(t, page, `id="lane-infra"`), `id="gm-project-infra"`, `popovertargetaction="hide"`)
	if !strings.Contains(menu, `name="dir" value="right"`) || strings.Contains(menu, `name="dir" value="left"`) {
		t.Fatalf("the first lane should move right only:\n%s", menu)
	}
	arrange(t, b, "/view/order", url.Values{"view": {"lanes"}, "group": {"infra"}, "dir": {"right"},
		"order": {"infra,ops"}, "next": {"/tasks"}})
	if got := order(); strings.Join(got, ",") != "ops,infra" {
		t.Errorf("after moving infra right the lanes are %v", got)
	}

	arrange(t, b, "/view/group", setGroup("lanes", "infra", "collapsed", "1", "/tasks"))
	collapsed := groupSection(t, b.page("/tasks"), `id="lane-infra"`)
	if !strings.Contains(collapsed, "is-collapsed") || strings.Contains(collapsed, "lane-card") {
		t.Errorf("a collapsed lane still draws its cards:\n%s", collapsed)
	}
	if !strings.Contains(collapsed, `name="set" value="collapsed"`) {
		t.Error("a collapsed lane offers no way to expand it")
	}
}

func TestACollapsedColumnStillTakesADrop(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := boardBrowser(f, "alice")
	b.createTask("infra", "a card")
	arrange(t, b, "/view/group", setGroup("board", "doing", "collapsed", "1", "/tasks"))
	col := groupSection(t, b.page("/tasks"), `data-state="doing"`)
	if !strings.HasPrefix(col, `<section class="column is-collapsed" data-state="doing"`) {
		t.Errorf("a collapsed column is not a .column with its state, so live.js cannot drop on it:\n%.200s", col)
	}
}

func TestTheListGroupsByProjectAndSharesTheLanes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "ops")
	b := f.as("alice")
	b.createTask("infra", "infra work")
	b.createTask("ops", "ops work")

	if strings.Contains(b.page("/tasks"), `class="listgroup`) {
		t.Fatal("the list is grouped before anybody asked")
	}
	resp := b.post("/listgroup", url.Values{"group": {web.ListGroupProject}, "next": {"/tasks"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/tasks")
	if got := refsIn(page, listGroupPat); strings.Join(got, ",") != "infra,ops" {
		t.Fatalf("list groups = %v", got)
	}
	ops := groupSection(t, page, `data-group="ops"`)
	if !strings.Contains(ops, "ops work") || strings.Contains(ops, "infra work") {
		t.Errorf("the ops group holds the wrong rows:\n%s", ops)
	}
	menu := between(t, ops, `id="gm-project-ops"`, `popovertargetaction="hide"`)
	for _, want := range []string{`name="view" value="lanes"`, "Move up", `href="/projects/ops"`,
		`href="/tasks?q=project:ops"`, `name="project_ref" value="ops"`} {
		if !strings.Contains(menu, want) {
			t.Errorf("the ops group's menu lacks %s", want)
		}
	}

	arrange(t, b, "/view/group", setGroup("lanes", "ops", "hidden", "1", "/tasks"))
	if got := refsIn(b.page("/tasks"), listGroupPat); strings.Join(got, ",") != "infra" {
		t.Errorf("a lane hidden on the lanes view still groups the list: %v", got)
	}
}

func TestAStatusHeadingAddsATaskInItsState(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/projects/infra")
	menu := between(t, groupSection(t, page, `data-state="doing"`), `id="gm-status-doing"`, `popovertargetaction="hide"`)
	add := between(t, menu, `<form method="post" action="/tasks" class="groupmenu-add">`, "</form>")
	if !strings.Contains(add, `name="status" value="doing"`) || !strings.Contains(add, `name="project_ref" value="infra"`) {
		t.Fatalf("the doing column's add field does not add into doing on infra:\n%s", add)
	}
	resp := b.post("/tasks", url.Values{"project_ref": {"infra"}, "status": {"doing"}, "title": {"from the column"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if !strings.Contains(groupSection(t, b.page("/projects/infra"), `data-state="doing"`), "from the column") {
		t.Error("the task added from the doing column is not in doing")
	}
}

func TestTheProjectBoardKeepsItsOwnArrangement(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	arrange(t, b, "/view/group", setGroup("project:infra", "done", "hidden", "1", "/projects/infra"))
	page := b.page("/projects/infra")
	if refs := refsIn(page, arrangedColumn); strings.Contains(strings.Join(refs, ","), "done") {
		t.Errorf("the project board still draws done: %v", refs)
	}
	if !strings.Contains(page, "Hidden: 1") {
		t.Error("the project board does not count its hidden column")
	}
	if !strings.Contains(boardBrowser(f, "alice").page("/tasks"), `data-state="done"`) {
		t.Error("a column hidden on the project board was hidden on the task screen")
	}
	arrange(t, b, "/view/reset", url.Values{"view": {"project:infra"}, "next": {"/projects/infra"}})
	if !strings.Contains(b.page("/projects/infra"), `data-state="done"`) {
		t.Error("a reset did not bring the column back")
	}
}

func TestArrangingRefusesWhatTheServiceRefuses(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, tc := range []struct {
		route string
		form  url.Values
	}{
		{"/view/group", setGroup("calendar", "done", "hidden", "1", "/tasks")},
		{"/view/group", setGroup("board", "done", "sideways", "1", "/tasks")},
		{"/view/order", url.Values{"view": {"board"}, "group": {"todo"}, "dir": {"left"}, "order": {"todo,doing"}, "next": {"/tasks"}}},
		{"/view/order", url.Values{"view": {"board"}, "group": {"todo"}, "dir": {"up"}, "order": {"todo,doing"}, "next": {"/tasks"}}},
	} {
		resp := b.post(tc.route, tc.form)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s %v answered %d, want 400", tc.route, tc.form, resp.StatusCode)
		}
	}
}
