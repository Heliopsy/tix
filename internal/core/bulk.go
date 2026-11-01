// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"slices"
	"strings"
)

// BulkAction names what a bulk task request does to each task it names.
type BulkAction string

// Bulk actions. Each is the single-task operation of the same meaning applied
// to every task in turn.
const (
	BulkMove      BulkAction = "move"
	BulkPriority  BulkAction = "priority"
	BulkTagAdd    BulkAction = "tag_add"
	BulkTagRemove BulkAction = "tag_remove"
	BulkAssign    BulkAction = "assign"
	BulkStar      BulkAction = "star"
	BulkUnstar    BulkAction = "unstar"
	BulkCopy      BulkAction = "copy"
	BulkDelete    BulkAction = "delete"
	BulkRestore   BulkAction = "restore"
)

// BulkActions lists every bulk action in the order the surfaces offer them.
var BulkActions = []BulkAction{
	BulkMove, BulkPriority, BulkTagAdd, BulkTagRemove, BulkAssign,
	BulkStar, BulkUnstar, BulkCopy, BulkDelete, BulkRestore,
}

// Valid reports whether a is a known bulk action.
func (a BulkAction) Valid() bool { return slices.Contains(BulkActions, a) }

// MaxBulkRefs bounds how many tasks one bulk request may name.
const MaxBulkRefs = 500

// BulkTaskInput is one action applied to many tasks. Only the argument the
// action needs is read: To for a move, Priority for priority, Tag for the tag
// actions, AssigneeActorID for assign (empty clears), ProjectRef for a copy
// and Cascade for a delete.
type BulkTaskInput struct {
	Action          BulkAction `json:"action" yaml:"action"`
	Refs            []string   `json:"refs" yaml:"refs"`
	To              string     `json:"to,omitempty" yaml:"to,omitempty"`
	Priority        Priority   `json:"priority,omitempty" yaml:"priority,omitempty"`
	Tag             string     `json:"tag,omitempty" yaml:"tag,omitempty"`
	AssigneeActorID string     `json:"assignee_actor_id,omitempty" yaml:"assignee_actor_id,omitempty"`
	ProjectRef      string     `json:"project_ref,omitempty" yaml:"project_ref,omitempty"`
	Cascade         bool       `json:"cascade,omitempty" yaml:"cascade,omitempty"`
}

// Normalize trims the arguments and folds repeated references, keeping the
// first occurrence of each, and returns the result validated.
func (in BulkTaskInput) Normalize() (BulkTaskInput, error) {
	seen := make(map[string]bool, len(in.Refs))
	refs := make([]string, 0, len(in.Refs))
	for _, ref := range in.Refs {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	in.Refs = refs
	in.To = strings.TrimSpace(in.To)
	in.Tag = strings.TrimSpace(in.Tag)
	in.AssigneeActorID = strings.TrimSpace(in.AssigneeActorID)
	in.ProjectRef = strings.TrimSpace(in.ProjectRef)
	return in, in.Validate()
}

// Validate checks the action, the number of references and the argument.
func (in BulkTaskInput) Validate() error {
	if !in.Action.Valid() {
		return Invalid("bulk action %q is not one of %s", in.Action, joinActions()).WithDetail(DetailField, "action")
	}
	switch {
	case len(in.Refs) == 0:
		return Invalid("a bulk action needs at least one task reference").WithDetail(DetailField, "refs")
	case len(in.Refs) > MaxBulkRefs:
		return Invalid("a bulk action names at most %d tasks, and %d were given", MaxBulkRefs, len(in.Refs)).
			WithDetail(DetailField, "refs")
	}
	switch in.Action {
	case BulkMove:
		if in.To == "" {
			return Invalid("a move needs the status to move to").WithDetail(DetailField, "to")
		}
	case BulkPriority:
		if !in.Priority.Valid() {
			return Invalid("priority must be between %d and %d", PriorityHighest, PriorityLowest).
				WithDetail(DetailField, "priority")
		}
	case BulkTagAdd, BulkTagRemove:
		if in.Tag == "" {
			return Invalid("tag name is required").WithDetail(DetailField, "tag")
		}
	case BulkCopy:
		if in.ProjectRef == "" {
			return Invalid("a copy needs the project to copy to").WithDetail(DetailField, "project_ref")
		}
	}
	return nil
}

func joinActions() string {
	names := make([]string, len(BulkActions))
	for i, a := range BulkActions {
		names[i] = string(a)
	}
	return strings.Join(names, ", ")
}

// BulkOutcome is what a bulk action did to one task.
type BulkOutcome struct {
	Ref     string `json:"ref" yaml:"ref"`
	Applied bool   `json:"applied" yaml:"applied"`
	Code    string `json:"code,omitempty" yaml:"code,omitempty"`
	Error   string `json:"error,omitempty" yaml:"error,omitempty"`
	// CopyRef names the new task a copy created.
	CopyRef string `json:"copy_ref,omitempty" yaml:"copy_ref,omitempty"`
}

// BulkResult reports a bulk action, one outcome per distinct reference in the
// order they were given.
type BulkResult struct {
	Action   BulkAction    `json:"action" yaml:"action"`
	Applied  int           `json:"applied" yaml:"applied"`
	Refused  int           `json:"refused" yaml:"refused"`
	Outcomes []BulkOutcome `json:"outcomes" yaml:"outcomes"`
}

// AppliedRefs lists the references the action landed on.
func (r BulkResult) AppliedRefs() []string {
	out := make([]string, 0, r.Applied)
	for _, o := range r.Outcomes {
		if o.Applied {
			out = append(out, o.Ref)
		}
	}
	return out
}
