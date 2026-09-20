package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

// CommentRequest carries a comment body.
type CommentRequest struct {
	Body string `json:"body"`
}

// TagRequest carries a tag name.
type TagRequest struct {
	Name string `json:"name"`
}

// DependencyRequest names the task a task waits for.
type DependencyRequest struct {
	DependsOn string `json:"depends_on"`
}

// registerTaskRoutes binds tasks and everything attached to them.
func (rt *Router) registerTaskRoutes() {
	rt.mux.HandleFunc("GET "+RouteTasks, rt.handleListTasks)
	rt.mux.HandleFunc("POST "+RouteTasks, rt.handleCreateTask)
	rt.mux.HandleFunc("GET "+RouteTask, rt.handleGetTask)
	rt.mux.HandleFunc("PATCH "+RouteTask, rt.handleUpdateTask)
	rt.mux.HandleFunc("DELETE "+RouteTask, rt.handleDeleteTask)
	rt.mux.HandleFunc("POST "+RouteTaskTransition, rt.handleTransitionTask)
	rt.mux.HandleFunc("POST "+RouteTaskRestore, rt.handleRestoreTask)
	rt.mux.HandleFunc("GET "+RouteTaskTree, rt.handleTaskTree)

	rt.mux.HandleFunc("GET "+RouteTaskDeps, rt.handleListDependencies)
	rt.mux.HandleFunc("POST "+RouteTaskDeps, rt.handleAddDependency)
	rt.mux.HandleFunc("DELETE "+RouteTaskDep, rt.handleRemoveDependency)

	rt.mux.HandleFunc("GET "+RouteTaskLabels, rt.handleTaskLabels)
	rt.mux.HandleFunc("POST "+RouteTaskLabels, rt.handleAddLabel)
	rt.mux.HandleFunc("DELETE "+RouteTaskLabel, rt.handleRemoveLabel)
	rt.mux.HandleFunc("GET "+RouteLabels, rt.handleListLabels)

	rt.mux.HandleFunc("GET "+RouteTaskComments, rt.handleListComments)
	rt.mux.HandleFunc("POST "+RouteTaskComments, rt.handleAddComment)
	rt.mux.HandleFunc("PATCH "+RouteComment, rt.handleEditComment)
	rt.mux.HandleFunc("DELETE "+RouteComment, rt.handleDeleteComment)

	rt.mux.HandleFunc("GET "+RouteTaskArtifacts, rt.handleListArtifacts)
	rt.mux.HandleFunc("PUT "+RouteTaskArtifacts, rt.handlePutArtifact)
	rt.mux.HandleFunc("GET "+RouteTaskAudit, rt.handleTaskAudit)
}

// handleListTasks returns a page of tasks.
func (rt *Router) handleListTasks(w http.ResponseWriter, r *http.Request) {
	f, err := taskFilterFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	page, err := rt.cfg.Service.ListTasks(r.Context(), f)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, page.Tasks, page.NextCursor)
}

// taskFilterFrom builds a task filter from the query string.
func taskFilterFrom(r *http.Request) (core.TaskFilter, error) {
	page, err := pageFrom(r)
	if err != nil {
		return core.TaskFilter{}, err
	}
	q := r.URL.Query()
	f := core.TaskFilter{
		ProjectKeys:    q["project"],
		ProjectIDs:     q["project_id"],
		Statuses:       q["status"],
		Tags:           q["tag"],
		AssigneeIDs:    q["assignee"],
		CreatorIDs:     q["creator"],
		ClaimedBy:      q["claimed_by"],
		ParentID:       q.Get("parent_id"),
		ParentIsNull:   boolParam(r, "root_only"),
		Claimed:        triState(q.Get("claimed")),
		Blocked:        triState(q.Get("blocked")),
		Query:          q.Get("q"),
		IncludeDeleted: boolParam(r, "include_deleted"),
		Page:           page,
	}
	for _, raw := range q["priority"] {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return core.TaskFilter{}, core.Invalid("priority %q is not a number", raw)
		}
		f.Priorities = append(f.Priorities, core.Priority(n))
	}
	if f.DueBefore, err = timeParam(q.Get("due_before")); err != nil {
		return core.TaskFilter{}, err
	}
	if f.DueAfter, err = timeParam(q.Get("due_after")); err != nil {
		return core.TaskFilter{}, err
	}
	if f.CustomFields, err = customFieldsFrom(q); err != nil {
		return core.TaskFilter{}, err
	}
	return f.Validate()
}

