// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"slices"
	"strings"
	"time"
)

// Views whose groups a reader may arrange. A view names a grouping, not a
// drawing: the task screen's board and its list grouped by status share
// ViewBoard, and its lanes and its list grouped by project share ViewLanes,
// so hiding a group in one drawing hides it in the other.
const (
	// ViewBoard is the task screen grouped by status. Its groups are state keys.
	ViewBoard = "board"
	// ViewLanes is the task screen grouped by project. Its groups are projects.
	ViewLanes = "lanes"
	// ViewProjectPrefix starts the view of one project's own board, as in
	// "project:infra". Its groups are the states of that project's workflow.
	ViewProjectPrefix = "project:"
	// ViewMine is the reader's own work grouped by deadline. Its groups are
	// the DueBuckets.
	ViewMine = "mine"
	// ViewLanesPrefix starts the view of the lanes grouped by something other
	// than the project, as in "lanes:assignee".
	ViewLanesPrefix = "lanes:"
)

// What a lane can be. LanesByProject is the original grouping and keeps the
// ViewLanes arrangement; each other one is arranged under LanesView of it.
const (
	LanesByProject  = "project"
	LanesByAssignee = "assignee"
	LanesByPriority = "priority"
	LanesByTag      = "tag"
	LanesByDue      = "due"
)

// LaneGroupings lists what a lane can be, in the order a control offers them.
var LaneGroupings = []string{LanesByProject, LanesByAssignee, LanesByPriority, LanesByTag, LanesByDue}

// GroupNone names the lane of the tasks with no assignee or no tag.
const GroupNone = "-"

// LanesView is the view the lanes grouped by by are arranged under: ViewLanes
// for projects, so an arrangement made before other groupings existed stays.
func LanesView(by string) string {
	if by == LanesByProject || by == "" {
		return ViewLanes
	}
	return ViewLanesPrefix + by
}

// PriorityKeys names the priorities, highest first, as a lane or a filter
// term spells them.
var PriorityKeys = []string{"highest", "high", "normal", "low", "lowest"}

// Key is the priority's name, or "" when it is out of range.
func (p Priority) Key() string {
	if !p.Valid() {
		return ""
	}
	return PriorityKeys[p-PriorityHighest]
}

// laneView reports whether view is one of the lanes:BY views.
func laneView(view string) bool {
	by, ok := strings.CutPrefix(view, ViewLanesPrefix)
	return ok && by != LanesByProject && slices.Contains(LaneGroupings, by)
}

// Deadline buckets: the groups of ViewMine, in the order it draws them.
const (
	DueBucketOverdue = "overdue"
	DueBucketToday   = "today"
	DueBucketWeek    = "week"
	DueBucketLater   = "later"
	DueBucketNone    = "none"
)

// DueBuckets lists the deadline buckets in drawing order.
var DueBuckets = []string{DueBucketOverdue, DueBucketToday, DueBucketWeek, DueBucketLater, DueBucketNone}

// DueBucketOf places a deadline in its bucket, read in loc: overdue before
// now, today until the end of the day, this week within seven days, later
// after that, and none without a deadline.
func DueBucketOf(due *time.Time, now time.Time, loc *time.Location) string {
	if due == nil {
		return DueBucketNone
	}
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	endOfDay := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
	switch {
	case due.Before(now):
		return DueBucketOverdue
	case due.Before(endOfDay):
		return DueBucketToday
	case due.Before(now.AddDate(0, 0, 7)):
		return DueBucketWeek
	default:
		return DueBucketLater
	}
}

// MaxGroupsOrdered bounds one OrderGroups call.
const MaxGroupsOrdered = 500

