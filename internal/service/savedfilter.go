// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
	"github.com/heliopsy/tix/internal/store"
)

// Audit actions of saved filters.
const (
	auditFilterSave   = "filter.save"
	auditFilterUpdate = "filter.update"
	auditFilterOrder  = "filter.order"
	auditFilterDelete = "filter.delete"
)

// savedFilters is what one reader can see, with their own filters first in
// their order and the ones others shared after, by name.
type savedFilters struct {
	all    []core.SavedFilter
	actor  *core.Actor
	manage bool
}

func (l *Local) readSavedFilters(ctx context.Context, tx store.Tx, actor *core.Actor) (savedFilters, error) {
	rows, err := tx.SavedFilters(ctx, actor.ID)
	if err != nil {
		return savedFilters{}, err
	}
	manage := l.policy.Allowed(actor, authz.ActionFilterManage, authz.Resource{TenantID: actor.TenantID})
	for i := range rows {
		rows[i].Mine = rows[i].OwnerID == actor.ID
		rows[i].Editable = rows[i].Mine || manage
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Mine != b.Mine {
			return a.Mine
		}
		if a.Mine && a.Position != b.Position {
			return a.Position < b.Position
		}
		if na, nb := strings.ToLower(a.Name), strings.ToLower(b.Name); na != nb {
			return na < nb
		}
		return a.ID < b.ID
	})
	return savedFilters{all: rows, actor: actor, manage: manage}, nil
}

// own is the reader's own filters, in their order.
func (s savedFilters) own() []core.SavedFilter {
	var out []core.SavedFilter
	for _, f := range s.all {
		if f.Mine {
			out = append(out, f)
		}
	}
	return out
}

// find resolves a reference: an identifier, then one of the reader's own
// names, then the name of a filter somebody shared.
func (s savedFilters) find(ref string) (core.SavedFilter, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return core.SavedFilter{}, core.Invalid("saved filter name or id is required")
	}
	for _, f := range s.all {
		if f.ID == ref {
			return f, nil
		}
	}
	var shared []core.SavedFilter
	for _, f := range s.all {
		if !strings.EqualFold(f.Name, ref) {
			continue
		}
		if f.Mine {
			return f, nil
		}
		shared = append(shared, f)
	}
	switch len(shared) {
	case 0:
		return core.SavedFilter{}, core.NotFound("no saved filter named %q", ref)
	case 1:
		return shared[0], nil
	default:
		return core.SavedFilter{}, core.Invalid("%d shared filters are named %q; name one by its id", len(shared), ref)
	}
}

// nameTaken reports whether an owner already has a filter of that name,
// other than the one being changed.
func nameTaken(filters []core.SavedFilter, ownerID, name, except string) bool {
	for _, f := range filters {
		if f.OwnerID == ownerID && f.ID != except && strings.EqualFold(f.Name, name) {
			return true
		}
	}
	return false
}

// checkFilterQuery refuses an expression the filter language cannot read, so
// a saved filter never opens on an error the reader did not type then.
func checkFilterQuery(expr string) error {
	if expr == "" {
		return nil
	}
	_, err := query.Parse(expr)
	return err
}