// FieldFilterPrefix marks a query parameter that filters on a custom field, as
// in "?field.severity=high".
const FieldFilterPrefix = "field."

// FieldFilterParam carries custom field filters as one typed JSON object, which
// is how a machine client keeps a numeric field numeric.
const FieldFilterParam = "custom_fields"

// customFieldsFrom reads custom field filters from the query string. A key given
// both as an object entry and as a shorthand parameter takes the shorthand.
func customFieldsFrom(q url.Values) (map[string]any, error) {
	out := map[string]any{}
	if raw := strings.TrimSpace(q.Get(FieldFilterParam)); raw != "" {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return nil, core.Invalid("%s must be a JSON object", FieldFilterParam)
		}
		for key, value := range decoded {
			if err := putFieldFilter(out, key, value); err != nil {
				return nil, err
			}
		}
	}
	for param, values := range q {
		key, ok := strings.CutPrefix(param, FieldFilterPrefix)
		if !ok {
			continue
		}
		if len(values) != 1 {
			return nil, core.Invalid("custom field filter %q takes one value", key)
		}
		if err := putFieldFilter(out, key, fieldFilterValue(values[0])); err != nil {
			return nil, err
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// putFieldFilter records one filter, refusing a key the store cannot address
// and a value it cannot compare.
func putFieldFilter(out map[string]any, key string, value any) error {
	if !validFieldKey(key) {
		return core.Invalid("custom field key %q must hold only letters, digits, underscores and dashes", key)
	}
	switch value.(type) {
	case string, bool, float64:
		out[key] = value
		return nil
	default:
		return core.Invalid("custom field filter %q takes a string, number or boolean", key)
	}
}

// fieldFilterValue types a shorthand value as the JSON scalar it spells, so that
// a number filters a number and a quoted number filters a string. Anything that
// is not JSON at all stays the string it was typed as.
func fieldFilterValue(raw string) any {
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return raw
	}
	return decoded
}

// validFieldKey mirrors what the stores accept in a JSON path.
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

// triState reads an optional boolean filter.
func triState(v string) core.TriState {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes":
		return core.Yes
	case "false", "0", "no":
		return core.No
	default:
		return core.Either
	}
}

// timeParam reads an optional RFC 3339 timestamp.
func timeParam(v string) (*time.Time, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, core.Invalid("timestamp %q must be RFC 3339", v)
	}
	return &t, nil
}

// handleCreateTask creates a task.
func (rt *Router) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var in core.CreateTaskInput
	if !readJSON(w, r, &in) {
		return
	}
	task, err := rt.cfg.Service.CreateTask(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, task)
}

// handleGetTask returns one task, addressed by identifier or human ref.
func (rt *Router) handleGetTask(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	task, err := rt.cfg.Service.GetTask(r.Context(), ref)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, task)
}

// handleUpdateTask changes one task.
func (rt *Router) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in core.UpdateTaskInput
	if !readJSON(w, r, &in) {
		return
	}
	task, err := rt.cfg.Service.UpdateTask(r.Context(), ref, in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, task)
}

// handleDeleteTask soft deletes a task, or removes it when hard is asked for.
func (rt *Router) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	in := core.DeleteTaskInput{Hard: boolParam(r, "hard"), Cascade: boolParam(r, "cascade")}
	if err := rt.cfg.Service.DeleteTask(r.Context(), ref, in); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleTransitionTask moves a task to a new status.
func (rt *Router) handleTransitionTask(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in core.TransitionInput
	if !readJSON(w, r, &in) {
		return
	}
	task, err := rt.cfg.Service.TransitionTask(r.Context(), ref, in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, task)
}

// handleRestoreTask undeletes a task.
func (rt *Router) handleRestoreTask(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	task, err := rt.cfg.Service.RestoreTask(r.Context(), ref)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, task)
}

