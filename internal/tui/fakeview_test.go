// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// The view arrangement of the fake: one reader's preferences, by view, held
// in memory the way the service holds them in a table.

func (f *fakeService) ListGroupPrefs(_ context.Context, view string) ([]core.GroupPref, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.GroupPref(nil), f.groupPrefs[view]...), nil
}

func (f *fakeService) SetGroupPref(_ context.Context, view, group string, in core.GroupPrefInput) (*core.GroupPref, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.groupPrefs == nil {
		f.groupPrefs = map[string][]core.GroupPref{}
	}
	prefs := f.groupPrefs[view]
	for i := range prefs {
		if prefs[i].Group == group {
			prefs[i] = in.Apply(prefs[i])
			p := prefs[i]
			return &p, nil
		}
	}
	p := in.Apply(core.GroupPref{View: view, Group: group})
	f.groupPrefs[view] = append(prefs, p)
	return &p, nil
}

func (f *fakeService) OrderGroups(_ context.Context, view string, in core.GroupOrderInput) ([]core.GroupPref, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.groupPrefs == nil {
		f.groupPrefs = map[string][]core.GroupPref{}
	}
	var out []core.GroupPref
	for i, group := range in.Groups {
		found := false
		for j := range f.groupPrefs[view] {
			if f.groupPrefs[view][j].Group == group {
				f.groupPrefs[view][j].Position = i + 1
				out = append(out, f.groupPrefs[view][j])
				found = true
			}
		}
		if !found {
			p := core.GroupPref{View: view, Group: group, Position: i + 1}
			f.groupPrefs[view] = append(f.groupPrefs[view], p)
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeService) ResetGroupPrefs(_ context.Context, view, group string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if group == "" {
		delete(f.groupPrefs, view)
		return nil
	}
	kept := f.groupPrefs[view][:0]
	for _, p := range f.groupPrefs[view] {
		if p.Group != group {
			kept = append(kept, p)
		}
	}
	f.groupPrefs[view] = kept
	return nil
}
