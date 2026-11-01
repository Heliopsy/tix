// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// quickCreate posts a create and decodes the task when it was created.
func (f *apiFixture) quickCreate(body any, status int) core.Task {
	f.t.Helper()
	resp := f.call(http.MethodPost, wire.RouteTasks, body)
	defer func() { _ = resp.Body.Close() }()
	mustStatus(f.t, resp, status)
	var task core.Task
	if status == http.StatusCreated {
		decodeBody(f.t, resp, &task)
	}
	return task
}

func TestCreateTaskParsesWhenAsked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	task := f.quickCreate(httpapi.CreateTaskRequest{
		CreateTaskInput: core.CreateTaskInput{ProjectRef: "infra", Title: "Fix login !high #auth due:2026-11-20"},
		Parse:           true,
	}, http.StatusCreated)
	if task.Title != "Fix login" || task.Priority != core.PriorityHigh || !slices.Equal(task.Tags, []string{"auth"}) {
		t.Errorf("a parsed create made %q priority %d tags %q", task.Title, task.Priority, task.Tags)
	}
	if task.DueAt == nil || task.DueAt.Format("2006-01-02") != "2026-11-20" {
		t.Errorf("due %v, want 2026-11-20", task.DueAt)
	}
}

func TestCreateTaskIsLiteralWithoutParse(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	task := f.quickCreate(core.CreateTaskInput{ProjectRef: "infra", Title: "Fix login !high #auth"}, http.StatusCreated)
	if task.Title != "Fix login !high #auth" || task.Priority != core.PriorityNormal || len(task.Tags) != 0 {
		t.Errorf("an unparsed create made %q priority %d tags %q", task.Title, task.Priority, task.Tags)
	}
}

func TestCreateTaskParseRefusals(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, tc := range []struct {
		name   string
		in     core.CreateTaskInput
		status int
	}{
		{"unknown project", core.CreateTaskInput{Title: "Fix login +nope"}, http.StatusNotFound},
		{"unknown handle", core.CreateTaskInput{ProjectRef: "infra", Title: "Fix login @nobody"}, http.StatusNotFound},
		{"contradiction", core.CreateTaskInput{ProjectRef: "infra", Title: "Fix !high", Priority: core.PriorityLow}, http.StatusBadRequest},
		{"bad date", core.CreateTaskInput{ProjectRef: "infra", Title: "Fix due:someday"}, http.StatusBadRequest},
	} {
		f.quickCreate(httpapi.CreateTaskRequest{CreateTaskInput: tc.in, Parse: true}, tc.status)
	}
	if titles := f.listTitles(""); len(titles) != 0 {
		t.Errorf("refused creates left tasks behind: %q", titles)
	}
}
