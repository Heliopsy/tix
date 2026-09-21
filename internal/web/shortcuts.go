package web

import (
	"encoding/json"
	"html/template"
	"net/http"
)

// KeyScheme names a shipped set of web keyboard shortcuts. It mirrors
// internal/tui/scheme.go's Scheme so the two surfaces agree on what "vim" and
// "emacs" mean, without either package importing the other.
type KeyScheme string

// The shipped schemes, in the order the settings menu and the help overlay
// offer them.
const (
	KeySchemeDefault KeyScheme = "default"
	KeySchemeVim     KeyScheme = "vim"
	KeySchemeEmacs   KeyScheme = "emacs"
)

// KeySchemes lists every shipped scheme.
func KeySchemes() []KeyScheme { return []KeyScheme{KeySchemeDefault, KeySchemeVim, KeySchemeEmacs} }

// KeySchemeCookie remembers the keyboard scheme this browser asked for. Like
// ThemeCookie, an absent or unrecognised value falls back to the default
// rather than refusing the page.
const KeySchemeCookie = "tix_keyscheme"

// ParseKeyScheme resolves a submitted or cookied value to a shipped scheme,
// the same way Themes is walked in setTheme: anything that is not one of the
// named non-default schemes resolves to the default.
func ParseKeyScheme(value string) KeyScheme {
	for _, s := range KeySchemes() {
		if s != KeySchemeDefault && string(s) == value {
			return s
		}
	}
	return KeySchemeDefault
}

// keySchemeOf reports the scheme this browser asked for.
func keySchemeOf(r *http.Request) KeyScheme {
	c, err := r.Cookie(KeySchemeCookie)
	if err != nil {
		return KeySchemeDefault
	}
	return ParseKeyScheme(c.Value)
}

// keySchemeLabel names a scheme for the settings menu and the help overlay.
func keySchemeLabel(s KeyScheme) string {
	switch s {
	case KeySchemeVim:
		return "Vim"
	case KeySchemeEmacs:
		return "Emacs"
	default:
		return "Default"
	}
}

// shortcutAction is one action the shortcut layer can drive, in the order the
// help overlay lists it. This is the entire vocabulary: a scheme may only
// choose which keys reach an action, never add an action of its own, which is
// what keeps a key from ever being bound to something the web UI cannot do.
type shortcutAction struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// shortcutActions covers exactly what the web UI can do today. Claiming a
// task is deliberately absent: the browser interface does not expose
// ClaimTask or ClaimNext at all (see Exemptions in routes.go, "claiming is
// the agent work queue"), so there is nothing here for a key to drive and
// binding one would be the bug the TUI just had to fix. Left/right and
// column navigation are absent for the same reason: the web UI has no
// keyboard concept of moving between board columns, only a single ordered
// list of rows.
var shortcutActions = []shortcutAction{
	{"up", "Move selection up"},
	{"down", "Move selection down"},
	{"top", "Jump to the first task"},
	{"bottom", "Jump to the last task"},
	{"open", "Open the selected task"},
	{"back", "Go back"},
	{"new", "Focus the new task field"},
	{"editTitle", "Focus the title field"},
	{"comment", "Focus the comment field"},
	{"transition", "Focus the move-to field"},
	{"filter", "Focus the filter field"},
	{"help", "Show this help"},
}

// baseKeyBindings are the default scheme's bindings, taken from
// internal/tui/keys.go's DefaultKeyMap so a key means the same thing on both
// surfaces: Up/Down/Top/Bottom already carry vi aliases (j/k, g/G) there,
// because that is the TUI's own scheme-independent default, not something
// specific to a "vim" choice.
var baseKeyBindings = map[string][]string{
	"up":         {"ArrowUp", "k"},
	"down":       {"ArrowDown", "j"},
	"top":        {"g", "Home"},
	"bottom":     {"G", "End"},
	"open":       {"Enter"},
	"back":       {"Escape"},
	"new":        {"n"},
	"editTitle":  {"e"},
	"comment":    {"m"},
	"transition": {"t"},
	"filter":     {"/"},
	"help":       {"?"},
}

// keySchemeOverrides are the actions each scheme rebinds away from the base,
// mirroring internal/tui/scheme.go's schemeKeys: an action this map does not
// name for a scheme keeps the base binding, so no scheme can leave one
// unbound.
//
// The emacs row diverges from internal/tui/scheme.go's SchemeEmacs on
// several keys because the browser itself owns those chords and never
// delivers them to page script, no matter how early preventDefault runs:
// ctrl+n opens a new window, ctrl+p opens the print dialog, ctrl+e is
// Chrome's "search this engine" omnibox shortcut, ctrl+s opens Save Page As,
// and ctrl+t opens a new tab. Binding them anyway would look like the vim
// scheme's "/" for Filter but silently do nothing, which is the same bug the
// TUI just had to fix for a different reason. Each is dropped in favour of
// the closest key that does reach the page, noted inline below.
var keySchemeOverrides = map[KeyScheme]map[string][]string{
	KeySchemeVim: {
		"new":       {"o"},
		"editTitle": {"i"},
		"comment":   {"a"},
		"filter":    {"/", ":"},
	},
	KeySchemeEmacs: {
		"up":     {"ArrowUp"},        // ctrl+p is the browser's print shortcut
		"down":   {"ArrowDown"},      // ctrl+n opens a new browser window
		"top":    {"Home", "ctrl+a"}, // ctrl+a (select all) is safe to intercept
		"bottom": {"End"},            // ctrl+e is Chrome's omnibox search shortcut
		"back":   {"Escape", "ctrl+g"},
		// Filter keeps the base "/": the TUI binds it only to ctrl+s, which
		// opens Save Page As and cannot be overridden from script.
		// EditTitle keeps the base "e": the TUI binds it to ctrl+t, which
		// opens a new browser tab and cannot be overridden from script.
	},
}

