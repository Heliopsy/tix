// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/i18n"
)

// templateChoicePrefix marks a New project picker value that names a built-in
// workflow template rather than a workflow. A workflow key cannot hold a colon,
// so the two never collide.
const templateChoicePrefix = "template:"

// workflowTemplateDialog is the Workflows screen's Add from template dialog.
const workflowTemplateDialog = "workflow-template"

// templateCard is one built-in workflow template as a picker draws it, in the
// reader's language, purpose first.
type templateCard struct {
	Key     string
	Name    string
	Purpose string
	States  string
	Fields  []string
	// Held reports whether the tenant already has a workflow under the
	// template's key.
	Held bool
}

// Choice is the value the New project picker submits for the template.
func (c templateCard) Choice() string { return templateChoicePrefix + c.Key }

// templateCards renders the templates for one language.
func templateCards(lang string, templates []core.WorkflowTemplate, workflows []core.Workflow) []templateCard {
	held := make(map[string]bool, len(workflows))
	for _, wf := range workflows {
		held[wf.Key] = true
	}
	out := make([]templateCard, 0, len(templates))
	for _, tpl := range templates {
		def := tpl.Definition
		def.States = make([]core.State, len(tpl.Definition.States))
		for i, s := range tpl.Definition.States {
			s.Label = i18n.WorkflowTemplateText(lang, s.Label)
			def.States[i] = s
		}
		fields := make([]string, 0, len(tpl.Fields))
		for _, f := range tpl.Fields {
			label := f.Label
			if label == "" {
				label = f.Key
			}
			fields = append(fields, label)
		}
		out = append(out, templateCard{
			Key:     tpl.Key,
			Name:    i18n.WorkflowTemplateText(lang, tpl.Name),
			Purpose: i18n.WorkflowTemplateText(lang, tpl.Purpose),
			States:  def.StatesSummary(),
			Fields:  fields,
			Held:    held[tpl.Key],
		})
	}
	return out
}

// splitWorkflowChoice reads the New project picker's answer as a workflow key
// or a template key.
func splitWorkflowChoice(choice string) (workflow, template string) {
	if key, ok := strings.CutPrefix(choice, templateChoicePrefix); ok {
		return "", key
	}
	return choice, ""
}

// applyWorkflowTemplate creates a workflow from the template the Add from
// template dialog picked.
func (h *handler) applyWorkflowTemplate(w http.ResponseWriter, r *http.Request) error {
	in := core.ApplyWorkflowTemplateInput{
		As:          strings.ToLower(field(r, "as")),
		OnCollision: core.CollisionPolicy(field(r, "on_collision")),
	}
	key := field(r, "template")
	if key == "" {
		return h.refuseIn(w, r, workflowTemplateDialog, RouteWorkflows,
			core.Invalid("choose a template").WithDetail(core.DetailField, "template"), h.showWorkflows)
	}
	wf, err := h.svc.ApplyWorkflowTemplate(r.Context(), key, in)
	if err != nil {
		if core.FieldOf(err) == "" && core.KindOf(err) == core.KindConflict {
			err = core.OnField(err, "as")
		}
		return h.refuseIn(w, r, workflowTemplateDialog, RouteWorkflows, err, h.showWorkflows)
	}
	redirect(w, r, RouteWorkflows+"/"+wf.Key, "workflow created from a template")
	return nil
}
