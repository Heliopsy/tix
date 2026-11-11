// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestMoveToIsOfferedToWritersOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "move me")
	link := `href="/tasks/` + ref + `/move" data-dialog-load`

	if bar := actionsMenu(t, f.as("alice").page("/tasks/"+ref), ref); !strings.Contains(bar, link) {
		t.Errorf("a writer is not offered Move to in the actions menu:\n%s", bar)
	}
	if page := f.as("viewer").page("/tasks/" + ref); strings.Contains(page, "/tasks/"+ref+"/move") {
		t.Errorf("a read-only reader is offered Move to")
	}
}

func TestTheMoveFormLeavesOutTheTasksOwnProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")

	form := formAt(t, b.page("/tasks/"+ref+"/move"), "/tasks/"+ref+"/move")
	if strings.Contains(form, `<option value="infra"`) {
		t.Errorf("the task's own project is offered:\n%s", form)
	}
	if !strings.Contains(form, `<option value="web" selected>`) {
		t.Errorf("another project is not offered:\n%s", form)
	}
}

func TestMovingOpensTheTaskUnderItsNewReferenceAndTheOldOneRedirects(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")

	resp := b.post("/tasks/"+ref+"/move", url.Values{"project_ref": {"web"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	moved := strings.TrimPrefix(loc.Path, "/tasks/")
	if !strings.HasPrefix(moved, "web-") || loc.Query().Get("flash") != "task moved" {
		t.Fatalf("redirect = %q, want the task under a web reference with a toast", loc)
	}

	old := b.get("/tasks/" + ref)
	_ = old.Body.Close()
	wantStatus(t, old, http.StatusMovedPermanently)
	if got := old.Header.Get("Location"); got != "/tasks/"+moved {
		t.Errorf("old reference redirects to %q, want /tasks/%s", got, moved)
	}
}

func TestTheBulkBarMovesEverySelectedTaskToTheChosenProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	one, two := b.createTask("infra", "one"), b.createTask("infra", "two")

	resp := b.post("/tasks/bulk?action=move_project", url.Values{
		"ref": {one, two}, "project": {"infra"}, "move_project": {"web"}, "next": {"/tasks"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if flash := loc.Query().Get("flash"); flash != "Moved 2 tasks to project web" {
		t.Errorf("flash = %q, want both moved to web", flash)
	}
	for _, ref := range []string{one, two} {
		if got := readTask(t, f, ref).Ref; !strings.HasPrefix(got, "web-") {
			t.Errorf("%s is now %s, want a web reference", ref, got)
		}
	}
}

func TestARefusedMoveSaysWhyInTheForm(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")

	resp := b.post("/tasks/"+ref+"/move", url.Values{"project_ref": {"nope"}})
	body := readBody(t, resp)
	wantStatus(t, resp, http.StatusNotFound)
	if !strings.Contains(formAt(t, body, "/tasks/"+ref+"/move"), "nope") {
		t.Errorf("the refusal is not in the form:\n%s", body)
	}
}
