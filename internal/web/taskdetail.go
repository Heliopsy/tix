// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// taskRoutes are the task list, detail and attachment screens.
func (h *handler) taskRoutes() []route {
	return []route{
		get(RouteTasks, "tasks.html", h.showTasks, "ListTasks", "ListProjects", "ListTags"),
		post(RouteTasks, h.createTask, "CreateTask"),
		get(RouteTask, "task.html", h.showTask, "GetTask", "TaskTree",
			"ListDependencies", "ListComments", "ListArtifacts", "ListFieldDefs", "GetActor"),
		post(RouteTask, h.updateTask, "UpdateTask"),
		post(RouteTaskMove, h.transitionTask, "TransitionTask"),
		post(RouteTaskComplete, h.completeTask, "TransitionTask"),
		post(RouteTaskDelete, h.deleteTask, "DeleteTask"),
		post(RouteTaskRestore, h.restoreTask, "RestoreTask"),
		post(RouteTaskDeps, h.addDependency, "AddDependency"),
		post(RouteTaskDepDel, h.removeDependency, "RemoveDependency"),
		post(RouteTaskTags, h.addTag, "AddTag"),
		post(RouteTaskTagDel, h.removeTag, "RemoveTag"),
		post(RouteTaskComments, h.addComment, "AddComment"),
		post(RouteCommentEdit, h.editComment, "EditComment"),
		post(RouteCommentDel, h.deleteComment, "DeleteComment"),
		post(RouteTaskArts, h.putArtifact, "PutArtifact"),
	}
}

// fieldValue pairs a custom field definition with the task's value for it.
type fieldValue struct {
	Def   core.FieldDef
	Value string
}

// historyRow is one audit entry with the fields it changed.
type historyRow struct {
	Entry   core.AuditEntry
	Changed []string
}

// taskView is what the task detail screen renders.
type taskView struct {
	Task          core.Task
	ProjectKey    string
	Fields        []fieldValue
	Subtasks      []core.Task
	Dependencies  []core.Dependency
	Comments      []core.Comment
	Artifacts     []core.Artifact
	History       []historyGroup
	Names         actorNames
	Targets       []core.State
	Priorities    []priorityChoice
	ArtifactKinds []core.ArtifactKind
	CanWrite      bool
	CanComment    bool
	CanDelete     bool
	CanAudit      bool
}

// actorIDs lists every actor the screen names, so one pass over the directory
// covers the whole page rather than one lookup per element.
func (v taskView) actorIDs() []string {
	out := []string{v.Task.CreatorActorID, v.Task.AssigneeActorID}
	for _, c := range v.Comments {
		out = append(out, c.AuthorActorID)
	}
	for _, g := range v.History {
		for _, h := range g.Hops {
			out = append(out, h.Entry.ActorID)
		}
	}
	return out
}

// showTask renders one task with every section attached to it.
func (h *handler) showTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := core.ParseTaskRef(r.PathValue("ref"))
	if err != nil {
		return err
	}
	task, err := h.svc.GetTask(r.Context(), ref)
	if err != nil {
		return err
	}
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return err
	}

	data := taskView{
		Task:          *task,
		ProjectKey:    ref.ProjectKey,
		Priorities:    priorityChoices,
		ArtifactKinds: core.ArtifactKinds,
		CanWrite:      actor.HasScope(core.ScopeTaskWrite),
		CanComment:    actor.HasScope(core.ScopeCommentWrite),
		CanDelete:     actor.HasScope(core.ScopeTaskDelete),
		CanAudit:      actor.HasScope(core.ScopeAuditRead),
	}
	if err := h.attachRelations(r, ref, &data); err != nil {
		return err
	}
	if err := h.attachProject(r, ref, &data); err != nil {
		return err
	}
	if data.CanAudit {
		if err := h.attachHistory(r, task.ID, &data); err != nil {
			return err
		}
	}
	data.Names = h.resolveActors(r, data.actorIDs()...)
	return h.render(w, r, "task.html", task.Ref, data)
}

// attachRelations loads the subtasks, dependencies, comments and artifacts.
func (h *handler) attachRelations(r *http.Request, ref core.TaskRef, data *taskView) error {
	tree, err := h.svc.TaskTree(r.Context(), ref, 1)
	if err != nil {
		return err
	}
	for _, t := range tree {
		if t.ID != data.Task.ID {
			data.Subtasks = append(data.Subtasks, t)
		}
	}
	if data.Dependencies, err = h.svc.ListDependencies(r.Context(), ref); err != nil {
		return err
	}
	if data.Comments, err = h.svc.ListComments(r.Context(), ref); err != nil {
		return err
	}
	data.Artifacts, err = h.svc.ListArtifacts(r.Context(), ref)
	return err
}

