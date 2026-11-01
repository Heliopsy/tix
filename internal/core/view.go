// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"strings"
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
)

// MaxGroupsOrdered bounds one OrderGroups call.
const MaxGroupsOrdered = 500

// GroupPref is one reader's arrangement of one group on one view: whether it
// is hidden, collapsed to a strip or starred to the front, and where it was
// moved to. It belongs to the actor who set it and changes nothing anybody
// else sees.
type GroupPref struct {
	View      string `json:"view" yaml:"view"`
	Group     string `json:"group" yaml:"group"`
	Hidden    bool   `json:"hidden" yaml:"hidden"`
	Collapsed bool   `json:"collapsed" yaml:"collapsed"`
	Starred   bool   `json:"starred" yaml:"starred"`
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
	Hidden    *bool `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	Collapsed *bool `json:"collapsed,omitempty" yaml:"collapsed,omitempty"`
	Starred   *bool `json:"starred,omitempty" yaml:"starred,omitempty"`
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
	Groups []string `json:"groups" yaml:"groups"`
}

// ProjectView is the view of one project's own board.
func ProjectView(projectRef string) string { return ViewProjectPrefix + projectRef }

// SplitView reads a view name: the task screen's two views, or a project's
// board with the project reference it names.
func SplitView(view string) (kind, project string, err error) {
	view = strings.TrimSpace(view)
	switch {
	case view == ViewBoard || view == ViewLanes:
		return view, "", nil
	case strings.HasPrefix(view, ViewProjectPrefix):
		ref := strings.TrimSpace(strings.TrimPrefix(view, ViewProjectPrefix))
		if ref == "" {
			return "", "", Invalid("view %q names no project", view)
		}
		return ViewProjectPrefix, ref, nil
	default:
		return "", "", Invalid("unknown view %q: use %s, %s or %sKEY", view, ViewBoard, ViewLanes, ViewProjectPrefix)
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
