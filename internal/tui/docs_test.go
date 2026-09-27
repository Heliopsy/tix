// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
)

// The page these tests read. They live in package tui rather than in a docs
// package because the view enumeration is unexported: a test outside the
// package would have to keep its own copy of the list, which is the second
// copy of a constant this whole exercise exists to remove.
const tuiPage = "docs/tui.md"

// viewsHeading matches the heading over the views table, which names the count
// in words.
func viewsHeading(t *testing.T, md string) string {
	t.Helper()
	got, err := docsmd.Heading(md, 2, func(text string) bool {
		return strings.HasPrefix(text, "The ") && strings.HasSuffix(text, " views")
	})
	if err != nil {
		t.Fatalf(`%s has no "## The N views" heading over the views table`, tuiPage)
	}
	return got
}

// numberWords spells the counts a heading can plausibly carry.
var numberWords = []string{
	"zero", "one", "two", "three", "four", "five", "six", "seven", "eight",
	"nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen",
	"sixteen", "seventeen", "eighteen", "nineteen", "twenty",
}

// viewScanLimit bounds the scan below, so a viewName that names everything
// would fail the test rather than loop for ever.
const viewScanLimit = 64

// enumeratedViews enumerates the interface's screens. viewName answers an
// unknown view with the help view's name, so the first value past the last
// constant ends the scan.
func enumeratedViews() []viewKind {
	var out []viewKind
	for v := viewKind(0); int(v) < viewScanLimit; v++ {
		if v != viewHelp && viewName(v) == viewName(viewHelp) {
			return out
		}
		out = append(out, v)
	}
	return out
}

// viewNames are the two names one view can be documented under: the one
// messages use, and the one the title bar draws.
func viewNames(v viewKind) []string {
	return []string{viewName(v), Model{view: v}.viewName()}
}

func readPage(t *testing.T) string {
	t.Helper()
	md, err := docsmd.Read(tuiPage)
	if err != nil {
		t.Fatalf("reading the page under test: %v", err)
	}
	return md
}

// TestDocsViewsTableListsEveryView reads the views table and nothing else. The
// count went eight, nine, ten while the table stood still, because a new view
// is added in one file and documented in another.
func TestDocsViewsTableListsEveryView(t *testing.T) {
	t.Parallel()
	md := readPage(t)
	section, err := docsmd.Section(md, "## "+viewsHeading(t, md))
	if err != nil {
		t.Fatalf("%s: %v", tuiPage, err)
	}
	rows, err := docsmd.Table(section, "View", "Opened by", "Shows")
	if err != nil {
		t.Fatalf("%s: %v", tuiPage, err)
	}
	listed := docsmd.Column(rows, 0)

	for _, v := range enumeratedViews() {
		names := viewNames(v)
		if !slices.ContainsFunc(listed, func(cell string) bool { return slices.Contains(names, cell) }) {
			t.Errorf("%s: the views table has no row for the %s view; add one naming what opens it and what it shows",
				tuiPage, viewName(v))
		}
	}
	for _, cell := range listed {
		if !slices.ContainsFunc(enumeratedViews(), func(v viewKind) bool { return slices.Contains(viewNames(v), cell) }) {
			t.Errorf("%s: the views table lists %q, which is not a view the interface has; remove the row",
				tuiPage, cell)
		}
	}
}

// TestDocsViewsHeadingCountsTheViews holds the number in the heading against
// the enumeration. The heading said eight while the table held nine.
func TestDocsViewsHeadingCountsTheViews(t *testing.T) {
	t.Parallel()
	md := readPage(t)
	heading := viewsHeading(t, md)
	n := len(enumeratedViews())
	if n >= len(numberWords) {
		t.Fatalf("%d views is past the words this test spells", n)
	}
	want := "The " + numberWords[n] + " views"
	if heading != want {
		t.Errorf("%s: the views heading says %q but the interface has %d views; make it %q",
			tuiPage, heading, n, want)
	}
}

// glyphs are the arrow keys as the bindings table draws them. The binding's
// own key name is "up"; a reader looking for it on the page sees an arrow.
var glyphs = map[string]string{"up": "↑", "down": "↓", "left": "←", "right": "→"}

// bindingRows reads the default bindings table and nothing else. A key named
// in one of the paragraphs around it does not count as documented.
func bindingRows(t *testing.T) []docsmd.Row {
	t.Helper()
	section, err := docsmd.Section(readPage(t), "### The default bindings")
	if err != nil {
		t.Fatalf("%s: %v", tuiPage, err)
	}
	rows, err := docsmd.Table(section, "Keys", "Action", "Where")
	if err != nil {
		t.Fatalf("%s: %v", tuiPage, err)
	}
	return rows
}

// documentedKeys are the keys the bindings table's first column names.
func documentedKeys(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, cell := range docsmd.Column(bindingRows(t), 0) {
		out = append(out, docsmd.Codes(cell)...)
	}
	return out
}

