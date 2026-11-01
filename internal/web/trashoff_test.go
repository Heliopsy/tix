// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// turnTrashOff turns the fixture tenant's trash off from the Tenant screen.
func turnTrashOff(t *testing.T, b *browser) {
	t.Helper()
	resp := b.post("/admin/tenant", url.Values{"name": {"Acme Works"}, "trash": {"off"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
}

// confirmForm is the permanent-delete confirmation's form, cut at its own
// closing tag.
func confirmForm(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<form method="post" class="cform delform"`, "</form>")
}

func TestTheTenantScreenTurnsTheTrashOff(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	gone := b.createTask("infra", "deleted while the trash was on")
	deleteTask(t, b, gone)

	choice := between(t, b.page("/admin/tenant"), `<select id="trash" name="trash">`, "</select>")
	if !strings.Contains(choice, `<option value="on" selected>`) {
		t.Fatalf("a new tenant's trash is not shown on:\n%s", choice)
	}
	turnTrashOff(t, b)
	page := b.page("/admin/tenant")
	choice = between(t, page, `<select id="trash" name="trash">`, "</select>")
	if !strings.Contains(choice, `<option value="off" selected>`) {
		t.Errorf("the saved trash setting is not shown off:\n%s", choice)
	}
	if held := between(t, page, `<p class="note trashheld">`, "</p>"); !strings.Contains(held, "1 task deleted while the trash was on is kept") {
		t.Errorf("the screen does not say the trash still holds one task: %q", held)
	}
}

func TestWithTheTrashOffTheSidebarAndTheTrashSaySo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	turnTrashOff(t, b)

	if menu := nav(t, b.page("/tasks")); strings.Contains(menu, "<span>Trash</span>") {
		t.Errorf("the sidebar still offers the trash:\n%s", menu)
	}
	trash := b.page(trashPath)
	if bar := between(t, trash, `<p class="filterfault" role="alert">`, "</p>"); !strings.Contains(bar, "trash is turned off for this tenant") {
		t.Errorf("the trash link does not say the trash is off: %q", bar)
	}
}

func TestWithTheTrashOffADeleteAsksFirstAndOffersNoUndo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	turnTrashOff(t, b)
	ref := b.createTask("infra", "permanent")

	asked := b.post("/tasks/"+ref+"/delete?next=/tasks", url.Values{})
	wantStatus(t, asked, http.StatusOK)
	form := confirmForm(t, body(t, asked))
	for _, want := range []string{`action="/tasks/` + ref + `/delete?next=%2Ftasks"`, `name="confirm" value="1"`, ref} {
		if !strings.Contains(form, want) {
			t.Errorf("the confirmation lacks %s:\n%s", want, form)
		}
	}
	still := b.get("/tasks/" + ref)
	_ = still.Body.Close()
	wantStatus(t, still, http.StatusOK)

	done := b.post("/tasks/"+ref+"/delete?next=/tasks", url.Values{"confirm": {"1"}})
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)
	if location := done.Header.Get("Location"); strings.Contains(location, "flashundo") {
		t.Errorf("a permanent delete offered an undo: %s", location)
	}
	gone := b.get("/tasks/" + ref)
	_ = gone.Body.Close()
	wantStatus(t, gone, http.StatusNotFound)
}

func TestWithTheTrashOffABulkDeleteAsksFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	turnTrashOff(t, b)
	refs := []string{b.createTask("infra", "one"), b.createTask("infra", "two")}

	asked := b.post("/tasks/bulk?action=delete", url.Values{"ref": refs, "next": {"/tasks"}})
	wantStatus(t, asked, http.StatusOK)
	form := confirmForm(t, body(t, asked))
	for _, ref := range refs {
		if !strings.Contains(form, `name="ref" value="`+ref+`"`) {
			t.Errorf("the confirmation does not carry %s:\n%s", ref, form)
		}
	}
	still := b.get("/tasks/" + refs[0])
	_ = still.Body.Close()
	wantStatus(t, still, http.StatusOK)

	done := b.post("/tasks/bulk?action=delete", url.Values{"ref": refs, "next": {"/tasks"}, "confirm": {"1"}})
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)
	if location := done.Header.Get("Location"); strings.Contains(location, "flashundo") {
		t.Errorf("a permanent bulk delete offered an undo: %s", location)
	}
}
