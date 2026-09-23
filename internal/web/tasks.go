// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// priorityChoice is one option of the priority control.
type priorityChoice struct {
	Value core.Priority
	Label string
}

// priorityChoices is the priority vocabulary the forms offer.
var priorityChoices = []priorityChoice{
	{core.PriorityHighest, "highest"},
	{core.PriorityHigh, "high"},
	{core.PriorityNormal, "normal"},
	{core.PriorityLow, "low"},
	{core.PriorityLowest, "lowest"},
}

// tasksView is what the filterable task list renders.
type tasksView struct {
	Tasks      []core.Task
	Projects   []core.Project
	Tags       []core.Tag
	Priorities []priorityChoice
	SortFields []string
	Query      string
	Sort       string
	NextCursor string

	// Summary is the one line under the heading. A tracker that only ever
	// shows rows tells you nothing about the shape of your day.
	Summary taskSummary

	// CompleteState maps a project id to the state a one click complete moves
	// its tasks into, so the list can offer a tick box without the reader
	// knowing the project's workflow.
	CompleteState map[string]string

	// Accent maps a project id to the stripe and icon its rows carry, so a
	// mixed list can be read one project at a time.
	Accent map[string]projectAccent

	// Visibility is the visibility control, and Hidden how many projects are
	// put away. Filtered is set when an explicit project: filter overrode
	// that choice, so a reader is never shown a shorter task list without
	// being told which rule produced it.
	Visibility []projectChoice
	Hidden     int
	Filtered   bool
	Empty      bool
}

// projectAccent is how one project marks its rows apart from another's.
type projectAccent struct {
	Key   string
	Name  string
	Color core.ProjectColor
	Icon  string
}

// projectAccents maps each project to its row marking.
func projectAccents(projects []core.Project) map[string]projectAccent {
	out := make(map[string]projectAccent, len(projects))
	for _, p := range projects {
		out[p.ID] = projectAccent{Key: p.Key, Name: p.Name, Color: p.Color, Icon: p.Icon}
	}
	return out
}

// taskSummary counts what is on the page, so the heading can say something.
// Projects is how many distinct lists the rows come from, which is the count
// that tells a reader whether they are looking at one project's work or at
// everything at once -- the same question the visibility control answers, so
// the two belong on screen together.
type taskSummary struct {
	Open     int
	Held     int
	Done     int
	Blocked  int
	Projects int
}

// Total is how many tasks the page carries.
func (s taskSummary) Total() int { return s.Open + s.Done }

// AllDone reports whether everything on the page is finished.
func (s taskSummary) AllDone() bool { return s.Total() > 0 && s.Done == s.Total() }

// summarise counts the page by what a reader cares about: what is left, what
// is moving, and what an agent is holding right now.
func summarise(tasks []core.Task, complete map[string]string, now time.Time) taskSummary {
	var out taskSummary
	seen := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		if t.Status == complete[t.ProjectID] {
			out.Done++
		} else {
			out.Open++
		}
		if t.Blocked {
			out.Blocked++
		}
		if t.ClaimedByActorID != "" && t.LeaseExpiresAt != nil && t.LeaseExpiresAt.After(now) {
			out.Held++
		}
		if t.ProjectID != "" && !seen[t.ProjectID] {
			seen[t.ProjectID] = true
			out.Projects++
		}
	}
	return out
}

// showTasks renders the filterable task list.
func (h *handler) showTasks(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()
	filter, err := ParseFilter(query.Get("q"))
	if err != nil {
		return err
	}
	filter.Page = core.Page{
		Cursor:    query.Get("cursor"),
		Sort:      query.Get("sort"),
		Direction: core.Ascending,
	}
	if filter.Page.Sort == "" {
		filter.Page.Sort = core.SortUrgency
	}
	projects, _, err := h.svc.ListProjects(r.Context(), core.ProjectFilter{})
	if err != nil {
		return err
	}
	visibility := projectChoices(projects, hiddenProjects(r))
	filtered := len(filter.ProjectKeys) > 0
	hidden := hiddenCount(visibility)
	// An explicit project: filter is the reader asking for that project by
	// name, so it wins over what the visibility control put away.
	if !filtered && hidden > 0 {
		filter.ProjectKeys = shownKeys(visibility)
	}
	var page core.TaskPage
	if len(filter.ProjectKeys) > 0 || hidden == 0 || len(projects) == 0 {
		if page, err = h.svc.ListTasks(r.Context(), filter); err != nil {
			return err
		}
	}
	tags, err := h.svc.ListTags(r.Context())
	if err != nil {
		return err
	}
	complete, err := h.completeStates(r, projects)
	if err != nil {
		return err
	}
	return h.render(w, r, "tasks.html", "Tasks", tasksView{
		Tasks:         page.Tasks,
		Projects:      projects,
		Tags:          tags,
		Priorities:    priorityChoices,
		SortFields:    core.TaskSortFields,
		Query:         query.Get("q"),
		Sort:          filter.Page.Sort,
		NextCursor:    page.NextCursor,
		CompleteState: complete,
		Accent:        projectAccents(projects),
		Summary:       summarise(page.Tasks, complete, time.Now()),
		Visibility:    visibility,
		Hidden:        hidden,
		Filtered:      filtered && hidden > 0,
		Empty:         hidden > 0 && hidden == len(visibility),
	})
}

