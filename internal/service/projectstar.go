// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"math"
	"slices"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// A project star is the Starred flag of the caller's own group preference for
// the project on the lanes view. Pinning a project to the sidebar wrote that
// flag before stars had a name, so every pin is a star with nothing migrated.

// ListStarredProjects lists the caller's starred projects in the lanes' order.
func (l *Local) ListStarredProjects(ctx context.Context) ([]core.Project, error) {
	actor, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var found []core.Project
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		prefs, err := tx.GroupPrefs(ctx, actor.ID, core.ViewLanes)
		if err != nil {
			return err
		}
		prefs = slices.Clone(prefs)
		slices.SortStableFunc(prefs, func(a, b core.GroupPref) int { return laneRank(a) - laneRank(b) })
		for _, pref := range prefs {
			if !pref.Starred {
				continue
			}
			p, err := tx.GetProject(ctx, pref.Group)
			if core.IsKind(err, core.KindNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			p.Starred = true
			found = append(found, *p)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	out := []core.Project{}
	for _, p := range found {
		if _, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{ProjectID: p.ID}); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// laneRank orders lane preferences the way the lanes draw them: those the
// reader moved, by where they moved them, then the rest as stored.
func laneRank(p core.GroupPref) int {
	if p.Position > 0 {
		return p.Position
	}
	return math.MaxInt32
}

// StarProject sets the caller's own star on a project.
func (l *Local) StarProject(ctx context.Context, ref string) error {
	return l.setProjectStar(ctx, ref, true)
}

// UnstarProject clears the caller's own star on a project.
func (l *Local) UnstarProject(ctx context.Context, ref string) error {
	return l.setProjectStar(ctx, ref, false)
}

// setProjectStar writes the star the way a lane star is written, with the same
// audit entry and event, under the authority of reading the project rather
// than arranging a view, so a token that may read projects may star them.
func (l *Local) setProjectStar(ctx context.Context, ref string, starred bool) error {
	actor, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		p, err := lookupProject(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{ProjectID: p.ID}); err != nil {
			return err
		}
		prefs, err := m.tx.GroupPrefs(ctx, actor.ID, core.ViewLanes)
		if err != nil {
			return err
		}
		before := core.GroupPref{View: core.ViewLanes, Group: p.ID}
		for _, pref := range prefs {
			if pref.Group == p.ID {
				before = pref
			}
		}
		after := core.GroupPrefInput{Starred: &starred}.Apply(before)
		if after == before {
			return nil
		}
		if err := saveOrForget(ctx, m.tx, actor.ID, after); err != nil {
			return err
		}
		return m.Record(auditViewGroup, core.EventViewArranged, "view", core.ViewLanes, p.ID,
			plainOrNil(before), plainOrNil(after), map[string]any{"view": core.ViewLanes, "groups": []string{p.Key}})
	})
}

// markProjectsStarred sets each project's Starred from the reader's own stars.
func markProjectsStarred(ctx context.Context, tx store.Tx, actorID string, projects []core.Project) error {
	if actorID == "" || len(projects) == 0 {
		return nil
	}
	prefs, err := tx.GroupPrefs(ctx, actorID, core.ViewLanes)
	if err != nil {
		return err
	}
	starred := make(map[string]bool, len(prefs))
	for _, p := range prefs {
		starred[p.Group] = p.Starred
	}
	for i := range projects {
		projects[i].Starred = starred[projects[i].ID]
	}
	return nil
}
