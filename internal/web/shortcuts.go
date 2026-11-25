// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/i18n"
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
	KeySchemeOff     KeyScheme = "off"
)

// KeySchemes lists every shipped scheme.
//
// internal/tui/scheme.go also ships nano and helix, which are deliberately not
// mirrored here. Every key that makes nano nano is a chord the browser owns
// and never delivers to page script: ctrl+w closes the tab, ctrl+x cuts,
// ctrl+o opens a file, ctrl+l focuses the address bar, ctrl+g and ctrl+p are
// find-next and print. Helix differs from vim mainly in Release and Settings,
// two actions the browser interface does not offer at all. Either one would
// therefore resolve to a table already shipped under another name, which is a
// second entry in the picker that changes nothing when chosen.
func KeySchemes() []KeyScheme {
	return []KeyScheme{KeySchemeDefault, KeySchemeVim, KeySchemeEmacs, KeySchemeOff}
}

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
	case KeySchemeOff:
		return "Off"
	default:
		return "Default"
	}
}

// The groups the help overlay heads its actions with, in the order it draws
// them. The app's help uses the same four.
const (
	groupNavigate = "Navigate"
	groupLists    = "Lists"
	groupTask     = "Task"
	groupGeneral  = "General"
)

// shortcutGroups lists the help overlay's groups in the order it draws them.
var shortcutGroups = []string{groupNavigate, groupLists, groupTask, groupGeneral}

// shortcutAction is one action the shortcut layer can drive, in the order the
// help overlay lists it. This is the entire vocabulary: a scheme may only
// choose which keys reach an action, never add an action of its own, which is
// what keeps a key from ever being bound to something the web UI cannot do.
type shortcutAction struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Group string `json:"group"`
}

// shortcutActions covers exactly what the web UI can do today, under the
// labels the app's help uses for the same actions (one key table for both
// surfaces, openspec/changes/unified-keys). Claiming a task is absent: the
// browser interface does not expose ClaimTask or ClaimNext at all (see
// Exemptions in routes.go). Go to Manage and Delete are in the shared table
// but not here: the web has no Manage landing page, and deleting from the
// keyboard is the app's alone.
var shortcutActions = []shortcutAction{
	{"goInbox", "Go to Inbox", groupNavigate},
	{"goMyWork", "Go to My work", groupNavigate},
	{"goTasks", "Go to Tasks", groupNavigate},
	{"goProjects", "Go to Projects", groupNavigate},
	{"goActivity", "Go to Activity", groupNavigate},
	{"goSettings", "Go to Settings", groupNavigate},
	{"goStats", "Go to Statistics", groupNavigate},
	{"down", "Move selection down", groupLists},
	{"up", "Move selection up", groupLists},
	{"top", "Jump to the first task", groupLists},
	{"bottom", "Jump to the last task", groupLists},
	{"pageDown", "Page down", groupLists},
	{"pageUp", "Page up", groupLists},
	{"open", "Open the selected task", groupLists},
	{"back", "Go back", groupLists},
	{"select", "Select or unselect the focused task", groupLists},
	{"extendDown", "Extend the selection down", groupLists},
	{"extendUp", "Extend the selection up", groupLists},
	{"selectAll", "Select every task in the list", groupLists},
	{"filter", "Focus the filter field", groupLists},
	{"refresh", "Refresh", groupLists},
	{"switchView", "Switch view", groupLists},
	{"new", "New task", groupLists},
	{"quickPaste", "Quick add from the clipboard", groupLists},
	{"complete", "Mark done", groupTask},
	{"editTitle", "Edit the title", groupTask},
	{"editDescription", "Edit the description", groupTask},
	{"transition", "Move to another status", groupTask},
	{"comment", "Comment", groupTask},
	{"tag", "Add a tag", groupTask},
	{"star", "Star or unstar", groupTask},
	{"priority", "Set the priority", groupTask},
	{"assign", "Assign", groupTask},
	{"due", "Set the due date", groupTask},
	{"snooze", "Snooze or unsnooze", groupTask},
	{"copyRef", "Copy the reference", groupTask},
	{"palette", "Open the command palette", groupGeneral},
	{"help", "Show this help", groupGeneral},
}

// shortcutDestinations are the screens the go-to actions open.
var shortcutDestinations = map[string]string{
	"goInbox":    RouteInbox,
	"goMyWork":   RouteMine,
	"goTasks":    RouteTasks,
	"goProjects": RouteProjects,
	"goActivity": RouteActivity,
	"goSettings": RouteSettings,
	"goStats":    RouteStats,
}

