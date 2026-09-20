package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thereisnotime/tix/internal/core"
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
	if !ok || m.svc == nil {
		return nil
	}
	svc, ctx, ref, projectRef := m.svc, m.ctx, core.TaskRef{ID: task.ID}, m.project.Key
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
		return out
	}
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
	svc, ctx, ref := m.svc, m.ctx, core.TaskRef{ID: task.ID}
	return func() tea.Msg {
		claim, err := svc.ClaimTask(ctx, ref, core.ClaimInput{})
		if err != nil {
			return actionMsg{kind: actionClaim, ref: ref, err: err}
		}
		return actionMsg{kind: actionClaim, ref: ref, token: claim.LeaseToken}
	}
}

// releaseCmd gives up the lease this session holds on a task.
func (m Model) releaseCmd(task core.Task) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, ref, token := m.svc, m.ctx, core.TaskRef{ID: task.ID}, m.leases[task.ID]
	return func() tea.Msg {
		err := svc.ReleaseLease(ctx, ref, token, core.ReleaseInput{})
		return actionMsg{kind: actionRelease, ref: ref, err: err}
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
