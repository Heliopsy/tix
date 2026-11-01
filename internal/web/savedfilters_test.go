// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// sidebarSaved is the sidebar's saved filter block of a rendered page.
func sidebarSaved(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<div id="side-saved" class="side-saved">`, `<div class="navgroup side-recent`)
}

// saveFilter posts the filter bar's Save form and answers the new filter's id.
func saveFilter(t *testing.T, b *browser, form url.Values) string {
	t.Helper()
	resp := b.post(web.RouteFilters, form)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("saving a filter = %d: %s", resp.StatusCode, body(t, resp))
	}
	list, err := b.fix.svc.ListSavedFilters(aliceCtx(b.fix))
	if err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, f := range list {
		if f.Name == form.Get("name") {
			id = f.ID
		}
	}
	if id == "" || !strings.Contains(sidebarSaved(t, b.page("/tasks")), `href="/filters/`+id+`"`) {
		t.Fatalf("the sidebar does not list %q after saving it", form.Get("name"))
	}
	return id
}

func aliceCtx(f *fixture) context.Context {
	return core.WithActor(core.WithTenant(context.Background(),
		core.TenantScope{TenantID: f.tenantA.ID}), f.actorFor("alice"))
}

func TestTheFilterBarSavesWhatItShowsAndTheSidebarOpensIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/tasks?q=tag:ops&due=overdue&sort=priority")
	form := formAt(t, page, web.RouteFilters)
	for _, want := range []string{
		`name="q" value="tag:ops"`, `name="due" value="overdue"`, `name="sort" value="priority"`,
		`name="view" value="list"`, `name="name"`, `name="shared" value="1"`,
	} {
		if !strings.Contains(form, want) {
			t.Errorf("the Save form lacks %s:\n%s", want, form)
		}
	}
	if plain := b.page("/tasks"); strings.Contains(plain, `action="`+web.RouteFilters+`"`) {
		t.Error("the Save form is offered with nothing to save")
	}

	resp := b.post(web.RouteFilters, url.Values{
		"name": {"Late ops"}, "q": {"tag:ops"}, "due": {"overdue"}, "sort": {"priority"}, "view": {"board"},
	})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if loc.Path != web.RouteTasks || loc.Query().Get("q") != "due:overdue tag:ops" || loc.Query().Get("sort") != "priority" {
		t.Errorf("saving lands on %s, want the task screen on the saved filter", loc)
	}
	if got := b.cookie(web.TaskViewCookie); got != "board" {
		t.Errorf("the view switch reads %q after saving a board filter", got)
	}

	saved := sidebarSaved(t, b.page("/projects"))
	if !strings.Contains(saved, ">Late ops<") || !strings.Contains(saved, `data-rail-name="Late ops"`) {
		t.Fatalf("the sidebar does not list the filter by name, with a rail name:\n%s", saved)
	}
	if strings.Contains(saved, `aria-current="page"`) {
		t.Error("a saved filter is marked current on another screen")
	}
	if here := sidebarSaved(t, b.page("/tasks?q="+url.QueryEscape("due:overdue tag:ops")+"&sort=priority")); !strings.Contains(here, `aria-current="page"`) {
		t.Error("the filter being shown is not marked current in the sidebar")
	}

	id := between(t, saved, `href="/filters/`, `"`)
	other := f.as("alice")
	open := other.get("/filters/" + id)
	_ = open.Body.Close()
	wantStatus(t, open, http.StatusSeeOther)
	if got := open.Header.Get("Location"); !strings.HasPrefix(got, web.RouteTasks+"?") || !strings.Contains(got, "sort=priority") {
		t.Errorf("opening the filter goes to %q", got)
	}
	if got := other.cookie(web.TaskViewCookie); got != "board" {
		t.Errorf("opening a board filter leaves the view switch at %q", got)
	}
	open = other.get("/filters/Late%20ops")
	_ = open.Body.Close()
	wantStatus(t, open, http.StatusSeeOther)
	missing := other.get("/filters/nope")
	_ = missing.Body.Close()
	wantStatus(t, missing, http.StatusNotFound)
}

func TestTheSidebarMenuRenamesSharesMovesAndDeletes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	first := saveFilter(t, b, url.Values{"name": {"first"}, "q": {"tag:a"}})
	second := saveFilter(t, b, url.Values{"name": {"second"}, "q": {"tag:b"}})

	saved := sidebarSaved(t, b.page("/tasks"))
	menu := between(t, saved, `id="sf-`+strings.ToLower(first)+`-menu"`, `class="flowclose"`)
	for _, want := range []string{
		`action="/filters/` + first + `/edit"`, `name="name" value="first"`,
		`name="shared" value="1"`, `name="dir" value="down"`, `action="/filters/` + first + `/delete"`,
		`hx-target="#side-saved"`, `data-group-control="` + first + `"`,
	} {
		if !strings.Contains(menu, want) {
			t.Errorf("the first filter's menu lacks %s:\n%s", want, menu)
		}
	}
	if strings.Contains(menu, `value="up"`) {
		t.Error("the first filter offers to move up")
	}

	post := func(path string, form url.Values) {
		t.Helper()
		form.Set("next", "/tasks")
		resp := b.post(path, form)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("POST %s = %d: %s", path, resp.StatusCode, body(t, resp))
		}
	}
	post("/filters/"+first+"/edit", url.Values{"name": {"renamed"}})
	post("/filters/"+first+"/edit", url.Values{"shared": {"1"}})
	post("/filters/"+second+"/move", url.Values{"dir": {"up"}})

	list, err := f.svc.ListSavedFilters(aliceCtx(f))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "second" || list[1].Name != "renamed" || !list[1].Shared || list[1].Query != "tag:a" {
		t.Fatalf("after rename, share and move: %+v", list)
	}

	taken := b.post("/filters/"+second+"/edit", url.Values{"name": {"RENAMED"}, "next": {"/tasks"}})
	defer func() { _ = taken.Body.Close() }()
	if taken.StatusCode != http.StatusConflict {
		t.Errorf("a rename onto a taken name answered %d", taken.StatusCode)
	}

	post("/filters/"+second+"/delete", url.Values{})
	if saved := sidebarSaved(t, b.page("/tasks")); strings.Contains(saved, ">second<") || !strings.Contains(saved, ">renamed") {
		t.Errorf("after deleting, the sidebar holds:\n%s", saved)
	}
}

