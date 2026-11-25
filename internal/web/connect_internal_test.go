// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/onboard"
)

// TestDocPagesAreTheDocsDirectory holds the redirect's list to docs/, so a
// link cannot point at a page that was renamed and a new page is reachable.
func TestDocPagesAreTheDocsDirectory(t *testing.T) {
	files, err := filepath.Glob("../../docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	var have []string
	for _, f := range files {
		have = append(have, filepath.Base(f))
	}
	want := slices.Clone(docPages)
	slices.Sort(have)
	slices.Sort(want)
	if !slices.Equal(have, want) {
		t.Fatalf("docs/ holds %v, the redirect serves %v", have, want)
	}
}

// TestScopePickerPlacesEveryScopeOnce fails on a scope the picker would not
// draw, which a reader could then never grant from the browser.
func TestScopePickerPlacesEveryScopeOnce(t *testing.T) {
	seen := map[core.Scope]int{}
	for _, g := range scopeGroups {
		for _, s := range g.Scopes {
			seen[s]++
			if scopeNotes[s] == "" {
				t.Errorf("%s has no note saying what it allows", s)
			}
		}
	}
	for _, s := range core.AllScopes {
		want := 1
		if onboard.UnpickedScopes[s] != "" {
			want = 0
		}
		if seen[s] != want {
			t.Errorf("%s is in %d picker groups, want %d", s, seen[s], want)
		}
	}
	if len(seen)+len(onboard.UnpickedScopes) != len(core.AllScopes) {
		t.Errorf("the picker groups %d scopes and leaves out %d, the vocabulary has %d",
			len(seen), len(onboard.UnpickedScopes), len(core.AllScopes))
	}
}
