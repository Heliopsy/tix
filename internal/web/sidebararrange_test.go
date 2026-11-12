// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// topEntry reads a top-level navigation entry's target: an anchor that
// opens with its href, which an indented entry (class first) does not.
var topEntry = regexp.MustCompile(`<a href="([^"]*)" data-rail-name=`)

// TestTheSidebarIsOrderedByUse pins the order of the top-level entries and
// keeps My work among them rather than indented under another.
func TestTheSidebarIsOrderedByUse(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	links := nav(t, b.page("/tasks"))
	top := links
	if at := strings.Index(top, `data-navgroup=`); at >= 0 {
		top = top[:at]
	}
	var got []string
	for _, m := range topEntry.FindAllStringSubmatch(top, -1) {
		got = append(got, m[1])
	}
	want := []string{"/inbox", "/mine", "/tasks", "/projects", "/templates", "/activity", "/stats", "/connect", "/docs"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the top-level entries are %v, want %v", got, want)
	}
	if strings.Contains(links, `class="side-sub" href="/mine"`) {
		t.Errorf("My work is drawn indented under another entry:\n%s", links)
	}
}

// overdueBadge is My work's badge, cut at its own closing tag.
func overdueBadge(t *testing.T, page string) string {
	t.Helper()
	at := strings.Index(page, `id="mywork-badge"`)
	if at < 0 {
		t.Fatalf("the page has no My work badge:\n%s", page)
	}
	open := strings.LastIndex(page[:at], "<span")
	end := strings.Index(page[at:], "</span></span>")
	return page[open : at+end+len("</span></span>")]
}

// TestMyWorkCountsTheReadersOverdueOpenTasks pins what the badge counts: the
// reader's own open work past its deadline, not somebody else's, not work
// that is due later, and not finished work.
func TestMyWorkCountsTheReadersOverdueOpenTasks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ctx := aliceCtx(f)

	if badge := overdueBadge(t, nav(t, b.page("/tasks"))); !strings.Contains(badge, " hidden") {
		t.Errorf("the badge shows with nothing overdue:\n%s", badge)
	}

	late, later := time.Now().Add(-48*time.Hour), time.Now().Add(30*24*time.Hour)
	mk := func(title string, due *time.Time, assignee string) *core.Task {
		t.Helper()
		task, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: title, DueAt: due, AssigneeActorID: assignee})
		if err != nil {
			t.Fatal(err)
		}
		return task
	}
	mk("late one", &late, f.actorA.ID)
	mk("late two", &late, f.actorA.ID)
	mk("due later", &later, f.actorA.ID)
	mk("late and nobody's", &late, "")
	done := mk("late and done", &late, f.actorA.ID)
	if _, err := f.svc.TransitionTask(ctx, core.TaskRef{ID: done.ID}, core.TransitionInput{To: "cancelled"}); err != nil {
		t.Fatalf("finishing: %v", err)
	}

	badge := overdueBadge(t, nav(t, b.page("/tasks")))
	if strings.Contains(badge, " hidden") || !strings.Contains(badge, `data-badge-count="2">2<`) {
		t.Errorf("the badge does not count two overdue tasks:\n%s", badge)
	}
	if !strings.Contains(badge, `<span class="sr-only"> overdue tasks,</span>`) {
		t.Errorf("the badge does not say what it counts:\n%s", badge)
	}

	fragment := b.page(web.RouteInboxBadge)
	if !strings.Contains(fragment, `id="inbox-badge"`) || !strings.Contains(overdueBadge(t, fragment), `data-badge-count="2"`) {
		t.Errorf("the live badge fragment does not carry both badges:\n%s", fragment)
	}
}

