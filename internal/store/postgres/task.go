package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/id"
	sqlb "github.com/thereisnotime/tix/internal/store/sql"
)

var taskColumns = []string{
	"tasks.id", "tasks.tenant_id", "tasks.project_id", "tasks.seq", "tasks.parent_id",
	"tasks.title", "tasks.body", "tasks.status", "tasks.priority",
	"tasks.assignee_actor_id", "tasks.creator_actor_id",
	"tasks.due_at", "tasks.started_at", "tasks.completed_at",
	"tasks.claimed_by_actor_id", "tasks.claimed_at", "tasks.lease_expires_at", "tasks.claim_count",
	"tasks.custom_fields", "tasks.version", "tasks.created_at", "tasks.updated_at", "tasks.deleted_at",
	"projects.key",
}

const projectJoin = "JOIN projects ON projects.id = tasks.project_id AND projects.tenant_id = tasks.tenant_id"

var taskSortColumns = map[string]string{
	core.SortCreatedAt: "tasks.created_at",
	core.SortUpdatedAt: "tasks.updated_at",
	core.SortPriority:  "tasks.priority",
	core.SortDueAt:     "tasks.due_at",
	core.SortSeq:       "tasks.seq",
	core.SortTitle:     "tasks.title",
}

// unclaimedPredicate reads a lease whose expiry has passed as unclaimed, so
// correctness never depends on the sweeper having run.
const unclaimedPredicate = "(tasks.claimed_by_actor_id IS NULL OR tasks.lease_expires_at IS NULL OR tasks.lease_expires_at <= ?)"

func scanTask(s scanner) (core.Task, error) {
	var (
		t            core.Task
		parent       sql.NullString
		assignee     sql.NullString
		due          sql.NullTime
		started      sql.NullTime
		completed    sql.NullTime
		claimedBy    sql.NullString
		claimedAt    sql.NullTime
		leaseExpires sql.NullTime
		custom       string
		created      sql.NullTime
		updated      sql.NullTime
		deleted      sql.NullTime
		projectKey   string
	)
	if err := s.Scan(&t.ID, &t.TenantID, &t.ProjectID, &t.Seq, &parent,
		&t.Title, &t.Body, &t.Status, &t.Priority,
		&assignee, &t.CreatorActorID,
		&due, &started, &completed,
		&claimedBy, &claimedAt, &leaseExpires, &t.ClaimCount,
		&custom, &t.Version, &created, &updated, &deleted, &projectKey); err != nil {
		return core.Task{}, mapErr(err, "scanning task")
	}

	t.ParentID = text(parent)
	t.AssigneeActorID = text(assignee)
	t.ClaimedByActorID = text(claimedBy)
	t.Ref = projectKey + "-" + strconv.FormatInt(t.Seq, 10)

	if err := sqlb.ParseJSON(custom, &t.CustomFields); err != nil {
		return core.Task{}, core.Internal("decoding custom fields of task %q", t.ID).Wrap(err)
	}

	t.DueAt = scanNullTime(due)
	t.StartedAt = scanNullTime(started)
	t.CompletedAt = scanNullTime(completed)
	t.ClaimedAt = scanNullTime(claimedAt)
	t.LeaseExpiresAt = scanNullTime(leaseExpires)
	t.CreatedAt = scanTime(created)
	t.UpdatedAt = scanTime(updated)
	t.DeletedAt = scanNullTime(deleted)
	return t, nil
}

