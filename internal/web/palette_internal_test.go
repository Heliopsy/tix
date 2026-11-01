// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The store is asked for one page of PaletteTaskLimit, newest change first,
// whatever the query: the answer's own cap would hide a larger page, but the
// store would still have read it.
func TestPaletteFilterAsksForOneSmallPage(t *testing.T) {
	t.Parallel()
	want := core.Page{Limit: PaletteTaskLimit, Sort: core.SortUpdatedAt, Direction: core.Descending}
	cases := []struct {
		q     string
		query string
	}{
		{"deploy api", "deploy api"},
		{"is:starred deploy", "deploy"},
		{"status:", "status:"},
	}
	for _, tc := range cases {
		f := paletteFilter(tc.q)
		if f.Page != want {
			t.Errorf("paletteFilter(%q).Page = %+v, want %+v", tc.q, f.Page, want)
		}
		if f.Query != tc.query {
			t.Errorf("paletteFilter(%q).Query = %q, want %q", tc.q, f.Query, tc.query)
		}
	}
	if f := paletteFilter("is:starred deploy"); f.Starred != core.Yes {
		t.Errorf("is:starred was not read as a filter term: %+v", f)
	}
}

// Screens the sidebar shows only with Advanced screens on are offered only
// then, and every section of Settings always.
func TestPaletteOffersWhatTheSidebarOffers(t *testing.T) {
	t.Parallel()
	hrefs := func(advanced bool) map[string]bool {
		out := map[string]bool{}
		for _, s := range paletteDataFor("en", advanced).Screens {
			out[s.Href] = true
		}
		return out
	}
	plain, advanced := hrefs(false), hrefs(true)
	for _, h := range []string{RouteTasks, RouteProjects, RouteActivity, RouteStats, RouteSettings, RouteSettings + "#keyboard"} {
		if !plain[h] || !advanced[h] {
			t.Errorf("%s is not offered in both modes", h)
		}
	}
	for _, h := range []string{RouteWorkflows, RouteTenant, RouteUsers, RouteSync} {
		if plain[h] {
			t.Errorf("%s is offered without Advanced screens", h)
		}
		if !advanced[h] {
			t.Errorf("%s is not offered with Advanced screens", h)
		}
	}
}
