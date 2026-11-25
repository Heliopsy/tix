// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// LanesParam names what each lane is in the address, beside ViewParam, so a
// link to lanes by assignee opens on lanes by assignee.
const LanesParam = "lanes"

// LanesGroupCookie remembers the task list's last lanes grouping, the way
// TaskViewCookie remembers its view. A project page neither reads nor writes
// it: that page does not read the remembered view either, and the remembered
// Project would draw it one lane.
const LanesGroupCookie = "tix_lanes_group"

// laneGroupLabels name each grouping on the control.
var laneGroupLabels = map[string]string{
	core.LanesByProject:  "Project",
	core.LanesByAssignee: "Assignee",
	core.LanesByPriority: "Priority",
	core.LanesByTag:      "Tag",
	core.LanesByDue:      "Due",
}

// laneNoneLabels name the lane of the tasks with no assignee or no tag.
var laneNoneLabels = map[string]string{
	core.LanesByAssignee: "Unassigned",
	core.LanesByTag:      "No tag",
}

// lanesOffered reports whether a screen offers a grouping: every one on the
// task list, every one but Project on a project page.
func lanesOffered(by string, scope *taskScope) bool {
	if _, ok := laneGroupLabels[by]; !ok {
		return false
	}
	return scope == nil || by != core.LanesByProject
}

// lanesParam is the grouping the address names, or "" for none this screen
// offers.
func lanesParam(r *http.Request, scope *taskScope) string {
	if v := r.URL.Query().Get(LanesParam); lanesOffered(v, scope) {
		return v
	}
	return ""
}

// lanesGroupOf is what each lane is: the address first, then on the task
// list the remembered choice and Project, and on a project page Assignee.
func lanesGroupOf(r *http.Request, scope *taskScope) string {
	if v := lanesParam(r, scope); v != "" {
		return v
	}
	if scope != nil {
		return core.LanesByAssignee
	}
	if v := cookieValue(r, LanesGroupCookie); lanesOffered(v, nil) {
		return v
	}
	return core.LanesByProject
}

// rememberLanesGroup stores the grouping a task list address names. Project
// is the default and stores nothing.
func (h *handler) rememberLanesGroup(w http.ResponseWriter, r *http.Request) {
	v := lanesParam(r, nil)
	if v == "" {
		return
	}
	value, age := v, cookieYear
	if v == core.LanesByProject {
		value, age = "", -1
	}
	if cookieValue(r, LanesGroupCookie) == value {
		return
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: LanesGroupCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
}

// lanesField is one hidden field the grouping form carries, so choosing a
// grouping keeps everything else the address said.
type lanesField struct {
	Name  string
	Value string
}

// lanesControl is the Group lanes by form: where it submits, what it keeps,
// and what it offers.
type lanesControl struct {
	Route   string
	Keep    []lanesField
	Choices []taskViewChoice
}

// newLanesControl builds the form from the request's own query, less what a
// view switch link leaves behind and less the grouping it replaces.
func newLanesControl(r *http.Request, route, current string, scope *taskScope) lanesControl {
	q := r.URL.Query()
	for _, k := range viewDropped {
		q.Del(k)
	}
	q.Del(LanesParam)
	q.Set(ViewParam, TaskViewLanes)
	out := lanesControl{Route: route}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range q[k] {
			out.Keep = append(out.Keep, lanesField{Name: k, Value: v})
		}
	}
	for _, by := range core.LaneGroupings {
		if lanesOffered(by, scope) {
			out.Choices = append(out.Choices, taskViewChoice{Value: by, Label: laneGroupLabels[by], Selected: by == current})
		}
	}
	return out
}

