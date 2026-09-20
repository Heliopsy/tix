package web

import (
	"net/http"

	"github.com/thereisnotime/tix/internal/core"
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
		filter.Page.Sort = core.SortCreatedAt
	}
	page, err := h.svc.ListTasks(r.Context(), filter)
	if err != nil {
		return err
	}
	projects, _, err := h.svc.ListProjects(r.Context(), core.ProjectFilter{})
	if err != nil {
		return err
	}
	tags, err := h.svc.ListTags(r.Context())
	if err != nil {
		return err
	}
	return h.render(w, r, "tasks.html", "Tasks", tasksView{
		Tasks:      page.Tasks,
		Projects:   projects,
		Tags:       tags,
		Priorities: priorityChoices,
		SortFields: core.TaskSortFields,
		Query:      query.Get("q"),
		Sort:       filter.Page.Sort,
		NextCursor: page.NextCursor,
	})
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
