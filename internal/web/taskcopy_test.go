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

// titleBar is the task screen's title bar, where Copy to is offered.
func titleBar(t *testing.T, page string) string {
	t.Helper()
	bar := between(t, page, `<div class="titlebar"`, "</div>")
	if bar == "" {
		t.Fatalf("the screen draws no title bar")
	}
	return bar
}

// aboutCard is the task screen's About card.
func aboutCard(t *testing.T, page string) string {
	t.Helper()
	card := between(t, page, "<h3>About</h3>", "</dl>")
	if card == "" {
		t.Fatalf("the screen draws no About card")
	}
	return card
}

// checkbox reads one checkbox input of a form by its name.
func checkbox(t *testing.T, form, name string) string {
	t.Helper()
	box := between(t, form, `type="checkbox" name="`+name+`"`, ">")
	if !strings.Contains(form, `type="checkbox" name="`+name+`"`) {
		t.Fatalf("the form has no %s checkbox:\n%s", name, form)
	}
	return box
}

func TestCopyToIsOfferedToWritersOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "copy me")
	link := `href="/tasks/` + ref + `/copy" data-dialog-load`

	if bar := titleBar(t, f.as("alice").page("/tasks/"+ref)); !strings.Contains(bar, link) {
		t.Errorf("a writer is not offered Copy to in the title bar:\n%s", bar)
	}
	if page := f.as("viewer").page("/tasks/" + ref); strings.Contains(page, "/tasks/"+ref+"/copy") {
		t.Errorf("a read-only reader is offered Copy to")
	}
}

func TestTheCopyFormStartsFromTheTask(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")

	form := formAt(t, b.page("/tasks/"+ref+"/copy"), "/tasks/"+ref+"/copy")
	if !strings.Contains(form, `<option value="infra" selected>`) {
		t.Errorf("the task's own project is not selected:\n%s", form)
	}
	if !strings.Contains(form, `<option value="web">`) {
		t.Errorf("another project is not offered:\n%s", form)
	}
	if got := inputValue(t, form, "copy-title"); got != "rotate keys" {
		t.Errorf("title = %q, want the task's", got)
	}
	if !strings.Contains(checkbox(t, form, "link"), "checked") {
		t.Errorf("link is not ticked by default")
	}
	for _, name := range []string{"keep_status", "with_comments", "with_subtasks", "with_dependencies"} {
		if strings.Contains(checkbox(t, form, name), "checked") {
			t.Errorf("%s is ticked by default", name)
		}
	}
}