// laneSource is what grouping a page into lanes other than projects needs.
type laneSource struct {
	By    string
	Route string
	Tasks []core.Task
	Moves laneMoves
	// Filter is the parsed filter, read for the people, priorities and tags
	// it names, which get a lane even when empty.
	Filter core.TaskFilter
	// Named are the actor ids the filter's assignee terms resolve to.
	Named []string
	// Names labels the assignees.
	Names actorNames
	// Bucket places a task in its deadline bucket.
	Bucket func(core.Task) string
	// Label translates a fixed heading.
	Label func(string) string
	// Prefs is the reader's arrangement of this grouping, which decides the
	// lane a card with several tags is drawn in.
	Prefs map[string]core.GroupPref
}

// laneSlot is one lane being built before its cards are known.
type laneSlot struct {
	key, name, term string
	named           bool
}

// buildGroupedLanes groups one page into lanes by person, priority, tag or
// deadline. A lane is drawn when a card lands in it or the filter names it;
// a deadline bucket only when a card lands in it.
func buildGroupedLanes(src laneSource) taskLanes {
	slots, place := laneSlots(src)
	cards := make(map[string][]core.Task, len(slots))
	for _, t := range src.Tasks {
		if key, ok := place(t); ok {
			cards[strings.ToLower(key)] = append(cards[strings.ToLower(key)], t)
		}
	}
	out := taskLanes{Write: src.Moves.HasScope != nil && src.Moves.HasScope(core.ScopeTaskWrite)}
	for _, s := range slots {
		tasks := cards[strings.ToLower(s.key)]
		if len(tasks) == 0 && !s.named {
			continue
		}
		l := lane{Named: s.named}
		for _, t := range tasks {
			def, ok := src.Moves.Workflows[t.ProjectID]
			c := newLaneCard(t, def, ok, src.Moves.HasScope)
			if !c.Finished {
				l.Open++
			}
			l.Cards = append(l.Cards, c)
		}
		l.Head = groupHead{Key: s.key, Name: s.name, Count: l.Open, Kind: src.By, Filter: laneFilter(src.Route, src.By, s.term)}
		out.Lanes = append(out.Lanes, l)
	}
	return out
}

// laneFilter is the screen narrowed to one lane's tasks, or "" for a lane no
// filter term says exactly.
func laneFilter(route, by, term string) string {
	if term == "" {
		return ""
	}
	q := url.Values{"q": {term}, ViewParam: {TaskViewLanes}, LanesParam: {by}}
	return route + "?" + q.Encode()
}

// laneSlots are the lanes a grouping can draw, in the order it draws them
// before the reader's arrangement, and where each task goes.
func laneSlots(src laneSource) ([]laneSlot, func(core.Task) (string, bool)) {
	switch src.By {
	case core.LanesByAssignee:
		return assigneeSlots(src)
	case core.LanesByPriority:
		return prioritySlots(src)
	case core.LanesByTag:
		return tagSlots(src)
	default:
		var slots []laneSlot
		for _, b := range core.DueBuckets {
			slots = append(slots, laneSlot{key: b, name: src.Label(dueBucketLabels[b])})
		}
		return slots, func(t core.Task) (string, bool) { return src.Bucket(t), true }
	}
}

// assigneeSlots are a lane per person on the page or named by the filter, A
// to Z by handle, and Unassigned last. A lane is keyed by the handle, which
// is what the arrangement names it by.
func assigneeSlots(src laneSource) ([]laneSlot, func(core.Task) (string, bool)) {
	named := map[string]bool{}
	for _, id := range src.Named {
		named[id] = true
	}
	key := func(id string) string {
		if id == "" {
			return core.GroupNone
		}
		if h := src.Names.labels[id]; h != "" {
			return h
		}
		return id
	}
	seen := map[string]bool{}
	var people []laneSlot
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		k := key(id)
		people = append(people, laneSlot{key: k, name: src.Names.Label(id), term: "assignee:" + k, named: named[id]})
	}
	for _, id := range src.Named {
		add(id)
	}
	for _, t := range src.Tasks {
		add(t.AssigneeActorID)
	}
	sort.SliceStable(people, func(a, b int) bool {
		return strings.ToLower(people[a].name) < strings.ToLower(people[b].name)
	})
	people = append(people, laneSlot{key: core.GroupNone, name: src.Label(laneNoneLabels[core.LanesByAssignee])})
	return people, func(t core.Task) (string, bool) { return key(t.AssigneeActorID), true }
}