// GroupPref is one reader's arrangement of one group on one view: whether it
// is hidden, collapsed to a strip or starred to the front, and where it was
// moved to. It belongs to the actor who set it and changes nothing anybody
// else sees.
type GroupPref struct {
	// View names the view: board, lanes, lanes:assignee, lanes:priority,
	// lanes:tag, lanes:due, mine or project:KEY.
	View string `json:"view" yaml:"view"`
	// Group names the group on that view: a state key on a board, a project
	// key on lanes, a handle, priority, tag or deadline bucket on the other
	// lanes (- for no assignee or no tag), or a deadline bucket on mine.
	Group string `json:"group" yaml:"group"`
	// Hidden leaves the group out of the view.
	Hidden bool `json:"hidden" yaml:"hidden"`
	// Collapsed draws the group as a narrow strip.
	Collapsed bool `json:"collapsed" yaml:"collapsed"`
	// Starred moves the group ahead of the unstarred ones.
	Starred bool `json:"starred" yaml:"starred"`
	// Position is the place the reader moved the group to, from 1, or 0 when
	// the group keeps the order the view draws it in.
	Position int `json:"position,omitempty" yaml:"position,omitempty"`
}

// Plain reports whether the preference says nothing, which is the state a
// group is in when the reader never touched it.
func (p GroupPref) Plain() bool {
	return !p.Hidden && !p.Collapsed && !p.Starred && p.Position == 0
}

// GroupPrefInput changes one group's preference. A nil field is left as it is.
type GroupPrefInput struct {
	// Hidden hides or shows the group.
	Hidden *bool `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	// Collapsed collapses or expands the group.
	Collapsed *bool `json:"collapsed,omitempty" yaml:"collapsed,omitempty"`
	// Starred stars or unstars the group.
	Starred *bool `json:"starred,omitempty" yaml:"starred,omitempty"`
}

// Validate checks that the input changes something.
func (in GroupPrefInput) Validate() error {
	if in.Hidden == nil && in.Collapsed == nil && in.Starred == nil {
		return Invalid("nothing to change: set hidden, collapsed or starred")
	}
	return nil
}

// Apply returns p with the input's fields set.
func (in GroupPrefInput) Apply(p GroupPref) GroupPref {
	if in.Hidden != nil {
		p.Hidden = *in.Hidden
	}
	if in.Collapsed != nil {
		p.Collapsed = *in.Collapsed
	}
	if in.Starred != nil {
		p.Starred = *in.Starred
	}
	return p
}

// GroupOrderInput is the order a reader put a view's groups in.
type GroupOrderInput struct {
	// Groups names the groups to put first, in that order: at most 500, each
	// once, spelled as GroupPref.Group is.
	Groups []string `json:"groups" yaml:"groups"`
}

// ProjectView is the view of one project's own board.
func ProjectView(projectRef string) string { return ViewProjectPrefix + projectRef }

// SplitView reads a view name: the task screen's two views, or a project's
// board with the project reference it names.
func SplitView(view string) (kind, project string, err error) {
	view = strings.TrimSpace(view)
	switch {
	case view == ViewBoard || view == ViewLanes || view == ViewMine || laneView(view):
		return view, "", nil
	case strings.HasPrefix(view, ViewProjectPrefix):
		ref := strings.TrimSpace(strings.TrimPrefix(view, ViewProjectPrefix))
		if ref == "" {
			return "", "", Invalid("view %q names no project", view)
		}
		return ViewProjectPrefix, ref, nil
	default:
		return "", "", Invalid("unknown view %q: use %s, %s, %s, %sassignee, %spriority, %stag, %sdue or %sKEY",
			view, ViewBoard, ViewLanes, ViewMine, ViewLanesPrefix, ViewLanesPrefix, ViewLanesPrefix, ViewLanesPrefix, ViewProjectPrefix)
	}
}

// ViewService covers each reader's own arrangement of the groups a view draws:
// the columns of a board and the lanes of the task screen.
type ViewService interface {
	// ListGroupPrefs returns the caller's own preferences on one view.
	ListGroupPrefs(ctx context.Context, view string) ([]GroupPref, error)
	// SetGroupPref hides, collapses or stars one group for the caller.
	SetGroupPref(ctx context.Context, view, group string, in GroupPrefInput) (*GroupPref, error)
	// OrderGroups puts the named groups first, in the order given.
	OrderGroups(ctx context.Context, view string, in GroupOrderInput) ([]GroupPref, error)
	// ResetGroupPrefs forgets the caller's preference on one group, or on
	// every group of the view when group is empty.
	ResetGroupPrefs(ctx context.Context, view, group string) error
}
