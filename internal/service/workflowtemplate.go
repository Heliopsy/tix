// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/workflows"
)

// maxRenameAttempts bounds the search for a free key when a template is
// applied with the rename policy.
const maxRenameAttempts = 100

// ListWorkflowTemplates returns the built-in workflow templates.
func (l *Local) ListWorkflowTemplates(ctx context.Context) ([]core.WorkflowTemplate, error) {
	if _, err := l.authorize(ctx, authz.ActionWorkflowRead, authz.Resource{}); err != nil {
		return nil, err
	}
	return workflows.Templates()
}

// ApplyWorkflowTemplate creates a workflow from a built-in template.
func (l *Local) ApplyWorkflowTemplate(ctx context.Context, key string, in core.ApplyWorkflowTemplateInput) (*core.Workflow, error) {
	actor, err := l.authorize(ctx, authz.ActionWorkflowWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	tpl, err := workflowTemplate(key)
	if err != nil {
		return nil, err
	}
	target := strings.ToLower(strings.TrimSpace(in.As))
	if target == "" {
		target = tpl.Key
	}
	var out *core.Workflow
	err = l.write(ctx, actor, func(m *mutation) error {
		wf, err := applyWorkflowTemplate(ctx, m, tpl, target, in.OnCollision)
		out = wf
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// workflowTemplate finds a built-in template by key, naming the known ones
// when there is no such template.
func workflowTemplate(key string) (core.WorkflowTemplate, error) {
	tpl, ok, err := workflows.Lookup(key)
	if err != nil {
		return tpl, err
	}
	if !ok {
		all, _ := workflows.Templates()
		keys := make([]string, 0, len(all))
		for _, t := range all {
			keys = append(keys, t.Key)
		}
		return tpl, core.NotFound("workflow template %q; the templates are %s", key, strings.Join(keys, ", ")).
			WithDetail("templates", keys)
	}
	return tpl, nil
}

// applyWorkflowTemplate writes a template's workflow under key inside a
// mutation, resolving a taken key by the policy.
func applyWorkflowTemplate(ctx context.Context, m *mutation, tpl core.WorkflowTemplate, key string,
	policy core.CollisionPolicy) (*core.Workflow, error) {
	before, err := findWorkflow(ctx, m.tx, key)
	if err != nil {
		return nil, err
	}
	if before != nil {
		switch policy {
		case core.CollisionSkip:
			return before, nil
		case core.CollisionRename:
			if key, err = freeWorkflowKey(ctx, m, key); err != nil {
				return nil, err
			}
			before = nil
		case core.CollisionReplace:
			if before.Builtin {
				return nil, core.Conflict("the builtin workflow %q cannot be replaced by a template; apply it under another key", key)
			}
			in := core.WorkflowInput{Key: key, Name: tpl.Name, Definition: tpl.Definition}
			if err := migrateRemovedStates(ctx, m, before, in); err != nil {
				return nil, err
			}
		default:
			return nil, core.Conflict("workflow %q already exists; choose another key, or say to skip, rename or replace it", key).
				WithDetail("key", key)
		}
	}
	wf := &core.Workflow{Key: key, Name: tpl.Name, Definition: cloneDefinition(tpl.Definition)}
	if before != nil {
		wf.ID = before.ID
	}
	if err := m.tx.PutWorkflow(ctx, wf); err != nil {
		return nil, err
	}
	return wf, m.Record(auditWorkflowPut, core.EventWorkflowUpdated, "workflow", wf.ID, "", before, wf,
		map[string]any{"key": wf.Key, "template": tpl.Key})
}

// freeWorkflowKey returns the first of key-2, key-3 and so on that no workflow
// holds.
func freeWorkflowKey(ctx context.Context, m *mutation, key string) (string, error) {
	for i := 2; i < maxRenameAttempts+2; i++ {
		candidate := key + "-" + strconv.Itoa(i)
		existing, err := findWorkflow(ctx, m.tx, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", core.Conflict("no free key near %q; choose one", key)
}

// cloneDefinition copies a definition so a stored workflow never shares the
// slices of the template it was made from.
func cloneDefinition(d core.WorkflowDefinition) core.WorkflowDefinition {
	d.States = slices.Clone(d.States)
	d.Transitions = slices.Clone(d.Transitions)
	return d
}

// projectTemplateWorkflow resolves the workflow a project created from a
// template runs on: the one under the template's key when the tenant holds
// it, otherwise one created from the template, which needs workflow:write.
func (l *Local) projectTemplateWorkflow(ctx context.Context, m *mutation, tpl core.WorkflowTemplate) (*core.Workflow, error) {
	existing, err := findWorkflow(ctx, m.tx, tpl.Key)
	if err != nil || existing != nil {
		return existing, err
	}
	if _, err := l.authorize(ctx, authz.ActionWorkflowWrite, authz.Resource{}); err != nil {
		return nil, err
	}
	return applyWorkflowTemplate(ctx, m, tpl, tpl.Key, "")
}

// putTemplateFields gives a project the custom fields its template suggests.
func (l *Local) putTemplateFields(ctx context.Context, m *mutation, p *core.Project, tpl core.WorkflowTemplate) error {
	if len(tpl.Fields) == 0 {
		return nil
	}
	if _, err := l.authorize(ctx, authz.ActionFieldWrite, authz.Resource{ProjectID: p.ID}); err != nil {
		return err
	}
	for _, f := range tpl.Fields {
		label := f.Label
		if label == "" {
			label = f.Key
		}
		f.EnumOptions = slices.Clone(f.EnumOptions)
		if _, err := putFieldDef(ctx, m, p, f.Key, label, f); err != nil {
			return err
		}
	}
	return nil
}
