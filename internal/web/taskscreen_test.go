// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// propsPanel is the task screen's properties sidebar, and nothing else.
func propsPanel(t *testing.T, page string) string {
	t.Helper()
	panel := between(t, page, `<aside class="taskprops"`, "</aside>")
	if panel == "" {
		t.Fatalf("the screen draws no properties sidebar")
	}
	return panel
}

// propRow is one property row of the sidebar, found by the key it carries.
// A row holds no div of its own, so the first close is its own.
func propRow(t *testing.T, page, key string) string {
	t.Helper()
	row := between(t, propsPanel(t, page), `<div class="prop" data-prop="`+key+`">`, "</div>")
	if row == "" {
		t.Fatalf("the properties sidebar has no %s row", key)
	}
	return row
}

// propValue is the value half of a property row, without its label or tip.
func propValue(t *testing.T, page, key string) string {
	t.Helper()
	return between(t, propRow(t, page, key)+"</div>", "<dd", "</dd>")
}

// actionsMenu is the one menu the task screen's header opens.
func actionsMenu(t *testing.T, page, ref string) string {
	t.Helper()
	menu := popover(t, page, "taskmenu-"+ref)
	if menu == "" {
		t.Fatalf("the task screen has no actions menu")
	}
	return menu
}

// header is the task screen's header: the reference, the title and the menu
// button, and not the menus it opens.
func header(t *testing.T, page string) string {
	t.Helper()
	head := between(t, page, `<header class="titlebar taskhead"`, "</header>")
	if head == "" {
		t.Fatalf("the screen draws no task header")
	}
	return head
}

// Every property is a row of the sidebar, and a writer edits each one where
// it is read: a form in the row that posts to the task. A reader who may not
// write sees the same rows with no form in them.
func TestTheTaskScreenKeepsEveryPropertyInTheSidebar(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	defined := b.post("/projects/infra/fields", url.Values{"key": {"severity"},
		"label": {"Severity"}, "type": {"enum"}, "enum_options": {"low, high"}})
	_ = defined.Body.Close()
	ref := b.createTask("infra", "the whole picture")
	page := b.page("/tasks/" + ref)

	for _, key := range []string{"status", "priority", "assignee", "due", "project", "tags", "repeat", "field.severity"} {
		propRow(t, page, key)
	}
	for _, key := range []string{"assignee", "due", "repeat", "field.severity"} {
		row := propRow(t, page, key)
		if !strings.Contains(row, `action="/tasks/`+ref+`">`) || !strings.Contains(row, `name="version"`) {
			t.Errorf("the %s row does not edit in place:\n%s", key, row)
		}
	}
	if !strings.Contains(propRow(t, page, "tags"), `action="/tasks/`+ref+`/tags"`) {
		t.Errorf("the tags row offers no way to add a tag")
	}
	if !strings.Contains(propRow(t, page, "field.severity"), `<option value="high">`) {
		t.Errorf("an enum field is not offered as its options")
	}
	meta := between(t, page, `<dl class="props propmeta">`, "</dl>")
	for _, term := range []string{"Created by", "Created", "Updated", "Version", "Id"} {
		if !strings.Contains(meta, "<dt>"+term+"</dt>") {
			t.Errorf("the record's facts lack %s:\n%s", term, meta)
		}
	}

	viewer := f.as("viewer").page("/tasks/" + ref)
	if panel := propsPanel(t, viewer); strings.Contains(panel, "<form") {
		t.Errorf("a reader who may not write is offered a form in the properties:\n%s", panel)
	}
	propRow(t, viewer, "assignee")
}

// One menu holds every action on the task as a whole, and none of them is
// drawn anywhere else on the screen.
func TestTheTaskScreenHasOneActionsMenu(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "act on me")
	page := b.page("/tasks/" + ref)

	if head := header(t, page); strings.Count(head, "popovertarget=") != 1 ||
		!strings.Contains(head, `popovertarget="taskmenu-`+ref+`"`) {
		t.Errorf("the header does not open exactly the one actions menu:\n%s", head)
	}
	menu := actionsMenu(t, page, ref)
	for _, want := range []string{
		`data-copy-link="/tasks/` + ref + `"`,
		`href="/tasks/` + ref + `/copy"`,
		`href="/tasks/` + ref + `/template"`,
		`popovertarget="snooze-` + ref + `"`,
		`action="/tasks/` + ref + `/delete"`,
	} {
		if !strings.Contains(menu, want) {
			t.Errorf("the actions menu lacks %s:\n%s", want, menu)
		}
	}
	for _, once := range []string{`href="/tasks/` + ref + `/copy"`, `href="/tasks/` + ref + `/template"`, `action="/tasks/` + ref + `/delete"`} {
		if n := strings.Count(page, once); n != 1 {
			t.Errorf("%s is drawn %d times, want once, in the menu", once, n)
		}
	}
	if strings.Contains(page, "Danger zone") {
		t.Error("the screen still draws a separate Danger zone")
	}
}

