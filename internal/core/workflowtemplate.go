// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "strings"

// WorkflowTemplatePrefix starts the key of every built-in workflow template
// and of the workflow it creates by default. A workflow put by hand may not
// take a new key under it, so a key that starts with it always came from a
// template.
const WorkflowTemplatePrefix = "tix-"

// ReservedWorkflowKey reports whether key falls under the prefix built-in
// templates own.
func ReservedWorkflowKey(key string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), WorkflowTemplatePrefix)
}

// WorkflowTemplate is a ready-made workflow shipped with tix, with the custom
// fields a project using it usually wants.
type WorkflowTemplate struct {
	// Key identifies the template and is the key of the workflow it creates
	// unless another is chosen.
	Key string `json:"key" yaml:"key"`
	// Name is the template's display name.
	Name string `json:"name" yaml:"name"`
	// Purpose says in one line what the workflow is for, and is shown first
	// wherever templates are listed.
	Purpose string `json:"purpose" yaml:"purpose"`
	// States is a one-line summary of the states, in order, the final ones
	// joined by a slash.
	States string `json:"states" yaml:"states,omitempty"`
	// Definition is the workflow the template creates.
	Definition WorkflowDefinition `json:"definition" yaml:"definition"`
	// Fields are the custom fields a project created from the template gets.
	// Fields belong to projects, not workflows, so applying the template on
	// its own creates only the workflow.
	Fields []FieldDefInput `json:"fields,omitempty" yaml:"fields,omitempty"`
}

// StatesSummary renders the states in order as labels joined by arrows, with
// the terminal states joined by a slash at the end.
func (d WorkflowDefinition) StatesSummary() string {
	var open, final []string
	for _, s := range d.States {
		label := s.Label
		if label == "" {
			label = s.Key
		}
		if s.Terminal {
			final = append(final, label)
		} else {
			open = append(open, label)
		}
	}
	if len(final) > 0 {
		open = append(open, strings.Join(final, " / "))
	}
	return strings.Join(open, " → ")
}

// ApplyWorkflowTemplateInput creates a workflow from a built-in template.
type ApplyWorkflowTemplateInput struct {
	// As is the key the workflow is created under; empty uses the template's
	// own key.
	As string `json:"as,omitempty" yaml:"as,omitempty"`
	// OnCollision decides what happens when the key is taken: skip keeps the
	// existing workflow, rename picks the next free key, replace redefines the
	// existing one. Empty refuses with a conflict.
	OnCollision CollisionPolicy `json:"on_collision,omitempty" yaml:"on_collision,omitempty"`
}

// Validate checks the input.
func (in ApplyWorkflowTemplateInput) Validate() error {
	if in.As != "" {
		if err := ValidateWorkflowKey(strings.ToLower(strings.TrimSpace(in.As))); err != nil {
			return OnField(err, "as")
		}
	}
	if in.OnCollision != "" && !in.OnCollision.Valid() {
		return Invalid("on_collision %q is not one of skip, rename, replace", in.OnCollision).
			WithDetail(DetailField, "on_collision")
	}
	return nil
}
