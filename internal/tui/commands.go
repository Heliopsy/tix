package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
)

// loadProjects lists the projects the caller can reach.
func (m Model) loadProjects() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		projects, _, err := svc.ListProjects(ctx, core.ProjectFilter{})
		if err != nil {
			return errMsg{err: err}
		}
		return projectsMsg{projects: projects}
	}
}

// loadBoard opens a project, its workflow and its tasks.
func (m Model) loadBoard(p core.Project) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, filter := m.svc, m.ctx, m.taskFilter(p.Key)
	return func() tea.Msg {
		def, err := workflowOf(ctx, svc, p)
		if err != nil {
			return errMsg{err: err}
		}
		page, err := svc.ListTasks(ctx, filter)
		if err != nil {
			return errMsg{err: err}
		}
		return boardMsg{project: p, workflow: def, tasks: page.Tasks}
	}
}

// workflowOf finds the workflow definition a project runs on.
func workflowOf(ctx context.Context, svc core.Service, p core.Project) (*core.WorkflowDefinition, error) {
	flows, err := svc.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range flows {
		if w.ID == p.WorkflowID {
			def := w.Definition
			return &def, nil
		}
	}
	return nil, core.NotFound("project %q references workflow %q, which is not readable", p.Key, p.WorkflowID)
}

// loadTasks refreshes the open project's tasks under the active filter.
func (m Model) loadTasks() tea.Cmd {
	if m.svc == nil || m.project.ID == "" {
		return nil
	}
	svc, ctx, filter := m.svc, m.ctx, m.taskFilter(m.project.Key)
	return func() tea.Msg {
		page, err := svc.ListTasks(ctx, filter)
		if err != nil {
			return errMsg{err: err}
		}
		return tasksMsg{tasks: page.Tasks}
	}
}

// taskFilter scopes the active filter to one project.
func (m Model) taskFilter(key string) core.TaskFilter {
	f := m.filter
	if len(f.ProjectKeys) == 0 {
		f.ProjectKeys = []string{key}
	}
	f.Page.Limit = core.MaxPageLimit
	f.Page.Cursor = ""
	return f
}

// loadDetail gathers everything the detail view shows about one task.
func (m Model) loadDetail() tea.Cmd {
	task, ok := TaskAt(m.columns, m.sel)
	if !ok {
		return nil
	}
	return m.detailFor(core.TaskRef{ID: task.ID})
}

// detailFor gathers everything the detail view shows about one task.
func (m Model) detailFor(ref core.TaskRef) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, projectRef := m.svc, m.ctx, m.project.Key
	return func() tea.Msg {
		full, err := svc.GetTask(ctx, ref)
		if err != nil {
			return errMsg{err: err}
		}
		out := detailMsg{task: *full}
		if tree, err := svc.TaskTree(ctx, ref, 1); err == nil {
			out.subtasks = childrenOf(tree, full.ID)
		}
		out.deps, _ = svc.ListDependencies(ctx, ref)
		out.comments, _ = svc.ListComments(ctx, ref)
		out.fieldDefs, _ = svc.ListFieldDefs(ctx, projectRef)
		out.artifacts, _ = svc.ListArtifacts(ctx, ref)
		out.actors = resolveActors(ctx, svc, actorIDs(*full, out.comments))
		return out
	}
}

// actorIDs collects the distinct, non-empty actor identifiers a detail view
// needs a handle for.
func actorIDs(t core.Task, comments []core.Comment) []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	add(t.AssigneeActorID)
	add(t.CreatorActorID)
	add(t.ClaimedByActorID)
	for _, c := range comments {
		add(c.AuthorActorID)
	}
	return ids
}

// resolveActors looks up each actor's handle. A lookup that fails, or an
// actor the tenant no longer has, is simply left out: the detail view falls
// back to a short identifier for that one actor rather than losing the rest
// of the task over one bad lookup.
func resolveActors(ctx context.Context, svc core.Service, ids []string) map[string]string {
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		actor, err := svc.GetActor(ctx, id)
		if err != nil || actor == nil || actor.Handle == "" {
			continue
		}
		out[id] = actor.Handle
	}
	return out
}

// childrenOf keeps only the direct children of a task from a tree listing.
func childrenOf(tree []core.Task, parent string) []core.Task {
	out := make([]core.Task, 0, len(tree))
	for _, t := range tree {
		if t.ParentID == parent {
			out = append(out, t)
		}
	}
	return out
}

// claim takes a lease on the selected task.
func (m Model) claim() tea.Cmd {
	task, ok := m.selectedTask()
	if !ok || m.svc == nil {
		return nil
	}
	svc, ctx, ref, label := m.svc, m.ctx, core.TaskRef{ID: task.ID}, task.Ref
	return func() tea.Msg {
		claim, err := svc.ClaimTask(ctx, ref, core.ClaimInput{})
		if err != nil {
			return actionMsg{kind: actionClaim, ref: ref, label: label, err: err}
		}
		return actionMsg{kind: actionClaim, ref: ref, label: label, token: claim.LeaseToken}
	}
}

// releaseCmd gives up the lease this session holds on a task.
func (m Model) releaseCmd(task core.Task) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, ref, token, label := m.svc, m.ctx, core.TaskRef{ID: task.ID}, m.leases[task.ID], task.Ref
	return func() tea.Msg {
		err := svc.ReleaseLease(ctx, ref, token, core.ReleaseInput{})
		return actionMsg{kind: actionRelease, ref: ref, label: label, err: err}
	}
}

