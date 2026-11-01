// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// templateID reads the identifier of the template a test saved by name.
func templateID(t *testing.T, f *fixture, name string) string {
	t.Helper()
	tpl, err := f.svc.GetTemplate(f.ctx(), name)
	if err != nil {
		t.Fatalf("reading template %q: %v", name, err)
	}
	return tpl.ID
}

func TestSaveAsTemplateIsOfferedToWritersOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "release")
	link := `href="/tasks/` + ref + `/template" data-dialog-load`

	if bar := actionsMenu(t, f.as("alice").page("/tasks/"+ref), ref); !strings.Contains(bar, link) {
		t.Errorf("a writer is not offered Save as template in the actions menu:\n%s", bar)
	}
	if page := f.as("viewer").page("/tasks/" + ref); strings.Contains(page, "/tasks/"+ref+"/template") {
		t.Errorf("a read-only reader is offered Save as template")
	}
}

func TestSavingATemplateReturnsToTheTaskAndRefusesATakenName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "release notes")

	form := formAt(t, b.page("/tasks/"+ref+"/template"), "/tasks/"+ref+"/template")
	if got := inputValue(t, form, "tpl-name"); got != "release notes" {
		t.Errorf("name = %q, want the task's title", got)
	}
	if !strings.Contains(between(t, form, `value="project"`, ">"), "checked") {
		t.Errorf("the template does not default to the task's project:\n%s", form)
	}

	resp := b.post("/tasks/"+ref+"/template", url.Values{"name": {"Release"}, "scope": {"tenant"}, "due_in": {"3d"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path != "/tasks/"+ref || loc.Query().Get("flash") != "saved as a template" {
		t.Fatalf("redirect = %q, want the task with a toast", loc)
	}
	tpl, err := f.svc.GetTemplate(f.ctx(), "release")
	if err != nil || !tpl.TenantWide() || tpl.DueIn.Human() != "3d" || tpl.Task.Title != "release notes" {
		t.Fatalf("saved = %+v, %v", tpl, err)
	}

	again := b.post("/tasks/"+ref+"/template", url.Values{"name": {"release"}, "scope": {"tenant"}, "due_in": {"2d"}})
	body := readAll(t, again)
	wantStatus(t, again, http.StatusConflict)
	refused := formAt(t, body, "/tasks/"+ref+"/template")
	if got := inputValue(t, refused, "tpl-name"); got != "release" {
		t.Errorf("the refused name was not kept: %q", got)
	}
	if got := inputValue(t, refused, "tpl-due"); got != "2d" {
		t.Errorf("the refused offset was not kept: %q", got)
	}
	if !strings.Contains(refused, `id="name-error"`) || !strings.Contains(refused, "already exists") {
		t.Errorf("the refusal is not drawn under the name:\n%s", refused)
	}
}

func TestTheTemplatesScreenListsAndEditsAndDeletes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "standup")
	if _, err := f.svc.SaveTemplate(f.ctx(), core.SaveTemplateInput{Name: "daily", TaskRef: ref}); err != nil {
		t.Fatal(err)
	}
	id := templateID(t, f, "daily")

	page := b.page("/templates")
	row := between(t, page, `<a href="/templates/`+id+`">`, "</tr>")
	if !strings.Contains(row, "daily") || !strings.Contains(row, "infra") || !strings.Contains(row, `/templates/new?template=`+id) {
		t.Errorf("the row does not name the template, its project and Use:\n%s", row)
	}
	if !strings.Contains(nav(t, page), `href="/templates"`) {
		t.Errorf("the navigation does not lead to Templates")
	}

	form := formAt(t, b.page("/templates/"+id), "/templates/"+id+"/update")
	if got := inputValue(t, form, "tpl-title"); got != "standup" {
		t.Errorf("title = %q, want the template's", got)
	}
	resp := b.post("/templates/"+id+"/update", url.Values{"name": {"morning"}, "project_ref": {""},
		"title": {"Standup {N}"}, "priority": {"2"}, "tags": {"daily, team"}, "due_in": {"1d"},
		"custom_fields": {""}, "body": {"- [ ] news"}, "repeat": {""}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	got, err := f.svc.GetTemplate(f.ctx(), "morning")
	if err != nil || !got.TenantWide() || got.Task.Title != "Standup {N}" || got.Task.Priority != core.PriorityHigh ||
		len(got.Task.Tags) != 2 || got.Task.Body != "- [ ] news" {
		t.Fatalf("edited = %+v, %v", got, err)
	}

	bad := b.post("/templates/"+id+"/update", url.Values{"name": {"morning"}, "title": {"x"}, "due_in": {"soon"}})
	body := readAll(t, bad)
	wantStatus(t, bad, http.StatusBadRequest)
	if refused := formAt(t, body, "/templates/"+id+"/update"); !strings.Contains(refused, `id="due_in-error"`) {
		t.Errorf("a bad offset is not refused under its field:\n%s", refused)
	}

	gone := b.post("/templates/"+id+"/delete", url.Values{})
	_ = gone.Body.Close()
	wantStatus(t, gone, http.StatusSeeOther)
	if _, err := f.svc.GetTemplate(f.ctx(), "morning"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the template survived its delete: %v", err)
	}
}

func TestNewFromTemplateFromTheAddBox(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "release {N}")
	if _, err := f.svc.SaveTemplate(f.ctx(), core.SaveTemplateInput{Name: "release", TaskRef: ref}); err != nil {
		t.Fatal(err)
	}
	id := templateID(t, f, "release")

	menu := between(t, b.page("/tasks"), `<div id="addtpl"`, "</div>")
	if !strings.Contains(menu, `href="/templates/new?template=`+id+`&amp;next=`) || !strings.Contains(menu, "popover") {
		t.Fatalf("the add box menu does not offer the template:\n%s", menu)
	}

	form := formAt(t, b.page("/templates/new?template="+id+"&next=/tasks"), "/templates/new")
	if !strings.Contains(form, `<option value="`+id+`" selected>`) {
		t.Errorf("the template asked for is not chosen:\n%s", form)
	}

	resp := b.post("/templates/new", url.Values{"template": {id}, "next": {"/tasks"}, "due_in": {"3d"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	made := loc.Query().Get("flashref")
	if loc.Path != "/tasks" || loc.Query().Get("flash") != "task created from a template" || made == "" || made == ref {
		t.Fatalf("redirect = %q, want the listing with a toast naming a new task", loc)
	}
	task, err := f.svc.GetTask(f.ctx(), core.MustParseTaskRef(made))
	if err != nil || task.Title != "release 1" || task.DueAt == nil {
		t.Fatalf("made = %+v, %v", task, err)
	}
}

func TestNewFromTemplateRefusesInPlace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "shared")
	if _, err := f.svc.SaveTemplate(f.ctx(), core.SaveTemplateInput{Name: "shared", TaskRef: ref, TenantWide: true}); err != nil {
		t.Fatal(err)
	}
	id := templateID(t, f, "shared")

	resp := b.post("/templates/new", url.Values{"template": {id}, "title": {"kept"}, "project_ref": {"nope"}})
	body := readAll(t, resp)
	wantStatus(t, resp, http.StatusNotFound)
	form := formAt(t, body, "/templates/new")
	if got := inputValue(t, form, "use-title"); got != "kept" {
		t.Errorf("the title was not kept: %q", got)
	}
	if !strings.Contains(form, `id="project_ref-error"`) {
		t.Errorf("the refusal is not drawn under the project:\n%s", form)
	}
}