// CreateTask inserts a task, reserving its per-project number when unset.
func (t *tx) CreateTask(ctx context.Context, task *core.Task) error {
	if task.ID == "" {
		task.ID = id.New()
	}
	task.TenantID = t.scope.TenantID
	if task.Seq == 0 {
		seq, err := t.NextSeq(ctx, task.ProjectID)
		if err != nil {
			return err
		}
		task.Seq = seq
	}
	if task.Priority == 0 {
		task.Priority = core.PriorityNormal
	}
	if task.Version == 0 {
		task.Version = 1
	}
	now := t.store.clock.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	custom, err := sqlb.JSONText(task.CustomFields, "{}")
	if err != nil {
		return core.Internal("encoding custom fields of task %q", task.ID).Wrap(err)
	}

	ins := t.insert("tasks").
		Set("id", task.ID).
		Set("project_id", task.ProjectID).
		Set("seq", task.Seq).
		Set("parent_id", nullText(task.ParentID)).
		Set("title", task.Title).
		Set("body", task.Body).
		Set("status", task.Status).
		Set("priority", int(task.Priority)).
		Set("assignee_actor_id", nullText(task.AssigneeActorID)).
		Set("creator_actor_id", task.CreatorActorID).
		Set("due_at", nullTimeArg(task.DueAt)).
		Set("started_at", nullTimeArg(task.StartedAt)).
		Set("completed_at", nullTimeArg(task.CompletedAt)).
		Set("claimed_by_actor_id", nullText(task.ClaimedByActorID)).
		Set("claimed_at", nullTimeArg(task.ClaimedAt)).
		Set("lease_expires_at", nullTimeArg(task.LeaseExpiresAt)).
		Set("claim_count", task.ClaimCount).
		Set("custom_fields", custom).
		Set("version", task.Version).
		Set("created_at", timeArg(task.CreatedAt)).
		Set("updated_at", timeArg(task.UpdatedAt)).
		Set("deleted_at", nullTimeArg(task.DeletedAt))
	if _, err := t.execInsert(ctx, ins, "creating task %q", task.Title); err != nil {
		return err
	}
	if task.Ref == "" {
		loaded, err := t.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		task.Ref = loaded.Ref
	}
	return nil
}

// NextSeq reserves the next per-project task number. The project row is locked
// first, so two concurrent writers cannot read the same highest number.
func (t *tx) NextSeq(ctx context.Context, projectID string) (int64, error) {
	lock := t.builder("projects").Select("1").Where("id = ?", projectID).Limit(1)
	lockQuery, lockArgs := lock.SelectQuery()
	if _, err := t.ex.ExecContext(ctx, lockQuery+" FOR UPDATE", lockArgs...); err != nil {
		return 0, mapErr(err, "locking project %q", projectID)
	}
	b := t.builder("tasks").
		Select("COALESCE(MAX(tasks.seq), 0) + 1").
		Where("tasks.project_id = ?", projectID)
	q, args := b.SelectQuery()
	var seq int64
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&seq); err != nil {
		return 0, mapErr(err, "reserving next task number for project %q", projectID)
	}
	return seq, nil
}

// GetTask returns one task addressed by identifier or by project reference.
func (t *tx) GetTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
	if !ref.Valid() {
		return nil, core.Invalid("task reference addresses nothing")
	}
	b := t.builder("tasks").Select(taskColumns...).Join(projectJoin).Limit(1)
	if ref.IsID() {
		b.Where("tasks.id = ?", ref.ID)
	} else {
		b.Where("projects.key = ?", ref.ProjectKey).Where("tasks.seq = ?", ref.Seq)
	}
	q, args := b.SelectQuery()
	task, err := scanTask(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("task %q", ref.String())
		}
		return nil, err
	}
	out := []core.Task{task}
	if err := t.fillTaskRelations(ctx, out); err != nil {
		return nil, err
	}
	return &out[0], nil
}

