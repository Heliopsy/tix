// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"os"
	"regexp"
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
		for _, s := range paletteDataFor("en", advanced, false).Screens {
			out[s.Href] = true
		}
		return out
	}
	plain, advanced := hrefs(false), hrefs(true)
	for _, h := range []string{RouteTasks, RouteProjects, RouteTemplates, RouteActivity, RouteStats, RouteSettings, RouteSettings + "#keyboard"} {
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

// The configuration screens are hinted as the sidebar group they sit in,
// Manage, not as Admin or the Configure and Data groups before it.
func TestPaletteHintsTheManageGroup(t *testing.T) {
	t.Parallel()
	for _, s := range paletteDataFor("en", true, true).Screens {
		if s.Hint == "Configure" || s.Hint == "Data" || s.Hint == "Admin" {
			t.Errorf("%s is hinted as %q, a group the sidebar no longer has", s.Href, s.Hint)
		}
	}
	for _, s := range paletteScreens {
		if s.Advanced && s.Hint != "Manage" {
			t.Errorf("%s sits in the Manage group and is hinted %q", s.Href, s.Hint)
		}
	}
}

// A section a touch-only device does not draw (.needs-keys on Settings) and
// the shortcuts overlay are not offered by the palette there either: their
// entries carry keys, which assets/palette.js leaves out on such a device.
// Every other entry stays offered.
func TestPaletteMarksWhatATouchDeviceDoesNotDraw(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("templates/settings.html")
	if err != nil {
		t.Fatalf("reading the settings template: %v", err)
	}
	hidden := map[string]bool{"shortcuts": true}
	for _, m := range regexp.MustCompile(`<section class="setgroup needs-keys" id="([a-z-]+)">`).FindAllStringSubmatch(string(raw), -1) {
		hidden["screen:"+RouteSettings+"#"+m[1]] = true
	}
	if len(hidden) < 2 {
		t.Fatal("the settings template has no keyboard-only section to check")
	}
	data := paletteDataFor("en", true, true)
	seen := 0
	for _, e := range append(data.Screens, data.Actions...) {
		if e.Keys != hidden[e.ID] {
			t.Errorf("palette entry %s keys = %v, want %v", e.ID, e.Keys, hidden[e.ID])
		}
		if hidden[e.ID] {
			seen++
		}
	}
	if seen != len(hidden) {
		t.Errorf("the palette offers %d of the %d keyboard-only entries", seen, len(hidden))
	}
}
