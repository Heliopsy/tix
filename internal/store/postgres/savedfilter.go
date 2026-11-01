// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var savedFilterCols = []string{
	"saved_filters.id", "saved_filters.actor_id", "saved_filters.name", "saved_filters.query",
	"saved_filters.view_name", "saved_filters.sort_field", "saved_filters.shared",
	"saved_filters.sort_order", "saved_filters.created_at", "saved_filters.updated_at",
}

// SavedFilters lists the filters an actor owns and the ones shared with the
// tenant, by owner position and then by identifier.
func (t *tx) SavedFilters(ctx context.Context, actorID string) ([]core.SavedFilter, error) {
	b := t.builder("saved_filters").
		Select(savedFilterCols...).
		Where("(saved_filters.actor_id = ? OR saved_filters.shared = 1)", actorID).
		OrderBy("saved_filters.id", core.Ascending).
		Limit(store.MaxListedFilters)
	rows, err := t.query(ctx, b, "reading saved filters")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []core.SavedFilter
	for rows.Next() {
		var f core.SavedFilter
		var shared, order int64
		var created, updated sql.NullTime
		if err := rows.Scan(&f.ID, &f.OwnerID, &f.Name, &f.Query, &f.View, &f.Sort,
			&shared, &order, &created, &updated); err != nil {
			return nil, mapErr(err, "scanning saved filter")
		}
		f.Shared, f.Position = sqlb.Bool(shared), int(order)
		f.CreatedAt, f.UpdatedAt = scanTime(created), scanTime(updated)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err, "reading saved filters")
	}
	return out, nil
}

// CreateSavedFilter inserts a filter, assigning its identifier and times.
func (t *tx) CreateSavedFilter(ctx context.Context, f *core.SavedFilter) error {
	if f.ID == "" {
		f.ID = id.New()
	}
	now := t.store.clock.Now().UTC()
	f.CreatedAt, f.UpdatedAt = now, now
	ins := t.insert("saved_filters").
		Set("id", f.ID).
		Set("actor_id", f.OwnerID).
		Set("name", f.Name).
		Set("query", f.Query).
		Set("view_name", f.View).
		Set("sort_field", f.Sort).
		Set("shared", sqlb.BoolInt(f.Shared)).
		Set("sort_order", f.Position).
		Set("created_at", timeArg(now)).
		Set("updated_at", timeArg(now))
	_, err := t.execInsert(ctx, ins, "saving filter %q", f.Name)
	return err
}

// UpdateSavedFilter writes every changeable column of a filter.
func (t *tx) UpdateSavedFilter(ctx context.Context, f core.SavedFilter) error {
	up := t.builder("saved_filters").
		Where("saved_filters.id = ?", f.ID).
		Set("name", f.Name).
		Set("query", f.Query).
		Set("view_name", f.View).
		Set("sort_field", f.Sort).
		Set("shared", sqlb.BoolInt(f.Shared)).
		Set("sort_order", f.Position).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, up, "updating saved filter %q", f.Name)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("saved filter %q", f.ID)
	}
	return nil
}

// DeleteSavedFilter removes one filter.
func (t *tx) DeleteSavedFilter(ctx context.Context, filterID string) (int64, error) {
	b := t.builder("saved_filters").Where("saved_filters.id = ?", filterID)
	return t.execDelete(ctx, b, "deleting saved filter %q", filterID)
}
