// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Group kinds: what a heading groups by, which decides what its menu offers.
const (
	groupStatus  = "status"
	groupProject = "project"
)

// groupHead is one heading of a board column, a lane or a list group: its
// name, its count, the reader's own arrangement of it, and what its menu
// offers.
type groupHead struct {
	// Key is what a form posts as the group: a state key, or a project key.
	Key   string
	Name  string
	Count int
	Kind  string

	Starred   bool
	Collapsed bool
	// Left and Right report whether the group has a drawn neighbour of the
	// same star state on that side, which is the only move offered.
	Left  bool
	Right bool

	// Projects are where a task added from a status heading may go, and
	// Status the state it is added in. A project heading adds to itself.
	Projects []core.Project
	Status   string
}

// DOMID is a fragment safe for an element id, unique within one view.
func (g groupHead) DOMID() string { return slug(g.Kind + "-" + g.Key) }

// hiddenGroup is a group the reader hid that this view would have drawn.
type hiddenGroup struct {
	Key  string
	Name string
}

// arrangement is one view's grouping as this reader arranged it: the view it
// is stored under, the drawn order, the hidden groups, and what every form in
// a heading menu needs to post and return.
type arrangement struct {
	View   string
	Self   string
	CSRF   string
	Region string
	Order  string
	Hidden []hiddenGroup
	// Stored reports whether the reader has any preference on this view, so
	// the View menu offers a reset only when there is something to reset.
	Stored bool
	// CanCreate reports whether the reader may add tasks, which is what the
	// add field in a heading menu needs.
	CanCreate bool
	// Vertical is set for a list, whose groups move up and down.
	Vertical bool
}

// groupMenu is what the "group-menu" partial draws: one heading's menu with
// the view it belongs to.
type groupMenu struct {
	A arrangement
	G groupHead
}

func newGroupMenu(a arrangement, g groupHead) groupMenu { return groupMenu{A: a, G: g} }

// noGroupHead is the head a menu over a whole view carries.
func noGroupHead() groupHead { return groupHead{} }

// groupPrefIndex keys a reader's preferences by group, case-folded, since a
// project key is typed by hand and a state key never differs by case.
func groupPrefIndex(prefs []core.GroupPref) map[string]core.GroupPref {
	out := make(map[string]core.GroupPref, len(prefs))
	for _, p := range prefs {
		out[strings.ToLower(p.Group)] = p
	}
	return out
}

// arrangeOrder is the order groups are drawn in: starred first, then the
// ones the reader moved by where they moved them, then the rest in the
// view's own order. It returns indexes into keys.
func arrangeOrder(keys []string, prefs map[string]core.GroupPref) []int {
	idx := make([]int, len(keys))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := prefs[strings.ToLower(keys[idx[a]])], prefs[strings.ToLower(keys[idx[b]])]
		if pa.Starred != pb.Starred {
			return pa.Starred
		}
		if (pa.Position > 0) != (pb.Position > 0) {
			return pa.Position > 0
		}
		return pa.Position < pb.Position
	})
	return idx
}

// arrangeGroups applies the reader's preferences to a view's groups, given
// in the view's own order: it returns the groups to draw, in the order to
// draw them, with their heads' state and moves decided, and the ones hidden.
func arrangeGroups[T any](groups []T, head func(*T) *groupHead, prefs map[string]core.GroupPref) ([]T, []hiddenGroup) {
	keys := make([]string, len(groups))
	for i := range groups {
		keys[i] = head(&groups[i]).Key
	}
	var drawn []T
	var hidden []hiddenGroup
	for _, i := range arrangeOrder(keys, prefs) {
		g := groups[i]
		h := head(&g)
		p := prefs[strings.ToLower(h.Key)]
		if p.Hidden {
			hidden = append(hidden, hiddenGroup{Key: h.Key, Name: h.Name})
			continue
		}
		h.Starred, h.Collapsed = p.Starred, p.Collapsed
		drawn = append(drawn, g)
	}
	for i := range drawn {
		h := head(&drawn[i])
		h.Left = i > 0 && head(&drawn[i-1]).Starred == h.Starred
		h.Right = i < len(drawn)-1 && head(&drawn[i+1]).Starred == h.Starred
	}
	return drawn, hidden
}

