// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/i18n"
	"github.com/heliopsy/tix/internal/output"
)

// bulkBar is what the selection bar under the task list offers: the states a
// move can name, and which actions this reader may take at all.
type bulkBar struct {
	States     []string
	Priorities []priorityChoice
	Move       bool
	Write      bool
	Copy       bool
	Delete     bool
	Star       bool
}

// Any reports whether the reader may take any bulk action, so a reader who
// may take none is not handed checkboxes that lead nowhere.
func (b bulkBar) Any() bool { return b.Move || b.Write || b.Copy || b.Delete || b.Star }

// newBulkBar builds the bar for a page. The states are every state the page's
// projects run, in workflow order, because a listing mixes workflows and the
// service answers per task where a state is out of reach.
func newBulkBar(r *http.Request, tasks []core.Task, moves map[string]core.WorkflowDefinition) bulkBar {
	has := readerHasScope(r)
	bar := bulkBar{
		Priorities: priorityChoices,
		Move:       has(core.ScopeTaskTransition),
		Write:      has(core.ScopeTaskWrite),
		Copy:       has(core.ScopeTaskWrite),
		Delete:     has(core.ScopeTaskDelete),
		Star:       has(core.ScopeTaskRead),
	}
	seenProject := map[string]bool{}
	for _, t := range tasks {
		if seenProject[t.ProjectID] {
			continue
		}
		seenProject[t.ProjectID] = true
		for _, st := range moves[t.ProjectID].States {
			if !slices.Contains(bar.States, st.Key) {
				bar.States = append(bar.States, st.Key)
			}
		}
	}
	return bar
}

// bulkBarUnlessTrash is the bar for a listing, or none in the trash, where
// the one thing to do with a task is restore it from its own row.
func bulkBarUnlessTrash(r *http.Request, tasks []core.Task, moves map[string]core.WorkflowDefinition, trash bool) bulkBar {
	if trash {
		return bulkBar{}
	}
	return newBulkBar(r, tasks, moves)
}

// undoBulkRestorePrefix marks an undo that restores the tasks a bulk delete
// removed, named after it.
const undoBulkRestorePrefix = "bulk-restore:"

// bulkTasks applies the action the selection bar submitted to every selected
// row in one request, and returns to the listing with a toast saying how many
// changed and which were refused and why.
func (h *handler) bulkTasks(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return core.Invalid("reading the form: %v", err)
	}
	in := core.BulkTaskInput{
		Action:          core.BulkAction(strings.TrimSpace(r.FormValue("action"))),
		Refs:            r.PostForm["ref"],
		To:              field(r, "to"),
		Tag:             field(r, "tag"),
		AssigneeActorID: field(r, "assignee"),
		ProjectRef:      field(r, "project"),
	}
	if len(in.Refs) == 1 && strings.Contains(in.Refs[0], ",") {
		in.Refs = strings.Split(in.Refs[0], ",")
	}
	priority, err := priorityField(r, "priority")
	if err != nil {
		return err
	}
	in.Priority = priority
	back := safeNext(field(r, "next"))
	if back == RouteProjects {
		back = RouteTasks
	}
	if in.Action == core.BulkDelete {
		if asked, err := h.askBeforePermanentDelete(w, r, in.Refs, back); asked || err != nil {
			return err
		}
	}
	result, err := h.svc.BulkTasks(r.Context(), in)
	if err != nil {
		if !refusable(err) {
			return err
		}
		redirect(w, r, back, safeMessage(err, core.KindOf(err)))
		return nil
	}
	undo := ""
	if result.Action == core.BulkDelete && result.Applied > 0 && !checked(r, confirmParam) {
		undo = undoBulkRestorePrefix + strings.Join(result.AppliedRefs(), ",")
	}
	redirect(w, r, withUndo(back, undo), bulkFlash(langOf(r), in, result))
	return nil
}

// maxRefusalsNamed bounds how many refused tasks a toast lists by name.
const maxRefusalsNamed = 5

// bulkDone is what a toast says landed, per action: the singular and plural
// forms, which format the count and then the action's argument when the form
// names one.
var bulkDone = map[core.BulkAction][2]string{
	core.BulkMove:      {"Moved %d task to %s", "Moved %d tasks to %s"},
	core.BulkPriority:  {"Set %d task to %s priority", "Set %d tasks to %s priority"},
	core.BulkTagAdd:    {"Tagged %d task with %s", "Tagged %d tasks with %s"},
	core.BulkTagRemove: {"Untagged %d task: %s", "Untagged %d tasks: %s"},
	core.BulkAssign:    {"Assigned %d task to %s", "Assigned %d tasks to %s"},
	core.BulkStar:      {"Starred %d task", "Starred %d tasks"},
	core.BulkUnstar:    {"Unstarred %d task", "Unstarred %d tasks"},
	core.BulkCopy:      {"Copied %d task to %s", "Copied %d tasks to %s"},
	core.BulkDelete:    {"Deleted %d task", "Deleted %d tasks"},
	core.BulkRestore:   {"Restored %d task", "Restored %d tasks"},
}

// bulkUnassigned is an assign that cleared the assignee.
var bulkUnassigned = [2]string{"Unassigned %d task", "Unassigned %d tasks"}

// Refusal texts a bulk toast adds after what landed.
const (
	bulkRefusedKey = "%d refused: %s"
	bulkMoreKey    = "and %d more"
)

