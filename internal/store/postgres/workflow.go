package postgres

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var workflowColumns = []string{
	"id", "tenant_id", "key", "name", "definition", "builtin", "created_at", "updated_at",
}

func scanWorkflow(s scanner) (core.Workflow, error) {
	var (
		w          core.Workflow
		definition string
		created    sql.NullTime
		updated    sql.NullTime
	)
	if err := s.Scan(&w.ID, &w.TenantID, &w.Key, &w.Name, &definition, &w.Builtin,
		&created, &updated); err != nil {
		return core.Workflow{}, mapErr(err, "scanning workflow")
	}
	if err := sqlb.ParseJSON(definition, &w.Definition); err != nil {
		return core.Workflow{}, core.Internal("decoding definition of workflow %q", w.Key).Wrap(err)
	}
	w.CreatedAt = scanTime(created)
	w.UpdatedAt = scanTime(updated)
	return w, nil
}

// PutWorkflow creates or replaces a workflow, keyed by its tenant-unique key.
func (t *tx) PutWorkflow(ctx context.Context, w *core.Workflow) error {
	w.TenantID = t.scope.TenantID
	definition, err := sqlb.JSONText(w.Definition, "{}")
	if err != nil {
		return core.Internal("encoding definition of workflow %q", w.Key).Wrap(err)
	}
	now := t.store.clock.Now()
	w.UpdatedAt = now

	upd := t.builder("workflows").
		Where("key = ?", w.Key).
		Set("name", w.Name).
		Set("definition", definition).
		Set("builtin", w.Builtin).
		Set("updated_at", timeArg(now))
	n, err := t.execUpdate(ctx, upd, "updating workflow %q", w.Key)
	if err != nil {
		return err
	}
	if n > 0 {
		existing, err := t.GetWorkflow(ctx, w.Key)
		if err != nil {
			return err
		}
		w.ID = existing.ID
		w.CreatedAt = existing.CreatedAt
		return nil
	}

	if w.ID == "" {
		w.ID = id.New()
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	ins := t.insert("workflows").
		Set("id", w.ID).
		Set("key", w.Key).
		Set("name", w.Name).
		Set("definition", definition).
		Set("builtin", w.Builtin).
		Set("created_at", timeArg(w.CreatedAt)).
		Set("updated_at", timeArg(w.UpdatedAt))
	_, err = t.execInsert(ctx, ins, "creating workflow %q", w.Key)
	return err
}

// GetWorkflow returns a workflow by its key.
func (t *tx) GetWorkflow(ctx context.Context, key string) (*core.Workflow, error) {
	return t.workflowWhere(ctx, "key = ?", key, "workflow %q", key)
}

// GetWorkflowByID returns a workflow by identifier.
func (t *tx) GetWorkflowByID(ctx context.Context, workflowID string) (*core.Workflow, error) {
	return t.workflowWhere(ctx, "id = ?", workflowID, "workflow %q", workflowID)
}

func (t *tx) workflowWhere(ctx context.Context, cond string, arg any, what string, whatArgs ...any) (*core.Workflow, error) {
	b := t.builder("workflows").Select(workflowColumns...).Where(cond, arg).Limit(1)
	q, args := b.SelectQuery()
	w, err := scanWorkflow(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound(what, whatArgs...)
		}
		return nil, err
	}
	return &w, nil
}

// ListWorkflows returns every workflow in this tenant.
func (t *tx) ListWorkflows(ctx context.Context) ([]core.Workflow, error) {
	b := t.builder("workflows").Select(workflowColumns...).OrderBy("key", core.Ascending)
	rows, err := t.query(ctx, b, "listing workflows")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Workflow{}
	for rows.Next() {
		v, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing workflows")
}

// DeleteWorkflow removes a workflow by key.
func (t *tx) DeleteWorkflow(ctx context.Context, key string) error {
	b := t.builder("workflows").Where("key = ?", key)
	n, err := t.execDelete(ctx, b, "deleting workflow %q", key)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("workflow %q", key)
	}
	return nil
}

// CountTasksInStates counts live tasks of a workflow's projects, per state.
func (t *tx) CountTasksInStates(ctx context.Context, workflowID string, states []string) (map[string]int, error) {
	out := make(map[string]int, len(states))
	for _, state := range states {
		b := t.builder("tasks").
			Join("JOIN projects ON projects.id = tasks.project_id AND projects.tenant_id = tasks.tenant_id").
			Where("projects.workflow_id = ?", workflowID).
			Where("tasks.status = ?", state).
			Where("tasks.deleted_at IS NULL")
		n, err := t.count(ctx, b, "counting tasks in state %q", state)
		if err != nil {
			return nil, err
		}
		out[state] = n
	}
	return out, nil
}
