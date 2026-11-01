// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Audit actions of the view arrangement.
const (
	auditViewGroup = "view.group.set"
	auditViewOrder = "view.order"
	auditViewReset = "view.reset"
)

// arrangedView is a view resolved for storage: the name it is stored under,
// the name a reader typed it as, and for a project's board the project and
// its workflow.
type arrangedView struct {
	stored  string
	shown   string
	kind    string
	project *core.Project
	def     core.WorkflowDefinition
}

// projectID is the project an event about this view belongs to, if any.
func (v arrangedView) projectID() string {
	if v.project == nil {
		return ""
	}
	return v.project.ID
}

func resolveView(ctx context.Context, tx store.Tx, view string) (arrangedView, error) {
	kind, ref, err := core.SplitView(view)
	if err != nil {
		return arrangedView{}, err
	}
	if kind != core.ViewProjectPrefix {
		return arrangedView{stored: kind, shown: kind, kind: kind}, nil
	}
	p, err := lookupProject(ctx, tx, ref)
	if err != nil {
		return arrangedView{}, err
	}
	wf, err := tx.GetWorkflowByID(ctx, p.WorkflowID)
	if err != nil {
		return arrangedView{}, err
	}
	return arrangedView{
		stored: core.ProjectView(p.ID), shown: core.ProjectView(p.Key),
		kind: kind, project: p, def: wf.Definition,
	}, nil
}

// resolveGroup turns a group a reader named into the key it is stored under
// and the key it is shown as.
func resolveGroup(ctx context.Context, tx store.Tx, v arrangedView, group string) (stored, shown string, err error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return "", "", core.Invalid("group is required")
	}
	switch v.kind {
	case core.ViewLanes:
		p, err := lookupProject(ctx, tx, group)
		if err != nil {
			return "", "", err
		}
		return p.ID, p.Key, nil
	case core.ViewProjectPrefix:
		if !v.def.HasState(group) {
			return "", "", core.Invalid("%s has no state %q", v.shown, group)
		}
		return group, group, nil
	default:
		workflows, err := tx.ListWorkflows(ctx)
		if err != nil {
			return "", "", err
		}
		for _, w := range workflows {
			if w.Definition.HasState(group) {
				return group, group, nil
			}
		}
		return "", "", core.Invalid("no workflow has a state %q", group)
	}
}

// shownGroup is how a stored group is named back to a reader. A lane whose
// project is gone keeps its identifier, so it can still be reset.
func shownGroup(ctx context.Context, tx store.Tx, v arrangedView, stored string) (string, error) {
	if v.kind != core.ViewLanes {
		return stored, nil
	}
	p, err := tx.GetProject(ctx, stored)
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return stored, nil
		}
		return "", err
	}
	return p.Key, nil
}

// shownPrefs names a view's stored preferences the way a reader typed them.
func shownPrefs(ctx context.Context, tx store.Tx, v arrangedView, prefs []core.GroupPref) ([]core.GroupPref, error) {
	out := make([]core.GroupPref, 0, len(prefs))
	for _, p := range prefs {
		name, err := shownGroup(ctx, tx, v, p.Group)
		if err != nil {
			return nil, err
		}
		p.View, p.Group = v.shown, name
		out = append(out, p)
	}
	return out, nil
}

