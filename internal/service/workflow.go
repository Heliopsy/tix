// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// PutWorkflow defines or redefines a workflow.
func (l *Local) PutWorkflow(ctx context.Context, in core.WorkflowInput) (*core.Workflow, error) {
	actor, err := l.authorize(ctx, authz.ActionWorkflowWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	in.Key = strings.ToLower(strings.TrimSpace(in.Key))
	if err := in.Validate(); err != nil {
		return nil, err
	}
	name := in.Name
	if name == "" {
		name = in.Key
	}

	var out *core.Workflow
	err = l.write(ctx, actor, func(m *mutation) error {
		before, err := findWorkflow(ctx, m.tx, in.Key)
		if err != nil {
			return err
		}
		builtin := false
		if before != nil {
			builtin = before.Builtin
			if err := migrateRemovedStates(ctx, m, before, in); err != nil {
				return err
			}
		}

		wf := &core.Workflow{
			Key:        in.Key,
			Name:       name,
			Definition: in.Definition,
			Builtin:    builtin,
		}
		if before != nil {
			wf.ID = before.ID
		}
		if err := m.tx.PutWorkflow(ctx, wf); err != nil {
			return err
		}
		out = wf
		return m.Record(auditWorkflowPut, core.EventWorkflowUpdated, "workflow", wf.ID, "", before, wf,
			map[string]any{"key": wf.Key})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetWorkflow returns a workflow by key or by identifier.
func (l *Local) GetWorkflow(ctx context.Context, ref string) (*core.Workflow, error) {
	actor, err := l.authorize(ctx, authz.ActionWorkflowRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.Workflow
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		wf, err := lookupWorkflow(ctx, tx, ref)
		out = wf
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListWorkflows returns every workflow in the current tenant.
func (l *Local) ListWorkflows(ctx context.Context) ([]core.Workflow, error) {
	actor, err := l.authorize(ctx, authz.ActionWorkflowRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := []core.Workflow{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := tx.ListWorkflows(ctx)
		out = found
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteWorkflow removes a workflow no project is using, addressed by key or
// by identifier. The builtin workflow may be copied under another key but
// never deleted.
func (l *Local) DeleteWorkflow(ctx context.Context, ref string) error {
	actor, err := l.authorize(ctx, authz.ActionWorkflowWrite, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		wf, err := lookupWorkflow(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		key := wf.Key
		if wf.Builtin {
			return core.Conflict("the builtin workflow %q cannot be deleted; copy it under another key instead", key)
		}
		users, err := projectsUsingWorkflow(ctx, m.tx, wf.ID)
		if err != nil {
			return err
		}
		if len(users) > 0 {
			keys := make([]string, 0, len(users))
			for _, p := range users {
				keys = append(keys, p.Key)
			}
			sort.Strings(keys)
			return core.Conflict("workflow %q is still assigned to project(s) %s", key, strings.Join(keys, ", ")).
				WithDetail("projects", keys)
		}
		if err := m.tx.DeleteWorkflow(ctx, key); err != nil {
			return err
		}
		return m.Record(auditWorkflowDel, eventWorkflowDeleted, "workflow", wf.ID, "", wf, nil,
			map[string]any{"key": key})
	})
}

// findWorkflow returns a workflow by key, or nil when it does not exist.
func findWorkflow(ctx context.Context, tx store.Tx, key string) (*core.Workflow, error) {
	wf, err := tx.GetWorkflow(ctx, key)
	if err == nil {
		return wf, nil
	}
	if core.IsKind(err, core.KindNotFound) {
		return nil, nil
	}
	return nil, err
}

// lookupWorkflow resolves a workflow by key or by identifier, accepting a key
// in any case because references are typed by hand. It mirrors lookupProject.
func lookupWorkflow(ctx context.Context, tx store.Tx, ref string) (*core.Workflow, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, core.Invalid("workflow reference is required")
	}
	if wf, err := tx.GetWorkflow(ctx, strings.ToLower(ref)); err == nil {
		return wf, nil
	} else if !core.IsKind(err, core.KindNotFound) {
		return nil, err
	}
	wf, err := tx.GetWorkflowByID(ctx, ref)
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("workflow %q", ref)
		}
		return nil, err
	}
	return wf, nil
}

// projectsUsingWorkflow returns every project assigned to a workflow.
func projectsUsingWorkflow(ctx context.Context, tx store.Tx, workflowID string) ([]core.Project, error) {
	var out []core.Project
	filter := core.ProjectFilter{
		IncludeArchived: true,
		Page:            core.Page{Limit: core.MaxPageLimit, Sort: core.SortCreatedAt},
	}
	for {
		page, err := tx.ListProjects(ctx, filter)
		if err != nil {
			return nil, err
		}
		for _, p := range page {
			if p.WorkflowID == workflowID {
				out = append(out, p)
			}
		}
		cursor := nextProjectCursor(filter.Page, page)
		if cursor == "" {
			return out, nil
		}
		filter.Page.Cursor = cursor
	}
}

// migrateRemovedStates refuses an edit that would leave a task in a state the
// new definition drops, unless the request maps that state to a surviving one.
func migrateRemovedStates(ctx context.Context, m *mutation, before *core.Workflow, in core.WorkflowInput) error {
	var removed []string
	for _, s := range before.Definition.States {
		if !in.Definition.HasState(s.Key) {
			removed = append(removed, s.Key)
		}
	}
	if len(removed) == 0 {
		return nil
	}

	counts, err := m.tx.CountTasksInStates(ctx, before.ID, removed)
	if err != nil {
		return err
	}
	plan := make(map[string]string, len(removed))
	for _, state := range removed {
		n := counts[state]
		if n == 0 {
			continue
		}
		target, ok := in.Migrate[state]
		if !ok {
			return core.Conflict("workflow %q no longer defines state %q, which %d task(s) are in; supply a migration for it",
				in.Key, state, n).
				WithDetail("state", state).
				WithDetail("tasks", n)
		}
		if !in.Definition.HasState(target) {
			return core.Invalid("migration maps state %q to %q, which the new definition does not define", state, target)
		}
		plan[state] = target
	}
	if len(plan) == 0 {
		return nil
	}

	projects, err := projectsUsingWorkflow(ctx, m.tx, before.ID)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(projects))
	for _, p := range projects {
		ids = append(ids, p.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	for state, target := range plan {
		if err := migrateTasks(ctx, m, ids, state, target); err != nil {
			return err
		}
	}
	return nil
}

// migrateTasks moves every task of these projects out of one state.
func migrateTasks(ctx context.Context, m *mutation, projectIDs []string, from, to string) error {
	for {
		tasks, err := m.tx.ListTasks(ctx, core.TaskFilter{
			ProjectIDs: projectIDs,
			Statuses:   []string{from},
			Page:       core.Page{Limit: core.MaxPageLimit},
		})
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			return nil
		}
		for i := range tasks {
			before := tasks[i]
			tasks[i].Status = to
			if err := m.tx.UpdateTask(ctx, &tasks[i]); err != nil {
				return err
			}
			if err := m.Record(auditTaskMigrate, core.EventTaskTransitioned, "task", tasks[i].ID,
				tasks[i].ProjectID, before, tasks[i], map[string]any{"from": from, "to": to}); err != nil {
				return err
			}
		}
	}
}