// handleTaskTree returns a task and its descendants.
func (rt *Router) handleTaskTree(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	depth := 0
	if raw := r.URL.Query().Get("depth"); raw != "" {
		depth, err = strconv.Atoi(raw)
		if err != nil {
			WriteError(w, core.Invalid("depth %q is not a number", raw))
			return
		}
	}
	tasks, err := rt.cfg.Service.TaskTree(r.Context(), ref, depth)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, tasks, "")
}

// handleListDependencies returns the tasks a task waits for.
func (rt *Router) handleListDependencies(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	deps, err := rt.cfg.Service.ListDependencies(r.Context(), ref)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, deps, "")
}

// handleAddDependency makes a task wait for another.
func (rt *Router) handleAddDependency(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in DependencyRequest
	if !readJSON(w, r, &in) {
		return
	}
	dep, err := core.ParseTaskRef(in.DependsOn)
	if err != nil {
		WriteError(w, err)
		return
	}
	if err := rt.cfg.Service.AddDependency(r.Context(), ref, dep); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleRemoveDependency drops a dependency edge.
func (rt *Router) handleRemoveDependency(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	dep, err := core.ParseTaskRef(r.PathValue("dep"))
	if err != nil {
		WriteError(w, err)
		return
	}
	if err := rt.cfg.Service.RemoveDependency(r.Context(), ref, dep); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleTaskLabels returns the tags attached to one task.
func (rt *Router) handleTaskLabels(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	task, err := rt.cfg.Service.GetTask(r.Context(), ref)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, task.Tags, "")
}

// handleAddLabel attaches a tag to a task.
func (rt *Router) handleAddLabel(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in TagRequest
	if !readJSON(w, r, &in) {
		return
	}
	if err := rt.cfg.Service.AddTag(r.Context(), ref, in.Name); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleRemoveLabel detaches a tag from a task.
func (rt *Router) handleRemoveLabel(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	if err := rt.cfg.Service.RemoveTag(r.Context(), ref, r.PathValue("name")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListLabels returns every tag in the resolved tenant.
func (rt *Router) handleListLabels(w http.ResponseWriter, r *http.Request) {
	tags, err := rt.cfg.Service.ListTags(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, tags, "")
}

// handleListComments returns a task's comments.
func (rt *Router) handleListComments(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	comments, err := rt.cfg.Service.ListComments(r.Context(), ref)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, comments, "")
}

// handleAddComment adds a comment to a task.
func (rt *Router) handleAddComment(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in CommentRequest
	if !readJSON(w, r, &in) {
		return
	}
	comment, err := rt.cfg.Service.AddComment(r.Context(), ref, in.Body)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, comment)
}

// handleEditComment rewrites a comment.
func (rt *Router) handleEditComment(w http.ResponseWriter, r *http.Request) {
	var in CommentRequest
	if !readJSON(w, r, &in) {
		return
	}
	comment, err := rt.cfg.Service.EditComment(r.Context(), r.PathValue("id"), in.Body)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, comment)
}

// handleDeleteComment removes a comment.
func (rt *Router) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteComment(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListArtifacts returns a task's artifacts.
func (rt *Router) handleListArtifacts(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	artifacts, err := rt.cfg.Service.ListArtifacts(r.Context(), ref)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, artifacts, "")
}

// handlePutArtifact records structured output on a task.
func (rt *Router) handlePutArtifact(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in core.ArtifactInput
	if !readJSON(w, r, &in) {
		return
	}
	artifact, err := rt.cfg.Service.PutArtifact(r.Context(), ref, in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, artifact)
}

// handleTaskAudit returns the audit trail of one task.
func (rt *Router) handleTaskAudit(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	task, err := rt.cfg.Service.GetTask(r.Context(), ref)
	if err != nil {
		WriteError(w, err)
		return
	}
	f, err := auditFilterFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	f.SubjectType = "task"
	f.SubjectID = task.ID
	entries, next, err := rt.cfg.Service.ListAudit(r.Context(), f)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, entries, next)
}
