// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// deletionFixture adds a second project and two tasks in infra, one in
// progress, so the deletion screen has something to move and a mapping to show.
func deletionFixture(t *testing.T) (*fixture, *browser) {
	t.Helper()
	f := newFixture(t)
	b := f.as("alice")
	created := b.post("/projects", url.Values{"key": {"ops"}, "name": {"Ops"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)
	ctx := core.WithActor(context.Background(), &f.actorA)
	for _, title := range []string{"first", "second"} {
		if _, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: title}); err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
	}
	return f, b
}

func TestTheDeleteButtonOpensAConfirmationRatherThanDeleting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	board := b.page("/projects/infra")
	danger := between(t, board, `<details class="panel danger">`, "</details>")
	if !strings.Contains(danger, `<form method="get" hx-boost="false" action="/projects/infra/delete">`) {
		t.Fatalf("the danger zone's delete does not open the confirmation:\n%s", danger)
	}
	if strings.Contains(danger, `method="post" action="/projects/infra/delete"`) {
		t.Error("the danger zone still deletes on one click")
	}

	page := b.page("/projects/infra/delete")
	if !strings.Contains(page, "infra holds no tasks.") {
		t.Errorf("an empty project's confirmation does not say it holds none:\n%s", page)
	}
	if strings.Contains(page, `name="choice"`) {
		t.Error("an empty project is asked about tasks it does not have")
	}
	if _, err := f.svc.GetProject(core.WithActor(context.Background(), &f.actorA), "infra"); err != nil {
		t.Errorf("opening the confirmation deleted the project: %v", err)
	}
	done := b.post("/projects/infra/delete", url.Values{"intent": {"delete"}})
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)
	if _, err := f.svc.GetProject(core.WithActor(context.Background(), &f.actorA), "infra"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("confirming did not delete the empty project: %v", err)
	}
}

func TestTheConfirmationOffersBothChoicesAndShowsTheMapping(t *testing.T) {
	t.Parallel()
	_, b := deletionFixture(t)

	page := b.page("/projects/infra/delete")
	form := formAt(t, page, "/projects/infra/delete")
	for _, want := range []string{
		"infra holds 2 tasks.",
		`type="radio" name="choice" value="move"`,
		`type="radio" name="choice" value="delete"`,
		`<option value="ops" selected>`,
		`name="confirm"`,
		`name="intent" value="preview"`,
		`class="danger" type="submit" name="intent" value="delete"`,
		"<td>todo</td><td>todo</td><td>2</td>",
		"permanent and cannot be undone",
	} {
		if !strings.Contains(form, want) {
			t.Errorf("the confirmation form lacks %q:\n%s", want, form)
		}
	}
	if !strings.Contains(page, `hx-boost="false" data-apply action="/projects/infra/delete"`) {
		t.Error("the confirmation is boosted, and htmx drops the preview's submitter")
	}
	if strings.Contains(form, `<option value="infra"`) {
		t.Error("the project is offered as its own target")
	}
}

func TestTheDeleteChoiceNeedsTheKeyTyped(t *testing.T) {
	t.Parallel()
	f, b := deletionFixture(t)
	ctx := core.WithActor(context.Background(), &f.actorA)

	wrong := b.post("/projects/infra/delete", url.Values{"intent": {"delete"}, "choice": {"delete"}, "confirm": {"ops"}})
	body := readAll(t, wrong)
	wantStatus(t, wrong, http.StatusBadRequest)
	if !strings.Contains(body, `id="confirm-error"`) {
		t.Errorf("the refusal is not beside the key field:\n%s", body)
	}
	if _, err := f.svc.GetProject(ctx, "infra"); err != nil {
		t.Fatalf("a mistyped key deleted the project: %v", err)
	}

	none := b.post("/projects/infra/delete", url.Values{"intent": {"delete"}})
	body = readAll(t, none)
	wantStatus(t, none, http.StatusUnprocessableEntity)
	if !strings.Contains(body, `id="choice-error"`) {
		t.Errorf("deleting without a choice is not refused beside the choice:\n%s", body)
	}

	right := b.post("/projects/infra/delete", url.Values{"intent": {"delete"}, "choice": {"delete"}, "confirm": {"infra"}})
	_ = right.Body.Close()
	wantStatus(t, right, http.StatusSeeOther)
	if _, err := f.svc.GetProject(ctx, "infra"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the confirmed deletion left the project: %v", err)
	}
}

func TestMovingFromTheBrowserLandsOnTheTarget(t *testing.T) {
	t.Parallel()
	f, b := deletionFixture(t)

	lost := b.post("/projects/infra/delete", url.Values{"move_to": {"ops"}, "choice": {"move"}})
	_ = lost.Body.Close()
	wantStatus(t, lost, http.StatusSeeOther)
	if got := lost.Header.Get("Location"); !strings.HasPrefix(got, "/projects/infra/delete?") {
		t.Errorf("a submission without the delete intent went to %q, want the confirmation again", got)
	}
	if _, err := f.svc.GetProject(core.WithActor(context.Background(), &f.actorA), "infra"); err != nil {
		t.Fatalf("a submission without the delete intent deleted the project: %v", err)
	}

	preview := b.post("/projects/infra/delete", url.Values{"intent": {"preview"}, "move_to": {"ops"}, "choice": {"move"}})
	_ = preview.Body.Close()
	wantStatus(t, preview, http.StatusSeeOther)
	if got := preview.Header.Get("Location"); !strings.HasPrefix(got, "/projects/infra/delete?") ||
		strings.Contains(got, "csrf") || !strings.Contains(got, "move_to=ops") {
		t.Errorf("the preview redirects to %q", got)
	}

	moved := b.post("/projects/infra/delete", url.Values{"intent": {"delete"}, "choice": {"move"}, "move_to": {"ops"}})
	_ = moved.Body.Close()
	wantStatus(t, moved, http.StatusSeeOther)
	if got := moved.Header.Get("Location"); !strings.HasPrefix(got, "/projects/ops") {
		t.Errorf("the move lands on %q, want the target's board", got)
	}
	page, err := f.svc.ListTasks(core.WithActor(context.Background(), &f.actorA),
		core.TaskFilter{ProjectKeys: []string{"ops"}})
	if err != nil || len(page.Tasks) != 2 {
		t.Errorf("tasks in ops after the move = %d, %v", len(page.Tasks), err)
	}
}
