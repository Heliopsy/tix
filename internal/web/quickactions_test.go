// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

var undoHidden = regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`)

// undoOf follows a quick action's redirect and reads the toast's Undo form:
// where it posts, and the fields it posts besides the token.
func undoOf(t *testing.T, b *browser, resp *http.Response) (string, url.Values, string) {
	t.Helper()
	page := followed(t, b, resp)
	toast := between(t, page, `<p class="flash">`, "</p>")
	m := undoForm.FindStringSubmatch(page)
	if m == nil || !strings.Contains(toast, `form="flash-undo"`) {
		t.Fatalf("the toast offers no Undo:\n%s", toast)
	}
	fields := url.Values{}
	for _, f := range undoHidden.FindAllStringSubmatch(m[2], -1) {
		if f[1] != "csrf_token" {
			fields.Add(f[1], f[2])
		}
	}
	return m[1], fields, toast
}

// TestEveryQuickActionOffersAnUndoThatTakesItBack drives each quick action
// from a listing, presses the toast's Undo and reads the task back from the
// service: the Undo has to restore what the action changed, not just exist.
func TestEveryQuickActionOffersAnUndoThatTakesItBack(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := aliceCtx(f)
	b := f.as("alice")
	alice := f.actorFor("alice").ID
	later := time.Now().Add(72 * time.Hour).Truncate(time.Minute)
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	read := func(ref string) *core.Task {
		t.Helper()
		parsed, err := core.ParseTaskRef(ref)
		if err != nil {
			t.Fatal(err)
		}
		task, err := f.svc.GetTask(ctx, parsed)
		if err != nil {
			t.Fatal(err)
		}
		return task
	}
	cases := []struct {
		name   string
		setup  func(ref string)
		path   string
		form   url.Values
		toast  string
		action string
		done   func(*core.Task) bool
		undone func(*core.Task) bool
	}{
		{name: "star", path: "/star", action: "/star/remove", toast: "starred",
			done: func(t *core.Task) bool { return t.Starred }, undone: func(t *core.Task) bool { return !t.Starred }},
		{name: "unstar", path: "/star/remove", action: "/star", toast: "unstarred",
			setup: func(ref string) { b.post("/tasks/"+ref+"/star", url.Values{"next": {"/tasks"}}).Body.Close() },
			done:  func(t *core.Task) bool { return !t.Starred }, undone: func(t *core.Task) bool { return t.Starred }},
		{name: "assign to me", path: "/assign", form: url.Values{"assignee": {"me"}}, action: "/assign", toast: "assigned to you",
			done:   func(t *core.Task) bool { return t.AssigneeActorID == alice },
			undone: func(t *core.Task) bool { return t.AssigneeActorID == "" }},
		{name: "unassign", path: "/assign", form: url.Values{"assignee": {""}}, action: "/assign", toast: "assignee cleared",
			setup: func(ref string) {
				b.post("/tasks/"+ref+"/assign", url.Values{"assignee": {"alice"}, "next": {"/tasks"}}).Body.Close()
			},
			done:   func(t *core.Task) bool { return t.AssigneeActorID == "" },
			undone: func(t *core.Task) bool { return t.AssigneeActorID == alice }},
		{name: "set the due date", path: "/due", form: url.Values{"due": {"2026-12-01"}}, action: "/due", toast: "due date set",
			done:   func(t *core.Task) bool { return t.DueAt != nil && t.DueAt.Equal(due) },
			undone: func(t *core.Task) bool { return t.DueAt == nil }},
		{name: "clear the due date", path: "/due", form: url.Values{"due": {""}}, action: "/due", toast: "due date cleared",
			setup: func(ref string) {
				b.post("/tasks/"+ref+"/due", url.Values{"due": {"2026-12-01"}, "next": {"/tasks"}}).Body.Close()
			},
			done:   func(t *core.Task) bool { return t.DueAt == nil },
			undone: func(t *core.Task) bool { return t.DueAt != nil && t.DueAt.Equal(due) }},
		{name: "add a tag", path: "/tags", form: url.Values{"tag": {"ops"}}, action: "/tags/remove", toast: "tag added",
			done:   func(t *core.Task) bool { return slices.Contains(t.Tags, "ops") },
			undone: func(t *core.Task) bool { return !slices.Contains(t.Tags, "ops") }},
		{name: "remove a tag", path: "/tags/remove", form: url.Values{"tag": {"ops"}}, action: "/tags", toast: "tag removed",
			setup: func(ref string) {
				b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"ops"}, "next": {"/tasks"}}).Body.Close()
			},
			done:   func(t *core.Task) bool { return !slices.Contains(t.Tags, "ops") },
			undone: func(t *core.Task) bool { return slices.Contains(t.Tags, "ops") }},
		{name: "snooze", path: "/snooze", form: url.Values{"at": {later.Format("2006-01-02T15:04")}}, action: "/snooze/remove", toast: "snoozed",
			done:   func(t *core.Task) bool { return t.SnoozedUntil != nil && t.SnoozedUntil.Equal(later) },
			undone: func(t *core.Task) bool { return t.SnoozedUntil == nil || !t.SnoozedUntil.After(time.Now()) }},
		{name: "unsnooze", path: "/snooze/remove", action: "/snooze", toast: "unsnoozed",
			setup: func(ref string) {
				b.post("/tasks/"+ref+"/snooze", url.Values{"at": {later.Format("2006-01-02T15:04")}, "next": {"/tasks"}}).Body.Close()
			},
			done:   func(t *core.Task) bool { return t.SnoozedUntil == nil || !t.SnoozedUntil.After(time.Now()) },
			undone: func(t *core.Task) bool { return t.SnoozedUntil != nil && t.SnoozedUntil.Equal(later) }},
	}
	for _, tc := range cases {
		ref := b.createTask("infra", tc.name)
		if tc.setup != nil {
			tc.setup(ref)
		}
		form := url.Values{"next": {"/tasks"}}
		for k, v := range tc.form {
			form[k] = v
		}
		action, fields, toast := undoOf(t, b, b.post("/tasks/"+ref+tc.path, form))
		if !strings.Contains(toast, tc.toast) {
			t.Errorf("%s: the toast does not say %q:\n%s", tc.name, tc.toast, toast)
		}
		if !tc.done(read(ref)) {
			t.Fatalf("%s: the action did not apply", tc.name)
		}
		if want := "/tasks/" + ref + tc.action; action != want {
			t.Fatalf("%s: Undo posts to %s, want %s", tc.name, action, want)
		}
		resp := b.post(action, fields)
		wantStatus(t, resp, http.StatusSeeOther)
		_ = resp.Body.Close()
		if !tc.undone(read(ref)) {
			t.Errorf("%s: Undo (%s %v) did not take the action back", tc.name, action, fields)
		}
	}
}

// TestAQuickActionThatChangesNothingOffersNoUndo: an Undo of nothing would
// undo something else, such as starring a task that was already starred and
// then unstarring it.
func TestAQuickActionThatChangesNothingOffersNoUndo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "already mine")
	for _, step := range []struct{ path, field, value string }{
		{"/assign", "assignee", "me"}, {"/star", "", ""}, {"/tags", "tag", "ops"}, {"/due", "due", ""},
	} {
		form := url.Values{"next": {"/tasks"}}
		if step.field != "" {
			form.Set(step.field, step.value)
		}
		if step.path != "/due" {
			b.post("/tasks/"+ref+step.path, form).Body.Close()
		}
		resp := b.post("/tasks/"+ref+step.path, form)
		_ = resp.Body.Close()
		loc, _ := url.Parse(resp.Header.Get("Location"))
		if loc.Query().Has("flashundo") {
			t.Errorf("%s again offers an Undo of nothing: %s", step.path, loc)
		}
	}
}

// TestAListRowHasTheLaneCardsActionsMenu pins the row's More actions menu:
// every quick action a lane card offers, each a form a browser without
// scripting can post, and Select into select mode.
func TestAListRowHasTheLaneCardsActionsMenu(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "row with a menu")
	b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"ops"}}).Body.Close()

	page := b.page("/tasks")
	row := between(t, page, `<li id="t-`+ref+`"`, `popovertarget="rmore-`+ref+`" popovertargetaction="hide"`)
	if !strings.Contains(row, `class="rowmore-btn" popovertarget="rmore-`+ref+`"`) {
		t.Fatalf("the row has no More actions button:\n%s", row)
	}
	menu := popoverBlock(t, page, "rmore-"+ref)
	if !strings.Contains(menu, `<a class="flowgo" href="/tasks/`+ref+`?next=%2Ftasks">`) {
		t.Errorf("the menu does not open the task:\n%s", menu)
	}
	forms := map[string][]string{
		"/tasks/" + ref + "/complete":    {`Complete`},
		"/tasks/" + ref + "/star":        {`Star`},
		"/tasks/" + ref + "/transition":  {`name="route" value="doing"`},
		"/tasks/" + ref + "/snooze":      {`name="until" value="tomorrow"`},
		"/tasks/" + ref + "/assign":      {`name="assignee"`, `list="row-handles"`},
		"/tasks/" + ref + "/due":         {`name="due" value=""`},
		"/tasks/" + ref + "/tags":        {`name="tag" required`},
		"/tasks/" + ref + "/tags/remove": {`name="tag" value="ops"`},
	}
	for action, wants := range forms {
		form := menuForm(t, menu, action)
		if form == "" {
			t.Errorf("the menu has no form posting to %s:\n%s", action, menu)
			continue
		}
		for _, want := range append([]string{`name="csrf_token"`, `name="next" value="/tasks"`}, wants...) {
			if !strings.Contains(form, want) {
				t.Errorf("the form posting to %s carries no %s:\n%s", action, want, form)
			}
		}
	}
	if form := menuForm(t, menu, "/tasks/"+ref+"/assign\" class=\"assignme"); !strings.Contains(form, `name="assignee" value="me"`) {
		t.Errorf("the menu has no Assign to me:\n%s", menu)
	}
	if !strings.Contains(menu, `data-select-row="`+ref+`"`) || !strings.Contains(menu, `href="/tasks?select=1"`) {
		t.Errorf("the menu has no Select leading into select mode:\n%s", menu)
	}
	if !strings.Contains(page, `<datalist id="row-handles">`) {
		t.Error("the assignee field has no handles to suggest")
	}

	b.post("/tasks/"+ref+"/assign", url.Values{"assignee": {"me"}}).Body.Close()
	menu = popoverBlock(t, b.page("/tasks"), "rmore-"+ref)
	if strings.Contains(menu, `class="assignme"`) {
		t.Error("Assign to me is offered on a task already assigned to the reader")
	}
}

// TestAssignToMeOnTheTaskScreenAndTheBulkBar covers the other two places the
// one-press assign lives.
func TestAssignToMeOnTheTaskScreenAndTheBulkBar(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := aliceCtx(f)
	b := f.as("alice")
	alice := f.actorFor("alice").ID
	one, two := b.createTask("infra", "one"), b.createTask("infra", "two")

	menu := actionsMenu(t, b.page("/tasks/"+one), one)
	if form := menuForm(t, menu, "/tasks/"+one+"/assign\" class=\"assignme"); !strings.Contains(form, `name="assignee" value="me"`) {
		t.Errorf("the task screen's menu has no Assign to me:\n%s", menu)
	}

	bar := between(t, b.page("/tasks?select=1"), `<div id="bulk-who"`, "</div>\n")
	if !strings.Contains(bar, `formaction="/tasks/bulk?action=assign" name="assign_me" value="1"`) {
		t.Fatalf("the bulk bar has no Assign to me:\n%s", bar)
	}
	resp := b.post("/tasks/bulk?action=assign", url.Values{"ref": {one, two}, "assign_me": {"1"}, "assignee": {""}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	for _, ref := range []string{one, two} {
		parsed, _ := core.ParseTaskRef(ref)
		task, err := f.svc.GetTask(ctx, parsed)
		if err != nil {
			t.Fatal(err)
		}
		if task.AssigneeActorID != alice {
			t.Errorf("%s is assigned to %q after Assign to me, want alice", ref, task.AssigneeActorID)
		}
	}
}
