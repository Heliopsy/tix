// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// maxGroupPrefs bounds one actor's preferences on one view. A view draws a
// workflow's states or a tenant's projects, so the bound is never reached by
// a reader, only by a script writing nonsense.
const maxGroupPrefs = 5000

// GroupPrefs lists an actor's preferences on one view, by group.
func (t *tx) GroupPrefs(ctx context.Context, actorID, view string) ([]core.GroupPref, error) {
	b := t.builder("group_prefs").
		Select("group_prefs.group_key", "group_prefs.hidden", "group_prefs.collapsed",
			"group_prefs.starred", "group_prefs.sort_order").
		Where("group_prefs.actor_id = ?", actorID).
		Where("group_prefs.view_name = ?", view).
		OrderBy("group_prefs.group_key", core.Ascending).
		Limit(maxGroupPrefs)
	rows, err := t.query(ctx, b, "reading group preferences on %q", view)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []core.GroupPref
	for rows.Next() {
		var hidden, collapsed, starred, order int64
		p := core.GroupPref{View: view}
		if err := rows.Scan(&p.Group, &hidden, &collapsed, &starred, &order); err != nil {
			return nil, mapErr(err, "scanning group preference")
		}
		p.Hidden, p.Collapsed, p.Starred, p.Position = sqlb.Bool(hidden), sqlb.Bool(collapsed), sqlb.Bool(starred), int(order)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err, "reading group preferences on %q", view)
	}
	return out, nil
}

// SaveGroupPref inserts or replaces an actor's preference on one group. It
// reads before it writes rather than swallowing a conflict, because on
// PostgreSQL a failed insert aborts the transaction the audit entry is in.
func (t *tx) SaveGroupPref(ctx context.Context, actorID string, p core.GroupPref) error {
	n, err := t.count(ctx, t.groupPrefRow(actorID, p.View, p.Group), "reading group preference %q", p.Group)
	if err != nil {
		return err
	}
	if n > 0 {
		up := t.groupPrefRow(actorID, p.View, p.Group).
			Set("hidden", sqlb.BoolInt(p.Hidden)).
			Set("collapsed", sqlb.BoolInt(p.Collapsed)).
			Set("starred", sqlb.BoolInt(p.Starred)).
			Set("sort_order", p.Position).
			Set("updated_at", t.now())
		_, err := t.execUpdate(ctx, up, "saving group preference %q", p.Group)
		return err
	}
	ins := t.insert("group_prefs").
		Set("actor_id", actorID).
		Set("view_name", p.View).
		Set("group_key", p.Group).
		Set("hidden", sqlb.BoolInt(p.Hidden)).
		Set("collapsed", sqlb.BoolInt(p.Collapsed)).
		Set("starred", sqlb.BoolInt(p.Starred)).
		Set("sort_order", p.Position).
		Set("updated_at", t.now())
	_, err = t.execInsert(ctx, ins, "saving group preference %q", p.Group)
	return err
}

// DeleteGroupPrefs removes an actor's preference on one group, or on every
// group of the view when group is empty.
func (t *tx) DeleteGroupPrefs(ctx context.Context, actorID, view, group string) (int64, error) {
	b := t.builder("group_prefs").
		Where("group_prefs.actor_id = ?", actorID).
		Where("group_prefs.view_name = ?", view)
	if group != "" {
		b.Where("group_prefs.group_key = ?", group)
	}
	return t.execDelete(ctx, b, "forgetting group preferences on %q", view)
}

func (t *tx) groupPrefRow(actorID, view, group string) *sqlb.Builder {
	return t.builder("group_prefs").
		Where("group_prefs.actor_id = ?", actorID).
		Where("group_prefs.view_name = ?", view).
		Where("group_prefs.group_key = ?", group)
}
