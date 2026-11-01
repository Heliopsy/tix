// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// cssRule is one style rule of the stylesheet with the at-rule it sits in.
type cssRule struct {
	media    string
	selector string
	body     string
}

var cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// cssRules splits a stylesheet into its style rules, one level of at-rule
// deep, which is all app.css uses.
func cssRules(sheet string) []cssRule {
	sheet = cssComment.ReplaceAllString(sheet, "")
	var rules []cssRule
	media := ""
	depth := 0
	start := 0
	for i := 0; i < len(sheet); i++ {
		switch sheet[i] {
		case '{':
			head := strings.TrimSpace(sheet[start:i])
			if strings.HasPrefix(head, "@") {
				media = head
				depth++
				start = i + 1
				continue
			}
			end := strings.IndexByte(sheet[i:], '}')
			if end < 0 {
				return rules
			}
			rules = append(rules, cssRule{media: media, selector: head, body: sheet[i+1 : i+end]})
			i += end
			start = i + 1
		case '}':
			if depth > 0 {
				depth--
				media = ""
			}
			start = i + 1
		}
	}
	return rules
}

// subjects returns the compound each selector of a list styles: the last
// one, so `.wrap:has(.board) .lanes` styles the lanes and not the column.
func subjects(selector string) []string {
	var out []string
	for _, one := range strings.Split(selector, ",") {
		fields := strings.Fields(strings.ReplaceAll(one, ">", " "))
		if len(fields) > 0 {
			out = append(out, fields[len(fields)-1])
		}
	}
	return out
}

var horizontalSpacing = regexp.MustCompile(`(?m)(^|;)\s*(margin|margin-left|margin-inline|margin-inline-start|padding|padding-left|padding-inline|padding-inline-start)\s*:([^;]*)`)

// TestTheContentInsetIsOneRule pins the gap between the sidebar and the page.
//
// The content column was centred under a max-width that per-screen :has()
// rules changed, so the projects screen started 276px from the sidebar at
// 1920px while the task list started 32px from it. Only the base rule may say
// where the column starts, and it must not centre it.
func TestTheContentInsetIsOneRule(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))

	base := 0
	for _, r := range cssRules(sheet) {
		styles := false
		for _, s := range subjects(r.selector) {
			if s == ".wrap" || strings.HasPrefix(s, ".wrap:") {
				styles = true
			}
		}
		if !styles {
			continue
		}
		spacing := horizontalSpacing.FindAllStringSubmatch(r.body, -1)
		if r.selector == ".wrap" && r.media == "" {
			base++
			for _, m := range spacing {
				if strings.Contains(m[3], "auto") {
					t.Errorf("the content column is centred again (%s:%s), so its left edge moves with each screen's measure", m[2], m[3])
				}
				if m[2] == "padding" && !strings.Contains(m[3], "var(--gutter)") {
					t.Errorf("the content column's inset is %q, not the gutter", m[3])
				}
			}
			continue
		}
		for _, m := range spacing {
			t.Errorf("%s %s sets %s:%s, a second rule for where the content starts", r.media, r.selector, m[2], m[3])
		}
	}
	if base != 1 {
		t.Fatalf("found %d base .wrap rules, want exactly one", base)
	}
}

// TestTheSidebarStateIsAppliedBeforePaint keeps sidebar.js in the head and
// undeferred, so a stored width or rail is on the root before the body draws.
func TestTheSidebarStateIsAppliedBeforePaint(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/tasks")
	head, _, found := strings.Cut(page, "</head>")
	if !found {
		t.Fatal("the page has no head")
	}
	if !strings.Contains(head, `<script src="/assets/sidebar.js"></script>`) {
		t.Errorf("sidebar.js is not loaded undeferred from the head, so the default sidebar flashes before the stored one:\n%s", head)
	}
}

// TestTheSidebarControlsNeedScripting checks the handle and the toggle are
// shown only with scripting, with the attributes a keyboard and a screen
// reader need.
func TestTheSidebarControlsNeedScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	side := sidebarOf(t, f.as("alice").page("/tasks"))

	handle := tagWith(t, side, "data-side-resize")
	for _, want := range []string{`role="separator"`, `aria-orientation="vertical"`, `aria-controls="side"`, `tabindex="0"`, `aria-label="Resize sidebar"`, "data-needs-js"} {
		if !strings.Contains(handle, want) {
			t.Errorf("the resize handle lacks %s:\n%s", want, handle)
		}
	}
	toggle := tagWith(t, side, "data-side-toggle")
	for _, want := range []string{`type="button"`, `aria-controls="side"`, `aria-expanded="true"`, `aria-label="Collapse sidebar"`, "data-needs-js"} {
		if !strings.Contains(toggle, want) {
			t.Errorf("the collapse toggle lacks %s:\n%s", want, toggle)
		}
	}
}