// attachProject loads the workflow targets and the custom field definitions.
func (h *handler) attachProject(r *http.Request, ref core.TaskRef, data *taskView) error {
	key := ref.ProjectKey
	if key == "" {
		return nil
	}
	project, err := h.svc.GetProject(r.Context(), key)
	if err != nil {
		return err
	}
	data.ProjectKey = project.Key
	workflow, err := h.workflowOf(r, project)
	if err != nil {
		return err
	}
	data.Targets = targetsFrom(workflow.Definition, data.Task.Status)

	defs, err := h.svc.ListFieldDefs(r.Context(), project.Key)
	if err != nil {
		return err
	}
	for _, def := range defs {
		data.Fields = append(data.Fields, fieldValue{
			Def: def, Value: renderFieldValue(data.Task.CustomFields[def.Key])})
	}
	return nil
}

// renderFieldValue renders a custom field value as text.
func renderFieldValue(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

// attachHistory loads the audit entries recorded against the task and folds
// a multi-hop workflow walk (completeTask, in tasks.go) into a single row per
// user action. See groupHistory for the rule that decides what chains.
func (h *handler) attachHistory(r *http.Request, taskID string, data *taskView) error {
	entries, _, err := h.svc.ListAudit(r.Context(), core.AuditFilter{
		SubjectType: "task",
		SubjectID:   taskID,
		Page:        core.Page{Sort: "seq", Direction: core.Descending},
	})
	if err != nil {
		if core.IsKind(err, core.KindForbidden) {
			data.CanAudit = false
			return nil
		}
		return err
	}
	// entries arrive newest first; chaining reads left to right in time, so
	// it runs oldest first and the result is reversed back to newest first.
	rows := make([]historyRow, len(entries))
	for i, entry := range entries {
		rows[len(entries)-1-i] = historyRow{Entry: entry, Changed: changedFields(entry)}
	}
	groups := groupHistory(rows)
	data.History = make([]historyGroup, len(groups))
	for i, g := range groups {
		data.History[len(groups)-1-i] = g
	}
	return nil
}

// changedFields names the fields one audit entry altered.
func changedFields(entry core.AuditEntry) []string {
	before, after := decodeSnapshot(entry.Before), decodeSnapshot(entry.After)
	seen := map[string]bool{}
	for key, value := range after {
		if !equalJSON(before[key], value) {
			seen[key] = true
		}
	}
	for key := range before {
		if _, ok := after[key]; !ok {
			seen[key] = true
		}
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// decodeSnapshot reads an audit snapshot into a map, tolerating any shape.
func decodeSnapshot(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// equalJSON compares two decoded values by their encoded form.
func equalJSON(a, b any) bool {
	left, errLeft := json.Marshal(a)
	right, errRight := json.Marshal(b)
	if errLeft != nil || errRight != nil {
		return false
	}
	return string(left) == string(right)
}

// taskRefOf parses the reference in the request path.
func taskRefOf(r *http.Request) (core.TaskRef, error) {
	return core.ParseTaskRef(r.PathValue("ref"))
}

// taskPath is the detail screen of the task in the request path.
func taskPath(r *http.Request) string { return RouteTasks + "/" + r.PathValue("ref") }

// updateTask saves the detail screen's edit form.
func (h *handler) updateTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	version, err := intField(r, "version")
	if err != nil {
		return err
	}
	custom, err := h.typedCustomFields(r, ref)
	if err != nil {
		return err
	}
	title, body, assignee := field(r, "title"), field(r, "body"), field(r, "assignee")
	in := core.UpdateTaskInput{
		Title: &title, Body: &body, AssigneeActorID: &assignee, Version: version,
		CustomFields: custom,
	}
	priority, err := priorityField(r, "priority")
	if err != nil {
		return err
	}
	if priority != 0 {
		in.Priority = &priority
	}
	if _, err := h.svc.UpdateTask(r.Context(), ref, in); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "task saved")
	return nil
}

// typedCustomFields reads the custom field inputs and converts each one to the
// type its definition declares, because a form submits only text.
func (h *handler) typedCustomFields(r *http.Request, ref core.TaskRef) (map[string]any, error) {
	submitted := customFieldsFrom(r)
	if len(submitted) == 0 || ref.ProjectKey == "" {
		return submitted, nil
	}
	defs, err := h.svc.ListFieldDefs(r.Context(), ref.ProjectKey)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(submitted))
	for _, def := range defs {
		raw, ok := submitted[def.Key]
		if !ok {
			continue
		}
		value, err := coerceField(def, fmt.Sprintf("%v", raw))
		if err != nil {
			return nil, err
		}
		if value != nil {
			out[def.Key] = value
		}
	}
	return out, nil
}

// coerceField converts one submitted text value to its declared type.
func coerceField(def core.FieldDef, raw string) (any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	switch def.Type {
	case core.FieldInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, core.Invalid("%s must be a whole number", def.Label)
		}
		return n, nil
	case core.FieldFloat:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, core.Invalid("%s must be a number", def.Label)
		}
		return n, nil
	case core.FieldBool:
		switch strings.ToLower(raw) {
		case "1", "true", "yes", "on":
			return true, nil
		case "0", "false", "no", "off":
			return false, nil
		default:
			return nil, core.Invalid("%s must be true or false", def.Label)
		}
	case core.FieldJSON:
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return nil, core.Invalid("%s must be valid json", def.Label)
		}
		return decoded, nil
	default:
		return raw, nil
	}
}