// drawnOrder is the comma separated keys a move posts, so the server writes
// the order the reader is looking at.
func drawnOrder[T any](groups []T, head func(*T) *groupHead) string {
	keys := make([]string, len(groups))
	for i := range groups {
		keys[i] = head(&groups[i]).Key
	}
	return strings.Join(keys, ",")
}

// newArrangement builds the view-wide half of every heading menu.
func newArrangement(r *http.Request, view, self, region, order string, hidden []hiddenGroup, prefs []core.GroupPref) arrangement {
	return arrangement{
		View: view, Self: self, CSRF: csrfFrom(r), Region: region,
		Order: order, Hidden: hidden, Stored: len(prefs) > 0,
		CanCreate: readerHasScope(r)(core.ScopeTaskWrite),
	}
}

// groupPrefs reads the reader's arrangement of a view. A reader the service
// will not tell is drawn the view unarranged rather than refused the screen.
func (h *handler) groupPrefs(r *http.Request, view string) []core.GroupPref {
	prefs, err := h.svc.ListGroupPrefs(r.Context(), view)
	if err != nil {
		return nil
	}
	return prefs
}

// viewRoutes are the forms of a heading menu and of the Hidden control.
func (h *handler) viewRoutes() []route {
	return []route{
		post(RouteViewGroup, h.setGroupPref, "SetGroupPref"),
		post(RouteViewOrder, h.orderGroups, "OrderGroups"),
		post(RouteViewReset, h.resetGroupPrefs, "ResetGroupPrefs"),
	}
}

// groupFlags are the switches a heading menu posts, by form name.
var groupFlags = map[string]func(*core.GroupPrefInput, *bool){
	"hidden":    func(in *core.GroupPrefInput, v *bool) { in.Hidden = v },
	"collapsed": func(in *core.GroupPrefInput, v *bool) { in.Collapsed = v },
	"starred":   func(in *core.GroupPrefInput, v *bool) { in.Starred = v },
}