// ListSavedFilters returns the caller's own filters in their order, then the
// filters others shared with the tenant, by name.
func (l *Local) ListSavedFilters(ctx context.Context) ([]core.SavedFilter, error) {
	actor, err := l.authorize(ctx, authz.ActionFilterSave, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.SavedFilter
	err = l.read(ctx, actor, func(tx store.Tx) error {
		s, err := l.readSavedFilters(ctx, tx, actor)
		out = s.all
		return err
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []core.SavedFilter{}
	}
	return out, nil
}

// GetSavedFilter returns one filter the caller can see, by id or name.
func (l *Local) GetSavedFilter(ctx context.Context, ref string) (*core.SavedFilter, error) {
	actor, err := l.authorize(ctx, authz.ActionFilterSave, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out core.SavedFilter
	err = l.read(ctx, actor, func(tx store.Tx) error {
		s, err := l.readSavedFilters(ctx, tx, actor)
		if err != nil {
			return err
		}
		out, err = s.find(ref)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateSavedFilter saves a new filter for the caller, after their others.
func (l *Local) CreateSavedFilter(ctx context.Context, in core.SavedFilterInput) (*core.SavedFilter, error) {
	actor, err := l.authorize(ctx, authz.ActionFilterSave, authz.Resource{})
	if err != nil {
		return nil, err
	}
	in, err = in.Normalize()
	if err != nil {
		return nil, err
	}
	if err := checkFilterQuery(in.Query); err != nil {
		return nil, err
	}
	var out core.SavedFilter
	err = l.write(ctx, actor, func(m *mutation) error {
		s, err := l.readSavedFilters(ctx, m.tx, actor)
		if err != nil {
			return err
		}
		own := s.own()
		if len(own) >= core.MaxSavedFilters {
			return core.Invalid("you already keep %d saved filters, the most there may be", core.MaxSavedFilters)
		}
		if nameTaken(own, actor.ID, in.Name, "") {
			return core.Conflict("you already have a saved filter named %q", in.Name)
		}
		last := 0
		for _, f := range own {
			last = max(last, f.Position)
		}
		out = core.SavedFilter{
			Name: in.Name, Query: in.Query, View: in.View, Sort: in.Sort, Shared: in.Shared,
			OwnerID: actor.ID, Position: last + 1,
		}
		if err := m.tx.CreateSavedFilter(ctx, &out); err != nil {
			return err
		}
		out.Mine, out.Editable = true, true
		return m.Record(auditFilterSave, core.EventFilterSaved, "filter", out.ID, "",
			nil, out, filterPayload(out))
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSavedFilter renames, edits or shares a filter. The owner may change
// their own; a shared filter somebody else owns needs an administrator. A
// change that changes nothing writes nothing.
func (l *Local) UpdateSavedFilter(ctx context.Context, ref string, in core.SavedFilterUpdate) (*core.SavedFilter, error) {
	actor, err := l.authorize(ctx, authz.ActionFilterSave, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out core.SavedFilter
	err = l.write(ctx, actor, func(m *mutation) error {
		s, err := l.readSavedFilters(ctx, m.tx, actor)
		if err != nil {
			return err
		}
		before, err := s.find(ref)
		if err != nil {
			return err
		}
		if err := l.mayChange(ctx, before); err != nil {
			return err
		}
		after, err := in.Apply(before)
		if err != nil {
			return err
		}
		if after.Query != before.Query {
			if err := checkFilterQuery(after.Query); err != nil {
				return err
			}
		}
		out = after
		if after == before {
			return nil
		}
		if !strings.EqualFold(after.Name, before.Name) || after.OwnerID != actor.ID {
			owners, err := m.tx.SavedFilters(ctx, after.OwnerID)
			if err != nil {
				return err
			}
			if nameTaken(owners, after.OwnerID, after.Name, after.ID) {
				if after.OwnerID == actor.ID {
					return core.Conflict("you already have a saved filter named %q", after.Name)
				}
				return core.Conflict("its owner already has a saved filter named %q", after.Name)
			}
		}
		if err := m.tx.UpdateSavedFilter(ctx, after); err != nil {
			return err
		}
		out.UpdatedAt = m.now
		return m.Record(auditFilterUpdate, core.EventFilterUpdated, "filter", after.ID, "",
			before, after, filterPayload(after))
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// OrderSavedFilters puts the caller's named filters first, in the order
// given, and the rest of their own after them in the order they had.
func (l *Local) OrderSavedFilters(ctx context.Context, in core.SavedFilterOrderInput) ([]core.SavedFilter, error) {
	actor, err := l.authorize(ctx, authz.ActionFilterSave, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if len(in.Filters) == 0 {
		return nil, core.Invalid("name at least one saved filter to order")
	}
	if len(in.Filters) > core.MaxSavedFilters {
		return nil, core.Invalid("at most %d saved filters can be ordered", core.MaxSavedFilters)
	}
	var out []core.SavedFilter
	err = l.write(ctx, actor, func(m *mutation) error {
		s, err := l.readSavedFilters(ctx, m.tx, actor)
		if err != nil {
			return err
		}
		own := s.own()
		mine := savedFilters{all: own, actor: actor}
		seen := map[string]bool{}
		var next []core.SavedFilter
		for _, ref := range in.Filters {
			f, err := mine.find(ref)
			if err != nil {
				if core.IsKind(err, core.KindNotFound) {
					return core.NotFound("you have no saved filter named %q; only your own filters can be ordered", ref)
				}
				return err
			}
			if seen[f.ID] {
				return core.Invalid("saved filter %q is named twice", f.Name)
			}
			seen[f.ID] = true
			next = append(next, f)
		}
		for _, f := range own {
			if !seen[f.ID] {
				next = append(next, f)
			}
		}
		var before, after []core.SavedFilter
		var names []string
		for i, f := range next {
			moved := f
			moved.Position = i + 1
			out = append(out, moved)
			if moved.Position == f.Position {
				continue
			}
			if err := m.tx.UpdateSavedFilter(ctx, moved); err != nil {
				return err
			}
			before, after = append(before, f), append(after, moved)
			names = append(names, f.Name)
		}
		if len(after) == 0 {
			return nil
		}
		return m.Record(auditFilterOrder, core.EventFilterUpdated, "filter", after[0].ID, "",
			before, after, map[string]any{"names": names, "ordered": true})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteSavedFilter removes a filter the caller may change.
func (l *Local) DeleteSavedFilter(ctx context.Context, ref string) error {
	actor, err := l.authorize(ctx, authz.ActionFilterSave, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		s, err := l.readSavedFilters(ctx, m.tx, actor)
		if err != nil {
			return err
		}
		f, err := s.find(ref)
		if err != nil {
			return err
		}
		if err := l.mayChange(ctx, f); err != nil {
			return err
		}
		if _, err := m.tx.DeleteSavedFilter(ctx, f.ID); err != nil {
			return err
		}
		return m.Record(auditFilterDelete, core.EventFilterDeleted, "filter", f.ID, "",
			f, nil, filterPayload(f))
	})
}

// mayChange lets an owner change their own filter, and an administrator a
// shared one. Another reader's private filter is never found at all.
func (l *Local) mayChange(ctx context.Context, f core.SavedFilter) error {
	if f.Mine {
		return nil
	}
	if _, err := l.authorize(ctx, authz.ActionFilterManage, authz.Resource{}); err != nil {
		return core.Forbidden("saved filter %q belongs to somebody else; only its owner or an administrator may change it", f.Name)
	}
	return nil
}

func filterPayload(f core.SavedFilter) map[string]any {
	return map[string]any{"name": f.Name, "shared": f.Shared}
}