// TestNoSidebarControlWaitsForItsScriptToAppear keeps every control that
// needs a script out of the hidden-then-revealed pattern. A boosted
// navigation swaps the whole body, so a control rendered hidden was missing
// for a frame on every navigation and everything under it jumped up and back.
// They are drawn from the root's "js" class instead, which sidebar.js sets in
// the head and a swap does not touch.
func TestNoSidebarControlWaitsForItsScriptToAppear(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.AdvancedCookie, "1")
	side := sidebarOf(t, b.page("/tasks"))

	for _, marker := range []string{"data-side-resize", "data-side-toggle", "data-palette-open", "data-shortcuts-open", `data-navgroup-toggle="configure"`} {
		tag := tagWith(t, side, marker)
		if regexp.MustCompile(`\shidden[\s>=]`).MatchString(tag) {
			t.Errorf("%s is rendered hidden for a script to reveal after the swap has painted:\n%s", marker, tag)
		}
		if !strings.Contains(tag, "data-needs-js") {
			t.Errorf("%s is not marked as needing scripting, so it shows without it:\n%s", marker, tag)
		}
	}
	sheet := body(t, b.get("/assets/app.css"))
	if rule := declarations(t, sheet, ":root:not(.js) [data-needs-js]"); !strings.Contains(rule, "display: none") {
		t.Errorf("nothing hides a control that needs scripting when there is none:\n%s", rule)
	}
	if rule := declarations(t, sheet, ":root.js .navgroup-name"); !strings.Contains(rule, "display: none") {
		t.Errorf("a group heading names itself twice with scripting:\n%s", rule)
	}
}

// TestTheSidebarIsDrawnFromItsCookie checks the server draws the width, the
// rail and the folded groups the tix_sidebar cookie holds, so the first paint
// of a load or a boosted navigation is the sidebar the reader left.
func TestTheSidebarIsDrawnFromItsCookie(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	cases := []struct {
		name, cookie, root, toggle string
		folded, open               []string
	}{
		{name: "no cookie", root: `<html lang="en">`, toggle: `aria-expanded="true"`, open: []string{"configure", "data", "recent"}},
		{name: "width rail and folds", cookie: "w=300&rail=1&fold=configure.recent", root: `<html lang="en" class="side-collapsed" style="--side-w: 300px">`, toggle: `aria-expanded="false"`, folded: []string{"configure", "recent"}, open: []string{"data"}},
		{name: "width clamped", cookie: "w=9000", root: `<html lang="en" style="--side-w: 480px">`, toggle: `aria-expanded="true"`, open: []string{"configure"}},
		{name: "garbage is the default", cookie: "w=wide&rail=yes&fold=trash.<b>", root: `<html lang="en">`, toggle: `aria-expanded="true"`, open: []string{"configure", "data"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := f.as("alice")
			b.setCookie(web.AdvancedCookie, "1")
			if tc.cookie != "" {
				b.setCookie(web.SidebarCookie, tc.cookie)
			}
			page := b.page("/tasks")
			if !strings.Contains(page, tc.root) {
				t.Errorf("the root is not drawn as %s:\n%s", tc.root, page[:min(len(page), 400)])
			}
			side := sidebarOf(t, page)
			if toggle := tagWith(t, side, "data-side-toggle"); !strings.Contains(toggle, tc.toggle) {
				t.Errorf("the rail toggle is not drawn %s:\n%s", tc.toggle, toggle)
			}
			for _, g := range tc.folded {
				if group := tagWith(t, side, `data-navgroup="`+g+`"`); !strings.Contains(group, "is-folded") {
					t.Errorf("the %s group is not drawn folded:\n%s", g, group)
				}
				if btn := tagWith(t, side, `data-navgroup-toggle="`+g+`"`); !strings.Contains(btn, `aria-expanded="false"`) {
					t.Errorf("the %s heading does not say it is folded:\n%s", g, btn)
				}
			}
			for _, g := range tc.open {
				if group := tagWith(t, side, `data-navgroup="`+g+`"`); strings.Contains(group, "is-folded") {
					t.Errorf("the %s group is drawn folded:\n%s", g, group)
				}
				if btn := tagWith(t, side, `data-navgroup-toggle="`+g+`"`); !strings.Contains(btn, `aria-expanded="true"`) {
					t.Errorf("the %s heading does not say it is open:\n%s", g, btn)
				}
			}
		})
	}
}

// TestEverySidebarGroupFoldsTheSameWay checks each foldable group's heading is
// a disclosure button that names the body it controls, and that the body is
// there under that id inside the same group.
func TestEverySidebarGroupFoldsTheSameWay(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.AdvancedCookie, "1")
	saveFilter(t, b, url.Values{"name": {"Mine"}, "q": {"assignee:me"}, "next": {"/tasks"}})
	side := sidebarOf(t, b.page("/tasks"))
	for _, g := range web.SidebarGroups {
		btn := tagWith(t, side, `data-navgroup-toggle="`+g+`"`)
		for _, want := range []string{`<button type="button"`, `aria-controls="navgroup-` + g + `"`, `aria-expanded="`} {
			if !strings.Contains(btn, want) {
				t.Errorf("the %s heading lacks %s:\n%s", g, want, btn)
			}
		}
		start := strings.Index(side, `data-navgroup="`+g+`"`)
		if start < 0 {
			t.Fatalf("the sidebar has no %s group", g)
		}
		group := side[start:]
		if next := strings.Index(group[1:], `data-navgroup="`); next >= 0 {
			group = group[:next+1]
		}
		if !strings.Contains(group, `data-navgroup-toggle="`+g+`"`) {
			t.Errorf("the %s heading is not inside its group", g)
		}
		if !strings.Contains(tagWith(t, group, `id="navgroup-`+g+`"`), "navgroup-body") {
			t.Errorf("the %s group has no body under the id its heading controls", g)
		}
	}
}