// bulkFlashKeys lists every catalog key a bulk toast can use.
func bulkFlashKeys() []string {
	out := []string{bulkUnassigned[0], bulkUnassigned[1], bulkRefusedKey, bulkMoreKey}
	for _, forms := range bulkDone {
		out = append(out, forms[0], forms[1])
	}
	return out
}

// bulkFlash is the toast a bulk action answers with, in the reader's
// language: what changed, then each refusal with its reason.
func bulkFlash(lang string, in core.BulkTaskInput, result *core.BulkResult) string {
	forms, arg := bulkDone[in.Action], ""
	switch in.Action {
	case core.BulkMove:
		arg = in.To
	case core.BulkPriority:
		arg = i18n.T(lang, priorityName(in.Priority))
	case core.BulkTagAdd, core.BulkTagRemove:
		arg = in.Tag
	case core.BulkAssign:
		arg = in.AssigneeActorID
		if arg == "" {
			forms = bulkUnassigned
		}
	case core.BulkCopy:
		arg = in.ProjectRef
	}
	var done string
	if strings.Contains(forms[0], "%s") {
		done = i18n.N(lang, forms[0], forms[1], result.Applied, arg)
	} else {
		done = i18n.N(lang, forms[0], forms[1], result.Applied)
	}
	if result.Refused == 0 {
		return done
	}
	var named []string
	for _, o := range result.Outcomes {
		if o.Applied {
			continue
		}
		if len(named) == maxRefusalsNamed {
			named = append(named, i18n.T(lang, bulkMoreKey, result.Refused-maxRefusalsNamed))
			break
		}
		named = append(named, o.Ref+" ("+o.Error+")")
	}
	return done + ". " + i18n.T(lang, bulkRefusedKey, result.Refused, strings.Join(named, ", "))
}

// UndoBulk lists the tasks the toast's Undo restores, when it undoes a bulk
// delete.
func (v view) UndoBulk() []string {
	refs, ok := strings.CutPrefix(v.FlashUndo, undoBulkRestorePrefix)
	if !ok || refs == "" {
		return nil
	}
	return strings.Split(refs, ",")
}

// csvExportLimit bounds how many rows one export walks.
const csvExportLimit = 10000

// exportTasksCSV downloads the listing the reader is looking at as CSV: the
// same filter, deadline window, sort and project visibility, every page of
// it up to csvExportLimit rows, in the columns their picker shows.
func (h *handler) exportTasksCSV(w http.ResponseWriter, r *http.Request) error {
	filter, _, refused, _, err := h.listingFilter(r, TaskViewList)
	if err != nil {
		return err
	}
	if refused != "" {
		return core.Invalid("%s", refused)
	}
	projects, _, err := h.allProjects(r, false)
	if err != nil {
		return err
	}
	if live := liveHiddenKeys(projects, hiddenProjects(r)); len(filter.ProjectKeys) == 0 && len(live) > 0 {
		filter.Exclude.ProjectKeys = append(filter.Exclude.ProjectKeys, live...)
	}
	filter.Page.Cursor = ""
	filter.Page.Limit = core.MaxPageLimit
	var tasks []core.Task
	for len(tasks) < csvExportLimit {
		page, err := h.svc.ListTasks(r.Context(), filter)
		if err != nil {
			return err
		}
		tasks = append(tasks, page.Tasks...)
		if page.NextCursor == "" {
			break
		}
		filter.Page.Cursor = page.NextCursor
	}
	if len(tasks) > csvExportLimit {
		tasks = tasks[:csvExportLimit]
	}
	names := h.resolveActors(r, rowActors(tasks)...)
	labels := make(map[string]string)
	for _, t := range tasks {
		if t.AssigneeActorID != "" {
			labels[t.AssigneeActorID] = names.Label(t.AssigneeActorID)
		}
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="tix-tasks.csv"`)
	out := output.NewTaskCSV(w, csvColumns(columnsOf(r)), labels)
	for _, t := range tasks {
		if err := out.Write(t); err != nil {
			return err
		}
	}
	return out.Close()
}

// csvColumnOf maps each optional column of the task list to the CSV column
// that carries the same thing.
var csvColumnOf = map[string]output.TaskColumn{
	"star":     output.ColumnStarred,
	"status":   output.ColumnStatus,
	"priority": output.ColumnPriority,
	"tags":     output.ColumnTags,
	"assignee": output.ColumnAssignee,
	"project":  output.ColumnProject,
	"due":      output.ColumnDue,
	"repeat":   output.ColumnRepeat,
	"updated":  output.ColumnUpdated,
	"ref":      output.ColumnRef,
}

// csvColumns is the title, which the list always shows, and then each
// optional column the reader's picker shows, in the order the list draws
// them.
func csvColumns(prefs columnPrefs) []output.TaskColumn {
	out := []output.TaskColumn{output.ColumnTitle}
	for _, c := range columnSets["tasks"] {
		if prefs.Show("tasks", c.Key) {
			out = append(out, csvColumnOf[c.Key])
		}
	}
	return out
}

// csvHref is the export of the listing this request is drawing: its filter,
// deadline window, sort and open-work choice, without the page position,
// since the export walks every page.
func csvHref(r *http.Request) string {
	query := url.Values{}
	for _, key := range []string{"q", DueParam, "sort", AllParam} {
		if v := r.URL.Query().Get(key); v != "" {
			query.Set(key, v)
		}
	}
	if len(query) == 0 {
		return RouteTasksCSV
	}
	return RouteTasksCSV + "?" + query.Encode()
}
