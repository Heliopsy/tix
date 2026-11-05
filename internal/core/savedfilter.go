// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// MeRef is the actor reference a filter term may use for whoever is reading,
// as in assignee:me. The service resolves it to the caller before a listing.
const MeRef = "me"

// The views a saved filter may open the task screen in. Empty keeps whatever
// view the reader has.
const (
	FilterViewList  = "list"
	FilterViewBoard = "board"
	FilterViewLanes = "lanes"
)

// FilterViews lists the views a saved filter may name, empty first.
var FilterViews = []string{"", FilterViewList, FilterViewBoard, FilterViewLanes}

// Bounds on saved filters.
const (
	// MaxSavedFilters is how many filters one reader may own.
	MaxSavedFilters = 100
	// MaxSavedFilterName bounds a filter's name, in characters.
	MaxSavedFilterName = 80
	// MaxSavedFilterQuery bounds a filter's expression, in bytes.
	MaxSavedFilterQuery = 2000
)

// SavedFilter is a named task filter one reader keeps: the expression, the
// view and the sort the task screen opens with. It belongs to its owner, and
// a shared one is listed for everybody in the tenant.
type SavedFilter struct {
	// ID identifies the filter.
	ID string `json:"id" yaml:"id"`
	// Name labels the filter, unique among its owner's filters ignoring case.
	Name string `json:"name" yaml:"name"`
	// Query is the task filter expression, as typed on the task screen.
	Query string `json:"query" yaml:"query"`
	// View is the view the task screen opens in: list, board or lanes, or
	// empty to keep the reader's current view.
	View string `json:"view,omitempty" yaml:"view,omitempty"`
	// Sort is the task field the listing is ordered by, empty for the default.
	Sort string `json:"sort,omitempty" yaml:"sort,omitempty"`
	// Shared lists the filter for every reader of the tenant. Only its owner
	// and an administrator may change it.
	Shared bool `json:"shared" yaml:"shared"`
	// OwnerID is the actor who owns the filter.
	OwnerID string `json:"owner_id" yaml:"owner_id"`
	// Position is where the owner put the filter among their own, from 1.
	Position int `json:"position" yaml:"position"`
	// Mine reports whether the caller owns the filter.
	Mine bool `json:"mine" yaml:"mine"`
	// Editable reports whether the caller may change the filter: its owner
	// or an administrator.
	Editable bool `json:"editable" yaml:"editable"`
	// CreatedAt is when the filter was saved, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt is when the filter last changed, RFC 3339.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// SavedFilterInput creates a saved filter.
type SavedFilterInput struct {
	// Name labels the filter: required, at most 80 characters, unique among
	// the caller's filters.
	Name string `json:"name" yaml:"name"`
	// Query is the task filter expression, at most 2000 bytes.
	Query string `json:"query" yaml:"query"`
	// View is list, board or lanes; empty keeps the reader's current view.
	View string `json:"view,omitempty" yaml:"view,omitempty"`
	// Sort is one of urgency, created_at, updated_at, priority, due_at, seq
	// or title; empty uses the default ordering.
	Sort string `json:"sort,omitempty" yaml:"sort,omitempty"`
	// Shared lists the filter for every reader of the tenant.
	Shared bool `json:"shared,omitempty" yaml:"shared,omitempty"`
}

// Normalize trims the input and checks it, returning the stored form.
func (in SavedFilterInput) Normalize() (SavedFilterInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Query = strings.TrimSpace(in.Query)
	in.View = strings.ToLower(strings.TrimSpace(in.View))
	in.Sort = strings.ToLower(strings.TrimSpace(in.Sort))
	if err := checkFilterName(in.Name); err != nil {
		return in, err
	}
	if len(in.Query) > MaxSavedFilterQuery {
		return in, Invalid("filter expression must be at most %d bytes", MaxSavedFilterQuery)
	}
	if !slices.Contains(FilterViews, in.View) {
		return in, Invalid("unknown view %q: use %s, %s or %s", in.View, FilterViewList, FilterViewBoard, FilterViewLanes)
	}
	if in.Sort != "" && !validSortField(in.Sort) {
		return in, Invalid("cannot sort tasks by %q; use one of %s", in.Sort, strings.Join(TaskSortFields, ", "))
	}
	return in, nil
}

// SavedFilterUpdate changes a saved filter. A nil field is left as it is.
type SavedFilterUpdate struct {
	// Name renames the filter, under the same rules as on creation.
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// Query replaces the filter expression.
	Query *string `json:"query,omitempty" yaml:"query,omitempty"`
	// View replaces the view; the empty string clears it.
	View *string `json:"view,omitempty" yaml:"view,omitempty"`
	// Sort replaces the ordering; the empty string restores the default.
	Sort *string `json:"sort,omitempty" yaml:"sort,omitempty"`
	// Shared shares the filter with the tenant or takes it back.
	Shared *bool `json:"shared,omitempty" yaml:"shared,omitempty"`
}

// Apply returns the filter with the update's fields set, checked.
func (in SavedFilterUpdate) Apply(f SavedFilter) (SavedFilter, error) {
	if in.Name == nil && in.Query == nil && in.View == nil && in.Sort == nil && in.Shared == nil {
		return f, Invalid("nothing to change: set name, query, view, sort or shared")
	}
	next := SavedFilterInput{Name: f.Name, Query: f.Query, View: f.View, Sort: f.Sort, Shared: f.Shared}
	if in.Name != nil {
		next.Name = *in.Name
	}
	if in.Query != nil {
		next.Query = *in.Query
	}
	if in.View != nil {
		next.View = *in.View
	}
	if in.Sort != nil {
		next.Sort = *in.Sort
	}
	if in.Shared != nil {
		next.Shared = *in.Shared
	}
	next, err := next.Normalize()
	if err != nil {
		return f, err
	}
	f.Name, f.Query, f.View, f.Sort, f.Shared = next.Name, next.Query, next.View, next.Sort, next.Shared
	return f, nil
}

// SavedFilterOrderInput is the order a reader put their own filters in.
type SavedFilterOrderInput struct {
	// Filters names the caller's own filters by id or name, in the order
	// wanted; filters left out keep their relative order after them.
	Filters []string `json:"filters" yaml:"filters"`
}

func checkFilterName(name string) error {
	if name == "" {
		return Invalid("filter name is required")
	}
	if utf8.RuneCountInString(name) > MaxSavedFilterName {
		return Invalid("filter name must be at most %d characters", MaxSavedFilterName)
	}
	return nil
}

// SavedFilterService covers each reader's named task filters.
type SavedFilterService interface {
	// ListSavedFilters returns the caller's own filters in their order, then
	// the filters others shared with the tenant, by name.
	ListSavedFilters(ctx context.Context) ([]SavedFilter, error)
	// GetSavedFilter returns one filter the caller can see, by id or name,
	// preferring the caller's own on a shared name; not_found otherwise.
	GetSavedFilter(ctx context.Context, ref string) (*SavedFilter, error)
	// CreateSavedFilter saves a new filter for the caller. A name the caller
	// already uses is a conflict, and one reader keeps at most 100 filters.
	CreateSavedFilter(ctx context.Context, in SavedFilterInput) (*SavedFilter, error)
	// UpdateSavedFilter renames, edits or shares a filter. Only its owner or an
	// administrator may change it, and a clashing name is a conflict.
	UpdateSavedFilter(ctx context.Context, ref string, in SavedFilterUpdate) (*SavedFilter, error)
	// OrderSavedFilters puts the caller's named filters first, in that order.
	OrderSavedFilters(ctx context.Context, in SavedFilterOrderInput) ([]SavedFilter, error)
	// DeleteSavedFilter removes a filter, which only its owner or an
	// administrator may do.
	DeleteSavedFilter(ctx context.Context, ref string) error
}
