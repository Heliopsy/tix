// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestDeletingAProjectOverHTTPTakesTheTaskChoice(t *testing.T) {
	f := newFixture(t)
	for _, key := range []string{"old", "ops"} {
		resp := f.call(http.MethodPost, wire.RouteProjects, core.CreateProjectInput{Key: key, Name: key})
		mustStatus(t, resp, http.StatusCreated)
		_ = resp.Body.Close()
	}
	task := f.call(http.MethodPost, wire.RouteTasks, core.CreateTaskInput{ProjectRef: "old", Title: "travels"})
	mustStatus(t, task, http.StatusCreated)
	_ = task.Body.Close()

	refused := f.call(http.MethodDelete, "/api/v1/projects/old", nil)
	mustStatus(t, refused, http.StatusUnprocessableEntity)
	_ = refused.Body.Close()

	planned := f.call(http.MethodGet, "/api/v1/projects/old/deletion?move_tasks_to=ops", nil)
	mustStatus(t, planned, http.StatusOK)
	var plan core.ProjectDeletion
	decodeBody(t, planned, &plan)
	if plan.Tasks != 1 || plan.MoveTo != "ops" || len(plan.Statuses) != 1 || plan.Statuses[0].To != "todo" {
		t.Errorf("plan = %+v", plan)
	}

	both := f.call(http.MethodDelete, "/api/v1/projects/old?move_tasks_to=ops&delete_tasks=true", nil)
	mustStatus(t, both, http.StatusBadRequest)
	_ = both.Body.Close()

	moved := f.call(http.MethodDelete, "/api/v1/projects/old?move_tasks_to=ops", nil)
	mustStatus(t, moved, http.StatusOK)
	var done core.ProjectDeletion
	decodeBody(t, moved, &done)
	if done.Tasks != 1 || done.MoveTo != "ops" {
		t.Errorf("result = %+v", done)
	}
	there := f.call(http.MethodGet, "/api/v1/tasks/ops-1", nil)
	mustStatus(t, there, http.StatusOK)
	_ = there.Body.Close()
}