// ListGroupPrefs returns the caller's own preferences on one view.
func (l *Local) ListGroupPrefs(ctx context.Context, view string) ([]core.GroupPref, error) {
	actor, err := l.authorize(ctx, authz.ActionViewArrange, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.GroupPref
	err = l.read(ctx, actor, func(tx store.Tx) error {
		v, err := resolveView(ctx, tx, view)
		if err != nil {
			return err
		}
		prefs, err := tx.GroupPrefs(ctx, actor.ID, v.stored)
		if err != nil {
			return err
		}
		out, err = shownPrefs(ctx, tx, v, prefs)
		return err
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []core.GroupPref{}
	}
	return out, nil
}

// SetGroupPref hides, collapses or stars one group for the caller. A change
// that leaves the group as it was writes nothing; one that leaves it plain
// removes its row.
func (l *Local) SetGroupPref(ctx context.Context, view, group string, in core.GroupPrefInput) (*core.GroupPref, error) {
	actor, err := l.authorize(ctx, authz.ActionViewArrange, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var out core.GroupPref
	err = l.write(ctx, actor, func(m *mutation) error {
		v, err := resolveView(ctx, m.tx, view)
		if err != nil {
			return err
		}
		key, name, err := resolveGroup(ctx, m.tx, v, group)
		if err != nil {
			return err
		}
		prefs, err := m.tx.GroupPrefs(ctx, actor.ID, v.stored)
		if err != nil {
			return err
		}
		before := core.GroupPref{View: v.stored, Group: key}
		for _, p := range prefs {
			if p.Group == key {
				before = p
			}
		}
		after := in.Apply(before)
		out = after
		out.View, out.Group = v.shown, name
		if after == before {
			return nil
		}
		if err := saveOrForget(ctx, m.tx, actor.ID, after); err != nil {
			return err
		}
		return m.Record(auditViewGroup, core.EventViewArranged, "view", v.stored, groupProject(v, key),
			plainOrNil(before), plainOrNil(after), map[string]any{"view": v.shown, "groups": []string{name}})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// OrderGroups puts the named groups first, in the order given, by writing
// their positions from 1. Groups not named keep the positions they had.
func (l *Local) OrderGroups(ctx context.Context, view string, in core.GroupOrderInput) ([]core.GroupPref, error) {
	actor, err := l.authorize(ctx, authz.ActionViewArrange, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if len(in.Groups) == 0 {
		return nil, core.Invalid("name at least one group to order")
	}
	if len(in.Groups) > core.MaxGroupsOrdered {
		return nil, core.Invalid("at most %d groups can be ordered at once", core.MaxGroupsOrdered)
	}
	var out []core.GroupPref
	err = l.write(ctx, actor, func(m *mutation) error {
		v, err := resolveView(ctx, m.tx, view)
		if err != nil {
			return err
		}
		prefs, err := m.tx.GroupPrefs(ctx, actor.ID, v.stored)
		if err != nil {
			return err
		}
		have := make(map[string]core.GroupPref, len(prefs))
		for _, p := range prefs {
			have[p.Group] = p
		}
		seen := map[string]bool{}
		var before, after []core.GroupPref
		var names []string
		for i, group := range in.Groups {
			key, name, err := resolveGroup(ctx, m.tx, v, group)
			if err != nil {
				return err
			}
			if seen[key] {
				return core.Invalid("group %q is named twice", name)
			}
			seen[key] = true
			cur, ok := have[key]
			if !ok {
				cur = core.GroupPref{View: v.stored, Group: key}
			}
			next := cur
			next.Position = i + 1
			shown := next
			shown.View, shown.Group = v.shown, name
			out = append(out, shown)
			if next == cur {
				continue
			}
			if err := m.tx.SaveGroupPref(ctx, actor.ID, next); err != nil {
				return err
			}
			before, after = append(before, cur), append(after, next)
			names = append(names, name)
		}
		if len(after) == 0 {
			return nil
		}
		return m.Record(auditViewOrder, core.EventViewArranged, "view", v.stored, v.projectID(),
			before, after, map[string]any{"view": v.shown, "groups": names})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ResetGroupPrefs forgets the caller's preference on one group, or on every
// group of the view when group is empty.
func (l *Local) ResetGroupPrefs(ctx context.Context, view, group string) error {
	actor, err := l.authorize(ctx, authz.ActionViewArrange, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		v, err := resolveView(ctx, m.tx, view)
		if err != nil {
			return err
		}
		key, name := "", ""
		if strings.TrimSpace(group) != "" {
			if key, name, err = resolveGroup(ctx, m.tx, v, group); err != nil {
				return err
			}
		}
		prefs, err := m.tx.GroupPrefs(ctx, actor.ID, v.stored)
		if err != nil {
			return err
		}
		var before []core.GroupPref
		var names []string
		for _, p := range prefs {
			if key != "" && p.Group != key {
				continue
			}
			before = append(before, p)
			shown, err := shownGroup(ctx, m.tx, v, p.Group)
			if err != nil {
				return err
			}
			names = append(names, shown)
		}
		if len(before) == 0 {
			return nil
		}
		if _, err := m.tx.DeleteGroupPrefs(ctx, actor.ID, v.stored, key); err != nil {
			return err
		}
		payload := map[string]any{"view": v.shown, "groups": names, "reset": true}
		if name != "" {
			payload["groups"] = []string{name}
		}
		return m.Record(auditViewReset, core.EventViewArranged, "view", v.stored, v.projectID(),
			before, nil, payload)
	})
}

// groupProject is the project an event about one group belongs to: the
// lane's own project, or the project whose board it is.
func groupProject(v arrangedView, key string) string {
	if v.kind == core.ViewLanes {
		return key
	}
	return v.projectID()
}

func saveOrForget(ctx context.Context, tx store.Tx, actorID string, p core.GroupPref) error {
	if p.Plain() {
		_, err := tx.DeleteGroupPrefs(ctx, actorID, p.View, p.Group)
		return err
	}
	return tx.SaveGroupPref(ctx, actorID, p)
}

// plainOrNil is a preference as an audit snapshot: nothing at all when it
// says nothing, which is what the store holds for it.
func plainOrNil(p core.GroupPref) any {
	if p.Plain() {
		return nil
	}
	return p
}
