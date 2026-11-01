// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// The saved filters of the fake: one reader's list, held in memory, and what
// the interface asked to keep.

func (f *fakeService) ListSavedFilters(context.Context) ([]core.SavedFilter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.SavedFilter(nil), f.savedFilters...), f.savedErr
}

func (f *fakeService) GetSavedFilter(_ context.Context, ref string) (*core.SavedFilter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, sf := range f.savedFilters {
		if sf.ID == ref || sf.Name == ref {
			out := sf
			return &out, nil
		}
	}
	return nil, core.NotFound("no saved filter named %q", ref)
}

func (f *fakeService) CreateSavedFilter(_ context.Context, in core.SavedFilterInput) (*core.SavedFilter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.savedInputs = append(f.savedInputs, in)
	if f.savedErr != nil {
		return nil, f.savedErr
	}
	sf := core.SavedFilter{ID: in.Name, Name: in.Name, Query: in.Query, Mine: true, Editable: true}
	f.savedFilters = append(f.savedFilters, sf)
	return &sf, nil
}

func (f *fakeService) UpdateSavedFilter(context.Context, string, core.SavedFilterUpdate) (*core.SavedFilter, error) {
	return nil, core.Invalid("the fake does not edit saved filters")
}

func (f *fakeService) OrderSavedFilters(context.Context, core.SavedFilterOrderInput) ([]core.SavedFilter, error) {
	return nil, core.Invalid("the fake does not order saved filters")
}

func (f *fakeService) DeleteSavedFilter(context.Context, string) error {
	return core.Invalid("the fake does not delete saved filters")
}
