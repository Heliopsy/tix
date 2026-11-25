// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// salesTemplate is a built-in template as the service lists it.
func salesTemplate() core.WorkflowTemplate {
	return core.WorkflowTemplate{
		Key: "tix-sales", Name: "Sales or BizDev pipeline", Purpose: "Track deals from first contact to signed.",
		Definition: core.WorkflowDefinition{Initial: "lead", States: []core.State{
			{Key: "lead", Label: "Lead"}, {Key: "won", Label: "Won", Terminal: true},
		}},
		Fields: []core.FieldDefInput{{Key: "company", Type: core.FieldString}},
	}
}

func TestANewProjectCanRunOnABuiltInTemplate(t *testing.T) {
	m := projectListModel(t, 3, 130, 30)
	svc := newFakeService()
	svc.workflows = []core.Workflow{{Key: "default", Name: "Default"}}
	svc.templates = []core.WorkflowTemplate{salesTemplate()}
	m.svc = svc
	m, cmd := typeInto(t, m, "n", "deals Deals")
	m, _ = m.reduce(run(t, cmd))
	if m.form.Kind != formNewProject {
		t.Fatalf("form = %+v, want the new project form", m.form)
	}
	if len(svc.projectsMade) != 0 {
		t.Fatalf("created before a workflow was chosen: %+v", svc.projectsMade)
	}
	if got := m.form.Value(attrWorkflow); got != "default" {
		t.Fatalf("workflow starts at %q, want default", got)
	}

	m, _ = m.reduce(pressKey("right"))
	if got := m.form.Value(attrWorkflow); got != "tix-sales" {
		t.Fatalf("workflow = %q after cycling, want tix-sales", got)
	}
	frame := stripANSI(m.Frame())
	purpose := strings.Index(frame, "Track deals from first contact to signed.")
	name := strings.Index(frame, "Sales or BizDev pipeline  ["+builtinLabel+"]")
	if purpose < 0 || name < 0 || purpose > name {
		t.Fatalf("the template is not described purpose first (purpose at %d, name at %d):\n%s", purpose, name, frame)
	}
	if !strings.Contains(frame, "Lead → Won") {
		t.Fatalf("the template's states are not drawn:\n%s", frame)
	}

	_, cmd = m.reduce(pressKey("enter"))
	run(t, cmd)
	if len(svc.projectsMade) != 1 {
		t.Fatalf("created = %+v", svc.projectsMade)
	}
	got := svc.projectsMade[0]
	if got.Key != "deals" || got.Name != "Deals" || got.WorkflowTemplate != "tix-sales" || got.WorkflowKey != "" {
		t.Fatalf("created = %+v, want deals on the tix-sales template", got)
	}
}

func TestANewProjectOnAWorkflowNamesTheWorkflow(t *testing.T) {
	form := NewProjectForm("ops", "Ops", []core.Workflow{{Key: "default"}, {Key: "review", Name: "Review"}}, nil)
	form = form.SetValue(attrWorkflow, "review")
	m := projectListModel(t, 3, 130, 30)
	svc := newFakeService()
	m.svc = svc
	_, cmd := m.applyNewProject(form)
	run(t, cmd)
	if len(svc.projectsMade) != 1 || svc.projectsMade[0].WorkflowKey != "review" || svc.projectsMade[0].WorkflowTemplate != "" {
		t.Fatalf("created = %+v, want the review workflow", svc.projectsMade)
	}
}