func TestASharedFilterIsListedForTheTenantAndAPrivateOneIsNot(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	saveFilter(t, b, url.Values{"name": {"mine only"}, "q": {"tag:a"}})
	saveFilter(t, b, url.Values{"name": {"for everyone"}, "q": {"tag:b"}, "shared": {"1"}})

	lena := seedActor(t, f.store, f.tenantA.ID, "lena", core.RoleMember)
	other := sidebarSaved(t, f.as("member:"+lena.ID).page("/tasks"))
	if strings.Contains(other, "mine only") {
		t.Error("another reader sees a private filter")
	}
	if !strings.Contains(other, "for everyone") || !strings.Contains(other, `class="savedby"`) {
		t.Errorf("another reader does not see the shared filter, marked shared:\n%s", other)
	}
	if strings.Contains(other, `data-group-btn="`) {
		t.Error("a member is offered a menu on somebody else's shared filter")
	}
	if admin := sidebarSaved(t, f.asActor(lena.ID).page("/tasks")); !strings.Contains(admin, `data-group-btn="`) {
		t.Error("an administrator is not offered a menu on a shared filter")
	}
}

func TestMyWorkGroupsTheReadersOwnTasksByDeadline(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ctx := aliceCtx(f)
	now := time.Now()
	mk := func(title string, due *time.Time, assignee string) *core.Task {
		t.Helper()
		task, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: title, DueAt: due, AssigneeActorID: assignee})
		if err != nil {
			t.Fatal(err)
		}
		return task
	}
	late, later := now.Add(-48*time.Hour), now.Add(30*24*time.Hour)
	mk("late work", &late, f.actorA.ID)
	mk("someday work", nil, f.actorA.ID)
	mk("future work", &later, f.actorA.ID)
	mk("not mine", &late, "")
	held := mk("held work", nil, "")
	if _, err := f.svc.ClaimTask(ctx, core.TaskRef{ID: held.ID}, core.ClaimInput{TTL: core.Duration(time.Hour)}); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	page := b.page(web.RouteMine)
	listing := between(t, page, `<div id="tasks-listing">`, `<template`)
	if listing == "" {
		listing = page
	}
	overdue := strings.Index(listing, `data-group="overdue"`)
	later2 := strings.Index(listing, `data-group="later"`)
	none := strings.Index(listing, `data-group="none"`)
	if overdue < 0 || later2 < 0 || none < 0 || overdue >= later2 || later2 >= none {
		t.Fatalf("My work does not draw overdue, later and no date in that order (%d %d %d)", overdue, later2, none)
	}
	if strings.Contains(listing, `data-group="today"`) {
		t.Error("an empty bucket is drawn")
	}
	if !strings.Contains(listing, "late work") || !strings.Contains(listing, "someday work") {
		t.Error("My work leaves out a task assigned to the reader")
	}
	if strings.Contains(listing, "not mine") || strings.Contains(listing, "held work") {
		t.Error("My work lists a task assigned to nobody")
	}
	if !strings.Contains(listing, `name="view" value="mine"`) {
		t.Error("a bucket heading's menu does not arrange the mine view")
	}
	if strings.Contains(between(t, page, `<h1>`, `</h1>`), "Tasks") {
		t.Error("My work is headed Tasks")
	}
	if !strings.Contains(nav(t, page), `href="/mine" data-rail-name="My work" aria-current="page"`) {
		t.Error("the sidebar does not mark My work as the page shown")
	}

	withHeld := b.page(web.RouteMine + "?held=1")
	if !strings.Contains(withHeld, "held work") || strings.Contains(withHeld, "not mine") {
		t.Error("Include tasks I hold does not add the held task, and only it")
	}
	if !strings.Contains(withHeld, `role="switch" aria-checked="true"`) {
		t.Error("the held switch does not say it is on")
	}

	if _, err := f.svc.SetGroupPref(ctx, core.ViewMine, "overdue", core.GroupPrefInput{Hidden: boolPtr(true)}); err != nil {
		t.Fatalf("hiding the overdue bucket: %v", err)
	}
	if strings.Contains(b.page(web.RouteMine), `data-group="overdue"`) {
		t.Error("a hidden bucket is still drawn")
	}
}

func boolPtr(v bool) *bool { return &v }

func TestTheSidebarCarriesTheRecentListHiddenForItsScript(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/tasks")
	block := between(t, page, `<div class="navgroup side-recent`, `</ul>`)
	if block == "" || !strings.Contains(block, `data-recent-for="`) || !strings.Contains(block, "hidden>") {
		t.Fatalf("the sidebar has no hidden recent list for recent.js:\n%s", block)
	}
	ref := f.as("alice").createTask("infra", "opened once")
	task := f.as("alice").page("/tasks/" + ref)
	if !strings.Contains(task, `data-recent-task="`+ref+`"`) {
		t.Error("the task screen does not mark itself for the recent list")
	}
}
