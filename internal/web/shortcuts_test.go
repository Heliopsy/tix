// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// The binding table is the single source both the keydown handler and the
// help overlay draw from, so its shape has to be trustworthy on its own,
// independent of driving a browser.

func TestParseKeyScheme(t *testing.T) {
	t.Parallel()
	cases := map[string]web.KeyScheme{
		"":        web.KeySchemeDefault,
		"default": web.KeySchemeDefault,
		"vim":     web.KeySchemeVim,
		"emacs":   web.KeySchemeEmacs,
		"off":     web.KeySchemeOff,
		"garbage": web.KeySchemeDefault,
		"VIM":     web.KeySchemeDefault, // case sensitive, unlike the cookie the browser sends back verbatim
	}
	for in, want := range cases {
		if got := web.ParseKeyScheme(in); got != want {
			t.Errorf("ParseKeyScheme(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every shipped scheme but Off must bind every action, so a key never simply
// does nothing because a scheme forgot one. Off unbinds on purpose.
func TestKeyBindingsForLeavesNoActionUnbound(t *testing.T) {
	t.Parallel()
	for _, scheme := range web.KeySchemes() {
		if scheme == web.KeySchemeOff {
			continue
		}
		table := web.KeyBindingsFor(scheme)
		for _, action := range web.ShortcutActionNames() {
			keys := table[action]
			if len(keys) == 0 {
				t.Errorf("scheme %q leaves action %q with no key", scheme, action)
			}
			for _, k := range keys {
				if k == "" {
					t.Errorf("scheme %q action %q carries an empty key", scheme, action)
				}
			}
		}
	}
}

// A scheme may bind a key to only one action within itself, the same
// collision internal/tui/scheme.go's Validate refuses for the TUI: a key
// meaning two different things at once is not a scheme, it is a bug.
func TestKeyBindingsForHaveNoCollision(t *testing.T) {
	t.Parallel()
	for _, scheme := range web.KeySchemes() {
		table := web.KeyBindingsFor(scheme)
		owner := map[string]string{}
		for action, keys := range table {
			for _, k := range keys {
				if other, ok := owner[k]; ok && other != action {
					t.Errorf("scheme %q binds %q to both %q and %q", scheme, k, other, action)
				}
				owner[k] = action
			}
		}
	}
}

// The default scheme is the Default column of the key table the web shares
// with the app (openspec/changes/unified-keys/design.md), every action the
// web can do, so a key means the same thing on both surfaces.
func TestDefaultSchemeIsTheSharedKeyTable(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"goInbox":         {"g i"},
		"goMyWork":        {"g m"},
		"goTasks":         {"g t"},
		"goProjects":      {"g p"},
		"goActivity":      {"g a"},
		"goSettings":      {"g s"},
		"goStats":         {"g S"},
		"down":            {"ArrowDown", "j"},
		"up":              {"ArrowUp", "k"},
		"top":             {"g", "Home"},
		"bottom":          {"G", "End"},
		"pageDown":        {"PageDown"},
		"pageUp":          {"PageUp"},
		"open":            {"Enter"},
		"back":            {"Escape"},
		"select":          {"x"},
		"extendDown":      {"shift+ArrowDown"},
		"extendUp":        {"shift+ArrowUp"},
		"selectAll":       {"ctrl+a", "meta+a"},
		"filter":          {"/"},
		"refresh":         {"r"},
		"switchView":      {"v"},
		"new":             {"n"},
		"quickPaste":      {"ctrl+shift+v", "meta+shift+v"},
		"complete":        {"d"},
		"editTitle":       {"e"},
		"editDescription": {"E"},
		"transition":      {"t"},
		"comment":         {"m"},
		"tag":             {"#"},
		"star":            {"s", "*"},
		"priority":        {"p"},
		"assign":          {"a"},
		"due":             {"u"},
		"snooze":          {"z"},
		"copyRef":         {"y"},
		"palette":         {"ctrl+k", "meta+k"},
		"help":            {"?"},
	}
	table := web.KeyBindingsFor(web.KeySchemeDefault)
	for action, keys := range want {
		if got := table[action]; !equalKeys(got, keys) {
			t.Errorf("default scheme %q = %v, want %v", action, got, keys)
		}
	}
	for _, action := range web.ShortcutActionNames() {
		if _, ok := want[action]; !ok {
			t.Errorf("action %q is not in the shared key table", action)
		}
	}
}

// The vim scheme's overrides are the shared table's Vim column. Comment
// stays m and assign stays a: vim's a used to be comment here, which was
// the one key the web and the app disagreed on.
func TestVimSchemeIsTheSharedVimColumn(t *testing.T) {
	t.Parallel()
	table := web.KeyBindingsFor(web.KeySchemeVim)
	want := map[string][]string{
		"back":            {"Escape", "h"},
		"open":            {"Enter", "l"},
		"top":             {"g g", "Home"},
		"pageDown":        {"ctrl+d", "PageDown"},
		"pageUp":          {"ctrl+u", "PageUp"},
		"select":          {"v"},
		"extendDown":      {"J", "shift+ArrowDown"},
		"extendUp":        {"K", "shift+ArrowUp"},
		"switchView":      {"g v"},
		"new":             {"o"},
		"editTitle":       {"i"},
		"editDescription": {"D"},
		"filter":          {"/", ":"},
		"comment":         {"m"},
		"assign":          {"a"},
		"complete":        {"d"},
		"up":              {"ArrowUp", "k"},
	}
	for action, keys := range want {
		if got := table[action]; !equalKeys(got, keys) {
			t.Errorf("vim scheme %q = %v, want %v", action, got, keys)
		}
	}
}

// The emacs scheme cannot carry over every chord the app's emacs scheme
// uses, because several are chords the browser itself owns (new window,
// print, the omnibox, save page, new tab, the address bar, find) and never
// delivers to a page's keydown handler. This asserts the divergence is
// deliberate and permanent, not a binding that quietly stopped doing
// anything because nobody noticed the browser eating it.
func TestEmacsSchemeDropsBrowserReservedChords(t *testing.T) {
	t.Parallel()
	table := web.KeyBindingsFor(web.KeySchemeEmacs)
	reserved := []string{"ctrl+n", "ctrl+p", "ctrl+t", "ctrl+e", "ctrl+s", "ctrl+f", "ctrl+w", "ctrl+l", "ctrl+b"}
	for action, keys := range table {
		for _, k := range keys {
			for _, r := range reserved {
				if k == r {
					t.Errorf("emacs scheme binds action %q to %q, which the browser reserves and will never deliver", action, r)
				}
			}
		}
	}
	want := map[string][]string{
		"palette":   {"ctrl+k", "meta+k", "alt+x"},
		"back":      {"Escape", "ctrl+g"},
		"top":       {"alt+<", "Home"},
		"bottom":    {"alt+>", "End"},
		"pageDown":  {"ctrl+v", "PageDown"},
		"pageUp":    {"alt+v", "PageUp"},
		"select":    {"ctrl+space", "x"},
		"selectAll": {"ctrl+a", "meta+a"},
		"star":      {"s", "*"},
		"filter":    {"/"},
		"refresh":   {"r"},
	}
	for action, keys := range want {
		if got := table[action]; !equalKeys(got, keys) {
			t.Errorf("emacs scheme %q = %v, want %v", action, got, keys)
		}
	}
}

// Off is chords only: no single key, and no sequence of them, runs
// anything, while the palette and select all still answer.
func TestOffSchemeKeepsOnlyChords(t *testing.T) {
	t.Parallel()
	table := web.KeyBindingsFor(web.KeySchemeOff)
	bound := map[string][]string{}
	for action, keys := range table {
		if len(keys) > 0 {
			bound[action] = keys
		}
	}
	want := map[string][]string{
		"palette":   {"ctrl+k", "meta+k"},
		"selectAll": {"ctrl+a", "meta+a"},
	}
	if len(bound) != len(want) {
		t.Errorf("off binds %v, want only %v", bound, want)
	}
	for action, keys := range want {
		if !equalKeys(bound[action], keys) {
			t.Errorf("off scheme %q = %v, want %v", action, bound[action], keys)
		}
	}
}

// A key is one press or two in a row ("g i"), never more, and never a blank
// half, which the shortcut script's one-key wait could not read.
func TestEveryKeyIsOneOrTwoPresses(t *testing.T) {
	t.Parallel()
	for _, scheme := range web.KeySchemes() {
		for action, keys := range web.KeyBindingsFor(scheme) {
			for _, k := range keys {
				parts := strings.Split(k, " ")
				if len(parts) > 2 {
					t.Errorf("scheme %q action %q key %q is more than two presses", scheme, action, k)
				}
				for _, p := range parts {
					if p == "" {
						t.Errorf("scheme %q action %q key %q has an empty press", scheme, action, k)
					}
				}
			}
		}
	}
}

// Every go-to key leads to a screen the reader can open, read from the table
// the page itself embeds rather than from the Go source.
func TestGoToKeysLeadToScreens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	blob := extractShortcutsJSON(t, b.page("/tasks"))
	want := map[string]string{
		"goInbox": "/inbox", "goMyWork": "/mine", "goTasks": "/tasks", "goProjects": "/projects",
		"goActivity": "/activity", "goSettings": "/settings", "goStats": "/stats",
	}
	if len(blob.Go) != len(want) {
		t.Errorf("the page embeds go-to screens %v, want %v", blob.Go, want)
	}
	for action, path := range want {
		if blob.Go[action] != path {
			t.Errorf("%s leads to %q, want %q", action, blob.Go[action], path)
			continue
		}
		b.page(path)
	}
}

// Claiming a task is not offered by the browser interface at all (see
// Exemptions in routes.go), so there must be no action a scheme could bind a
// key to for it.
func TestShortcutActionsExcludeClaim(t *testing.T) {
	t.Parallel()
	for _, name := range web.ShortcutActionNames() {
		if strings.EqualFold(name, "claim") {
			t.Fatalf("shortcut actions include %q, which the browser interface cannot perform", name)
		}
	}
}

// The settings menu round-trips a scheme choice, and the setting persists the
// same way the theme does: a cookie, applied on the next render.
func TestSetKeySchemePersists(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/settings")
	if !hasPlainForm(page, "/keyscheme") {
		t.Fatalf("the settings page offers no keyboard scheme control:\n%s", page)
	}

	resp := b.post("/keyscheme", url.Values{"keyscheme": {"vim"}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	settings := b.page("/settings")
	if !strings.Contains(between(t, settings, `<select id="keyscheme"`, "</select>"), `<option value="off" >Off</option>`) {
		t.Errorf("the scheme picker does not offer Off:\n%s", settings)
	}
	if !strings.Contains(settings, `<option value="vim" selected>Vim</option>`) {
		t.Errorf("the vim scheme did not stick across a reload:\n%s", settings)
	}

	// Every page, not only the settings page, carries the active scheme's
	// bindings for the shortcut script to read, and it has to be the one that
	// was just chosen.
	after := b.page("/tasks")
	blob := extractShortcutsJSON(t, after)
	if blob.Scheme != "vim" {
		t.Errorf("embedded scheme = %q, want %q", blob.Scheme, "vim")
	}
	if !equalKeys(blob.Bindings["vim"]["new"], []string{"o"}) {
		t.Errorf("embedded vim bindings for new = %v, want [o]", blob.Bindings["vim"]["new"])
	}
}

// An invalid scheme submitted by hand falls back to the default rather than
// being stored verbatim, the same defence setTheme applies to an unknown
// theme name.
func TestSetKeySchemeRefusesAnUnknownScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/keyscheme", url.Values{"keyscheme": {"colemak"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	page := b.page("/settings")
	if !strings.Contains(page, `<option value="default" selected>Default</option>`) {
		t.Errorf("an unknown scheme was not rejected back to default:\n%s", page)
	}
}

// The help overlay markup is present, carries a real dialog role rather than
// a plain div, and its list starts empty: script fills it from the same
// table the handler reads, never a hardcoded list.
func TestHelpOverlayMarkupIsAnEmptyDialogShell(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/tasks")

	if !strings.Contains(page, `id="shortcuts-help"`) {
		t.Fatalf("the page carries no help overlay:\n%s", page)
	}
	if !strings.Contains(page, `role="dialog"`) || !strings.Contains(page, `aria-modal="true"`) {
		t.Errorf("the help overlay is not marked up as a modal dialog:\n%s", page)
	}
	start := strings.Index(page, `id="shortcuts-help-list"`)
	if start < 0 {
		t.Fatalf("the help overlay has nowhere for the script to render bindings:\n%s", page)
	}
	rest := page[start:]
	listEnd := strings.Index(rest, "</div>")
	if listEnd < 0 {
		t.Fatalf("the help overlay list is never closed:\n%s", page)
	}
	if strings.Contains(rest[:listEnd], "<dt>") || strings.Contains(rest[:listEnd], "<h3>") {
		t.Errorf("the help overlay ships a hardcoded binding list rather than an empty shell:\n%s", page)
	}
}

// -- test helpers --------------------------------------------------------

type shortcutsBlob struct {
	Scheme   string                         `json:"scheme"`
	Bindings map[string]map[string][]string `json:"bindings"`
	Go       map[string]string              `json:"go"`
	Groups   []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"groups"`
	Actions []struct {
		Name  string `json:"name"`
		Group string `json:"group"`
	} `json:"actions"`
}

func extractShortcutsJSON(t *testing.T, page string) shortcutsBlob {
	t.Helper()
	const marker = `id="tix-shortcuts-data" type="application/json">`
	at := strings.Index(page, marker)
	if at < 0 {
		t.Fatalf("no embedded shortcuts data in page:\n%s", page)
	}
	rest := page[at+len(marker):]
	end := strings.Index(rest, "</script>")
	if end < 0 {
		t.Fatalf("unterminated shortcuts data in page:\n%s", page)
	}
	var blob shortcutsBlob
	if err := json.Unmarshal([]byte(rest[:end]), &blob); err != nil {
		t.Fatalf("embedded shortcuts data is not valid JSON: %v\n%s", err, rest[:end])
	}
	return blob
}

func equalKeys(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	g := append([]string{}, got...)
	w := append([]string{}, want...)
	sort.Strings(g)
	sort.Strings(w)
	for i := range g {
		if g[i] != w[i] {
			return false
		}
	}
	return true
}

func containsKey(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}

// A scheme that resolves to a table another scheme already ships is a second
// name for the same keys: a picker entry that changes nothing when chosen.
// This is the guard that keeps a scheme out of the list unless it earns its
// place, which is why nano and helix stay on the TUI side (see KeySchemes).
func TestNoTwoSchemesShipTheSameBindings(t *testing.T) {
	t.Parallel()
	schemes := web.KeySchemes()
	for i, a := range schemes {
		for _, b := range schemes[i+1:] {
			ta, tb := web.KeyBindingsFor(a), web.KeyBindingsFor(b)
			same := true
			for _, action := range web.ShortcutActionNames() {
				if !equalKeys(ta[action], tb[action]) {
					same = false
					break
				}
			}
			if same {
				t.Errorf("schemes %q and %q bind every action identically", a, b)
			}
		}
	}
}

// The same fault internal/tui/scheme.go's KeyMapFor now panics on exists here
// in a quieter form: an override naming an action shortcutActions does not
// have adds a dead row to the table while the real action keeps the base key,
// so the scheme silently does less than it claims.
func TestNoSchemeOverridesAnActionThatDoesNotExist(t *testing.T) {
	t.Parallel()
	known := map[string]bool{}
	for _, name := range web.ShortcutActionNames() {
		known[name] = true
	}
	for _, scheme := range web.KeySchemes() {
		for action := range web.KeyBindingsFor(scheme) {
			if !known[action] {
				t.Errorf("scheme %q carries a binding for %q, which is not a shortcut action", scheme, action)
			}
		}
	}
}

// The help overlay heads its actions Navigate, Lists, Task and General, in
// that order, the same four the app's help uses, and every action sits under
// one of them.
func TestHelpGroupsAreTheSharedFour(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	blob := extractShortcutsJSON(t, b.page("/tasks"))
	var groups []string
	for _, g := range blob.Groups {
		groups = append(groups, g.Value)
	}
	if want := []string{"Navigate", "Lists", "Task", "General"}; strings.Join(groups, ",") != strings.Join(want, ",") {
		t.Fatalf("help groups = %v, want %v", groups, want)
	}
	for _, a := range blob.Actions {
		if !containsKey(groups, a.Group) {
			t.Errorf("action %q sits in group %q, which the overlay does not draw", a.Name, a.Group)
		}
	}
	if len(blob.Actions) != len(web.ShortcutActionNames()) {
		t.Errorf("the page embeds %d actions, want %d", len(blob.Actions), len(web.ShortcutActionNames()))
	}
}

// Each task key reaches a control the task screen already draws, marked with
// data-shortcut-target. The check reads the one element carrying the mark,
// so a mark that drifted onto some other control fails it.
func TestTaskScreenMarksTheControlsTheTaskKeysPress(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one")
	page := b.page("/tasks/" + ref)
	want := map[string]string{
		"star":            `class="star`,
		"priority":        `class="priobtn"`,
		"assign":          `name="assignee"`,
		"due":             `name="due_at"`,
		"tag":             `name="tag"`,
		"snooze":          `class="flowgo snoozebtn"`,
		"editDescription": `name="body"`,
		"copyRef":         `data-shortcut-ref="` + strings.ToUpper(ref) + `"`,
	}
	for action, mark := range want {
		tag := openTag(t, page, `data-shortcut-target="`+action+`"`)
		if tag == "" {
			t.Errorf("the task screen marks nothing for %s", action)
			continue
		}
		if !strings.Contains(tag, mark) {
			t.Errorf("the %s mark sits on %s, want the element carrying %s", action, tag, mark)
		}
	}
}

// A list row carries its own star and priority marks, so s and p act on the
// focused row, and the view switch is marked for v.
func TestTaskRowsMarkTheirOwnControls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one")
	page := b.page("/tasks")
	row := between(t, page, `<li id="t-`+ref+`"`, "</li>")
	for action, mark := range map[string]string{"star": `class="star`, "priority": `class="priobtn"`} {
		tag := openTag(t, row, `data-shortcut-target="`+action+`"`)
		if !strings.Contains(tag, mark) {
			t.Errorf("the row's %s mark sits on %q, want the element carrying %s", action, tag, mark)
		}
	}
	if tag := openTag(t, page, `data-shortcut-target="switchView"`); !strings.Contains(tag, `id="tasks-views"`) {
		t.Errorf("the switchView mark sits on %q, want the view switch", tag)
	}
}

// The settings comparison has an Off column, and a key Off does not bind
// reads as a dash there rather than as an empty cell.
func TestSettingsComparisonShowsOff(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/settings")
	if !strings.Contains(page, `<th scope="col">Off</th>`) {
		t.Fatalf("the scheme comparison has no Off column:\n%s", page)
	}
	row := between(t, page, `<td data-label="Action">Go back</td>`, "</tr>")
	if !strings.Contains(row, `<td data-label="Off">&mdash;</td>`) {
		t.Errorf("Off's unbound Go back is not a dash: %s", row)
	}
	palette := between(t, page, `<td data-label="Action">Open the command palette</td>`, "</tr>")
	if !strings.Contains(palette, `<td data-label="Off"><kbd>ctrl&#43;k</kbd>`) {
		t.Errorf("Off does not show the palette chord: %s", palette)
	}
}