// customFieldsFrom reads the custom field inputs the detail form carries.
func customFieldsFrom(r *http.Request) map[string]any {
	out := map[string]any{}
	for name, values := range r.PostForm {
		if len(values) == 0 || len(name) <= len("field.") || name[:len("field.")] != "field." {
			continue
		}
		out[name[len("field."):]] = values[0]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// transitionTask applies a state change from the detail screen.
func (h *handler) transitionTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	in := core.TransitionInput{To: field(r, "to"), Comment: field(r, "comment")}
	if _, err := h.svc.TransitionTask(r.Context(), ref, in); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "task moved to "+in.To)
	return nil
}

// deleteTask soft deletes a task.
func (h *handler) deleteTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	in := core.DeleteTaskInput{Hard: checked(r, "hard"), Cascade: checked(r, "cascade")}
	if err := h.svc.DeleteTask(r.Context(), ref, in); err != nil {
		return err
	}
	redirect(w, r, RouteTasks, "task deleted")
	return nil
}

// restoreTask brings a soft deleted task back.
func (h *handler) restoreTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	if _, err := h.svc.RestoreTask(r.Context(), ref); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "task restored")
	return nil
}

// addDependency records that the task waits for another.
func (h *handler) addDependency(w http.ResponseWriter, r *http.Request) error {
	ref, dep, err := refPair(r)
	if err != nil {
		return err
	}
	if err := h.svc.AddDependency(r.Context(), ref, dep); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "dependency added")
	return nil
}

// removeDependency drops a dependency edge.
func (h *handler) removeDependency(w http.ResponseWriter, r *http.Request) error {
	ref, dep, err := refPair(r)
	if err != nil {
		return err
	}
	if err := h.svc.RemoveDependency(r.Context(), ref, dep); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "dependency removed")
	return nil
}

// refPair parses the task in the path and the task the form names.
func refPair(r *http.Request) (core.TaskRef, core.TaskRef, error) {
	ref, err := taskRefOf(r)
	if err != nil {
		return core.TaskRef{}, core.TaskRef{}, err
	}
	dep, err := core.ParseTaskRef(field(r, "depends_on"))
	if err != nil {
		return core.TaskRef{}, core.TaskRef{}, err
	}
	return ref, dep, nil
}

// addTag attaches a tag to the task.
func (h *handler) addTag(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	if err := h.svc.AddTag(r.Context(), ref, field(r, "tag")); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "tag added")
	return nil
}

// removeTag detaches a tag from the task.
func (h *handler) removeTag(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	if err := h.svc.RemoveTag(r.Context(), ref, field(r, "tag")); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "tag removed")
	return nil
}

// addComment posts a comment on the task.
func (h *handler) addComment(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	if _, err := h.svc.AddComment(r.Context(), ref, field(r, "body")); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "comment added")
	return nil
}

// editComment rewrites one comment.
func (h *handler) editComment(w http.ResponseWriter, r *http.Request) error {
	if _, err := h.svc.EditComment(r.Context(), field(r, "id"), field(r, "body")); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "comment edited")
	return nil
}

// deleteComment removes one comment.
func (h *handler) deleteComment(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteComment(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "comment deleted")
	return nil
}

// putArtifact attaches structured output to the task.
func (h *handler) putArtifact(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	in := core.ArtifactInput{
		Kind:    core.ArtifactKind(field(r, "kind")),
		Name:    field(r, "name"),
		Payload: pairs(field(r, "payload")),
	}
	if _, err := h.svc.PutArtifact(r.Context(), ref, in); err != nil {
		return err
	}
	redirect(w, r, taskPath(r), "artifact attached")
	return nil
}
