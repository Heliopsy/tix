// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// workflowShape is the identity a board merges columns on.
//
// A name is not an identity. Two projects can each run a workflow called
// "default" whose states differ, and the state vocabulary has just widened,
// so two same-named workflows can now differ in ways that decide where a task
// may go. Merging on the name, or on the label, would draw one set of columns
// over two different state machines and offer a card a move its own project
// forbids.
//
// What is compared is everything a board reads or a move depends on: the
// states in their declared order, because that order is the column order and
// the labels are the column headings, and the transitions in their declared
// order, because they are the edges core.Routes walks and the order it
// enumerates equal-length routes in. Two definitions with the same shape draw
// the same columns and offer every task the same moves, whatever the workflow
// is called and whichever row it is stored in.
//
// Deliberately excluded, because none of them can change a column or a move:
// Initial (where a new task starts), DefaultLease, and the per-state lease
// reversion fields. Including them would refuse boards that are drawable, and
// a refusal is cheap but not free. Everything else is included, because the
// cost of being wrong the other way is a card offering a move that is then
// refused by the service after the reader has already acted.
func workflowShape(d core.WorkflowDefinition) string {
	var b strings.Builder
	for _, s := range d.States {
		b.WriteString(s.Key)
		b.WriteByte('|')
		b.WriteString(core.StateLabel(s))
		b.WriteByte('|')
		b.WriteString(strconv.FormatBool(s.Terminal))
		b.WriteByte('|')
		b.WriteString(string(s.Category))
		b.WriteByte(';')
	}
	b.WriteString("=>")
	for _, t := range d.Transitions {
		b.WriteString(t.From)
		b.WriteString(core.RouteSep)
		b.WriteString(t.To)
		b.WriteByte('|')
		b.WriteString(string(t.RequiresScope))
		b.WriteByte('|')
		b.WriteString(strconv.FormatBool(t.RequiresComment))
		b.WriteByte(';')
	}
	return b.String()
}

// boardGroup is a set of projects whose workflows are the same workflow as
// far as a board is concerned, together with the definition their board would
// be drawn from.
type boardGroup struct {
	Workflow core.Workflow
	Projects []core.Project
	// Href is the task screen narrowed to exactly these projects, this
	// reader's filter and sort intact, so a refused board is one click from
	// a drawable one rather than a dead end.
	Href string
	// Label is how the refusal names this group's workflow. It is the name,
	// except where another group's workflow carries the same name, which is
	// the case the merge rule exists for: two workflows called "default" that
	// are different machines. Two entries reading "Default" tell a reader
	// nothing, so the key disambiguates them.
	Label string
	// Known is false for a group of projects whose workflow this request
	// could not resolve at all. Such a project is named in the refusal
	// rather than dropped from it: a board missing a project it was asked
	// for is the silence this whole screen is meant to avoid.
	Known bool
}

// Name is what to call the group's workflow on screen.
func (g boardGroup) Name() string {
	switch {
	case !g.Known:
		return "no workflow"
	case g.Workflow.Name != "":
		return g.Workflow.Name
	default:
		return g.Workflow.Key
	}
}

// Keys names the group's projects, in the order they are listed.
func (g boardGroup) Keys() []string {
	out := make([]string, 0, len(g.Projects))
	for _, p := range g.Projects {
		out = append(out, p.Key)
	}
	return out
}

// taskBoard is the board the task screen draws, or the reason it cannot.
type taskBoard struct {
	// Columns are the merged workflow's states, in declared order, each
	// holding the tasks of this page that sit in it.
	Columns []column
	// Groups are the distinct workflows the selected projects run. One group
	// is a board; more than one is a refusal naming every group.
	Groups []boardGroup
	// Stray are tasks on this page whose status is in no column, which can
	// only happen if a task's state left its workflow. They are listed
	// rather than dropped, so the board is never quietly short.
	Stray []core.Task
	// Agreed reports whether the selected projects share one workflow, which
	// is the only case a board is drawn in.
	Agreed bool
}

// Workflow names the workflow a drawn board is drawn from.
func (b taskBoard) Workflow() string {
	if !b.Agreed || len(b.Groups) == 0 {
		return ""
	}
	return b.Groups[0].Name()
}

// Merged reports whether the drawn board covers more than one project, which
// is what the heading says and what puts a project mark on every card.
func (b taskBoard) Merged() bool {
	return b.Agreed && len(b.Groups) == 1 && len(b.Groups[0].Projects) > 1
}

// candidateProjects are the projects the listing may draw a task from: the
// ones an explicit project: filter names, or every project the reader has not
// put away, minus anything the filter excludes either way.
//
// The board's columns are decided from these rather than from the projects
// the current page happens to carry. A page is a page: deciding on its
// contents would merge two workflows on page one and refuse on page three,
// and an empty page would have no columns at all.
func candidateProjects(projects []core.Project, filter core.TaskFilter) []core.Project {
	want := map[string]bool{}
	for _, key := range filter.ProjectKeys {
		want[strings.ToLower(key)] = true
	}
	skip := map[string]bool{}
	for _, key := range filter.Exclude.ProjectKeys {
		skip[strings.ToLower(key)] = true
	}
	out := make([]core.Project, 0, len(projects))
	for _, p := range projects {
		key := strings.ToLower(p.Key)
		if len(want) > 0 && !want[key] {
			continue
		}
		if skip[key] {
			continue
		}
		out = append(out, p)
	}
	return out
}