// A key is written the way assets/decide.js's comboOf names a keydown: the
// key itself, or ctrl+, meta+, alt+ or shift+ and the key. Two keys pressed
// one after the other are written with a space between them ("g i"); the
// shortcut script waits up to a second for the second one.
//
// The palette is ctrl+k, as it is in the TUI, and meta+k for a Mac's command
// key. No scheme takes either chord away.
//
// baseKeyBindings are the default scheme's bindings, the Default column of
// the key table the web and the app share. j/k and g/G are there in every
// scheme, as they are in the TUI's own default.
var baseKeyBindings = map[string][]string{
	"goInbox":    {"g i"},
	"goMyWork":   {"g m"},
	"goTasks":    {"g t"},
	"goProjects": {"g p"},
	"goActivity": {"g a"},
	"goSettings": {"g s"},
	"goStats":    {"g S"},
	"down":       {"ArrowDown", "j"},
	"up":         {"ArrowUp", "k"},
	"top":        {"g", "Home"},
	"bottom":     {"G", "End"},
	"pageDown":   {"PageDown"},
	"pageUp":     {"PageUp"},
	"open":       {"Enter"},
	"back":       {"Escape"},
	"select":     {"x"},
	"extendDown": {"shift+ArrowDown"},
	"extendUp":   {"shift+ArrowUp"},
	"selectAll":  {"ctrl+a", "meta+a"},
	"filter":     {"/"},
	"refresh":    {"r"},
	"switchView": {"v"},
	"new":        {"n"},
	// The app's chord. In a text field it stays the browser's paste as plain
	// text, since the shortcut layer stands down there.
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

// keySchemeOverrides are the actions each scheme rebinds away from the base,
// mirroring internal/tui/scheme.go's schemeKeys: an action this map does not
// name for a scheme keeps the base binding. A nil list unbinds the action.
//
// Some of the app's keys for these schemes are chords the browser itself
// owns and never delivers to page script, no matter how early preventDefault
// runs: ctrl+n opens a new window, ctrl+p prints, ctrl+s saves the page,
// ctrl+l focuses the address bar, ctrl+f is find and ctrl+b toggles the
// bookmarks bar in Firefox. The web drops each and keeps the other keys the
// shared table gives the action.
var keySchemeOverrides = map[KeyScheme]map[string][]string{
	KeySchemeVim: {
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
	},
	KeySchemeEmacs: {
		"palette":  {"ctrl+k", "meta+k", "alt+x"},
		"back":     {"Escape", "ctrl+g"},
		"down":     {"ArrowDown"}, // ctrl+n opens a new browser window
		"up":       {"ArrowUp"},   // ctrl+p is the browser's print shortcut
		"top":      {"alt+<", "Home"},
		"bottom":   {"alt+>", "End"},
		"pageDown": {"ctrl+v", "PageDown"},
		"pageUp":   {"alt+v", "PageUp"},
		"select":   {"ctrl+space", "x"},
		// Filter keeps "/": the app's ctrl+s opens Save Page As. Refresh
		// keeps "r": the app's ctrl+l focuses the address bar.
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

// IsChord reports whether a key is pressed with ctrl or meta held, the only
// keys the Off scheme keeps.
func IsChord(key string) bool {
	return strings.HasPrefix(key, "ctrl+") || strings.HasPrefix(key, "meta+")
}

// KeyBindingsFor resolves one scheme's full binding table, base bindings
// first and the scheme's own overrides layered on top, exactly as
// internal/tui/scheme.go's KeyMapFor layers schemeKeys over DefaultKeyMap.
// Off is the default table with every key but the ctrl and meta chords taken
// out, so typing a letter anywhere does nothing while ctrl+k still opens the
// palette.
func KeyBindingsFor(scheme KeyScheme) map[string][]string {
	out := make(map[string][]string, len(baseKeyBindings))
	for action, keys := range baseKeyBindings {
		out[action] = append([]string{}, keys...)
	}
	for action, keys := range keySchemeOverrides[scheme] {
		out[action] = append([]string{}, keys...)
	}
	if scheme == KeySchemeOff {
		for action, keys := range out {
			out[action] = slices.DeleteFunc(keys, func(k string) bool { return !IsChord(k) })
		}
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
	Groups   []shortcutSchemeChoice         `json:"groups"`
	Actions  []shortcutAction               `json:"actions"`
	Bindings map[string]map[string][]string `json:"bindings"`
	Go       map[string]string              `json:"go"`
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
	groups := make([]shortcutSchemeChoice, 0, len(shortcutGroups))
	for _, g := range shortcutGroups {
		groups = append(groups, shortcutSchemeChoice{Value: g, Label: g})
	}
	return shortcutData{
		Scheme:   string(active),
		Schemes:  choices,
		Groups:   groups,
		Actions:  shortcutActions,
		Bindings: bindings,
		Go:       shortcutDestinations,
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
func shortcutsJSON(active KeyScheme, lang string) template.JS {
	data := shortcutDataFor(active)
	data.Actions = slices.Clone(data.Actions)
	for i := range data.Actions {
		data.Actions[i].Label = i18n.T(lang, data.Actions[i].Label)
	}
	for i := range data.Schemes {
		data.Schemes[i].Label = i18n.T(lang, data.Schemes[i].Label)
	}
	for i := range data.Groups {
		data.Groups[i].Label = i18n.T(lang, data.Groups[i].Label)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return template.JS("{}") // #nosec G203 -- fixed fallback, not caller input
	}
	// #nosec G203 -- json.Marshal escapes '<', '>' and '&' by default, and
	// every value marshalled here is a fixed action, label or key name; none
	// of it is user-supplied HTML.
	return template.JS(raw)
}
