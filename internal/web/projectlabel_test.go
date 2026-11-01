// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// labelFixture is a tenant with two projects whose names and keys differ, a
// task in each, and a template, so every project picker has something to
// offer and something to be opened from.
func labelFixture(t *testing.T) (*fixture, *browser, string) {
	t.Helper()
	f := newFixture(t)
	name := "Infrastructure"
	if _, err := f.svc.UpdateProject(f.ctx(), "infra", core.UpdateProjectInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	seedProject(t, f.store, f.tenantA.ID, "ops")
	b := f.as("alice")
	ref := b.createTask("infra", "label me")
	b.createTask("ops", "and me")
	if _, err := f.svc.SaveTemplate(f.ctx(), core.SaveTemplateInput{Name: "daily", TaskRef: ref}); err != nil {
		t.Fatal(err)
	}
	return f, b, ref
}

// projectPickers names every control a project is chosen in: the screen it is
// on, and the text its select starts with. Each is read on its own, so a
// label drawn by one picker cannot stand in for another.
func projectPickers(t *testing.T, f *fixture, ref string) map[string][2]string {
	t.Helper()
	id := templateID(t, f, "daily")
	return map[string][2]string{
		"add box":            {"/tasks", `<select name="project_ref" aria-label="Project">`},
		"add with details":   {"/tasks", `<select id="new-project"`},
		"bulk copy":          {"/tasks", `<select id="bulk-copy-project"`},
		"heading new task":   {"/tasks", `<form method="post" action="/tasks" class="groupmenu-add">`},
		"copy to":            {"/tasks/" + ref + "/copy", `<select id="copy-project"`},
		"new from template":  {"/templates/new?template=" + id, `<select id="use-project"`},
		"template's project": {"/templates/" + id, `<select id="tpl-project"`},
		"delete move target": {"/projects/ops/delete", `<select id="move_to"`},
		"statistics filter":  {"/stats", `<select id="stats-project"`},
	}
}

// pickerAt returns one picker's options, from its opening text to its close.
func pickerAt(t *testing.T, page, start string) string {
	t.Helper()
	from := strings.Index(page, start)
	if from < 0 {
		t.Fatalf("no picker starting %s on the page", start)
	}
	rest := page[from:]
	end := strings.Index(rest, "</select>")
	if end < 0 {
		t.Fatalf("the picker starting %s never closes", start)
	}
	return rest[:end]
}

// Every picker names a project the way the reader chose, and every one keeps
// the key as the value a form submits.
func TestEveryProjectPickerNamesProjectsTheWayTheReaderChose(t *testing.T) {
	f, b, ref := labelFixture(t)
	b.setCookie(web.ListGroupCookie, web.ListGroupStatus)
	pickers := projectPickers(t, f, ref)
	styles := []struct {
		cookie, want string
	}{
		{"", ">Infrastructure (INFRA)</option>"},
		{web.ProjectLabelKeyName, ">INFRA · Infrastructure</option>"},
		{web.ProjectLabelName, ">Infrastructure</option>"},
		{web.ProjectLabelKey, ">INFRA</option>"},
		{"something stale", ">Infrastructure (INFRA)</option>"},
	}
	for _, s := range styles {
		b.setCookie(web.ProjectLabelCookie, s.cookie)
		pages := map[string]string{}
		for name, at := range pickers {
			page, ok := pages[at[0]]
			if !ok {
				page = b.page(at[0])
				pages[at[0]] = page
			}
			picker := pickerAt(t, page, at[1])
			if !strings.Contains(picker, `<option value="infra"`) {
				t.Errorf("%s: the option's value is no longer the key:\n%s", name, picker)
			}
			if !strings.Contains(picker, s.want) {
				t.Errorf("%s under %q: no option reads %s:\n%s", name, s.cookie, s.want, picker)
			}
		}
	}
}

// The setting is a form on the settings screen, and posting it stores the
// choice, while the default and anything unknown store nothing.
func TestTheProjectLabelIsChosenOnSettings(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	form := between(t, b.page(web.RouteSettings), `action="`+web.RouteProjectLabel+`"`, "</form>")
	for _, want := range []string{
		`<select id="projectlabel" name="label"`,
		`<option value="name-key" selected>Name (KEY)</option>`,
		`<option value="key-name" >KEY · Name</option>`,
		`<option value="name" >Name</option>`,
		`<option value="key" >KEY</option>`,
	} {
		if !strings.Contains(form, want) {
			t.Errorf("the settings form lacks %s:\n%s", want, form)
		}
	}

	resp := b.post(web.RouteProjectLabel, url.Values{"label": {web.ProjectLabelKeyName}, "next": {"/settings"}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	if got := b.cookie(web.ProjectLabelCookie); got != web.ProjectLabelKeyName {
		t.Fatalf("the cookie holds %q, want %q", got, web.ProjectLabelKeyName)
	}
	if form := between(t, b.page(web.RouteSettings), `action="`+web.RouteProjectLabel+`"`, "</form>"); !strings.Contains(form, `<option value="key-name" selected>`) {
		t.Errorf("the stored choice is not the one selected:\n%s", form)
	}

	for _, value := range []string{web.ProjectLabelNameKey, "bogus"} {
		resp = b.post(web.RouteProjectLabel, url.Values{"label": {value}, "next": {"/settings"}})
		_ = resp.Body.Close()
		if got := b.cookie(web.ProjectLabelCookie); got != "" {
			t.Errorf("posting %q left %q behind, want nothing", value, got)
		}
	}
}

// A long name is cut short, and the key after it never is.
func TestALongProjectNameIsShortenedButNeverItsKey(t *testing.T) {
	f := newFixture(t)
	name := "An extremely long project name that goes on"
	if _, err := f.svc.UpdateProject(f.ctx(), "infra", core.UpdateProjectInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	b := f.as("alice")
	picker := pickerAt(t, b.page("/tasks"), `<select name="project_ref" aria-label="Project">`)
	if !strings.Contains(picker, ">An extremely long proje… (INFRA)</option>") {
		t.Errorf("the long name is not cut with its key whole:\n%s", picker)
	}
}

// The add box and a heading's add field are capped narrow on a phone, so each
// of their options carries the key alone for the script to switch to,
// whenever the label holds a name that would push the key out of sight.
func TestCompactPickersCarryTheKeyAlone(t *testing.T) {
	f, b, ref := labelFixture(t)
	b.setCookie(web.ListGroupCookie, web.ListGroupStatus)
	pickers := projectPickers(t, f, ref)
	for _, s := range []struct {
		cookie string
		want   bool
	}{{"", true}, {web.ProjectLabelKeyName, true}, {web.ProjectLabelName, false}, {web.ProjectLabelKey, false}} {
		b.setCookie(web.ProjectLabelCookie, s.cookie)
		page := b.page("/tasks")
		for _, name := range []string{"add box", "heading new task"} {
			picker := pickerAt(t, page, pickers[name][1])
			has := strings.Contains(picker, `<option value="infra" data-short="INFRA">`)
			if has != s.want {
				t.Errorf("%s under %q: short form present %v, want %v:\n%s", name, s.cookie, has, s.want, picker)
			}
		}
	}
}

// The task screen's Project row names the project the way the reader's
// pickers do, and still links by the key.
func TestTheTaskScreenNamesItsProjectTheWayThePickersDo(t *testing.T) {
	_, b, ref := labelFixture(t)
	for _, s := range []struct{ cookie, want string }{
		{"", `<a href="/projects/infra">Infrastructure (INFRA)</a>`},
		{web.ProjectLabelKeyName, `<a href="/projects/infra">INFRA · Infrastructure</a>`},
		{web.ProjectLabelKey, `<a href="/projects/infra">INFRA</a>`},
	} {
		b.setCookie(web.ProjectLabelCookie, s.cookie)
		row := between(t, b.page("/tasks/"+ref), `data-prop="project"`, "</div>")
		if !strings.Contains(row, s.want) {
			t.Errorf("under %q the Project row is not %s:\n%s", s.cookie, s.want, row)
		}
	}
}