// completeStates resolves, per project, the state that finishing a task moves
// it to: the first terminal state in the done category, or failing that the
// first terminal state at all.
func (h *handler) completeStates(r *http.Request, projects []core.Project) (map[string]string, error) {
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return nil, err
	}
	byID := make(map[string]core.Workflow, len(workflows))
	for _, w := range workflows {
		byID[w.ID] = w
	}
	out := make(map[string]string, len(projects))
	for _, p := range projects {
		w, ok := byID[p.WorkflowID]
		if !ok {
			continue
		}
		if key := doneState(w.Definition); key != "" {
			out[p.ID] = key
		}
	}
	return out, nil
}

// doneState picks the state a completed task belongs in.
func doneState(d core.WorkflowDefinition) string {
	fallback := ""
	for _, st := range d.States {
		if !st.Terminal {
			continue
		}
		if st.Category == core.CategoryDone {
			return st.Key
		}
		if fallback == "" {
			fallback = st.Key
		}
	}
	return fallback
}

// createTask creates a task from the list screen's form.
func (h *handler) createTask(w http.ResponseWriter, r *http.Request) error {
	priority, err := priorityField(r, "priority")
	if err != nil {
		return err
	}
	in := core.CreateTaskInput{
		ProjectRef: field(r, "project_ref"),
		Title:      field(r, "title"),
		Body:       field(r, "body"),
		Priority:   priority,
		Tags:       fieldList(r, "tags"),
	}
	task, err := h.svc.CreateTask(r.Context(), in)
	if err != nil {
		return err
	}
	redirect(w, r, RouteTasks+"/"+task.Ref, "task created")
	return nil
}

// completeTask finishes a task in one click. A workflow rarely allows a jump
// straight from the opening state to a terminal one, so this walks the
// shortest legal path and applies each transition. Doing it any other way
// would mean the tick box works only for tasks that happen to be one step
// from done.
func (h *handler) completeTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	task, err := h.svc.GetTask(r.Context(), ref)
	if err != nil {
		return err
	}
	// The project and the workflow are addressed by key on the service, while
	// a task carries their identifiers, so both are matched by id here.
	project, err := h.projectByID(r, task.ProjectID)
	if err != nil {
		return err
	}
	flow, err := h.workflowOf(r, project)
	if err != nil {
		return err
	}
	// The control is a tick box, so it toggles: a finished task reopens into
	// the workflow's starting state. Treating it as complete-only left the
	// click on an already finished row doing nothing at all.
	target, done := doneState(flow.Definition), "completed "
	if flow.Definition.IsTerminal(task.Status) {
		target, done = flow.Definition.Initial, "reopened "
	}
	if target == "" {
		return core.Precondition("workflow %q has no state to move %q into", flow.Key, task.Ref)
	}
	path := pathBetween(flow.Definition, task.Status, target)
	if path == nil {
		return core.Precondition("no route from %q to %q in workflow %q", task.Status, target, flow.Key)
	}
	for _, next := range path {
		if _, err := h.svc.TransitionTask(r.Context(), ref, core.TransitionInput{To: next}); err != nil {
			return err
		}
	}
	redirect(w, r, RouteTasks, done+task.Ref)
	return nil
}

// projectByID finds a project by identifier. The service addresses a project
// by key, while a task carries the identifier.
func (h *handler) projectByID(r *http.Request, id string) (*core.Project, error) {
	projects, _, err := h.svc.ListProjects(r.Context(), core.ProjectFilter{})
	if err != nil {
		return nil, err
	}
	for i := range projects {
		if projects[i].ID == id {
			return &projects[i], nil
		}
	}
	return nil, core.NotFound("project %q", id)
}

// pathBetween returns the states to move through to get from one state to
// another, excluding the starting state. It returns nil when no route exists,
// and an empty slice when the task is already there.
func pathBetween(d core.WorkflowDefinition, from, to string) []string {
	if from == to {
		return []string{}
	}
	next := map[string][]string{}
	for _, t := range d.Transitions {
		next[t.From] = append(next[t.From], t.To)
	}
	prev := map[string]string{from: ""}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, n := range next[cur] {
			if _, seen := prev[n]; seen {
				continue
			}
			prev[n] = cur
			if n == to {
				return walkBack(prev, from, to)
			}
			queue = append(queue, n)
		}
	}
	return nil
}

// walkBack turns a breadth-first predecessor map into a forward path.
func walkBack(prev map[string]string, from, to string) []string {
	var out []string
	for at := to; at != from; at = prev[at] {
		out = append(out, at)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
