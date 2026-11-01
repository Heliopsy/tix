// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// addForm is Add with details as the task screen draws it, popover or in place.
func addForm(t *testing.T, page string) string {
	t.Helper()
	open := between(t, page, `<div id="addmore"`, "</form>")
	if open == "" {
		t.Fatalf("the task screen has no Add with details form")
	}
	return open
}

// projectForm is the new project form as the projects screen draws it.
func projectForm(t *testing.T, page string) string {
	t.Helper()
	form := between(t, page, `<div id="newproject"`, "</form>")
	if form == "" {
		t.Fatalf("the projects screen has no new project form")
	}
	return form
}

// order fails unless every marker appears in markup, in the order given.
func order(t *testing.T, markup string, markers ...string) {
	t.Helper()
	last := -1
	for _, m := range markers {
		at := strings.Index(markup, m)
		if at < 0 {
			t.Fatalf("the form has no %s:\n%s", m, markup)
		}
		if at < last {
			t.Errorf("%s comes before the field drawn ahead of it", m)
		}
		last = at
	}
}

func TestAddWithDetailsLeadsWithTheTitleAndFoldsTheRareFields(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	form := addForm(t, f.as("alice").page("/tasks"))

	order(t, form, `name="title"`, `name="body"`, `name="project_ref"`, `name="priority"`,
		`name="due_at"`, `name="assignee"`, `name="tags"`, `<details class="cform-more">`, `name="repeat"`)
	title := tagWith(t, form, `name="title"`)
	for _, want := range []string{"autofocus", "required", `class="cform-lead"`} {
		if !strings.Contains(title, want) {
			t.Errorf("the title field lacks %s:\n%s", want, title)
		}
	}
	label := between(t, form, `<label for="new-title">`, "</div>")
	if !strings.Contains(label, `class="req"`) {
		t.Errorf("the title is not marked as required:\n%s", label)
	}
	more := between(t, form, `<details class="cform-more"`, `<div class="cform-foot">`)
	for _, name := range []string{"repeat", "parent", "depends_on"} {
		if !strings.Contains(more, `name="`+name+`"`) {
			t.Errorf("%s is not under More options:\n%s", name, more)
		}
	}
	if strings.HasPrefix(more, " open") {
		t.Error("More options is open on a form holding nothing")
	}
	if !strings.Contains(form, `data-form`) || !strings.Contains(form, `data-form-keys`) {
		t.Error("the form is not marked for the keyboard handling in forms.js")
	}
}

