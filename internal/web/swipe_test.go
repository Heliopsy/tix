// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

// rowTag is the opening tag of one task list row, where its swipe targets are
// carried, and nothing inside the row.
func rowTag(t *testing.T, page, ref string) string {
	t.Helper()
	start := strings.Index(page, `<li id="t-`+ref+`"`)
	if start < 0 {
		t.Fatalf("the listing has no row for %s", ref)
	}
	end := strings.Index(page[start:], ">")
	return page[start : start+end+1]
}

// taskStatus reads a task's state back from the service.
func taskStatus(t *testing.T, f *fixture, ref string) string {
	t.Helper()
	task, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)})
	if err != nil {
		t.Fatalf("reading %s: %v", ref, err)
	}
	return task.Status
}

// A row a finger may swipe names what each way does, only for the reader who
// may do it and only for what is left to do: a finished row cannot be
// completed again, a viewer's rows and the trash's stay still.
func TestOnlyActionableRowsCarryTheirSwipeTargets(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	open := b.createTask("infra", "swipe me")
	done := b.createTask("infra", "already done")
	gone := b.createTask("infra", "in the trash")
	_ = followed(t, b, b.post("/tasks/"+done+"/complete", url.Values{"next": {"/tasks"}}))
	_ = followed(t, b, b.post("/tasks/"+gone+"/delete", url.Values{}))

	page := b.page("/tasks?all=1")
	row := rowTag(t, page, open)
	if !strings.Contains(row, `data-swipe-complete="/tasks/`+open+`/complete"`) {
		t.Errorf("an open row carries no complete target: %s", row)
	}
	if !strings.Contains(row, `data-swipe-snooze="/tasks/`+open+`/snooze"`) ||
		!strings.Contains(row, `data-swipe-until="`+query.UntilTomorrow+`"`) {
		t.Errorf("an open row does not snooze until tomorrow morning: %s", row)
	}
	finished := rowTag(t, page, done)
	if strings.Contains(finished, "data-swipe-complete") {
		t.Errorf("a finished row offers to complete it again: %s", finished)
	}
	if !strings.Contains(finished, "data-swipe-snooze") {
		t.Errorf("a finished row offers no snooze, as the app's does: %s", finished)
	}

	viewer := rowTag(t, f.as("viewer").page("/tasks"), open)
	if strings.Contains(viewer, "data-swipe-") {
		t.Errorf("a viewer's row can be swiped: %s", viewer)
	}
	trash := rowTag(t, b.page(trashPath), gone)
	if strings.Contains(trash, "data-swipe-") {
		t.Errorf("a deleted row can be swiped: %s", trash)
	}
}

// Completing from the list offers the state it left back, and the Undo is
// an ordinary form that reopens it.
func TestACompleteOffersAnUndoThatReopensIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "done and undone")
	before := taskStatus(t, f, ref)

	page := followed(t, b, b.post("/tasks/"+ref+"/complete", url.Values{"next": {"/tasks"}}))
	if taskStatus(t, f, ref) == before {
		t.Fatal("the complete route did not finish the task")
	}
	m := undoForm.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("completing offers no Undo:\n%s", between(t, page, `<div id="toast">`, "</div>"))
	}
	if m[1] != "/tasks/"+ref+"/transition" || !strings.Contains(m[2], `name="route" value="`+before) {
		t.Fatalf("Undo posts to %s with %s, want the route back to %s", m[1], m[2], before)
	}
	route := between(t, m[2], `name="route" value="`, `"`)
	_ = followed(t, b, b.post(m[1], url.Values{"route": {route}, "next": {"/tasks"}}))
	if got := taskStatus(t, f, ref); got != before {
		t.Errorf("after Undo the task is %s, want %s", got, before)
	}
}

// Snoozing from the list offers to wake the task, and the Undo does.
func TestASnoozeOffersAnUndoThatWakesIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "snoozed and woken")

	page := followed(t, b, b.post("/tasks/"+ref+"/snooze", url.Values{"until": {query.UntilTomorrow}, "next": {"/tasks"}}))
	if snoozedUntil(t, f, ref) == nil {
		t.Fatal("the snooze route did not snooze the task")
	}
	m := undoForm.FindStringSubmatch(page)
	if m == nil || m[1] != "/tasks/"+ref+"/snooze/remove" {
		t.Fatalf("snoozing offers no Undo that wakes the task: %v", m)
	}
	resp := b.post(m[1], url.Values{"next": {"/tasks"}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	if until := snoozedUntil(t, f, ref); until != nil {
		t.Errorf("after Undo the task still sleeps until %v", until)
	}
}

// Snoozing a task that already slept offers its old wake time back.
func TestASecondSnoozeOffersTheOldWakeTimeBack(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "snoozed twice")
	_ = followed(t, b, b.post("/tasks/"+ref+"/snooze", url.Values{"until": {query.UntilNextWeek}}))
	first := snoozedUntil(t, f, ref)

	page := followed(t, b, b.post("/tasks/"+ref+"/snooze", url.Values{"until": {query.UntilTomorrow}, "next": {"/tasks"}}))
	m := undoForm.FindStringSubmatch(page)
	if m == nil || m[1] != "/tasks/"+ref+"/snooze" || !strings.Contains(m[2], `name="at"`) {
		t.Fatalf("the second snooze does not offer the first wake time back: %v", m)
	}
	at := between(t, m[2], `name="at" value="`, `"`)
	_ = followed(t, b, b.post(m[1], url.Values{"at": {at}, "next": {"/tasks"}}))
	if got := snoozedUntil(t, f, ref); got == nil || !got.Equal(*first) {
		t.Errorf("after Undo the task wakes at %v, want %v", got, first)
	}
}
