package service

import (
	"context"
	"encoding/json"
	"github.com/heliopsy/tix/internal/id"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// CreateTask creates a task, defaulting every attribute except its title.
func (l *Local) CreateTask(ctx context.Context, in core.CreateTaskInput) (*core.Task, error) {
	in.Title = strings.TrimSpace(in.Title)
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if len(in.Body) > core.MaxBodyLength {
		return nil, core.Invalid("task body must be at most %d characters", core.MaxBodyLength)
	}
	actor, err := l.authorize(ctx, authz.ActionTaskCreate, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Task
	err = l.write(ctx, actor, func(m *mutation) error {
		project, err := projectForTask(ctx, m.tx, in.ProjectRef)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionTaskCreate, authz.Resource{ProjectID: project.ID}); err != nil {
			return err
		}
		wf, err := m.tx.GetWorkflowByID(ctx, project.WorkflowID)
		if err != nil {
			return err
		}

		status := wf.Definition.Initial
		if in.Status != "" {
			if !wf.Definition.HasState(in.Status) {
				return core.Invalid("status %q is not a state of workflow %q", in.Status, wf.Key)
			}
			status = in.Status
		}

		defs, err := m.tx.ListFieldDefs(ctx, project.ID)
		if err != nil {
			return err
		}
		fields, err := validateTaskFields(defs, in.CustomFields, true)
		if err != nil {
			return err
		}

		parentID := ""
		if in.ParentRef != "" {
			parent, err := taskByRef(ctx, m.tx, in.ParentRef)
			if err != nil {
				return err
			}
			parentID = parent.ID
		}

		seq, err := m.tx.NextSeq(ctx, project.ID)
		if err != nil {
			return err
		}

		priority := in.Priority
		if priority == 0 {
			priority = core.PriorityNormal
		}
		task := &core.Task{
			ProjectID:       project.ID,
			Seq:             seq,
			ParentID:        parentID,
			Title:           in.Title,
			Body:            in.Body,
			Status:          status,
			Priority:        priority,
			AssigneeActorID: in.AssigneeActorID,
			CreatorActorID:  actor.ID,
			DueAt:           in.DueAt,
			CustomFields:    fields,
		}
		if wf.Definition.IsTerminal(status) {
			completed := m.now
			task.CompletedAt = &completed
		}
		if err := m.tx.CreateTask(ctx, task); err != nil {
			return err
		}

		for _, name := range in.Tags {
			if _, err := attachTaskLabel(ctx, m.tx, task.ID, name); err != nil {
				return err
			}
		}
		for _, ref := range in.DependsOn {
			dep, err := taskByRef(ctx, m.tx, ref)
			if err != nil {
				return err
			}
			if err := linkDependency(ctx, m.tx, task, dep); err != nil {
				return err
			}
		}

		if err := m.Record("task.create", core.EventTaskCreated, "task", task.ID, project.ID,
			nil, task, map[string]any{"ref": task.Ref, "title": task.Title, "status": task.Status}); err != nil {
			return err
		}
		out, err = m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetTask returns one live task by identifier or human reference.
func (l *Local) GetTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.Task
	err = l.read(ctx, actor, func(tx store.Tx) error {
		task, err := tx.GetTask(ctx, ref)
		if err != nil {
			return err
		}
		if task.Deleted() {
			return core.NotFound("task %q", ref.String())
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskRead, task); err != nil {
			return err
		}
		out = task
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListTasks returns one keyset-paginated page of tasks.
func (l *Local) ListTasks(ctx context.Context, f core.TaskFilter) (core.TaskPage, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return core.TaskPage{}, err
	}
	f, err = f.Validate()
	if err != nil {
		return core.TaskPage{}, err
	}
	if actor.ScopedToProject() {
		f.ProjectIDs = []string{actor.ProjectID}
	}

	limit := f.Page.Limit
	probe := f
	probe.Page.Limit = limit + 1
	if probe.Page.Limit > core.MaxPageLimit {
		probe.Page.Limit = core.MaxPageLimit
	}

	var tasks []core.Task
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		tasks, err = tx.ListTasks(ctx, probe)
		return err
	}); err != nil {
		return core.TaskPage{}, err
	}

	more := len(tasks) > limit
	if more {
		tasks = tasks[:limit]
	} else if limit >= core.MaxPageLimit && len(tasks) == limit {
		more = true
	}
	page := core.TaskPage{Tasks: tasks}
	if more && len(tasks) > 0 {
		last := tasks[len(tasks)-1]
		page.NextCursor = core.Cursor{
			SortValue:  taskSortValue(f.Page.Sort, last),
			SortValue2: taskSortValue2(f.Page.Sort, last),
			ID:         last.ID,
			Sort:       f.Page.Sort,
			Direction:  f.Page.Direction,
		}.Encode()
	}
	return page, nil
}

// UpdateTask changes only the attributes the input supplies.
func (l *Local) UpdateTask(ctx context.Context, ref core.TaskRef, in core.UpdateTaskInput) (*core.Task, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Task
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskUpdate, task); err != nil {
			return err
		}
		if err := checkVersion(task, in.Version); err != nil {
			return err
		}
		before := *task

		if in.Title != nil {
			title := strings.TrimSpace(*in.Title)
			if title == "" {
				return core.Invalid("task title is required")
			}
			if len(title) > core.MaxTitleLength {
				return core.Invalid("task title must be at most %d characters", core.MaxTitleLength)
			}
			task.Title = title
		}
		if in.Body != nil {
			if len(*in.Body) > core.MaxBodyLength {
				return core.Invalid("task body must be at most %d characters", core.MaxBodyLength)
			}
			task.Body = *in.Body
		}
		if in.Priority != nil {
			if !in.Priority.Valid() {
				return core.Invalid("priority %d is out of range", *in.Priority)
			}
			task.Priority = *in.Priority
		}
		if in.AssigneeActorID != nil {
			assignee, err := resolveAssignee(ctx, m.tx, *in.AssigneeActorID)
			if err != nil {
				return err
			}
			task.AssigneeActorID = assignee
		}
		if in.DueAt != nil {
			task.DueAt = *in.DueAt
		}
		if in.ParentRef != nil {
			parentID, err := resolveParent(ctx, m.tx, task, *in.ParentRef)
			if err != nil {
				return err
			}
			task.ParentID = parentID
		}
		if in.CustomFields != nil {
			defs, err := m.tx.ListFieldDefs(ctx, task.ProjectID)
			if err != nil {
				return err
			}
			fields, err := mergeTaskFields(defs, task.CustomFields, in.CustomFields)
			if err != nil {
				return err
			}
			task.CustomFields = fields
		}

		if err := m.tx.UpdateTask(ctx, task); err != nil {
			return err
		}
		if in.Tags != nil {
			if err := replaceTaskLabels(ctx, m.tx, task, *in.Tags); err != nil {
				return err
			}
		}
		payload := map[string]any{"ref": task.Ref, "version": task.Version}
		if fields := updatedFields(in); len(fields) > 0 {
			payload["fields"] = fields
		}
		if err := m.Record("task.update", core.EventTaskUpdated, "task", task.ID, task.ProjectID,
			before, task, payload); err != nil {
			return err
		}
		out, err = m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// updatedFields names the fields an update supplied, in the wire spelling the
// rest of the system labels them by. Subscribers see only an event's payload,
// so without this a title change and a due date change are the same line.
func updatedFields(in core.UpdateTaskInput) []string {
	var out []string
	if in.Title != nil {
		out = append(out, "title")
	}
	if in.Body != nil {
		out = append(out, "body")
	}
	if in.Priority != nil {
		out = append(out, "priority")
	}
	if in.AssigneeActorID != nil {
		out = append(out, "assignee_actor_id")
	}
	if in.ParentRef != nil {
		out = append(out, "parent_ref")
	}
	if in.DueAt != nil {
		out = append(out, "due_at")
	}
	if in.CustomFields != nil {
		out = append(out, "custom_fields")
	}
	if in.Tags != nil {
		out = append(out, "tags")
	}
	return out
}

// changedTaskFields names the fields two snapshots of a task differ in, using
// the same spelling updatedFields does, for a writer that applies a whole task
// rather than a list of changes.
func changedTaskFields(before, after *core.Task) []string {
	var out []string
	if before.Title != after.Title {
		out = append(out, "title")
	}
	if before.Body != after.Body {
		out = append(out, "body")
	}
	if before.Status != after.Status {
		out = append(out, "status")
	}
	if before.Priority != after.Priority {
		out = append(out, "priority")
	}
	if before.AssigneeActorID != after.AssigneeActorID {
		out = append(out, "assignee_actor_id")
	}
	if before.ParentID != after.ParentID {
		out = append(out, "parent_ref")
	}
	if !timePtrEqual(before.DueAt, after.DueAt) {
		out = append(out, "due_at")
	}
	if !customFieldsEqual(before.CustomFields, after.CustomFields) {
		out = append(out, "custom_fields")
	}
	if !tagsEqual(before.Tags, after.Tags) {
		out = append(out, "tags")
	}
	return out
}

// TransitionTask moves a task along an edge its project's workflow permits.
func (l *Local) TransitionTask(ctx context.Context, ref core.TaskRef, in core.TransitionInput) (*core.Task, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	actor, err := l.authorize(ctx, authz.ActionTaskTransition, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Task
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskTransition, task); err != nil {
			return err
		}
		if err := checkVersion(task, in.Version); err != nil {
			return err
		}
		def, err := workflowForTask(ctx, m.tx, task)
		if err != nil {
			return err
		}
		if !def.HasState(in.To) {
			return core.Invalid("status %q is not a state of this project's workflow", in.To)
		}
		edge, ok := def.CanTransition(task.Status, in.To)
		if !ok {
			return core.Precondition("cannot move task %q from %q to %q; allowed targets are %s",
				task.Ref, task.Status, in.To, strings.Join(allowedTargets(def, task.Status), ", "))
		}
		comment := strings.TrimSpace(in.Comment)
		if edge.RequiresComment && comment == "" {
			return core.Invalid("moving from %q to %q requires a comment", task.Status, in.To)
		}
		if len(comment) > core.MaxCommentLength {
			return core.Invalid("comment must be at most %d characters", core.MaxCommentLength)
		}
		if edge.RequiresScope != "" && !actor.HasScope(edge.RequiresScope) {
			return core.Forbidden("moving from %q to %q requires scope %q",
				task.Status, in.To, edge.RequiresScope).WithDetail("scope", string(edge.RequiresScope))
		}
		if err := checkLease(ctx, m.tx, task, in.LeaseToken, m.now); err != nil {
			return err
		}

		if in.CustomFields != nil {
			defs, err := m.tx.ListFieldDefs(ctx, task.ProjectID)
			if err != nil {
				return err
			}
			fields, err := mergeTaskFields(defs, task.CustomFields, in.CustomFields)
			if err != nil {
				return err
			}
			task.CustomFields = fields
		}

		before := *task
		task.Status = in.To
		now := m.now
		if def.IsTerminal(in.To) {
			task.CompletedAt = &now
		} else {
			task.CompletedAt = nil
			if task.StartedAt == nil && in.To != def.Initial {
				task.StartedAt = &now
			}
		}
		if err := m.tx.UpdateTask(ctx, task); err != nil {
			return err
		}
		if comment != "" {
			c := &core.Comment{TaskID: task.ID, AuthorActorID: actor.ID, Body: comment}
			if err := m.tx.CreateComment(ctx, c); err != nil {
				return err
			}
		}
		if err := m.Record("task.transition", core.EventTaskTransitioned, "task", task.ID, task.ProjectID,
			before, task, map[string]any{"ref": task.Ref, "from": before.Status, "to": task.Status}); err != nil {
			return err
		}
		out, err = m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteTask soft deletes a task, or removes it permanently when Hard is set.
// A task with children is refused unless Cascade is set, in which case the
// whole subtree goes in the same transaction, deepest first.
func (l *Local) DeleteTask(ctx context.Context, ref core.TaskRef, in core.DeleteTaskInput) error {
	actor, err := l.authorize(ctx, authz.ActionTaskDelete, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskDelete, task); err != nil {
			return err
		}
		subtree, err := deleteOrder(ctx, m.tx, task)
		if err != nil {
			return err
		}
		if len(subtree) > 1 && !in.Cascade {
			return core.Precondition("task %q has %d descendant task(s); pass cascade or delete them first",
				task.Ref, len(subtree)-1).WithDetail("descendants", len(subtree)-1)
		}
		for i := range subtree {
			t := &subtree[i]
			if err := l.authorizeTask(ctx, authz.ActionTaskDelete, t); err != nil {
				return err
			}
			if err := m.tx.DeleteTask(ctx, t.ID, in.Hard); err != nil {
				return err
			}
			if err := m.Record("task.delete", core.EventTaskDeleted, "task", t.ID, t.ProjectID,
				t, nil, map[string]any{"ref": t.Ref, "hard": in.Hard, "cascade": in.Cascade}); err != nil {
				return err
			}
		}
		return nil
	})
}

// deleteOrder returns the task and every descendant, deepest first, so each
// row is removed before its parent. It refuses a subtree deeper than
// maxSubtreeDepth, which a cycle in parent links would otherwise turn into an
// unbounded walk.
func deleteOrder(ctx context.Context, tx store.Tx, root *core.Task) ([]core.Task, error) {
	out := []core.Task{*root}
	seen := map[string]bool{root.ID: true}
	for i := 0; i < len(out); i++ {
		if i >= maxSubtreeSize {
			return nil, core.Precondition("task %q has more than %d descendants; delete them in smaller batches",
				root.Ref, maxSubtreeSize)
		}
		children, err := tx.Children(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		for _, c := range children {
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, c)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// maxSubtreeSize bounds a cascading delete so one call cannot walk an
// unbounded number of rows inside a write transaction.
const maxSubtreeSize = 10000

// RestoreTask clears a task's soft deletion.
func (l *Local) RestoreTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskDelete, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.Task
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := m.tx.GetTask(ctx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskDelete, task); err != nil {
			return err
		}
		if err := m.tx.RestoreTask(ctx, task.ID); err != nil {
			return err
		}
		if err := m.Record("task.restore", core.EventTaskUpdated, "task", task.ID, task.ProjectID,
			task, nil, map[string]any{"ref": task.Ref, "restored": true}); err != nil {
			return err
		}
		out, err = m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TaskTree returns a task and its live descendants in pre-order.
func (l *Local) TaskTree(ctx context.Context, ref core.TaskRef, depth int) ([]core.Task, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.Task
	err = l.read(ctx, actor, func(tx store.Tx) error {
		root, err := liveTask(ctx, tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskRead, root); err != nil {
			return err
		}
		out = append(out, *root)
		return appendDescendants(ctx, tx, root.ID, depth, 1, &out)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// appendDescendants walks children depth-first in pre-order, stopping at the
// depth limit. It visits each task once and refuses a subtree larger than
// maxSubtreeSize: a cycle in parent links would otherwise make the walk
// unbounded, and as a recursion that is a stack overflow, which is fatal and
// cannot be recovered by the caller.
func appendDescendants(ctx context.Context, tx store.Tx, parentID string, limit, level int, out *[]core.Task) error {
	type frame struct {
		task  core.Task
		level int
	}
	var stack []frame
	push := func(children []core.Task, level int) {
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, frame{task: children[i], level: level})
		}
	}

	seen := map[string]bool{parentID: true}
	children, err := tx.Children(ctx, parentID)
	if err != nil {
		return err
	}
	push(children, level)

	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if limit > 0 && cur.level > limit {
			continue
		}
		if seen[cur.task.ID] {
			continue
		}
		seen[cur.task.ID] = true
		if len(*out) >= maxSubtreeSize {
			return core.Precondition("task subtree is larger than %d tasks; ask for a bounded depth",
				maxSubtreeSize).WithDetail("max_subtree_size", maxSubtreeSize)
		}
		*out = append(*out, cur.task)
		children, err := tx.Children(ctx, cur.task.ID)
		if err != nil {
			return err
		}
		push(children, cur.level+1)
	}
	return nil
}

// authorizeTask re-checks the policy once the task's project is known.
func (l *Local) authorizeTask(ctx context.Context, action authz.Action, task *core.Task) error {
	_, err := l.authorize(ctx, action, authz.Resource{
		TenantID:  task.TenantID,
		ProjectID: task.ProjectID,
		OwnerID:   task.CreatorActorID,
	})
	return err
}

// taskByRef resolves a task reference in either form.
func taskByRef(ctx context.Context, tx store.Tx, ref string) (*core.Task, error) {
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		return nil, err
	}
	return tx.GetTask(ctx, parsed)
}

// liveTask loads a task that has not been soft deleted.
func liveTask(ctx context.Context, tx store.Tx, ref core.TaskRef) (*core.Task, error) {
	task, err := tx.GetTask(ctx, ref)
	if err != nil {
		return nil, err
	}
	if task.Deleted() {
		return nil, core.NotFound("task %q", ref.String())
	}
	return task, nil
}

// projectForTask resolves the project a task belongs to, defaulting to the
// tenant's only project when the caller names none.
func projectForTask(ctx context.Context, tx store.Tx, ref string) (*core.Project, error) {
	if ref != "" {
		return tx.GetProject(ctx, ref)
	}
	projects, err := tx.ListProjects(ctx, core.ProjectFilter{Page: core.Page{Limit: 2}})
	if err != nil {
		return nil, err
	}
	switch len(projects) {
	case 0:
		return nil, core.NotFound("this tenant has no project; create one first")
	case 1:
		return &projects[0], nil
	}
	// A tenant seeded on first run has several lists, one of which is the
	// documented zero-config target. Falling back to it is what keeps
	// "tix task add buy milk" working on a fresh install; a tenant whose
	// lists were all named by hand still has to be told which one.
	switch p, err := tx.GetProject(ctx, DefaultProjectKey); {
	case err == nil:
		return p, nil
	case !core.IsKind(err, core.KindNotFound):
		return nil, err
	}
	return nil, core.Invalid("this tenant has several projects; name the one to create the task in")
}

// workflowForTask returns the state machine governing a task.
func workflowForTask(ctx context.Context, tx store.Tx, task *core.Task) (core.WorkflowDefinition, error) {
	project, err := tx.GetProject(ctx, task.ProjectID)
	if err != nil {
		return core.WorkflowDefinition{}, err
	}
	wf, err := tx.GetWorkflowByID(ctx, project.WorkflowID)
	if err != nil {
		return core.WorkflowDefinition{}, err
	}
	return wf.Definition, nil
}

// allowedTargets names the states reachable from one state.
func allowedTargets(def core.WorkflowDefinition, from string) []string {
	out := []string{}
	for _, t := range def.Transitions {
		if t.From == from {
			out = append(out, t.To)
		}
	}
	if len(out) == 0 {
		return []string{"none"}
	}
	return out
}

// checkVersion enforces optimistic concurrency when the caller supplies a version.
func checkVersion(task *core.Task, version int) error {
	if version == 0 || version == task.Version {
		return nil
	}
	return core.Conflict("task %q has moved on since version %d", task.Ref, version).
		WithDetail("current_version", task.Version)
}

// checkLease rejects a change to a task held under someone else's live lease.
func checkLease(ctx context.Context, tx store.Tx, task *core.Task, token string, now time.Time) error {
	if !task.ClaimedAtTime(now) {
		return nil
	}
	if token == "" {
		return core.LeaseExpired("task %q is claimed; supply the lease token", task.Ref)
	}
	ok, err := tx.RenewLease(ctx, task.ID, token, *task.LeaseExpiresAt)
	if err != nil {
		return err
	}
	if !ok {
		return core.LeaseExpired("the lease on task %q is not held by this token", task.Ref)
	}
	return nil
}

// resolveParent validates a new parent reference, refusing to close a cycle.
func resolveParent(ctx context.Context, tx store.Tx, task *core.Task, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", nil
	}
	parent, err := taskByRef(ctx, tx, ref)
	if err != nil {
		return "", err
	}
	if parent.ID == task.ID {
		return "", core.Invalid("a task cannot be its own parent")
	}
	seen := map[string]bool{parent.ID: true}
	for cur := parent; cur.ParentID != ""; {
		next, err := tx.GetTask(ctx, core.TaskRef{ID: cur.ParentID})
		if err != nil {
			return "", err
		}
		if next.ID == task.ID {
			return "", core.Invalid("task %q is already an ancestor of %q", task.Ref, parent.Ref)
		}
		if seen[next.ID] {
			return "", core.Invalid("task %q sits under a parent cycle; repair it before reparenting", parent.Ref)
		}
		seen[next.ID] = true
		cur = next
	}
	return parent.ID, nil
}

// parentLookup answers a task's parent identifier. An importer resolves it
// from links it has not written yet; everything else reads the store.
type parentLookup func(ctx context.Context, taskID string) (string, error)

// closesParentCycle reports whether making parentID the parent of taskID would
// put taskID on its own ancestor chain. A chain longer than maxSubtreeSize
// counts as cyclic: no legitimate hierarchy is that deep, and treating it as
// sound is what lets a walk run without end.
func closesParentCycle(ctx context.Context, taskID, parentID string, parentOf parentLookup) (bool, error) {
	if parentID == "" {
		return false, nil
	}
	if parentID == taskID {
		return true, nil
	}
	seen := map[string]bool{taskID: true}
	cur := parentID
	for range maxSubtreeSize {
		if cur == "" {
			return false, nil
		}
		if seen[cur] {
			return true, nil
		}
		seen[cur] = true
		next, err := parentOf(ctx, cur)
		if err != nil {
			return false, err
		}
		cur = next
	}
	return true, nil
}

// storeParentOf reads a task's parent from the transaction, treating a task
// that does not exist as having none.
func storeParentOf(tx store.Tx) parentLookup {
	return func(ctx context.Context, taskID string) (string, error) {
		task, err := tx.GetTask(ctx, core.TaskRef{ID: taskID})
		if core.IsKind(err, core.KindNotFound) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return task.ParentID, nil
	}
}

// taskSortValue renders the first sort key a cursor carries for a listing. A
// missing due date renders as the sentinel the due-date orderings coalesce to
// in SQL, or the keyset predicate would compare against NULL and drop every
// undated task from the listing.
func taskSortValue(sort string, task core.Task) string {
	switch sort {
	case core.SortUrgency, core.SortPriority:
		return strconv.Itoa(int(task.Priority))
	case core.SortUpdatedAt:
		return sqlb.TimeText(task.UpdatedAt)
	case core.SortDueAt:
		if task.DueAt == nil {
			return sqlb.NoDueSentinel
		}
		return sqlb.TimeText(*task.DueAt)
	case core.SortSeq:
		return strconv.FormatInt(task.Seq, 10)
	case core.SortTitle:
		return task.Title
	default:
		return sqlb.TimeText(task.CreatedAt)
	}
}

// taskSortValue2 renders the second sort key of a compound ordering. It is
// empty for every single-key sort; SortUrgency is the only one today.
func taskSortValue2(sort string, task core.Task) string {
	if sort != core.SortUrgency {
		return ""
	}
	if task.DueAt == nil {
		return sqlb.NoDueSentinel
	}
	return sqlb.TimeText(*task.DueAt)
}

// validateTaskFields checks custom field values against their definitions,
// applying declared defaults and enforcing required fields when creating.
func validateTaskFields(defs []core.FieldDef, values map[string]any, applyDefaults bool) (map[string]any, error) {
	byKey := make(map[string]core.FieldDef, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}
	out := make(map[string]any, len(values))
	for k, v := range values {
		def, ok := byKey[k]
		if !ok {
			return nil, core.Invalid("custom field %q is not defined for this project", k)
		}
		if v == nil {
			continue
		}
		coerced, err := coerceFieldValue(def, v)
		if err != nil {
			return nil, err
		}
		out[k] = coerced
	}
	if !applyDefaults {
		return out, nil
	}
	for _, d := range defs {
		if _, ok := out[d.Key]; ok {
			continue
		}
		if d.Default != nil {
			coerced, err := coerceFieldValue(d, d.Default)
			if err != nil {
				return nil, err
			}
			out[d.Key] = coerced
			continue
		}
		if d.Required {
			return nil, core.Invalid("custom field %q is required", d.Key)
		}
	}
	return out, nil
}

// mergeTaskFields layers an update's field values over the stored ones, where a
// null value clears the field.
func mergeTaskFields(defs []core.FieldDef, current, update map[string]any) (map[string]any, error) {
	validated, err := validateTaskFields(defs, update, false)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(current)+len(validated))
	for k, v := range current {
		out[k] = v
	}
	for k := range update {
		delete(out, k)
	}
	for k, v := range validated {
		out[k] = v
	}
	for _, d := range defs {
		if !d.Required {
			continue
		}
		if _, ok := out[d.Key]; !ok {
			return nil, core.Invalid("custom field %q is required", d.Key)
		}
	}
	return out, nil
}

// coerceFieldValue converts one value to the form its definition declares.
func coerceFieldValue(def core.FieldDef, v any) (any, error) {
	switch def.Type {
	case core.FieldString, core.FieldText, core.FieldActor:
		s, ok := v.(string)
		if !ok {
			return nil, fieldTypeError(def, v)
		}
		return s, nil
	case core.FieldEnum:
		s, ok := v.(string)
		if !ok {
			return nil, fieldTypeError(def, v)
		}
		for _, opt := range def.EnumOptions {
			if opt == s {
				return s, nil
			}
		}
		return nil, core.Invalid("custom field %q accepts only %s", def.Key, strings.Join(def.EnumOptions, ", "))
	case core.FieldInt:
		n, ok := toInt(v)
		if !ok {
			return nil, fieldTypeError(def, v)
		}
		return n, nil
	case core.FieldFloat:
		f, ok := toFloat(v)
		if !ok {
			return nil, fieldTypeError(def, v)
		}
		return f, nil
	case core.FieldBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fieldTypeError(def, v)
		}
		return b, nil
	case core.FieldDate:
		return coerceTime(def, v, "2006-01-02")
	case core.FieldDateTime:
		return coerceTime(def, v, time.RFC3339)
	case core.FieldJSON:
		if _, err := json.Marshal(v); err != nil {
			return nil, core.Invalid("custom field %q must hold valid JSON", def.Key)
		}
		return v, nil
	default:
		return nil, core.Invalid("custom field %q has unsupported type %q", def.Key, def.Type)
	}
}

func coerceTime(def core.FieldDef, v any, layout string) (any, error) {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(layout), nil
	case string:
		if _, err := time.Parse(layout, t); err != nil {
			return nil, core.Invalid("custom field %q must be formatted as %q", def.Key, layout)
		}
		return t, nil
	default:
		return nil, fieldTypeError(def, v)
	}
}

