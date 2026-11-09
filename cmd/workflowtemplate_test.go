// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestWorkflowTemplateLsLeadsWithThePurpose(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("workflow", "template", "ls")
	var rows [][]string
	for _, line := range strings.Split(got.out, "\n") {
		if !strings.Contains(line, "│") {
			continue
		}
		var cells []string
		for _, cell := range strings.Split(strings.Trim(strings.TrimSpace(line), "│"), "│") {
			cells = append(cells, strings.TrimSpace(cell))
		}
		rows = append(rows, cells)
	}
	if len(rows) != 13 {
		t.Fatalf("ls printed %d rows, want a header and 12 templates:\n%s", len(rows), got.out)
	}
	if rows[0][0] != "PURPOSE" {
		t.Fatalf("the first column is %q, want PURPOSE:\n%s", rows[0][0], got.out)
	}
	if rows[1][0] != "Move cards left to right, nothing else." || rows[1][1] != "tix-kanban" {
		t.Fatalf("the first row is %v, want the kanban template's purpose then its key", rows[1])
	}

	var templates []core.WorkflowTemplate
	if err := json.Unmarshal([]byte(c.mustRun("workflow", "template", "ls", "-o", "json").out), &templates); err != nil {
		t.Fatalf("not json: %v", err)
	}
	if len(templates) != 12 || templates[0].Key != "tix-kanban" || templates[0].Purpose == "" {
		t.Fatalf("templates = %+v", templates)
	}
}

func TestWorkflowTemplateApply(t *testing.T) {
	c := newCLI(t)
	if got := c.mustRun("workflow", "template", "apply", "tix-sales", "--dry-run"); !strings.Contains(got.out, statusPlanned) {
		t.Fatalf("dry run = %q, want a plan", got.out)
	}
	if got := c.run("workflow", "get", "tix-sales"); got.code != core.ExitNotFound {
		t.Fatalf("the dry run wrote the workflow (exit %d)", got.code)
	}

	c.mustRun("workflow", "template", "apply", "tix-sales")
	if got := c.run("workflow", "template", "apply", "tix-sales"); got.code != core.ExitConflict {
		t.Fatalf("applying onto a taken key exited %d, want %d: %s", got.code, core.ExitConflict, got.err)
	}
	c.mustRun("workflow", "template", "apply", "tix-sales", "--on-collision", "rename")
	c.mustRun("workflow", "template", "apply", "tix-sales", "--as", "deals")
	if got := c.run("workflow", "template", "apply", "tix-nope"); got.code != core.ExitNotFound {
		t.Fatalf("an unknown template exited %d, want %d", got.code, core.ExitNotFound)
	}
	listed := c.mustRun("workflow", "ls").out
	for _, key := range []string{"tix-sales", "tix-sales-2", "deals"} {
		if !strings.Contains(listed, key) {
			t.Errorf("workflow ls does not list %s:\n%s", key, listed)
		}
	}

	c.mustRun("project", "create", "crm", "CRM", "--workflow-template", "tix-hiring")
	if fields := c.mustRun("field", "ls", "crm").out; !strings.Contains(fields, "salary_range") {
		t.Fatalf("the project made from the template has no fields:\n%s", fields)
	}

	doc := "key: tix-mine\ndefinition:\n  initial: a\n  states:\n    - {key: a}\n    - {key: b, terminal: true}\n  transitions:\n    - {from: a, to: b}\n"
	if got := c.runIn(doc, "workflow", "put", "-f", "-"); got.code != core.ExitUsage || !strings.Contains(got.err, "reserved") {
		t.Fatalf("putting a tix- workflow by hand exited %d: %s", got.code, got.err)
	}
}