// setGroupPref hides, shows, stars, collapses or expands one group.
func (h *handler) setGroupPref(w http.ResponseWriter, r *http.Request) error {
	set, ok := groupFlags[field(r, "set")]
	if !ok {
		return core.Invalid("unknown group preference %q", field(r, "set"))
	}
	value := field(r, "value") == "1"
	var in core.GroupPrefInput
	set(&in, &value)
	if _, err := h.svc.SetGroupPref(r.Context(), field(r, "view"), field(r, "group"), in); err != nil {
		return err
	}
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// orderGroups moves one group past its neighbour in the drawn order, and
// stores the whole drawn order so the reader sees what they asked for.
func (h *handler) orderGroups(w http.ResponseWriter, r *http.Request) error {
	order, err := movedOrder(fieldList(r, "order"), field(r, "group"), field(r, "dir"))
	if err != nil {
		return err
	}
	if _, err := h.svc.OrderGroups(r.Context(), field(r, "view"), core.GroupOrderInput{Groups: order}); err != nil {
		return err
	}
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// movedOrder swaps group with its neighbour on the side dir names.
func movedOrder(order []string, group, dir string) ([]string, error) {
	at := -1
	for i, key := range order {
		if strings.EqualFold(key, group) {
			at = i
		}
	}
	if at < 0 {
		return nil, core.Invalid("group %q is not in the order", group)
	}
	to := at - 1
	if dir == "right" {
		to = at + 1
	} else if dir != "left" {
		return nil, core.Invalid("move %q: use left or right", dir)
	}
	if to < 0 || to >= len(order) {
		return nil, core.Invalid("group %q has no neighbour on the %s", group, dir)
	}
	out := append([]string(nil), order...)
	out[at], out[to] = out[to], out[at]
	return out, nil
}

// resetGroupPrefs forgets the reader's arrangement of a view, or of one group.
func (h *handler) resetGroupPrefs(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.ResetGroupPrefs(r.Context(), field(r, "view"), field(r, "group")); err != nil {
		return err
	}
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// colHead and its siblings reach the head of each kind of group.
func colHead(c *column) *groupHead     { return &c.Head }
func laneHead(l *lane) *groupHead      { return &l.Head }
func listHead(g *listGroup) *groupHead { return &g.Head }

// statusHead is the heading of a column or list group of one state.
func statusHead(state core.State, count int, projects []core.Project) groupHead {
	return groupHead{Key: state.Key, Name: core.StateLabel(state), Count: count, Kind: groupStatus,
		Projects: projects, Status: state.Key}
}

// projectHead is the heading of a lane or list group of one project.
func projectHead(p core.Project, count int) groupHead {
	return groupHead{Key: p.Key, Name: p.Name, Count: count, Kind: groupProject, Projects: []core.Project{p}}
}

// hiddenOf names every group the reader hid on a view that this screen can
// name, whether or not this page would have drawn it, so a group hidden on
// one page can be shown again from any other.
func hiddenOf(prefs []core.GroupPref, name func(key string) (string, bool)) []hiddenGroup {
	var out []hiddenGroup
	for _, p := range prefs {
		if !p.Hidden {
			continue
		}
		if label, ok := name(p.Group); ok {
			out = append(out, hiddenGroup{Key: p.Group, Name: label})
		}
	}
	return out
}

// stateNamer names the states of the given workflows, first one wins.
func stateNamer(defs ...core.WorkflowDefinition) func(string) (string, bool) {
	return func(key string) (string, bool) {
		for _, d := range defs {
			if s, ok := d.State(key); ok {
				return core.StateLabel(s), true
			}
		}
		return "", false
	}
}

// projectNamer names projects by key.
func projectNamer(projects []core.Project) func(string) (string, bool) {
	return func(key string) (string, bool) {
		for _, p := range projects {
			if strings.EqualFold(p.Key, key) {
				return p.Name, true
			}
		}
		return "", false
	}
}

// listGroup is one heading of a grouped list and the rows under it.
type listGroup struct {
	Head  groupHead
	Tasks []core.Task
}

// buildListGroups splits one page of rows by project or by state. Projects
// come in the order the screen lists them, states in their workflows' order;
// a row keeps its place in the sort inside its group.
func buildListGroups(by string, projects []core.Project, tasks []core.Task, moves map[string]core.WorkflowDefinition) []listGroup {
	var out []listGroup
	at := map[string]int{}
	add := func(key string, head groupHead, t core.Task) {
		i, ok := at[key]
		if !ok {
			i = len(out)
			at[key] = i
			out = append(out, listGroup{Head: head})
		}
		out[i].Tasks = append(out[i].Tasks, t)
		out[i].Head.Count++
	}
	if by == ListGroupProject {
		byID := map[string][]core.Task{}
		for _, t := range tasks {
			byID[t.ProjectID] = append(byID[t.ProjectID], t)
		}
		for _, p := range projects {
			for _, t := range byID[p.ID] {
				add(p.Key, projectHead(p, 0), t)
			}
		}
		return out
	}
	rank := map[string]int{}
	states := map[string]core.State{}
	makers := map[string][]core.Project{}
	for _, p := range projects {
		def := moves[p.ID]
		for _, s := range def.States {
			if _, ok := rank[s.Key]; !ok {
				rank[s.Key] = len(rank)
				states[s.Key] = s
			}
			makers[s.Key] = append(makers[s.Key], p)
		}
	}
	sorted := append([]core.Task(nil), tasks...)
	sort.SliceStable(sorted, func(a, b int) bool {
		ra, oka := rank[sorted[a].Status]
		rb, okb := rank[sorted[b].Status]
		if oka != okb {
			return oka
		}
		return ra < rb
	})
	for _, t := range sorted {
		s, ok := states[t.Status]
		if !ok {
			s = core.State{Key: t.Status}
		}
		add(t.Status, statusHead(s, 0, makers[t.Status]), t)
	}
	return out
}

// tasksRegion is the part of the task screen a heading menu redraws.
const tasksRegion = "#tasks-listing"

// arrangeBoard draws the task screen's board as the reader arranged it.
func (h *handler) arrangeBoard(r *http.Request, board *taskBoard, self string) arrangement {
	prefs := h.groupPrefs(r, core.ViewBoard)
	makers := board.Groups[0].Projects
	for i := range board.Columns {
		c := &board.Columns[i]
		c.Head = statusHead(c.State, len(c.Tasks), makers)
	}
	board.Columns, _ = arrangeGroups(board.Columns, colHead, groupPrefIndex(prefs))
	hidden := hiddenOf(prefs, stateNamer(board.Groups[0].Workflow.Definition))
	return newArrangement(r, core.ViewBoard, self, tasksRegion, drawnOrder(board.Columns, colHead), hidden, prefs)
}

// arrangeLanes draws the lanes as the reader arranged them.
func (h *handler) arrangeLanes(r *http.Request, lanes *taskLanes, projects []core.Project, self string) arrangement {
	prefs := h.groupPrefs(r, core.ViewLanes)
	for i := range lanes.Lanes {
		l := &lanes.Lanes[i]
		l.Head = projectHead(l.Project, l.Open)
	}
	lanes.Lanes, _ = arrangeGroups(lanes.Lanes, laneHead, groupPrefIndex(prefs))
	hidden := hiddenOf(prefs, projectNamer(projects))
	return newArrangement(r, core.ViewLanes, self, tasksRegion, drawnOrder(lanes.Lanes, laneHead), hidden, prefs)
}

// arrangeList groups the list's rows and arranges the groups. A list grouped
// by project shares the lanes' arrangement and one grouped by state the
// board's, since each is the same grouping drawn another way.
func (h *handler) arrangeList(r *http.Request, by string, projects []core.Project, tasks []core.Task,
	moves map[string]core.WorkflowDefinition, self string) ([]listGroup, arrangement) {
	groups := buildListGroups(by, projects, tasks, moves)
	view, name := core.ViewLanes, projectNamer(projects)
	if by == ListGroupStatus {
		defs := make([]core.WorkflowDefinition, 0, len(projects))
		for _, p := range projects {
			defs = append(defs, moves[p.ID])
		}
		view, name = core.ViewBoard, stateNamer(defs...)
	}
	prefs := h.groupPrefs(r, view)
	groups, _ = arrangeGroups(groups, listHead, groupPrefIndex(prefs))
	a := newArrangement(r, view, self, tasksRegion, drawnOrder(groups, listHead), hiddenOf(prefs, name), prefs)
	a.Vertical = true
	return groups, a
}

// arrangeProjectBoard draws a project's own board as the reader arranged it.
func (h *handler) arrangeProjectBoard(r *http.Request, p core.Project, def core.WorkflowDefinition, columns []column) ([]column, arrangement) {
	view := core.ProjectView(p.Key)
	prefs := h.groupPrefs(r, view)
	for i := range columns {
		c := &columns[i]
		c.Head = statusHead(c.State, len(c.Tasks), []core.Project{p})
	}
	columns, _ = arrangeGroups(columns, colHead, groupPrefIndex(prefs))
	self := strings.Replace(RouteProject, "{key}", p.Key, 1)
	return columns, newArrangement(r, view, self, "#project-board", drawnOrder(columns, colHead), hiddenOf(prefs, stateNamer(def)), prefs)
}

// taskRow is what the "task-row" partial draws: one task and the page it is
// drawn on, which the row reads its preferences from.
type taskRow struct {
	Root any
	Task core.Task
}

func newTaskRow(root any, t core.Task) taskRow { return taskRow{Root: root, Task: t} }