func TestAddWithDetailsSetsTheDueDateAssigneeParentAndDependencies(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	parent := b.createTask("infra", "the parent")
	blocker := b.createTask("infra", "the blocker")

	resp := b.post("/tasks", url.Values{"project_ref": {"infra"}, "title": {"the child"},
		"due_at": {"2026-12-01"}, "assignee": {"alice"}, "parent": {parent}, "depends_on": {blocker}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page, err := f.svc.ListTasks(f.ctx(), core.TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var child *core.Task
	for i := range page.Tasks {
		if page.Tasks[i].Title == "the child" {
			child = &page.Tasks[i]
		}
	}
	if child == nil {
		t.Fatal("the task was not created")
	}
	want := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	if child.DueAt == nil || !child.DueAt.Equal(want) {
		t.Errorf("due = %v, want %v", child.DueAt, want)
	}
	if child.AssigneeActorID != f.actorA.ID {
		t.Errorf("assignee = %q, want alice", child.AssigneeActorID)
	}
	if child.ParentID == "" {
		t.Error("the task was created without its parent")
	}
	deps, err := f.svc.ListDependencies(f.ctx(), core.TaskRef{ID: child.ID})
	if err != nil || len(deps) != 1 {
		t.Errorf("dependencies = %+v, %v; want the blocker", deps, err)
	}
}

func TestTheAddFormsPreselectTheFilteredProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	made := b.post("/projects", url.Values{"key": {"web"}, "name": {"Web"}})
	_ = made.Body.Close()
	wantStatus(t, made, http.StatusSeeOther)

	page := b.page("/tasks?q=" + url.QueryEscape("project:web"))
	quick := between(t, page, `class="quickadd">`, "</form>")
	if !strings.Contains(quick, `<option value="web" selected data-short="WEB">`) {
		t.Errorf("the one-line box does not preselect web:\n%s", quick)
	}
	if !strings.Contains(addForm(t, page), `<option value="web" selected>`) {
		t.Error("Add with details does not preselect web")
	}
	if strings.Contains(b.page("/tasks"), `<option value="web" selected`) {
		t.Error("an unfiltered list preselects a project")
	}
}

func TestARefusedAddKeepsWhatWasTyped(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		form   url.Values
		field  string
		status int
	}{
		{"unknown parent", url.Values{"parent": {"infra-99"}}, "parent", http.StatusNotFound},
		{"bad repeat rule", url.Values{"repeat": {"whenever"}}, "repeat", http.StatusBadRequest},
		{"bad due date", url.Values{"due_at": {"someday"}}, "due_at", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			b := f.as("alice")
			tc.form.Set("project_ref", "infra")
			tc.form.Set("title", "write it down")
			tc.form.Set("body", "kept")
			resp := b.postFrom("/tasks?q=tag%3Aops", "/tasks", tc.form)
			page := body(t, resp)
			wantStatus(t, resp, tc.status)

			if !strings.Contains(page, `<div id="addmore" class="addpop is-inplace">`) {
				t.Fatalf("the form is not drawn in place:\n%s", page)
			}
			if got := inputValue(t, page, "new-title"); got != "write it down" {
				t.Errorf("the title came back as %q", got)
			}
			if !strings.Contains(between(t, page, `id="new-body"`, "</textarea>"), "kept") {
				t.Error("the description was lost")
			}
			control := tagWith(t, addForm(t, page), `name="`+tc.field+`"`)
			if !strings.Contains(control, `aria-describedby="`+tc.field+`-error"`) {
				t.Errorf("the %s field does not point at its message:\n%s", tc.field, control)
			}
			if !strings.Contains(page, `<p class="field-error" id="`+tc.field+`-error"`) {
				t.Errorf("no message under %s", tc.field)
			}
			if tc.field != "due_at" && !strings.Contains(page, `<details class="cform-more" open>`) {
				t.Error("More options is closed over the field the refusal is about")
			}
			if !strings.Contains(page, `<input name="q" value="tag:ops"`) {
				t.Error("the listing under the form lost its filter")
			}
		})
	}
}

func TestNewProjectStartsFromTheName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/projects")

	head := between(t, page, `<div class="pagehead">`, `<div id="newproject"`)
	if !strings.Contains(head, `popovertarget="newproject"`) {
		t.Errorf("New project is not a button in the header:\n%s", head)
	}
	form := projectForm(t, page)
	order(t, form, `name="name"`, `name="key"`, `name="workflow_key"`, `name="description"`, `<legend>Appearance</legend>`, `name="color"`, `name="icon"`)
	name := tagWith(t, form, `name="name"`)
	if !strings.Contains(name, "autofocus") || !strings.Contains(name, "data-key-source") {
		t.Errorf("the name field is not focused or does not drive the key:\n%s", name)
	}
	key := tagWith(t, form, `name="key"`)
	if !strings.Contains(key, `data-taken="infra"`) {
		t.Errorf("the key field does not know which keys are taken:\n%s", key)
	}
	appearance := between(t, form, `<fieldset class="cform-side cform-group">`, "</fieldset>")
	if !strings.Contains(appearance, `name="color"`) || !strings.Contains(appearance, `name="icon"`) {
		t.Errorf("colour and icon are not grouped under Appearance:\n%s", appearance)
	}
	if strings.Contains(page, `<details class="panel addmore">`) {
		t.Error("the projects screen still folds New project under the list")
	}
}