// transition moves the selected task to another workflow state.
func (m Model) transition(to string) tea.Cmd {
	task, ok := m.selectedTask()
	if !ok || m.svc == nil {
		return nil
	}
	svc, ctx, ref := m.svc, m.ctx, core.TaskRef{ID: task.ID}
	in := core.TransitionInput{To: to, LeaseToken: m.leases[task.ID]}
	return func() tea.Msg {
		_, err := svc.TransitionTask(ctx, ref, in)
		return actionMsg{kind: actionTransition, ref: ref, err: err}
	}
}

// subscribe follows the event stream from a sequence, reporting every event.
func (m Model) subscribe(since int64) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		events, err := svc.Subscribe(ctx, core.EventFilter{SinceSeq: since})
		if err != nil {
			return streamMsg{err: err}
		}
		return streamMsg{connected: true, events: events}
	}
}

// nextEvent waits for one more event from an open subscription.
func nextEvent(events <-chan core.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return streamMsg{}
		}
		return eventMsg{event: event}
	}
}

// reconnect waits before opening a dropped subscription again.
func reconnect() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return reconnectMsg{} })
}

// createTask adds a task to the open project.
func (m Model) createTask(title string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	in := core.CreateTaskInput{ProjectRef: m.project.Key, Title: title}
	return func() tea.Msg {
		task, err := svc.CreateTask(ctx, in)
		if err != nil {
			return actionMsg{kind: actionCreate, err: err}
		}
		return actionMsg{kind: actionCreate, ref: core.TaskRef{ID: task.ID}, label: task.Ref}
	}
}

// updateTask applies one field change to a task.
func (m Model) updateTask(task core.Task, in core.UpdateTaskInput) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, ref, label := m.svc, m.ctx, core.TaskRef{ID: task.ID}, task.Ref
	return func() tea.Msg {
		_, err := svc.UpdateTask(ctx, ref, in)
		return actionMsg{kind: actionUpdate, ref: ref, label: label, err: err}
	}
}

// comment records a comment on a task.
func (m Model) comment(task core.Task, body string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, ref, label := m.svc, m.ctx, core.TaskRef{ID: task.ID}, task.Ref
	return func() tea.Msg {
		_, err := svc.AddComment(ctx, ref, body)
		return actionMsg{kind: actionComment, ref: ref, label: label, err: err}
	}
}

// tag attaches a tag to a task.
func (m Model) tag(task core.Task, name string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, ref, label := m.svc, m.ctx, core.TaskRef{ID: task.ID}, task.Ref
	return func() tea.Msg {
		return actionMsg{kind: actionTag, ref: ref, label: label, err: svc.AddTag(ctx, ref, name)}
	}
}

// untag detaches a tag from a task.
func (m Model) untag(task core.Task, name string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, ref, label := m.svc, m.ctx, core.TaskRef{ID: task.ID}, task.Ref
	return func() tea.Msg {
		return actionMsg{kind: actionUntag, ref: ref, label: label, err: svc.RemoveTag(ctx, ref, name)}
	}
}

// depend records that a task waits on another, named the way the CLI names it.
func (m Model) depend(task core.Task, on string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, ref, label := m.svc, m.ctx, core.TaskRef{ID: task.ID}, task.Ref
	target, err := core.ParseTaskRef(on)
	if err != nil {
		return func() tea.Msg { return actionMsg{kind: actionDepend, ref: ref, label: label, err: err} }
	}
	return func() tea.Msg {
		return actionMsg{kind: actionDepend, ref: ref, label: label, err: svc.AddDependency(ctx, ref, target)}
	}
}

// reloadDetail fetches an open task again after it has been changed.
func (m Model) reloadDetail(task core.Task) tea.Cmd {
	return m.detailFor(core.TaskRef{ID: task.ID})
}

// claimNext takes a lease on the next task the queue offers.
func (m Model) claimNext() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	in := core.ClaimNextInput{ProjectRefs: []string{m.project.Key}}
	return func() tea.Msg {
		claim, err := svc.ClaimNext(ctx, in)
		if err != nil {
			return actionMsg{kind: actionClaim, err: err}
		}
		ref, label := core.TaskRef{}, ""
		if claim.Task != nil {
			ref.ID, label = claim.Task.ID, claim.Task.Ref
		}
		return actionMsg{kind: actionClaim, ref: ref, label: label, token: claim.LeaseToken}
	}
}

// renew extends the lease this session holds on a task.
func (m Model) renew(task core.Task) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, ref, token, label := m.svc, m.ctx, core.TaskRef{ID: task.ID}, m.leases[task.ID], task.Ref
	return func() tea.Msg {
		_, err := svc.RenewLease(ctx, ref, token, 0)
		return actionMsg{kind: actionRenew, ref: ref, label: label, err: err}
	}
}

// createProject adds a project from the project list.
func (m Model) createProject(key, name string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	in := core.CreateProjectInput{Key: key, Name: name}
	return func() tea.Msg {
		project, err := svc.CreateProject(ctx, in)
		if err != nil {
			return actionMsg{kind: actionNewProject, err: err}
		}
		return actionMsg{kind: actionNewProject, label: project.Key}
	}
}
