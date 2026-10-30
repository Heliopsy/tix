// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestTheTaskResourceCarriesItsRepeatRule(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodPost, wire.RouteTasks,
		map[string]any{"project_ref": "infra", "title": "standup", "recurrence": "daily"})
	mustStatus(t, resp, http.StatusCreated)
	var task core.Task
	decodeBody(t, resp, &task)
	_ = resp.Body.Close()
	if task.Recurrence != "every day" {
		t.Fatalf("created recurrence = %q, want every day", task.Recurrence)
	}
	f.createTask("one-off")

	path := strings.Replace(wire.RouteTask, "{ref}", task.Ref, 1)
	bad := f.call(http.MethodPatch, path, map[string]any{"recurrence": "every blue moon"})
	mustStatus(t, bad, http.StatusBadRequest)
	_ = bad.Body.Close()

	if got := f.listTitles("?recurring=true"); !slices.Equal(got, []string{"standup"}) {
		t.Fatalf("recurring=true listed %v", got)
	}
	if got := f.listTitles("?recurring=false"); !slices.Equal(got, []string{"one-off"}) {
		t.Fatalf("recurring=false listed %v", got)
	}

	cleared := f.call(http.MethodPatch, path, map[string]any{"recurrence": ""})
	mustStatus(t, cleared, http.StatusOK)
	var after core.Task
	decodeBody(t, cleared, &after)
	_ = cleared.Body.Close()
	if after.Recurrence != "" || after.Title != "standup" {
		t.Fatalf("an empty recurrence left %q on %q", after.Recurrence, after.Title)
	}
}