// starProject posts a project's star toggle, as the projects screen's form
// does without scripting, and returns where it redirected to.
func starProject(t *testing.T, b *browser, key string, star bool) string {
	t.Helper()
	route := web.RouteProjectUnstar
	if star {
		route = web.RouteProjectStar
	}
	resp := b.post(strings.Replace(route, "{key}", key, 1), url.Values{"next": {"/projects"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	return resp.Header.Get("Location")
}

// pins is the sidebar's list of starred projects, empty when there is none.
func pins(t *testing.T, page string) string {
	t.Helper()
	return between(t, nav(t, page), `<ul class="side-pins"`, "</ul>")
}

// starForm is one project's star toggle on a page, cut at its form's end.
func starForm(t *testing.T, page, key string) string {
	t.Helper()
	for _, action := range []string{"/projects/" + key + "/star\"", "/projects/" + key + "/star/remove\""} {
		if strings.Contains(page, `<form method="post" class="projstar" action="`+action) {
			return between(t, page, `<form method="post" class="projstar" action="`+action, "</form>")
		}
	}
	t.Fatalf("no star toggle for %s", key)
	return ""
}

// TestAStarredProjectIsListedUnderProjects pins the round trip: the projects
// screen offers the star, the sidebar lists the project under Projects
// linking to its board, the lane star is the same mark, and another reader's
// sidebar is untouched.
func TestAStarredProjectIsListedUnderProjects(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	row := starForm(t, b.page("/projects"), "infra")
	for _, want := range []string{`name="next" value="/projects"`, `aria-pressed="false"`, `aria-label="Star INFRA"`, `title="Star this project"`, `#star-outline"`} {
		if !strings.Contains(row, want) {
			t.Errorf("the projects screen's star toggle lacks %s:\n%s", want, row)
		}
	}
	collapse := b.post(web.RouteViewGroup, url.Values{
		"view": {core.ViewLanes}, "group": {"infra"}, "set": {"collapsed"}, "value": {"1"}, "next": {"/projects"},
	})
	_ = collapse.Body.Close()
	wantStatus(t, collapse, http.StatusSeeOther)
	if got := pins(t, b.page("/tasks")); got != "" {
		t.Fatalf("a lane folded but not starred is listed:\n%s", got)
	}

	if loc := starProject(t, b, "infra", true); !strings.HasPrefix(loc, "/projects?flash=") || !strings.HasSuffix(loc, "#p-INFRA") {
		t.Errorf("starring without scripting does not return to the row: %q", loc)
	}
	page := b.page("/tasks")
	list := pins(t, page)
	if !strings.Contains(list, `aria-label="Starred projects"`) {
		t.Errorf("the sidebar list is not named Starred projects:\n%s", list)
	}
	if !strings.Contains(list, `<a href="/projects/infra" data-rail-name="INFRA (INFRA)"`) || !strings.Contains(list, `<span>INFRA (INFRA)</span></a>`) {
		t.Errorf("the starred project is not listed with its board and its label:\n%s", list)
	}
	if !strings.Contains(list, `<span class="side-pin-mark nav-icon" aria-hidden="true">I</span>`) {
		t.Errorf("the starred project has no mark:\n%s", list)
	}
	links := nav(t, page)
	if at, pin, after := strings.Index(links, `href="/projects" `), strings.Index(links, `class="side-pins"`), strings.Index(links, `href="/activity"`); at < 0 || pin < at || after < pin {
		t.Errorf("the starred projects are not between Projects and Activity (%d %d %d)", at, pin, after)
	}
	if !strings.Contains(pins(t, b.page("/projects/infra")), `aria-current="page"`) {
		t.Error("the starred project is not marked current on its board")
	}
	b.setCookie(web.ProjectLabelCookie, web.ProjectLabelKey)
	if list := pins(t, b.page("/tasks")); !strings.Contains(list, `<span>INFRA</span></a>`) {
		t.Errorf("the starred project ignores the reader's label preference:\n%s", list)
	}

	if prefs, err := f.svc.ListGroupPrefs(aliceCtx(f), core.ViewLanes); err != nil || len(prefs) != 1 || !prefs[0].Starred || !prefs[0].Collapsed {
		t.Errorf("a project star is not the lane star: %+v %v", prefs, err)
	}
	row = starForm(t, b.page("/projects"), "infra")
	for _, want := range []string{`action="/projects/infra/star/remove"`, `aria-pressed="true"`, `title="Unstar"`, `#star-filled"`} {
		if !strings.Contains(`action="/projects/infra/star/remove"`+row, want) {
			t.Errorf("a starred project's toggle lacks %s:\n%s", want, row)
		}
	}
	other := f.as("member:someone-else")
	if got := pins(t, other.page("/tasks")); got != "" {
		t.Errorf("another reader's sidebar lists the star:\n%s", got)
	}
	if theirs := other.page("/projects"); strings.Contains(theirs, `id="projs-starred"`) || strings.Contains(starForm(t, theirs, "infra"), `aria-pressed="true"`) {
		t.Error("another reader's projects screen shows the star")
	}

	starProject(t, b, "infra", false)
	if got := pins(t, b.page("/tasks")); got != "" {
		t.Errorf("an unstarred project is still listed:\n%s", got)
	}
}

// TestALaneStarMadeBeforeStarsIsAProjectStar is the back-compat guard: a pin
// was written as the lanes view's star, and that row is what a project star
// reads, so nothing a reader pinned is lost.
func TestALaneStarMadeBeforeStarsIsAProjectStar(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post(web.RouteViewGroup, url.Values{
		"view": {core.ViewLanes}, "group": {"infra"}, "set": {"starred"}, "value": {"1"}, "next": {"/projects"},
	})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	page := b.page("/projects")
	section := between(t, page, `<ul class="projlist rows" data-projlist="starred">`, "</ul>")
	if !strings.Contains(section, `id="p-INFRA"`) {
		t.Errorf("the old pin is not under Starred:\n%s", section)
	}
	if !strings.Contains(starForm(t, b.page("/projects/infra"), "infra"), `aria-pressed="true"`) {
		t.Error("the project page's header does not show the old pin as a star")
	}
}

// TestTheSidebarEndsALongStarredListWithShowAll pins the cap: twelve starred
// projects drawn, then Show all to the starred filter.
func TestTheSidebarEndsALongStarredListWithShowAll(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := aliceCtx(f)
	for i := range 13 {
		key := fmt.Sprintf("p%02d", i)
		if _, err := f.svc.CreateProject(ctx, core.CreateProjectInput{Key: key, Name: key, WorkflowKey: "default"}); err != nil {
			t.Fatal(err)
		}
		if i < 12 {
			if err := f.svc.StarProject(ctx, key); err != nil {
				t.Fatal(err)
			}
		}
	}
	b := f.as("alice")
	if list := pins(t, b.page("/tasks")); strings.Count(list, `<li class="side-pin`) != 12 || strings.Contains(list, "/projects?starred=1") {
		t.Errorf("twelve starred projects are not drawn whole:\n%s", list)
	}
	if err := f.svc.StarProject(ctx, "p12"); err != nil {
		t.Fatal(err)
	}
	list := pins(t, b.page("/tasks"))
	if strings.Count(list, `<a href="/projects/p`) != 12 || !strings.Contains(list, `<a href="/projects?starred=1"`) {
		t.Errorf("thirteen starred projects do not end with Show all:\n%s", list)
	}
}

// adminBody is the Manage group's body (its id is still admin), cut at the group's end.
func adminBody(t *testing.T, page string) string {
	t.Helper()
	return between(t, nav(t, page), `<div class="navgroup-body" id="navgroup-admin">`, "</div>")
}

// TestManageHoldsTrashConfigureAndData pins the one Manage group: Trash and the
// configuration and data screens under it, no Configure or Data group left,
// Trash gone with the tenant trash, and a reader with neither gets no group.
func TestManageHoldsTrashConfigureAndData(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.AdvancedCookie, "1")
	page := b.page("/tasks")
	body := adminBody(t, page)
	for _, href := range []string{`href="/tasks?q=is:trashed"`, `href="/workflows"`, `href="/admin/users"`, `href="/transfer"`, `href="/sync"`} {
		if !strings.Contains(body, href) {
			t.Errorf("Manage lacks %s:\n%s", href, body)
		}
	}
	links := nav(t, page)
	for _, gone := range []string{`data-navgroup="configure"`, `data-navgroup="data"`} {
		if strings.Contains(links, gone) {
			t.Errorf("the sidebar still draws %s", gone)
		}
	}
	if strings.Count(links, "<span>Trash</span>") != 1 {
		t.Errorf("Trash is not listed exactly once:\n%s", links)
	}
	if !strings.Contains(adminBody(t, b.page("/admin/users")), `href="/admin/users" data-rail-name="Users" aria-current="page"`) {
		t.Error("the Manage entry for the page being read is not marked current")
	}

	member := f.as("member:plain")
	member.setCookie(web.AdvancedCookie, "0")
	if got := adminBody(t, member.page("/tasks")); !strings.Contains(got, "<span>Trash</span>") || strings.Contains(got, `href="/workflows"`) {
		t.Errorf("without Advanced screens Manage is not Trash alone:\n%s", got)
	}

	turnTrashOff(t, b)
	if got := adminBody(t, b.page("/tasks")); strings.Contains(got, "Trash") || !strings.Contains(got, `href="/workflows"`) {
		t.Errorf("with the trash off Manage still lists Trash, or lost the rest:\n%s", got)
	}
	if links := nav(t, member.page("/tasks")); strings.Contains(links, `data-navgroup="admin"`) {
		t.Errorf("a reader with nothing under Manage gets the heading:\n%s", links)
	}
}

// accountMenu is the account popover, cut at the list's end.
func accountMenu(t *testing.T, side string) string {
	t.Helper()
	return between(t, side, `<div id="account-menu" class="flowmenu accountmenu" popover>`, "</ul>")
}

// TestTheAccountMenuHoldsTheReadersControls pins the foot of the sidebar:
// one button naming the reader opens a popover holding Settings, the scheme
// switch, Keyboard shortcuts and Sign out, none of which is drawn outside
// it, with the version under the button.
func TestTheAccountMenuHoldsTheReadersControls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	side := sidebarOf(t, f.as("alice").page("/tasks"))

	open := tagWith(t, side, "data-account-open")
	for _, want := range []string{`<button type="button"`, `popovertarget="account-menu"`, `data-rail-name="alice"`, `aria-label="Account menu for alice"`} {
		if !strings.Contains(open, want) {
			t.Errorf("the account button lacks %s:\n%s", want, open)
		}
	}
	menu := accountMenu(t, side)
	for _, want := range []string{`<a href="/settings"`, `action="/theme"`, `data-shortcuts-open`, `action="/logout"`} {
		if !strings.Contains(menu, want) {
			t.Errorf("the account menu lacks %s:\n%s", want, menu)
		}
		if strings.Count(side, want) != 1 {
			t.Errorf("%s is drawn %d times in the sidebar, want once, inside the menu", want, strings.Count(side, want))
		}
	}
	if strings.Contains(side, `class="who"`) || strings.Contains(side, `class="side-foot"`) {
		t.Error("the stacked handle and foot blocks are still drawn")
	}
	button, version := strings.Index(side, "data-account-open"), strings.Index(side, `<a class="running" href="/release#changes"`)
	if version < 0 || version < button {
		t.Errorf("the version is not under the account button (%d, %d)", button, version)
	}
	if strings.Contains(nav(t, side), `class="version"`) {
		t.Error("the version is still inside the navigation")
	}
}