func TestARefusedProjectKeepsWhatWasTyped(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/projects", url.Values{"key": {"infra"}, "name": {"Infra two"}, "description": {"kept"}})
	page := body(t, resp)
	wantStatus(t, resp, http.StatusConflict)
	if !strings.Contains(page, `<div id="newproject" class="formsheet is-inplace">`) {
		t.Fatalf("the form is not drawn in place:\n%s", page)
	}
	if got := inputValue(t, page, "name"); got != "Infra two" {
		t.Errorf("the name came back as %q", got)
	}
	key := tagWith(t, projectForm(t, page), `name="key"`)
	if !strings.Contains(key, `aria-invalid="true"`) || !strings.Contains(key, "key-error") {
		t.Errorf("the key field is not marked as refused:\n%s", key)
	}
	if !strings.Contains(page, `<p class="field-error" id="key-error" role="alert">project key &#34;infra&#34; is already in use</p>`) {
		t.Errorf("the refusal is not under the key:\n%s", between(t, page, `name="key"`, "</fieldset>"))
	}

	saved := b.post("/projects/infra/update", url.Values{"name": {""}, "description": {"changed"}})
	board := body(t, saved)
	wantStatus(t, saved, http.StatusBadRequest)
	if !strings.Contains(board, `<details class="panel projsettings" open>`) {
		t.Error("the settings are not open after a refused save")
	}
	if !strings.Contains(board, `id="name-error"`) || !strings.Contains(between(t, board, `id="description"`, "</textarea>"), "changed") {
		t.Error("the refused save lost what was typed or its message")
	}
}

func TestARefusedProjectKeepsItsIconChoice(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").post("/projects", url.Values{"key": {"infra"}, "name": {"Infra two"},
		"icon_kind": {"builtin"}, "icon_name": {"rocket"}})
	page := body(t, resp)
	wantStatus(t, resp, http.StatusConflict)
	appearance := between(t, projectForm(t, page), `<fieldset class="cform-side cform-group">`, `<div class="cform-foot">`)
	for _, want := range []string{`value="builtin" checked`, `value="rocket" checked`} {
		if !strings.Contains(appearance, want) {
			t.Errorf("the refused form lost the icon choice (%s)", want)
		}
	}
}

func TestARefusedProjectKeepsItsCustomColour(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for path, form := range map[string]url.Values{
		"/projects":              {"key": {"infra"}, "name": {"Infra two"}, "color": {"#custom"}, "color_custom": {"#3a7bd5"}},
		"/projects/infra/update": {"name": {""}, "color": {"#custom"}, "color_custom": {"#3a7bd5"}},
	} {
		resp := b.post(path, form)
		page := body(t, resp)
		if resp.StatusCode < 400 {
			t.Fatalf("%s: the submission was not refused (%d)", path, resp.StatusCode)
		}
		field := colourFieldOf(t, page, "color")
		if !strings.Contains(field, `value="#custom" checked`) || !strings.Contains(field, `value="#3a7bd5"`) {
			t.Errorf("%s: the refused form lost the custom colour:\n%s", path, field)
		}
	}
}

// A group's "New task in" field names the state; a refused add keeps it, so
// correcting the title still lands the task in that column.
func TestARefusedAddKeepsTheStateItWasAddedIn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/tasks", url.Values{"project_ref": {"infra"}, "title": {"in a column"},
		"status": {"doing"}, "parent": {"infra-99"}})
	page := body(t, resp)
	wantStatus(t, resp, http.StatusNotFound)
	if !strings.Contains(addForm(t, page), `<input type="hidden" name="status" value="doing">`) {
		t.Error("the refused form dropped the state it was added in")
	}
	ok := b.post("/tasks", url.Values{"project_ref": {"infra"}, "title": {"in a column"}, "status": {"doing"},
		"due_at": {"2026-12-01"}, "assignee": {"alice"}})
	_ = ok.Body.Close()
	wantStatus(t, ok, http.StatusSeeOther)
	list, err := f.svc.ListTasks(f.ctx(), core.TaskFilter{})
	if err != nil || len(list.Tasks) != 1 || list.Tasks[0].Status != "doing" || list.Tasks[0].DueAt == nil || list.Tasks[0].AssigneeActorID == "" {
		t.Fatalf("the add lost its state, due date or assignee: %+v, %v", list.Tasks, err)
	}
}
