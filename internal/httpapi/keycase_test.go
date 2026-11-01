// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// The API takes a ref or a project key in any case and answers with the
// stored, lower case one, so a client that reads INFRA-1 off a screen reaches
// the task and a client that compares JSON values sees what it always has.
func TestTheAPIAcceptsAnyCaseAndAnswersTheStoredOne(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("upper")
	shown := strings.ToUpper(task.Ref)

	var got core.Task
	resp := f.call(http.MethodGet, "/api/v1/tasks/"+shown, nil)
	mustStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &got)
	if got.ID != task.ID || got.Ref != task.Ref {
		t.Errorf("GET /tasks/%s answered %s %q, want %s %q", shown, got.ID, got.Ref, task.ID, task.Ref)
	}

	var project core.Project
	resp = f.call(http.MethodGet, "/api/v1/projects/INFRA", nil)
	mustStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &project)
	if project.Key != "infra" {
		t.Errorf("the project's key is %q in JSON, want the stored infra", project.Key)
	}

	for _, query := range []string{"?project=infra", "?project=INFRA", "?project=Infra"} {
		var page wire.Page[core.Task]
		resp = f.call(http.MethodGet, "/api/v1/tasks"+query, nil)
		mustStatus(t, resp, http.StatusOK)
		decodeBody(t, resp, &page)
		if len(page.Items) != 1 || page.Items[0].Ref != task.Ref {
			t.Errorf("%s listed %+v, want only %s", query, page.Items, task.Ref)
		}
	}
}