// TestDocsBindingsTableNamesEveryAction asserts the page lists a key for every
// action the interface dispatches. L, -, M, X and y all shipped undocumented,
// and w, f, O, u and H followed them.
func TestDocsBindingsTableNamesEveryAction(t *testing.T) {
	t.Parallel()
	keys := documentedKeys(t)
	k := DefaultKeyMap()
	for _, action := range ActionNames() {
		b, ok := k.Binding(action)
		if !ok || len(b.Keys()) == 0 {
			t.Errorf("action %s carries no default binding", action)
			continue
		}
		first := b.Keys()[0]
		want := []string{first}
		if g, ok := glyphs[first]; ok {
			want = append(want, g)
		}
		if !slices.ContainsFunc(want, func(key string) bool { return slices.Contains(keys, key) }) {
			t.Errorf("%s: the default bindings table lists no key for %s, which is bound to %q (%s); add a row",
				tuiPage, action, first, b.Help().Desc)
		}
	}
}

// TestDocsBindingsTableInventsNoKey is the other direction. A documented key
// with no binding behind it is drift too: it promises a keystroke that does
// nothing.
func TestDocsBindingsTableInventsNoKey(t *testing.T) {
	t.Parallel()
	bound := map[string]bool{}
	k := DefaultKeyMap()
	for _, action := range ActionNames() {
		b, ok := k.Binding(action)
		if !ok {
			continue
		}
		for _, name := range b.Keys() {
			bound[name] = true
			if g, ok := glyphs[name]; ok {
				bound[g] = true
			}
		}
	}
	for _, documented := range documentedKeys(t) {
		if !bound[documented] {
			t.Errorf("%s: the default bindings table lists %q, which no action is bound to; remove it or bind it",
				tuiPage, documented)
		}
	}
}

// TestDocsMarkerTableMatchesTheLegend holds the page against the legend the
// help overlay renders. The page missed the deleted marker for two days.
func TestDocsMarkerTableMatchesTheLegend(t *testing.T) {
	t.Parallel()
	rows, err := docsmd.Table(readPage(t), "Marker", "Means")
	if err != nil {
		t.Fatalf("%s: %v", tuiPage, err)
	}
	var documented []string
	for _, cell := range docsmd.Column(rows, 0) {
		documented = append(documented, docsmd.Codes(cell)...)
	}
	for _, e := range CardLegend {
		marker, meaning, _ := strings.Cut(e, " ")
		if !slices.Contains(documented, marker) {
			t.Errorf("%s: the marker table has no row for %q (%s), which a card can carry; add one",
				tuiPage, marker, meaning)
		}
	}
	for _, marker := range documented {
		if !slices.ContainsFunc(CardLegend, func(e string) bool {
			m, _, _ := strings.Cut(e, " ")
			return m == marker
		}) {
			t.Errorf("%s: the marker table lists %q, which no card draws; remove the row",
				tuiPage, marker)
		}
	}
}

// TestEnumeratedViewsMatchTheHandList keeps the scan these docs tests walk in
// step with the list the gating tests walk. Two enumerations that disagree
// mean one of them is documenting or gating a screen the other does not know
// about, and both are silent about it.
func TestEnumeratedViewsMatchTheHandList(t *testing.T) {
	t.Parallel()
	for _, v := range enumeratedViews() {
		if !slices.Contains(everyView, v) {
			t.Errorf("the %s view is missing from everyView in access_test.go, so nothing checks its gating",
				viewName(v))
		}
	}
	for _, v := range everyView {
		if !slices.Contains(enumeratedViews(), v) {
			t.Errorf("everyView lists a view viewName does not name, so the title bar calls it %q",
				viewName(v))
		}
	}
}

// TestDocsIndexCountsTheViewsAndSchemes reads the tui.md row of the guide
// index. The count in the index is a second copy of the count in the heading,
// and the second copy is the one nobody remembers to change.
func TestDocsIndexCountsTheViewsAndSchemes(t *testing.T) {
	t.Parallel()
	index, err := docsmd.Read("docs/README.md")
	if err != nil {
		t.Fatalf("reading the guide index: %v", err)
	}
	rows, err := docsmd.Table(index, "Guide", "Covers")
	if err != nil {
		t.Fatalf("docs/README.md: %v", err)
	}
	var covers string
	for _, row := range rows {
		if len(row) > 1 && strings.Contains(row[0], "(tui.md)") && !strings.Contains(row[0], "#") {
			covers = row[1]
			break
		}
	}
	if covers == "" {
		t.Fatal("docs/README.md: the guide index has no row for tui.md")
	}
	for _, want := range []struct {
		phrase string
		n      int
	}{
		{"views", len(enumeratedViews())},
		{"keybinding schemes", len(Schemes())},
	} {
		if n := len(numberWords); want.n >= n {
			t.Fatalf("%d is past the words this test spells", want.n)
		}
		phrase := numberWords[want.n] + " " + want.phrase
		if !strings.Contains(covers, phrase) {
			t.Errorf("docs/README.md: the tui.md row does not say %q; the interface has %d %s",
				phrase, want.n, want.phrase)
		}
	}
}
