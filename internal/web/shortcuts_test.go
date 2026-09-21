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
		"garbage": web.KeySchemeDefault,
		"VIM":     web.KeySchemeDefault, // case sensitive, unlike the cookie the browser sends back verbatim
	}
	for in, want := range cases {
		if got := web.ParseKeyScheme(in); got != want {
			t.Errorf("ParseKeyScheme(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every shipped scheme must bind every action, so a key never simply does
// nothing because a scheme forgot one.
func TestKeyBindingsForLeavesNoActionUnbound(t *testing.T) {
	t.Parallel()
	for _, scheme := range web.KeySchemes() {
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

// The default scheme mirrors internal/tui/keys.go's DefaultKeyMap for the
// actions the web UI shares with the TUI, so a key means the same thing on
// both surfaces.
func TestDefaultSchemeMirrorsTheTUIDefaults(t *testing.T) {
	t.Parallel()
	table := web.KeyBindingsFor(web.KeySchemeDefault)
	want := map[string][]string{
		"up":         {"ArrowUp", "k"},
		"down":       {"ArrowDown", "j"},
		"top":        {"g", "Home"},
		"bottom":     {"G", "End"},
		"open":       {"Enter"},
		"new":        {"n"},
		"editTitle":  {"e"},
		"comment":    {"m"},
		"transition": {"t"},
		"filter":     {"/"},
		"help":       {"?"},
	}
	for action, keys := range want {
		if got := table[action]; !equalKeys(got, keys) {
			t.Errorf("default scheme %q = %v, want %v", action, got, keys)
		}
	}
}

// The vim scheme adds the recognisable vim reflexes without disturbing an
// action it does not name.
func TestVimSchemeAddsItsOwnReflexes(t *testing.T) {
	t.Parallel()
	table := web.KeyBindingsFor(web.KeySchemeVim)
	want := map[string][]string{
		"new":       {"o"},
		"editTitle": {"i"},
		"comment":   {"a"},
		"filter":    {"/", ":"},
		"up":        {"ArrowUp", "k"}, // untouched, inherited from the base
	}
	for action, keys := range want {
		if got := table[action]; !equalKeys(got, keys) {
			t.Errorf("vim scheme %q = %v, want %v", action, got, keys)
		}
	}
}

// The emacs scheme cannot carry over every chord internal/tui/scheme.go's
// SchemeEmacs uses, because several of them are chords the browser itself
// owns (new window, print, the omnibox, save page, new tab) and never
// delivers to a page's keydown handler. This asserts the divergence is
// deliberate and permanent, not a binding that quietly stopped doing
// anything because nobody noticed the browser eating it.
func TestEmacsSchemeDropsBrowserReservedChords(t *testing.T) {
	t.Parallel()
	table := web.KeyBindingsFor(web.KeySchemeEmacs)
	reserved := []string{"ctrl+n", "ctrl+p", "ctrl+t", "ctrl+e", "ctrl+s", "ctrl+f", "ctrl+w", "ctrl+l"}
	for action, keys := range table {
		for _, k := range keys {
			for _, r := range reserved {
				if k == r {
					t.Errorf("emacs scheme binds action %q to %q, which the browser reserves and will never deliver", action, r)
				}
			}
		}
	}
	// The two chords that are safe to intercept are still offered.
	if !containsKey(table["top"], "ctrl+a") {
		t.Errorf("emacs scheme top = %v, want ctrl+a kept", table["top"])
	}
	if !containsKey(table["back"], "ctrl+g") {
		t.Errorf("emacs scheme back = %v, want ctrl+g kept", table["back"])
	}
	// Filter and EditTitle fall back to the shared default because their TUI
	// chords (ctrl+s, ctrl+t) are unreachable from script.
	if got := table["filter"]; !equalKeys(got, []string{"/"}) {
		t.Errorf("emacs scheme filter = %v, want the base \"/\"", got)
	}
	if got := table["editTitle"]; !equalKeys(got, []string{"e"}) {
		t.Errorf("emacs scheme editTitle = %v, want the base \"e\"", got)
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

	page := b.page("/tasks")
	if !hasPlainForm(page, "/keyscheme") {
		t.Fatalf("the settings menu offers no keyboard scheme control:\n%s", page)
	}

	resp := b.post("/keyscheme", url.Values{"keyscheme": {"vim"}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	after := b.page("/tasks")
	if !strings.Contains(after, `<option value="vim" selected>Vim</option>`) {
		t.Errorf("the vim scheme did not stick across a reload:\n%s", after)
	}

	// A page carries the active scheme's bindings for the shortcut script to
	// read, and it has to be the one that was just chosen.
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

	page := b.page("/tasks")
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
	listEnd := strings.Index(rest, "</dl>")
	if listEnd < 0 {
		t.Fatalf("the help overlay list is never closed:\n%s", page)
	}
	if strings.Contains(rest[:listEnd], "<dt>") {
		t.Errorf("the help overlay ships a hardcoded binding list rather than an empty shell:\n%s", page)
	}
}

// -- test helpers --------------------------------------------------------

type shortcutsBlob struct {
	Scheme   string                         `json:"scheme"`
	Bindings map[string]map[string][]string `json:"bindings"`
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
