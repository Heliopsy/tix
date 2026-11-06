// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/query"
	"github.com/heliopsy/tix/internal/quickadd"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/taskio"
)

// maxExportHistory bounds the history entries one exported task carries, so a
// task changed ten thousand times does not turn one cell into a megabyte.
const maxExportHistory = 500

// ExportTasks writes one project's tasks in the chosen format as it walks them.
func (l *Local) ExportTasks(ctx context.Context, in core.TaskExportInput, w io.Writer) error {
	actor, err := l.authorize(ctx, authz.ActionTaskExport, authz.Resource{})
	if err != nil {
		return err
	}
	in = in.Normalize()
	if err := in.Validate(); err != nil {
		return err
	}
	date, err := exportDates(in)
	if err != nil {
		return err
	}
	return l.read(ctx, actor, func(tx store.Tx) error {
		project, err := lookupProject(ctx, tx, in.ProjectRef)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionTaskExport, authz.Resource{ProjectID: project.ID}); err != nil {
			return err
		}
		x, err := newTaskExport(ctx, tx, l.clock.Now(), actor, project, in)
		if err != nil {
			return err
		}
		out, err := taskio.NewWriter(w, in.Format, x.columns,
			taskio.WriteOptions{Delimiter: core.TaskDelimiters[in.Delimiter], Date: date})
		if err != nil {
			return err
		}
		if err := x.walk(ctx, func(t core.Task) error {
			values, err := x.row(ctx, t)
			if err != nil {
				return err
			}
			return out.Write(values)
		}); err != nil {
			return err
		}
		return out.Close()
	})
}

// exportDates is how an export renders an instant.
func exportDates(in core.TaskExportInput) (func(time.Time) string, error) {
	switch in.Dates {
	case core.TaskDatesDate:
		return func(t time.Time) string { return t.UTC().Format(time.DateOnly) }, nil
	case core.TaskDatesDisplay:
		format := in.TimeFormat
		if format == output.TimeRelative {
			return nil, core.Invalid("a relative time format cannot be exported; choose iso, rfc3339, short or us").
				WithDetail(core.DetailField, "time_format")
		}
		style, err := output.NewTimeStyle(format, in.Timezone)
		if err != nil {
			return nil, core.Invalid("%v", err).WithDetail(core.DetailField, "time_format")
		}
		return style.Absolute, nil
	default:
		return func(t time.Time) string { return t.UTC().Format(time.RFC3339) }, nil
	}
}

// taskExport is one export run: what it selects, the columns it writes, and
// the lookups it memoises so a cell costs a query once per distinct value.
type taskExport struct {
	tx       store.Tx
	project  *core.Project
	workflow core.WorkflowDefinition
	filter   core.TaskFilter
	columns  []string
	finished bool

	handles map[string]string
	refs    map[string]string
}

func newTaskExport(ctx context.Context, tx store.Tx, now time.Time, actor *core.Actor,
	project *core.Project, in core.TaskExportInput) (*taskExport, error) {
	f, err := query.ParseAt(in.Query, now)
	if err != nil {
		var ce *core.Error
		if errors.As(err, &ce) {
			return nil, ce.WithDetail(core.DetailField, "query")
		}
		return nil, err
	}
	if len(f.ProjectKeys) > 0 || len(f.ProjectIDs) > 0 {
		return nil, core.Invalid("the query names a project; an export is of the one project it was asked for").
			WithDetail(core.DetailField, "query")
	}
	f.ProjectIDs = []string{project.ID}
	f.StarredBy = actor.ID
	f.IncludeDeleted = f.IncludeDeleted || in.IncludeTrashed
	if in.ExcludeSubtasks && f.ParentID == "" {
		f.ParentIsNull = true
	}
	if f.Snoozed == core.Either {
		f.IncludeSnoozed = true
	}
	bindMe(&f, actor.ID)
	if f, err = f.Validate(); err != nil {
		return nil, err
	}
	if err := resolveTaskFilter(ctx, tx, &f); err != nil {
		return nil, err
	}
	wf, err := tx.GetWorkflowByID(ctx, project.WorkflowID)
	if err != nil {
		return nil, err
	}
	columns := in.Columns
	if len(columns) == 0 {
		defs, err := tx.ListFieldDefs(ctx, project.ID)
		if err != nil {
			return nil, err
		}
		columns = slices.Clone(core.DefaultTaskColumns)
		for _, d := range defs {
			columns = append(columns, core.CustomColumn(d.Key))
		}
	}
	return &taskExport{
		tx: tx, project: project, workflow: wf.Definition, filter: f, columns: columns,
		finished: !in.ExcludeFinished,
		handles:  map[string]string{}, refs: map[string]string{},
	}, nil
}