// groupByWorkflow gathers projects whose workflows have the same shape.
//
// Groups are ordered by workflow name and projects within a group by key, so
// a refusal names the same projects in the same order on every request rather
// than in whatever order a map happened to answer in.
func groupByWorkflow(projects []core.Project, workflows map[string]core.Workflow) []boardGroup {
	byShape := map[string]*boardGroup{}
	for _, p := range projects {
		w, known := workflows[p.WorkflowID]
		shape := "?" + p.WorkflowID
		if known {
			shape = workflowShape(w.Definition)
		}
		g, seen := byShape[shape]
		if !seen {
			g = &boardGroup{Workflow: w, Known: known}
			byShape[shape] = g
		}
		// Two stored workflows can describe one machine, so the group has a
		// choice of names and takes the first alphabetically rather than
		// whichever project happened to be listed first. A heading that moved
		// when a project was renamed would be reporting nothing.
		if known && g.Known && workflowOrder(w) < workflowOrder(g.Workflow) {
			g.Workflow = w
		}
		g.Projects = append(g.Projects, p)
	}
	out := make([]boardGroup, 0, len(byShape))
	for _, g := range byShape {
		sort.Slice(g.Projects, func(i, j int) bool { return g.Projects[i].Key < g.Projects[j].Key })
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if a, b := out[i].Name(), out[j].Name(); a != b {
			return a < b
		}
		return strings.Join(out[i].Keys(), ",") < strings.Join(out[j].Keys(), ",")
	})
	return labelled(out)
}

// labelled names each group, adding the workflow's key wherever the name alone
// would not tell two groups apart.
func labelled(groups []boardGroup) []boardGroup {
	shared := map[string]int{}
	for _, g := range groups {
		shared[g.Name()]++
	}
	for i := range groups {
		groups[i].Label = groups[i].Name()
		if shared[groups[i].Name()] > 1 && groups[i].Workflow.Key != "" {
			groups[i].Label += " (" + groups[i].Workflow.Key + ")"
		}
	}
	return groups
}

// workflowOrder is how two workflows describing one machine are ranked, so
// the name a merged board shows does not depend on project order.
func workflowOrder(w core.Workflow) string { return w.Name + "\x00" + w.Key }

// buildBoard draws the board for one agreed group, or reports the groups that
// disagree.
//
// Every card's moves come from its own project's workflow, never from the
// definition the columns were drawn from. On an agreed board the two answer
// alike by construction; asking the task's own project anyway is what makes
// it impossible for a change to the merge rule to start offering a card a
// move its project forbids.
func buildBoard(groups []boardGroup, tasks []core.Task, moves map[string]core.WorkflowDefinition) taskBoard {
	out := taskBoard{Groups: groups}
	if len(groups) != 1 || !groups[0].Known {
		return out
	}
	out.Agreed = true
	within := make(map[string]bool, len(groups[0].Projects))
	for _, p := range groups[0].Projects {
		within[p.ID] = true
	}
	def := groups[0].Workflow.Definition
	out.Columns = make([]column, 0, len(def.States))
	placed := make(map[string]bool, len(tasks))
	for _, state := range def.States {
		col := column{State: state}
		for _, task := range tasks {
			if task.Status != state.Key || !within[task.ProjectID] {
				continue
			}
			placed[task.ID] = true
			col.Tasks = append(col.Tasks, card{Task: task, Routes: flowRoutes(moves[task.ProjectID], task.Status)})
		}
		out.Columns = append(out.Columns, col)
	}
	for _, task := range tasks {
		if !placed[task.ID] {
			out.Stray = append(out.Stray, task)
		}
	}
	return out
}

// narrowHref is the task screen filtered to the named projects, keeping the
// reader's own filter, deadline window, sort and page size.
//
// The cursor and its trail are dropped: they address positions in one ordered
// result set, and under a narrower filter they name rows this reader was
// never on, which is why the filter form drops them too.
func narrowHref(r *http.Request, keys []string) string {
	query := r.URL.Query()
	query.Del(CursorParam)
	query.Del(TrailParam)
	query.Del("flash")
	terms := make([]string, 0, len(keys))
	for _, key := range keys {
		terms = append(terms, "project:"+key)
	}
	expression := strings.TrimSpace(strings.TrimSpace(query.Get("q")) + " " + strings.Join(terms, " "))
	query.Set("q", expression)
	return RouteTasks + "?" + query.Encode()
}

// taskBoardFor resolves the board for this request: which projects the
// listing may draw from, which workflows they run, and either the columns or
// the disagreement.
func (h *handler) taskBoardFor(r *http.Request, projects []core.Project, filter core.TaskFilter,
	tasks []core.Task, moves map[string]core.WorkflowDefinition) (taskBoard, error) {
	workflows, err := h.workflowsByID(r)
	if err != nil {
		return taskBoard{}, err
	}
	groups := groupByWorkflow(candidateProjects(projects, filter), workflows)
	for i := range groups {
		groups[i].Href = narrowHref(r, groups[i].Keys())
	}
	return buildBoard(groups, tasks, moves), nil
}

// workflowsByID is every workflow this tenant has, by identifier. A project
// carries the identifier; the service addresses a workflow by key.
func (h *handler) workflowsByID(r *http.Request) (map[string]core.Workflow, error) {
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return nil, err
	}
	out := make(map[string]core.Workflow, len(workflows))
	for _, w := range workflows {
		out[w.ID] = w
	}
	return out, nil
}