// ShortcutActionNames lists every action a scheme can bind a key to, in the
// order the help overlay shows it.
func ShortcutActionNames() []string {
	out := make([]string, 0, len(shortcutActions))
	for _, a := range shortcutActions {
		out = append(out, a.Name)
	}
	return out
}

// KeyBindingsFor resolves one scheme's full binding table, base bindings
// first and the scheme's own overrides layered on top, exactly as
// internal/tui/scheme.go's KeyMapFor layers schemeKeys over DefaultKeyMap.
func KeyBindingsFor(scheme KeyScheme) map[string][]string {
	out := make(map[string][]string, len(baseKeyBindings))
	for action, keys := range baseKeyBindings {
		out[action] = append([]string{}, keys...)
	}
	for action, keys := range keySchemeOverrides[scheme] {
		out[action] = append([]string{}, keys...)
	}
	return out
}

// shortcutSchemeChoice is one option the settings menu and the help overlay
// offer.
type shortcutSchemeChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// shortcutData is everything a page embeds for its shortcut script to read.
// It is the one place both the keydown handler and the help overlay draw
// from, client side, so the two cannot drift apart the way a hardcoded help
// list and a hardcoded handler table could.
type shortcutData struct {
	Scheme   string                         `json:"scheme"`
	Schemes  []shortcutSchemeChoice         `json:"schemes"`
	Actions  []shortcutAction               `json:"actions"`
	Bindings map[string]map[string][]string `json:"bindings"`
}

// shortcutDataFor builds the embedded binding table for one active scheme,
// carrying every scheme's table so the client need not round-trip to the
// server merely to preview another one.
func shortcutDataFor(active KeyScheme) shortcutData {
	schemes := KeySchemes()
	bindings := make(map[string]map[string][]string, len(schemes))
	choices := make([]shortcutSchemeChoice, 0, len(schemes))
	for _, s := range schemes {
		bindings[string(s)] = KeyBindingsFor(s)
		choices = append(choices, shortcutSchemeChoice{Value: string(s), Label: keySchemeLabel(s)})
	}
	return shortcutData{
		Scheme:   string(active),
		Schemes:  choices,
		Actions:  shortcutActions,
		Bindings: bindings,
	}
}

// shortcutTable is the settings page's scheme comparison: one column per
// scheme, one row per action, built from the same shortcutActions and
// KeyBindingsFor that drive the embedded JSON, so the two can never disagree
// about what a scheme binds.
type shortcutTable struct {
	Schemes []shortcutSchemeChoice
	Rows    []shortcutTableRow
}

// shortcutTableRow is one action's binding across every scheme, in the same
// order as shortcutTable.Schemes.
type shortcutTableRow struct {
	Label string
	Cells [][]string
}

// buildShortcutTable renders the comparison every shipped scheme offers, so a
// reader can weigh vim or emacs against the default before switching, rather
// than switching, checking the help overlay, and switching back.
func buildShortcutTable() shortcutTable {
	schemes := KeySchemes()
	choices := make([]shortcutSchemeChoice, 0, len(schemes))
	bindings := make([]map[string][]string, 0, len(schemes))
	for _, s := range schemes {
		choices = append(choices, shortcutSchemeChoice{Value: string(s), Label: keySchemeLabel(s)})
		bindings = append(bindings, KeyBindingsFor(s))
	}
	rows := make([]shortcutTableRow, 0, len(shortcutActions))
	for _, a := range shortcutActions {
		row := shortcutTableRow{Label: a.Label}
		for _, table := range bindings {
			row.Cells = append(row.Cells, table[a.Name])
		}
		rows = append(rows, row)
	}
	return shortcutTable{Schemes: choices, Rows: rows}
}

// shortcutsJSON renders the active scheme's binding table as the JSON a page
// embeds inside a <script type="application/json"> element. The data is
// fixed literals plus the resolved cookie value, so marshalling cannot fail
// in practice; the empty object is a defensive fallback only.
func shortcutsJSON(active KeyScheme) template.JS {
	raw, err := json.Marshal(shortcutDataFor(active))
	if err != nil {
		return template.JS("{}") // #nosec G203 -- fixed fallback, not caller input
	}
	// #nosec G203 -- json.Marshal escapes '<', '>' and '&' by default, and
	// every value marshalled here is a fixed action, label or key name; none
	// of it is user-supplied HTML.
	return template.JS(raw)
}