// prioritySlots are the priorities, highest first, named ones drawn empty.
func prioritySlots(src laneSource) ([]laneSlot, func(core.Task) (string, bool)) {
	var slots []laneSlot
	for _, c := range priorityChoices {
		slots = append(slots, laneSlot{key: c.Label, name: src.Label(c.Label), term: "priority:" + c.Label,
			named: slices.Contains(src.Filter.Priorities, c.Value)})
	}
	return slots, func(t core.Task) (string, bool) { return t.Priority.Key(), t.Priority.Valid() }
}

// tagSlots are a lane per tag on the page or named by the filter, the named
// ones first in the filter's order, then A to Z, and No tag last. A card is
// drawn once, in the first lane among its tags that the reader's arrangement
// draws, so the lanes add up to the page and every element id stays unique.
func tagSlots(src laneSource) ([]laneSlot, func(core.Task) (string, bool)) {
	seen := map[string]bool{}
	var named, rest []laneSlot
	for _, tag := range src.Filter.Tags {
		if k := strings.ToLower(tag); !seen[k] {
			seen[k] = true
			named = append(named, laneSlot{key: tag, name: tag, term: "tag:" + tag, named: true})
		}
	}
	for _, t := range src.Tasks {
		for _, tag := range t.Tags {
			if k := strings.ToLower(tag); !seen[k] {
				seen[k] = true
				rest = append(rest, laneSlot{key: tag, name: tag, term: "tag:" + tag})
			}
		}
	}
	sort.SliceStable(rest, func(a, b int) bool { return strings.ToLower(rest[a].key) < strings.ToLower(rest[b].key) })
	slots := append(append(named, rest...), laneSlot{key: core.GroupNone, name: src.Label(laneNoneLabels[core.LanesByTag])})
	keys := make([]string, len(slots))
	for i, s := range slots {
		keys[i] = s.key
	}
	rank := map[string]int{}
	for at, i := range arrangeOrder(keys, src.Prefs) {
		k := strings.ToLower(keys[i])
		if !src.Prefs[k].Hidden {
			rank[k] = at + 1
		}
	}
	return slots, func(t core.Task) (string, bool) {
		if len(t.Tags) == 0 {
			return core.GroupNone, true
		}
		best, at := "", 0
		for _, tag := range t.Tags {
			if r := rank[strings.ToLower(tag)]; r > 0 && (at == 0 || r < at) {
				best, at = tag, r
			}
		}
		return best, at > 0
	}
}

// laneNamer names the hidden lanes of a grouping other than projects. A
// person is named by the handle the arrangement already holds.
func laneNamer(by string, label func(string) string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		switch {
		case key == core.GroupNone && laneNoneLabels[by] != "":
			return label(laneNoneLabels[by]), true
		case by == core.LanesByPriority:
			return label(key), slices.Contains(core.PriorityKeys, key)
		case by == core.LanesByDue:
			text, ok := dueBucketLabels[key]
			return label(text), ok
		}
		return key, true
	}
}

// namedAssignees resolves the filter's assignee terms to actor ids, so a
// person the reader asked for gets a lane even with nothing on the page. A
// term naming nobody is the filter's business, not the lanes'.
func (h *handler) namedAssignees(r *http.Request, f core.TaskFilter) []string {
	var out []string
	for _, ref := range f.AssigneeIDs {
		if strings.EqualFold(strings.TrimSpace(ref), core.MeRef) {
			if a, ok := core.ActorFrom(r.Context()); ok {
				out = append(out, a.ID)
			}
			continue
		}
		if a, err := h.svc.GetActor(r.Context(), ref); err == nil && a != nil {
			out = append(out, a.ID)
		}
	}
	return out
}