// The description is read as text and edited in place: Edit is a switch over
// the text, not a second card holding another copy of it.
func TestTheDescriptionEditsInPlace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	task, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{ProjectRef: "infra", Title: "written", Body: "the plan"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	page := b.page("/tasks/" + task.Ref)

	edit := between(t, page, `<details class="descedit">`, "</form>")
	if !strings.Contains(edit, `<span class="when-shut">Edit</span>`) {
		t.Errorf("the description has no Edit switch:\n%s", edit)
	}
	if !strings.Contains(edit, `id="body" name="body"`) || !strings.Contains(edit, `data-dirty-save`) {
		t.Errorf("the editor is not inside the switch:\n%s", edit)
	}
	if strings.Contains(page, "<h2>Edit</h2>") {
		t.Error("the screen still draws a separate Edit card")
	}
	if strings.Count(page, ">the plan</textarea>") != 1 || !strings.Contains(between(t, page, `<div id="task-desc">`, "</section>"), "<p>the plan</p>") {
		t.Errorf("the text is not read above its editor")
	}

	bare := b.page("/tasks/" + b.createTask("infra", "empty"))
	if !strings.Contains(between(t, bare, `<details class="descedit">`, "</summary>"), "Add a description") {
		t.Errorf("an empty description does not offer to add one")
	}
	if strings.Contains(between(t, f.as("viewer").page("/tasks/"+task.Ref), `<section class="tsec tdesc`, "</section>"), "descedit") {
		t.Error("a reader who may not write is offered the editor")
	}
}

// The due date is a property like the others: a form naming it sets it, a
// blank one clears it, and a form that does not name it leaves it alone.
func TestTheDueDateEditsFromTheSidebar(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "deadline")
	save := func(fields url.Values) {
		t.Helper()
		got, err := f.svc.GetTask(f.ctx(), core.MustParseTaskRef(ref))
		if err != nil {
			t.Fatal(err)
		}
		fields.Set("version", strconv.Itoa(got.Version))
		fields.Set("title", "deadline")
		resp := b.post("/tasks/"+ref, fields)
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
	}
	due := func() *time.Time {
		t.Helper()
		got, err := f.svc.GetTask(f.ctx(), core.MustParseTaskRef(ref))
		if err != nil {
			t.Fatal(err)
		}
		return got.DueAt
	}

	save(url.Values{"due_at": {"2099-03-04"}})
	if got := due(); got == nil || got.UTC().Format(time.DateOnly) != "2099-03-04" {
		t.Fatalf("due = %v, want 2099-03-04", got)
	}
	if got := inputValue(t, b.page("/tasks/"+ref), "due"); got != "2099-03-04" {
		t.Errorf("the due field holds %q", got)
	}
	save(url.Values{"repeat": {""}})
	if due() == nil {
		t.Fatal("a save that did not name the due date cleared it")
	}
	save(url.Values{"due_at": {""}})
	if got := due(); got != nil {
		t.Fatalf("due = %v, want it cleared", got)
	}
}

// Every block the stylesheet opens, it closes. A merge once dropped the two
// braces closing the phone rule for the snooze menu, and every rule after it
// (the inbox, the templates, the task screen) was parsed as nested inside a
// popover that is open only on a phone, so none of it applied anywhere else.
func TestTheStylesheetClosesEveryBlock(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))
	depth, line := 0, 1
	for i := 0; i < len(sheet); i++ {
		switch {
		case strings.HasPrefix(sheet[i:], "/*"):
			end := strings.Index(sheet[i+2:], "*/")
			if end < 0 {
				t.Fatalf("a comment opened on line %d never closes", line)
			}
			line += strings.Count(sheet[i:i+2+end], "\n")
			i += end + 3
		case sheet[i] == '"':
			end := strings.IndexByte(sheet[i+1:], '"')
			if end < 0 {
				t.Fatalf("a string opened on line %d never closes", line)
			}
			i += end + 1
		case sheet[i] == '\n':
			line++
		case sheet[i] == '{':
			depth++
		case sheet[i] == '}':
			depth--
			if depth < 0 {
				t.Fatalf("line %d closes a block that was never opened", line)
			}
		}
	}
	if depth != 0 {
		t.Fatalf("the stylesheet ends with %d blocks still open", depth)
	}
}