func fieldTypeError(def core.FieldDef, v any) error {
	return core.Invalid("custom field %q expects a %s value, got %T", def.Key, def.Type, v)
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if n != float64(int64(n)) {
			return 0, false
		}
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// resolveAssignee turns an assignee reference into an actor identifier.
//
// Assignment deliberately accepts an identifier this tenant cannot resolve,
// because actor identifiers are globally unique and a task may name someone
// from elsewhere; the interface renders an unresolvable one as a generated
// name rather than leaking a handle. So an identifier-shaped value is passed
// through untouched, exactly as before.
//
// Anything else is a handle, and used to reach the database as if it were an
// identifier, where it failed a foreign key and surfaced the constraint error
// verbatim. A handle now resolves, and an unknown one is reported as not found
// rather than as a schema error.
func resolveAssignee(ctx context.Context, tx store.Tx, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ref, nil
	}
	// A handle wins when it resolves here. Only a value with the exact shape of
	// a generated identifier falls through unresolved, which is what keeps an
	// actor from another tenant assignable: those never resolve locally,
	// because actors are tenant scoped. A name that resolves nowhere and is not
	// identifier-shaped is a mistake, and is reported as one.
	if actor, err := lookupActor(ctx, tx, ref); err == nil {
		return actor.ID, nil
	}
	if id.Valid(ref) {
		return ref, nil
	}
	return "", core.NotFound("no actor with handle or id %q", ref)
}