// TestAFoldedGroupStillShowsWhereTheReaderIs pins the rule that hides a
// folded group's entries to everything but the current one, and keeps it off
// the rail, which draws every entry as an icon.
func TestAFoldedGroupStillShowsWhereTheReaderIs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))
	found := 0
	for _, r := range cssRules(sheet) {
		if !strings.Contains(r.selector, ".navgroup.is-folded") {
			continue
		}
		found++
		if !strings.Contains(r.selector, `:not([aria-current="page"], .is-current, :has([aria-current="page"]))`) {
			t.Errorf("a folded group hides the entry for the page being read: %s", r.selector)
		}
		if r.media == "" && !strings.HasPrefix(r.selector, ":root:not(.side-collapsed)") {
			t.Errorf("folding applies on the rail too, which then loses icons: %s", r.selector)
		}
		if strings.TrimSpace(r.body) != "display: none;" {
			t.Errorf("a folded group's rule does more than hide: %s", r.body)
		}
	}
	if found != 2 {
		t.Fatalf("found %d folded-group rules, want the desktop one and the phone one", found)
	}
}

// TestTheRecentListSurvivesABoostedNavigation keeps the client-drawn recent
// list in place across a body swap: without hx-preserve every navigation
// brought back the empty hidden block and recent.js filled it a frame later,
// pushing Projects and everything under it down.
func TestTheRecentListSurvivesABoostedNavigation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	side := sidebarOf(t, f.as("alice").page("/tasks"))
	tag := tagWith(t, side, "data-recent-for")
	for _, want := range []string{`id="side-recent"`, `hx-preserve="true"`} {
		if !strings.Contains(tag, want) {
			t.Errorf("the recent list lacks %s:\n%s", want, tag)
		}
	}
}

// tagWith returns the opening tag in markup that carries an attribute.
func tagWith(t *testing.T, markup, attr string) string {
	t.Helper()
	at := strings.Index(markup, " "+attr)
	if at < 0 {
		t.Fatalf("nothing carries %s:\n%s", attr, markup)
	}
	open := strings.LastIndex(markup[:at], "<")
	end := strings.Index(markup[at:], ">")
	return markup[open : at+end+1]
}

var navEntry = regexp.MustCompile(`<a href="[^"]*" data-rail-name="([^"]*)"[^>]*>.*?<span>([^<]*)</span></a>`)

// TestEveryRailEntryKeepsItsName checks that what the rail hides is the
// label's paint and not the label: every entry names itself, and the rule
// that hides the labels clips them rather than removing them.
func TestEveryRailEntryKeepsItsName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	links := nav(t, b.page("/tasks"))

	entries := navEntry.FindAllStringSubmatch(links, -1)
	if want := strings.Count(links, "<a href="); len(entries) != want || want == 0 {
		t.Errorf("%d of %d navigation links carry a rail name over a span label:\n%s", len(entries), want, links)
	}
	for _, m := range entries {
		if m[1] != m[2] {
			t.Errorf("the rail names %q as %q", m[2], m[1])
		}
	}

	sheet := body(t, b.get("/assets/app.css"))
	hide := declarations(t, sheet, ".side-collapsed nav.main a > span")
	if strings.Contains(hide, "display: none") || strings.Contains(hide, "visibility: hidden") || !strings.Contains(hide, "clip:") {
		t.Errorf("the rail hides its labels from assistive technology too:\n%s", hide)
	}
}

// TestTheRailIsOnlyADesktopLayout keeps every collapsed and handle rule
// inside the breakpoint where the sidebar is a column, so a browser that
// collapsed it on a desktop gets the ordinary phone bar.
func TestTheRailIsOnlyADesktopLayout(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))

	seen := 0
	for _, r := range cssRules(sheet) {
		if !strings.Contains(r.selector, ".side-collapsed") && !strings.Contains(r.selector, ".side-resize") && !strings.Contains(r.selector, ".side-toggle") {
			continue
		}
		seen++
		if r.media == "@media (min-width: 900px)" {
			continue
		}
		if strings.TrimSpace(r.body) != "display: none;" {
			t.Errorf("%q applies outside the desktop layout (%q):\n%s", r.selector, r.media, r.body)
		}
	}
	if seen < 3 {
		t.Fatalf("found %d sidebar rail rules, the stylesheet lost them", seen)
	}
}
