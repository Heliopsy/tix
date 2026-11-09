// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// builtinLabel marks a built-in workflow template in the new project form.
const builtinLabel = "Built-in"

// newProjectChoiceMsg carries what the new project form offers: the tenant's
// workflows and the built-in workflow templates.
type newProjectChoiceMsg struct {
	key, name string
	workflows []core.Workflow
	templates []core.WorkflowTemplate
}

// startNewProject reads the workflows and templates a new project could run
// on. With nothing to choose between, it creates the project at once on the
// default workflow, which is what the one-line prompt always did.
func (m Model) startNewProject(key, name string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		workflows, _ := svc.ListWorkflows(ctx)
		templates, _ := svc.ListWorkflowTemplates(ctx)
		if len(templates) == 0 && len(workflows) <= 1 {
			return createProjectMsg(svc.CreateProject(ctx, core.CreateProjectInput{Key: key, Name: name}))
		}
		return newProjectChoiceMsg{key: key, name: name, workflows: workflows, templates: templates}
	}
}

// createProjectMsg reports a project creation.
func createProjectMsg(project *core.Project, err error) tea.Msg {
	if err != nil {
		return actionMsg{kind: actionNewProject, err: err}
	}
	return actionMsg{kind: actionNewProject, label: output.ShowKey(project.Key)}
}

// NewProjectForm asks which workflow a new project runs on: one of the
// tenant's, or a built-in template, described under the form by its purpose
// first, then its name with a Built-in label, its states and its fields.
func NewProjectForm(key, name string, workflows []core.Workflow, templates []core.WorkflowTemplate) Form {
	about := map[string][]string{}
	var options []string
	held := map[string]bool{}
	for _, wf := range workflows {
		held[wf.Key] = true
	}
	isTemplate := map[string]bool{}
	for _, t := range templates {
		isTemplate[t.Key] = true
	}
	for _, wf := range workflows {
		if isTemplate[wf.Key] {
			continue
		}
		options = append(options, wf.Key)
		about[wf.Key] = []string{wf.Name, wf.Definition.StatesSummary()}
	}
	for _, t := range templates {
		options = append(options, t.Key)
		lines := []string{t.Purpose, t.Name + "  [" + builtinLabel + "]", t.Definition.StatesSummary()}
		if len(t.Fields) > 0 {
			keys := make([]string, 0, len(t.Fields))
			for _, f := range t.Fields {
				keys = append(keys, f.Key)
			}
			lines = append(lines, "fields: "+strings.Join(keys, ", "))
		}
		if held[t.Key] {
			lines = append(lines, "already in this tenant: the project reuses it")
		}
		about[t.Key] = lines
	}
	value := ""
	if len(options) > 0 {
		value = options[0]
	}
	for _, o := range options {
		if o == "default" {
			value = o
		}
	}
	carry := map[string]string{"key": key, "name": name}
	for k := range isTemplate {
		carry["template:"+k] = "1"
	}
	return Form{
		Kind:  formNewProject,
		Title: "new project " + output.ShowKey(key),
		Fields: []FormField{{Key: attrWorkflow, Label: "workflow", Options: options, Value: value,
			About: about}},
		Carry: carry,
	}
}

// applyNewProject creates the project the new project form describes.
func (m Model) applyNewProject(form Form) (Model, tea.Cmd) {
	if m.svc == nil {
		return m, nil
	}
	in := core.CreateProjectInput{Key: form.Carry["key"], Name: form.Carry["name"]}
	choice := form.Value(attrWorkflow)
	if form.Carry["template:"+choice] != "" {
		in.WorkflowTemplate = choice
	} else {
		in.WorkflowKey = choice
	}
	svc, ctx := m.svc, m.ctx
	return m, func() tea.Msg { return createProjectMsg(svc.CreateProject(ctx, in)) }
}