// walk hands every selected task to fn in sequence order, one page at a time.
func (x *taskExport) walk(ctx context.Context, fn func(core.Task) error) error {
	f := x.filter
	f.Page = core.Page{Limit: transferPage, Sort: core.SortSeq, Direction: core.Ascending}
	for {
		tasks, err := x.tx.ListTasks(ctx, f)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if !x.finished && x.workflow.IsTerminal(t.Status) {
				continue
			}
			if err := fn(t); err != nil {
				return err
			}
		}
		if len(tasks) < transferPage {
			return nil
		}
		last := tasks[len(tasks)-1]
		f.Page.Cursor = core.Cursor{
			SortValue: taskSortValue(core.SortSeq, last),
			ID:        last.ID,
			Sort:      core.SortSeq,
			Direction: core.Ascending,
		}.Encode()
	}
}

// row is one task's cells in column order.
func (x *taskExport) row(ctx context.Context, t core.Task) ([]any, error) {
	out := make([]any, len(x.columns))
	for i, c := range x.columns {
		v, err := x.cell(ctx, t, c)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (x *taskExport) cell(ctx context.Context, t core.Task, column string) (any, error) {
	if key, ok := core.CustomColumnKey(column); ok {
		return t.CustomFields[key], nil
	}
	switch column {
	case core.TaskColumnRef:
		return t.Ref, nil
	case core.TaskColumnID:
		return t.ID, nil
	case core.TaskColumnProject:
		return x.project.Key, nil
	case core.TaskColumnTitle:
		return t.Title, nil
	case core.TaskColumnBody:
		return t.Body, nil
	case core.TaskColumnStatus:
		return t.Status, nil
	case core.TaskColumnPriority:
		return quickadd.PriorityName(t.Priority), nil
	case core.TaskColumnAssignee:
		return x.handle(ctx, t.AssigneeActorID)
	case core.TaskColumnCreator:
		return x.handle(ctx, t.CreatorActorID)
	case core.TaskColumnTags:
		return nonNil(t.Tags), nil
	case core.TaskColumnDue:
		return t.DueAt, nil
	case core.TaskColumnParent:
		return x.ref(ctx, t.ParentID)
	case core.TaskColumnDependsOn:
		return x.refList(ctx, t.DependsOn)
	case core.TaskColumnRecurrence:
		return t.Recurrence, nil
	case core.TaskColumnCreatedAt:
		return t.CreatedAt, nil
	case core.TaskColumnUpdatedAt:
		return t.UpdatedAt, nil
	case core.TaskColumnStartedAt:
		return t.StartedAt, nil
	case core.TaskColumnCompletedAt:
		return t.CompletedAt, nil
	case core.TaskColumnDeletedAt:
		return t.DeletedAt, nil
	case core.TaskColumnComments:
		return x.comments(ctx, t)
	case core.TaskColumnHistory:
		return x.history(ctx, t)
	}
	return nil, core.Invalid("column %q is not supported", column)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// handle is an actor's handle, or its identifier when it no longer resolves.
func (x *taskExport) handle(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", nil
	}
	if h, ok := x.handles[id]; ok {
		return h, nil
	}
	h := id
	actor, err := x.tx.GetActor(ctx, id)
	switch {
	case err == nil && actor.Handle != "":
		h = actor.Handle
	case err != nil && !core.IsKind(err, core.KindNotFound):
		return "", err
	}
	x.handles[id] = h
	return h, nil
}

// ref is a task's reference, or empty when the task is gone.
func (x *taskExport) ref(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", nil
	}
	if r, ok := x.refs[id]; ok {
		return r, nil
	}
	t, err := x.tx.GetTask(ctx, core.TaskRef{ID: id})
	switch {
	case core.IsKind(err, core.KindNotFound):
		x.refs[id] = ""
		return "", nil
	case err != nil:
		return "", err
	}
	x.refs[id] = t.Ref
	return t.Ref, nil
}

func (x *taskExport) refList(ctx context.Context, ids []string) ([]string, error) {
	out := []string{}
	for _, id := range ids {
		r, err := x.ref(ctx, id)
		if err != nil {
			return nil, err
		}
		if r != "" {
			out = append(out, r)
		}
	}
	return out, nil
}

func (x *taskExport) comments(ctx context.Context, t core.Task) ([]taskio.Comment, error) {
	found, err := x.tx.ListComments(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	out := make([]taskio.Comment, 0, len(found))
	for _, c := range found {
		author, err := x.handle(ctx, c.AuthorActorID)
		if err != nil {
			return nil, err
		}
		out = append(out, taskio.Comment{Author: author, At: c.CreatedAt, Body: c.Body})
	}
	return out, nil
}

func (x *taskExport) history(ctx context.Context, t core.Task) ([]taskio.Change, error) {
	entries, err := x.tx.ListAudit(ctx, core.AuditFilter{
		SubjectType: "task", SubjectID: t.ID,
		Page: core.Page{Limit: maxExportHistory, Sort: core.SortSeq, Direction: core.Ascending},
	})
	if err != nil {
		return nil, err
	}
	out := make([]taskio.Change, 0, len(entries))
	for _, e := range entries {
		who, err := x.handle(ctx, e.ActorID)
		if err != nil {
			return nil, err
		}
		out = append(out, taskio.Change{Actor: who, At: e.OccurredAt, Action: e.Action})
	}
	return out, nil
}
