// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

// patchTask sends body as a task update and returns the task as GET reads it
// back afterwards, so the assertion is on what was stored.
func (f *apiFixture) patchTask(ref string, body any) core.Task {
	f.t.Helper()
	resp := f.call(http.MethodPatch, "/api/v1/tasks/"+ref, body)
	mustStatus(f.t, resp, http.StatusOK)
	_ = resp.Body.Close()
	got := f.call(http.MethodGet, "/api/v1/tasks/"+ref, nil)
	mustStatus(f.t, got, http.StatusOK)
	var task core.Task
	decodeBody(f.t, got, &task)
	return task
}

func TestUpdateTaskDueAtIsAbsentNullOrAValue(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("renew certificate")
	due := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)

	set := f.patchTask(task.Ref, map[string]any{"due_at": due.Format(time.RFC3339)})
	if set.DueAt == nil || !set.DueAt.Equal(due) {
		t.Fatalf("after a value due_at = %v, want %v", set.DueAt, due)
	}

	untouched := f.patchTask(task.Ref, map[string]any{"title": "renew the certificate"})
	if untouched.DueAt == nil || !untouched.DueAt.Equal(due) {
		t.Fatalf("after leaving it out due_at = %v, want %v kept", untouched.DueAt, due)
	}

	cleared := f.patchTask(task.Ref, map[string]any{"due_at": nil})
	if cleared.DueAt != nil {
		t.Fatalf("after null due_at = %v, want it cleared", cleared.DueAt)
	}
	if cleared.Title != "renew the certificate" {
		t.Fatalf("clearing the due date changed the title to %q", cleared.Title)
	}
}

// The command line and the terminal clear a due date by handing the client a
// pointer to a nil time, which it sends as null; this drives that through the
// real router and store.
func TestClientClearsTheDueDateOverHTTP(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	task := f.createTask("renew certificate")
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()
	ref := core.TaskRef{ID: task.Ref}

	due := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	dueAt := &due
	if _, err := remote.UpdateTask(ctx, ref, core.UpdateTaskInput{DueAt: &dueAt}); err != nil {
		t.Fatalf("setting the due date: %v", err)
	}
	var none *time.Time
	if _, err := remote.UpdateTask(ctx, ref, core.UpdateTaskInput{DueAt: &none}); err != nil {
		t.Fatalf("clearing the due date: %v", err)
	}
	got, err := remote.GetTask(ctx, ref)
	if err != nil {
		t.Fatalf("reading the task back: %v", err)
	}
	if got.DueAt != nil {
		t.Fatalf("due_at = %v after the client cleared it, want none", got.DueAt)
	}
}

// Every other optional task field clears with a value JSON can carry, which
// docs/api.md lists. This keeps that list honest.
func TestUpdateTaskClearsEveryOptionalField(t *testing.T) {
	f := newFixture(t)
	parent := f.createTask("certificates")
	task := f.createTask("renew certificate")
	set := f.patchTask(task.Ref, map[string]any{
		"assignee_actor_id": f.actorA.ID, "parent_ref": parent.Ref, "recurrence": "monthly",
		"lease": "2h", "tags": []string{"ops"}, "due_at": "2026-12-01T09:00:00Z",
	})
	if set.AssigneeActorID == "" || set.ParentID == "" || set.Recurrence == "" || set.Lease == 0 ||
		len(set.Tags) == 0 || set.DueAt == nil {
		t.Fatalf("the fields were not all set first: %+v", set)
	}

	cleared := f.patchTask(task.Ref, map[string]any{
		"assignee_actor_id": "", "parent_ref": "", "recurrence": "", "lease": "0", "tags": []string{}, "due_at": nil,
	})
	for field, still := range map[string]bool{
		"assignee_actor_id": cleared.AssigneeActorID != "",
		"parent_ref":        cleared.ParentID != "",
		"recurrence":        cleared.Recurrence != "",
		"lease":             cleared.Lease != 0,
		"tags":              len(cleared.Tags) != 0,
		"due_at":            cleared.DueAt != nil,
	} {
		if still {
			t.Errorf("%s survived its clear: %+v", field, cleared)
		}
	}
}
