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
	ID    string `json:"id" yaml:"id"`
	Name  string `json:"name" yaml:"name"`
	Query string `json:"query" yaml:"query"`
	View  string `json:"view,omitempty" yaml:"view,omitempty"`
	Sort  string `json:"sort,omitempty" yaml:"sort,omitempty"`
	// Shared lists the filter for every reader of the tenant. Only its owner
	// and an administrator may change it.
	Shared  bool   `json:"shared" yaml:"shared"`
	OwnerID string `json:"owner_id" yaml:"owner_id"`
	// Position is where the owner put the filter among their own, from 1.
	Position int `json:"position" yaml:"position"`
	// Mine reports whether the reader owns the filter, and Editable whether
	// the reader may change it. Both are answered for the caller.
	Mine      bool      `json:"mine" yaml:"mine"`
	Editable  bool      `json:"editable" yaml:"editable"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// SavedFilterInput creates a saved filter.
type SavedFilterInput struct {
	Name   string `json:"name" yaml:"name"`
	Query  string `json:"query" yaml:"query"`
	View   string `json:"view,omitempty" yaml:"view,omitempty"`
	Sort   string `json:"sort,omitempty" yaml:"sort,omitempty"`
	Shared bool   `json:"shared,omitempty" yaml:"shared,omitempty"`
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
	Name   *string `json:"name,omitempty" yaml:"name,omitempty"`
	Query  *string `json:"query,omitempty" yaml:"query,omitempty"`
	View   *string `json:"view,omitempty" yaml:"view,omitempty"`
	Sort   *string `json:"sort,omitempty" yaml:"sort,omitempty"`
	Shared *bool   `json:"shared,omitempty" yaml:"shared,omitempty"`
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
	// GetSavedFilter returns one filter the caller can see, by id or name.
	GetSavedFilter(ctx context.Context, ref string) (*SavedFilter, error)
	// CreateSavedFilter saves a new filter for the caller.
	CreateSavedFilter(ctx context.Context, in SavedFilterInput) (*SavedFilter, error)
	// UpdateSavedFilter renames, edits or shares a filter.
	UpdateSavedFilter(ctx context.Context, ref string, in SavedFilterUpdate) (*SavedFilter, error)
	// OrderSavedFilters puts the caller's named filters first, in that order.
	OrderSavedFilters(ctx context.Context, in SavedFilterOrderInput) ([]SavedFilter, error)
	// DeleteSavedFilter removes a filter.
	DeleteSavedFilter(ctx context.Context, ref string) error
}