// ListTasks returns tasks matching the filter, keyset paginated.
func (t *tx) ListTasks(ctx context.Context, f core.TaskFilter) ([]core.Task, error) {
	f, err := f.Validate()
	if err != nil {
		return nil, err
	}
	spec, err := resolvePage(f.Page, core.SortCreatedAt, taskSortColumns)
	if err != nil {
		return nil, err
	}

	b := t.builder("tasks").Select(taskColumns...).Join(projectJoin)
	if err := t.applyTaskFilter(b, f); err != nil {
		return nil, err
	}
	rows, err := t.query(ctx, spec.apply(b, "tasks.id"), "listing tasks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Task{}
	for rows.Next() {
		v, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := mapRowsErr(rows, "listing tasks"); err != nil {
		return nil, err
	}
	if err := t.fillTaskRelations(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (t *tx) applyTaskFilter(b *sqlb.Builder, f core.TaskFilter) error {
	if !f.IncludeDeleted {
		b.Where("tasks.deleted_at IS NULL")
	}
	if len(f.ProjectIDs) > 0 {
		b.WhereIn("tasks.project_id", f.ProjectIDs)
	}
	if len(f.ProjectKeys) > 0 {
		b.WhereIn("projects.key", f.ProjectKeys)
	}
	if len(f.Statuses) > 0 {
		b.WhereIn("tasks.status", f.Statuses)
	}
	if len(f.AssigneeIDs) > 0 {
		b.WhereIn("tasks.assignee_actor_id", f.AssigneeIDs)
	}
	if len(f.CreatorIDs) > 0 {
		b.WhereIn("tasks.creator_actor_id", f.CreatorIDs)
	}
	if len(f.ClaimedBy) > 0 {
		b.WhereIn("tasks.claimed_by_actor_id", f.ClaimedBy)
	}
	if len(f.Priorities) > 0 {
		values := make([]string, len(f.Priorities))
		for i, p := range f.Priorities {
			values[i] = strconv.Itoa(int(p))
		}
		b.WhereIn("CAST(tasks.priority AS TEXT)", values)
	}
	if len(f.Tags) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(f.Tags)), ", ")
		args := make([]any, 0, len(f.Tags))
		for _, l := range f.Tags {
			args = append(args, l)
		}
		b.Where("EXISTS (SELECT 1 FROM task_tags tl JOIN tags l"+
			" ON l.id = tl.tag_id AND l.tenant_id = tl.tenant_id"+
			" WHERE tl.tenant_id = tasks.tenant_id AND tl.task_id = tasks.id AND l.name IN ("+marks+"))",
			args...)
	}
	if f.DueBefore != nil {
		b.Where("tasks.due_at IS NOT NULL AND tasks.due_at <= ?", timeArg(*f.DueBefore))
	}
	if f.DueAfter != nil {
		b.Where("tasks.due_at IS NOT NULL AND tasks.due_at >= ?", timeArg(*f.DueAfter))
	}
	if f.ParentID != "" {
		b.Where("tasks.parent_id = ?", f.ParentID)
	}
	if f.ParentIsNull {
		b.Where("tasks.parent_id IS NULL")
	}
	switch f.Claimed {
	case core.Yes:
		b.Where("NOT "+unclaimedPredicate, t.now())
	case core.No:
		b.Where(unclaimedPredicate, t.now())
	case core.Either:
	}
	switch f.Blocked {
	case core.Yes:
		b.Where("EXISTS " + blockingSubquery)
	case core.No:
		b.Where("NOT EXISTS " + blockingSubquery)
	case core.Either:
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		b.Where("tasks.search_tsv @@ plainto_tsquery('simple', ?)", q)
	}
	for key, value := range f.CustomFields {
		if !validFieldKey(key) {
			return core.Invalid("custom field key %q contains unsupported characters", key)
		}
		b.Where("tasks.custom_fields ->> '"+key+"' = ?", fmt.Sprint(value))
	}
	return nil
}

// blockingSubquery matches a task with at least one unfinished dependency.
const blockingSubquery = `(SELECT 1 FROM task_deps d JOIN tasks dep
   ON dep.id = d.depends_on AND dep.tenant_id = d.tenant_id
 WHERE d.tenant_id = tasks.tenant_id AND d.task_id = tasks.id
   AND dep.deleted_at IS NULL AND dep.completed_at IS NULL)`

func validFieldKey(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// fillTaskRelations loads tags, dependencies and the blocked flag for a page
// of tasks in three statements rather than three per row.
func (t *tx) fillTaskRelations(ctx context.Context, tasks []core.Task) error {
	if len(tasks) == 0 {
		return nil
	}
	ids := make([]string, len(tasks))
	index := make(map[string]*core.Task, len(tasks))
	for i := range tasks {
		ids[i] = tasks[i].ID
		index[tasks[i].ID] = &tasks[i]
	}

	tags := t.builder("task_tags").
		Select("task_tags.task_id", "tags.name").
		Join("JOIN tags ON tags.id = task_tags.tag_id AND tags.tenant_id = task_tags.tenant_id").
		OrderBy("tags.name", core.Ascending)
	tags.WhereIn("task_tags.task_id", ids)
	if err := t.eachPair(ctx, tags, "loading task tags", func(taskID, name string) {
		if task := index[taskID]; task != nil {
			task.Tags = append(task.Tags, name)
		}
	}); err != nil {
		return err
	}

	deps := t.builder("task_deps").
		Select("task_deps.task_id", "task_deps.depends_on").
		OrderBy("task_deps.depends_on", core.Ascending)
	deps.WhereIn("task_deps.task_id", ids)
	if err := t.eachPair(ctx, deps, "loading task dependencies", func(taskID, dependsOn string) {
		if task := index[taskID]; task != nil {
			task.DependsOn = append(task.DependsOn, dependsOn)
		}
	}); err != nil {
		return err
	}

	blocked := t.builder("task_deps").
		Select("task_deps.task_id", "task_deps.task_id").
		Join("JOIN tasks dep ON dep.id = task_deps.depends_on AND dep.tenant_id = task_deps.tenant_id").
		Where("dep.deleted_at IS NULL").
		Where("dep.completed_at IS NULL")
	blocked.WhereIn("task_deps.task_id", ids)
	return t.eachPair(ctx, blocked, "loading blocked tasks", func(taskID, _ string) {
		if task := index[taskID]; task != nil {
			task.Blocked = true
		}
	})
}

func (t *tx) eachPair(ctx context.Context, b *sqlb.Builder, what string, fn func(a, c string)) error {
	rows, err := t.query(ctx, b, "%s", what)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var a, c string
		if err := rows.Scan(&a, &c); err != nil {
			return mapErr(err, "%s", what)
		}
		fn(a, c)
	}
	return mapRowsErr(rows, what)
}

// UpdateTask writes a task's mutable fields and bumps its version.
func (t *tx) UpdateTask(ctx context.Context, task *core.Task) error {
	task.UpdatedAt = t.store.clock.Now()
	custom, err := sqlb.JSONText(task.CustomFields, "{}")
	if err != nil {
		return core.Internal("encoding custom fields of task %q", task.ID).Wrap(err)
	}

	b := t.builder("tasks").
		Where("tasks.id = ?", task.ID).
		Set("parent_id", nullText(task.ParentID)).
		Set("title", task.Title).
		Set("body", task.Body).
		Set("status", task.Status).
		Set("priority", int(task.Priority)).
		Set("assignee_actor_id", nullText(task.AssigneeActorID)).
		Set("due_at", nullTimeArg(task.DueAt)).
		Set("started_at", nullTimeArg(task.StartedAt)).
		Set("completed_at", nullTimeArg(task.CompletedAt)).
		Set("custom_fields", custom).
		Set("updated_at", timeArg(task.UpdatedAt)).
		SetExpr("version", "version + 1")
	n, err := t.execUpdate(ctx, b, "updating task %q", task.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("task %q", task.ID)
	}
	task.Version++
	return nil
}

// DeleteTask soft deletes a task, or removes it outright when hard is set.
func (t *tx) DeleteTask(ctx context.Context, taskID string, hard bool) error {
	if hard {
		b := t.builder("tasks").Where("tasks.id = ?", taskID)
		n, err := t.execDelete(ctx, b, "deleting task %q", taskID)
		if err != nil {
			return err
		}
		if n == 0 {
			return core.NotFound("task %q", taskID)
		}
		return nil
	}
	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where("tasks.deleted_at IS NULL").
		Set("deleted_at", t.now()).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "soft deleting task %q", taskID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("task %q", taskID)
	}
	return nil
}

// RestoreTask clears a task's soft deletion.
func (t *tx) RestoreTask(ctx context.Context, taskID string) error {
	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where("tasks.deleted_at IS NOT NULL").
		Set("deleted_at", nil).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "restoring task %q", taskID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("deleted task %q", taskID)
	}
	return nil
}

// Children returns a task's direct children.
func (t *tx) Children(ctx context.Context, parentID string) ([]core.Task, error) {
	b := t.builder("tasks").
		Select(taskColumns...).
		Join(projectJoin).
		Where("tasks.parent_id = ?", parentID).
		Where("tasks.deleted_at IS NULL").
		OrderBy("tasks.seq", core.Ascending)
	rows, err := t.query(ctx, b, "listing children of task %q", parentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Task{}
	for rows.Next() {
		v, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := mapRowsErr(rows, "listing children"); err != nil {
		return nil, err
	}
	return out, t.fillTaskRelations(ctx, out)
}