func TestCopyingStaysWithAToastLinkingTheCopy(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")
	child := b.createTask("infra", "the child")
	if _, err := f.svc.UpdateTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, child)},
		core.UpdateTaskInput{ParentRef: &ref}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AddComment(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)}, "a note"); err != nil {
		t.Fatal(err)
	}

	resp := b.post("/tasks/"+ref+"/copy", url.Values{"project_ref": {"web"}, "title": {"rotate again"},
		"with_comments": {"1"}, "with_subtasks": {"1"}, "link": {"1"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	copied := loc.Query().Get("flashref")
	if loc.Path != "/tasks/"+ref || loc.Query().Get("flash") != "task copied" || !strings.HasPrefix(copied, "web-") {
		t.Fatalf("redirect = %q, want the source with a toast naming a web task", loc)
	}

	toast := between(t, b.page(resp.Header.Get("Location")), `<p class="flash">`, "</p>")
	if !strings.Contains(toast, "task copied") || !strings.Contains(toast, `href="/tasks/`+copied+`"`) {
		t.Errorf("the toast does not link the copy:\n%s", toast)
	}

	tree, err := f.svc.TaskTree(f.ctx(), core.TaskRef{ProjectKey: "web", Seq: seqOfRef(t, copied)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 2 || tree[0].Title != "rotate again" || tree[1].Title != "the child" {
		t.Errorf("copied tree = %+v, want the renamed task and its subtask", tree)
	}
	comments, err := f.svc.ListComments(f.ctx(), core.TaskRef{ID: tree[0].ID})
	if err != nil || len(comments) != 1 || !strings.HasSuffix(comments[0].Body, "a note") {
		t.Errorf("copied comments = %+v, %v", comments, err)
	}

	about := aboutCard(t, b.page("/tasks/"+copied))
	if !strings.Contains(about, "Copied from") || !strings.Contains(about, `href="/tasks/`+ref+`"`) {
		t.Errorf("the copy's About card does not name its source:\n%s", about)
	}
	if strings.Contains(aboutCard(t, b.page("/tasks/"+ref)), "Copied from") {
		t.Errorf("the source claims to be a copy")
	}
}

func TestCopyingOpensTheCopyWhenAddingDoes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.OpenAfterAddCookie, "1")
	ref := b.createTask("infra", "rotate keys")

	resp := b.post("/tasks/"+ref+"/copy", url.Values{"project_ref": {"infra"}, "title": {"rotate keys"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path == "/tasks/"+ref || !strings.HasPrefix(loc.Path, "/tasks/infra-") || loc.Query().Get("flash") != "task copied" {
		t.Fatalf("redirect = %q, want the copy's own screen", loc)
	}
	copied := strings.TrimPrefix(loc.Path, "/tasks/")
	if strings.Contains(aboutCard(t, b.page(loc.Path)), "Copied from") {
		t.Errorf("an unlinked copy %s names a source", copied)
	}
}

func TestTheCopyFormDrawsInTheDialog(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")

	resp := b.getDialog("/tasks/" + ref + "/copy")
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if !strings.HasPrefix(strings.TrimSpace(page), `<div class="taskdlg-frag" data-task-dialog="`+ref+`"`) {
		t.Fatalf("the dialog's drawing is not a dialog fragment:\n%.300s", page)
	}
	if strings.Contains(page, "<html") || !strings.Contains(page, `id="dlg-copy-title"`) {
		t.Errorf("the dialog drew the whole page or ids that collide with the page under it")
	}

	flashed := body(t, b.getDialog("/tasks/"+ref+"?flash=task+copied&flashref=infra-9"))
	toast := between(t, flashed, `<p class="flash" role="status">`, "</p>")
	if !strings.Contains(toast, `href="/tasks/infra-9"`) {
		t.Errorf("the dialog's toast does not link the copy:\n%s", toast)
	}
}

func TestTheCopyFormIsLaidOutAsTheCreateForms(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")
	page := b.page("/tasks/" + ref + "/copy")
	form := formAt(t, page, "/tasks/"+ref+"/copy")
	if !strings.Contains(page, `<form method="post" class="cform copyform" data-form action="/tasks/`+ref+`/copy">`) {
		t.Errorf("the copy form is not a shared create form:\n%s", form)
	}
	if !strings.Contains(form, `class="cform-lead" name="title"`) {
		t.Errorf("the title is not the lead field:\n%s", form)
	}
	side := between(t, form, `<div class="cform-side">`, "</select>")
	if !strings.Contains(side, `name="project_ref"`) {
		t.Errorf("the project is not in the side column:\n%s", side)
	}
	group := between(t, form, `<fieldset class="cform-group copyopts">`, "</fieldset>")
	for _, name := range []string{"keep_status", "with_comments", "with_subtasks", "with_dependencies", "link"} {
		if !strings.Contains(group, `name="`+name+`"`) {
			t.Errorf("%s is not in the options group:\n%s", name, group)
		}
	}
	foot := between(t, form, `<div class="cform-foot">`, "</button>") + "</button>"
	order(t, foot, "data-form-keys", `href="/tasks/`+ref+`" data-form-cancel`, `<button type="submit">Copy</button>`)
}

func TestARefusedCopyKeepsWhatWasChosen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")
	cases := []struct {
		name   string
		form   url.Values
		status int
		field  string
	}{
		{"unknown project", url.Values{"project_ref": {"nope"}, "title": {"kept title"}, "with_subtasks": {"1"}, "next": {"/tasks?q=tag%3Aops"}},
			http.StatusNotFound, "project_ref"},
		{"title too long", url.Values{"project_ref": {"infra"}, "title": {strings.Repeat("x", core.MaxTitleLength+1)}, "with_subtasks": {"1"}, "next": {"/tasks?q=tag%3Aops"}},
			http.StatusBadRequest, "title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := b.post("/tasks/"+ref+"/copy", tc.form)
			page := body(t, resp)
			wantStatus(t, resp, tc.status)
			form := formAt(t, page, "/tasks/"+ref+"/copy")
			if got := inputValue(t, form, "copy-title"); got != tc.form.Get("title") {
				t.Errorf("title = %.40q, want what was typed", got)
			}
			if !strings.Contains(checkbox(t, form, "with_subtasks"), "checked") || strings.Contains(checkbox(t, form, "link"), "checked") {
				t.Errorf("the checkboxes were not kept as submitted:\n%s", form)
			}
			if !strings.Contains(form, `name="next" value="/tasks?q=tag%3Aops"`) {
				t.Errorf("the listing to return to was dropped:\n%s", form)
			}
			if !strings.Contains(form, `<p class="field-error" id="`+tc.field+`-error" role="alert">`) {
				t.Errorf("the message is not under %s:\n%s", tc.field, form)
			}
		})
	}

	req, err := http.NewRequest(http.MethodPost, f.server.URL+"/tasks/"+ref+"/copy",
		strings.NewReader(url.Values{"project_ref": {"nope"}, "title": {"x"}, "csrf_token": {b.csrf()}}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(web.DialogHeader, "1")
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if !strings.HasPrefix(strings.TrimSpace(page), `<div class="taskdlg-frag" data-task-dialog="`+ref+`"`) ||
		!strings.Contains(page, `id="project_ref-error"`) {
		t.Errorf("a refusal in the dialog is not the form drawn again for the dialog:\n%.400s", page)
	}
}

func TestALaneMenuCopiesAndReturnsToTheListing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "copy from the lane")
	b.chooseLanes("/tasks")
	menu := moreMenu(t, laneCard(t, b.page("/tasks"), ref), ref)
	link := `href="/tasks/` + ref + `/copy?next=%2ftasks" data-dialog-load data-copy-to`
	if !strings.Contains(strings.ToLower(menu), strings.ToLower(link)) {
		t.Fatalf("the lane menu does not offer Copy to returning to the listing:\n%s", menu)
	}

	form := formAt(t, b.page("/tasks/"+ref+"/copy?next=%2Ftasks%3Fq%3Dtag%3Aops%26flash%3Dold"), "/tasks/"+ref+"/copy")
	if !strings.Contains(form, `name="next" value="/tasks?q=tag%3Aops"`) {
		t.Errorf("the form does not carry the listing, minus its old toast:\n%s", form)
	}
	if !strings.Contains(form, `class="secondary cform-cancel" href="/tasks?q=tag%3Aops" data-form-cancel`) {
		t.Errorf("Cancel does not return to the listing:\n%s", form)
	}

	resp := b.post("/tasks/"+ref+"/copy", url.Values{"project_ref": {"web"}, "title": {"copy from the lane"},
		"next": {"/tasks?q=tag%3Aops"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path != "/tasks" || loc.Query().Get("q") != "tag:ops" || loc.Query().Get("flash") != "task copied" ||
		!strings.HasPrefix(loc.Query().Get("flashref"), "web-") {
		t.Fatalf("redirect = %q, want the listing with a toast naming the copy", loc)
	}

	for _, next := range []string{"https://evil.example/tasks", "/tasks/" + ref, "/settings"} {
		resp := b.post("/tasks/"+ref+"/copy", url.Values{"title": {"again"}, "next": {next}})
		_ = resp.Body.Close()
		if got := resp.Header.Get("Location"); !strings.HasPrefix(got, "/tasks/"+ref+"?") {
			t.Errorf("next %q sent the reader to %q, want the source task", next, got)
		}
	}
}
