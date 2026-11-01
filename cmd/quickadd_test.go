// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// quickTask is the part of a task document these tests read.
type quickTask struct {
	Title    string        `json:"title"`
	Ref      string        `json:"ref"`
	Priority core.Priority `json:"priority"`
	Status   string        `json:"status"`
	Tags     []string      `json:"tags"`
	DueAt    string        `json:"due_at"`
	Assignee string        `json:"assignee_actor_id"`
	ParentID string        `json:"parent_id"`
}

// addJSON runs task add with -o json and decodes the task it printed.
func addJSON(t *testing.T, c *cli, args ...string) quickTask {
	t.Helper()
	got := c.mustRun(append([]string{"task", "add", "-o", "json"}, args...)...)
	var task quickTask
	if err := json.Unmarshal([]byte(got.out), &task); err != nil {
		t.Fatalf("decoding %q: %v", got.out, err)
	}
	return task
}

func TestTaskAddParsesQuickAddSyntaxWhenAsked(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Website")
	c.mustRun("user", "create", "raj@example.com", "--handle", "raj")
	parent := addJSON(t, c, "-p", "infra", "the parent")

	task := addJSON(t, c, "--parse",
		"Fix login !high #auth @raj due:2026-11-20 +web ^"+parent.Ref+" status:doing")
	if task.Title != "Fix login" {
		t.Errorf("title %q, want the line without its tokens", task.Title)
	}
	if task.Priority != core.PriorityHigh || task.Status != "doing" || !slices.Equal(task.Tags, []string{"auth"}) {
		t.Errorf("priority %d, status %q, tags %q", task.Priority, task.Status, task.Tags)
	}
	if !strings.HasPrefix(task.DueAt, "2026-11-20") || task.Assignee == "" || task.ParentID == "" {
		t.Errorf("due %q, assignee %q, parent %q", task.DueAt, task.Assignee, task.ParentID)
	}
	if !strings.HasPrefix(task.Ref, "web-") {
		t.Errorf("the task went to %s, though the line said +web", task.Ref)
	}
}

// Literal is the default, so a script passing an issue title gets the title
// it passed.
func TestTaskAddStoresTheTitleLiterallyByDefault(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	task := addJSON(t, c, "-p", "infra", "Fix login !high #auth due:fri")
	if task.Title != "Fix login !high #auth due:fri" || task.Priority != core.PriorityNormal || len(task.Tags) != 0 || task.DueAt != "" {
		t.Errorf("an unparsed add became %+v", task)
	}
}

func TestTaskAddParseRefusals(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	tests := []struct {
		name    string
		args    []string
		code    int
		mention string
	}{
		{"unknown handle", []string{"-p", "infra", "--parse", "Fix login @nobody"}, core.KindNotFound.ExitCode(), "nobody"},
		{"unknown project", []string{"--parse", "Fix login +nope"}, core.KindNotFound.ExitCode(), "nope"},
		{"contradicting flag", []string{"-p", "infra", "--parse", "--priority", "low", "Fix login !high"}, core.KindInvalid.ExitCode(), "priority"},
		{"contradicting project", []string{"-p", "infra", "--parse", "Fix login +web"}, core.KindInvalid.ExitCode(), "project"},
		{"bad date", []string{"-p", "infra", "--parse", "Fix login due:someday"}, core.KindInvalid.ExitCode(), "someday"},
		{"nothing left", []string{"-p", "infra", "--parse", "#auth"}, core.KindInvalid.ExitCode(), "#auth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.run(append([]string{"task", "add"}, tt.args...)...)
			if got.code != tt.code || !strings.Contains(got.err, tt.mention) {
				t.Errorf("exit %d, stderr %q; want exit %d naming %q", got.code, got.err, tt.code, tt.mention)
			}
		})
	}
	listed := c.mustRun("task", "ls", "--all", "-o", "ndjson")
	if strings.TrimSpace(listed.out) != "" {
		t.Errorf("a refused add created a task: %s", listed.out)
	}
}

// The configured default project is a default: a line naming a project wins.
func TestTaskAddLineProjectBeatsTheConfiguredDefault(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Website")
	writeFile(t, filepath.Join(c.home, "conf", "tix", "config.yaml"), "project: infra\n")
	if task := addJSON(t, c, "--parse", "Fix login +web"); !strings.HasPrefix(task.Ref, "web-") {
		t.Errorf("the line said +web and the task went to %s", task.Ref)
	}
	if task := addJSON(t, c, "--parse", "Fix login"); !strings.HasPrefix(task.Ref, "infra-") {
		t.Errorf("with no project on the line the task went to %s, not the default", task.Ref)
	}
}
