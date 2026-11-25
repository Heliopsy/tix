// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// projectRow is one project's row on the projects screen.
func projectRow(t *testing.T, page, key string) string {
	t.Helper()
	start := strings.Index(page, `id="p-`+key+`"`)
	if start < 0 {
		t.Fatalf("no row for %s on the projects screen", key)
	}
	row, _, _ := strings.Cut(page[start:], "</li>")
	return row
}

// TestTheProjectsScreenCountsEachProjectsOpenTasks pins the count beside each
// project: its unfinished tasks only, per project, and a plain word for none.
func TestTheProjectsScreenCountsEachProjectsOpenTasks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := aliceCtx(f)
	if _, err := f.svc.CreateProject(ctx, core.CreateProjectInput{Key: "web", Name: "Web", WorkflowKey: "default"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateProject(ctx, core.CreateProjectInput{Key: "one", Name: "One", WorkflowKey: "default"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"a", "b", "c"} {
		if _, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	done, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "finished"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.TransitionTask(ctx, core.TaskRef{ID: done.ID}, core.TransitionInput{To: "cancelled"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "one", Title: "only"}); err != nil {
		t.Fatal(err)
	}

	page := f.as("alice").page("/projects")
	for key, want := range map[string]string{
		"INFRA": `data-open="3">3 open tasks<`,
		"ONE":   `data-open="1">1 open task<`,
		"WEB":   `data-open="0">no open tasks<`,
	} {
		if row := projectRow(t, page, key); !strings.Contains(row, want) {
			t.Errorf("%s does not say %s:\n%s", key, want, row)
		}
	}
}
