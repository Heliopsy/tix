package tui

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/heliopsy/tix/internal/core"
)

// Scheme names a shipped set of keybindings. People arrive with muscle memory
// from another editor, so the interface offers presets rather than asking
// everyone to rebind every action by hand.
type Scheme string

// The shipped schemes.
const (
	SchemeDefault Scheme = "default"
	SchemeVim     Scheme = "vim"
	SchemeEmacs   Scheme = "emacs"
	SchemeNano    Scheme = "nano"
	SchemeHelix   Scheme = "helix"
)

// There is deliberately no "mac" scheme. A terminal never sees the command
// key, and the ctrl chords macOS applies to every text field system wide
// (ctrl-a, ctrl-e, ctrl-b, ctrl-f, ctrl-n, ctrl-p, ctrl-k) are the emacs ones,
// inherited from Cocoa's emacs key bindings. A mac scheme would therefore be
// SchemeEmacs under a second name, so the emacs description names mac instead.

// Schemes lists every shipped scheme, in the order the settings view offers.
func Schemes() []Scheme {
	return []Scheme{SchemeDefault, SchemeVim, SchemeEmacs, SchemeNano, SchemeHelix}
}

// SchemeDescription says what a scheme changes, for the settings view.
func SchemeDescription(s Scheme) string {
	switch s {
	case SchemeVim:
		return "vim reflexes: o opens a new task, i edits, : opens the filter"
	case SchemeEmacs:
		return "emacs and mac reflexes: ctrl-n, ctrl-p, ctrl-a, ctrl-e, ctrl-g"
	case SchemeNano:
		return "nano reflexes: ctrl-w searches, ctrl-g helps, ctrl-x quits, ctrl-o applies"
	case SchemeHelix:
		return "helix reflexes: x opens the row, o adds, i edits, d releases, space opens settings"
	default:
		return "the shipped bindings"
	}
}

// ParseScheme resolves a scheme name, naming the alternatives when it cannot.
func ParseScheme(name string) (Scheme, error) {
	want := Scheme(strings.ToLower(strings.TrimSpace(name)))
	if want == "" {
		return SchemeDefault, nil
	}
	for _, s := range Schemes() {
		if s == want {
			return s, nil
		}
	}
	return SchemeDefault, core.Invalid("keybinding scheme %q is not one of %s", name, schemeNames())
}

// schemeNames renders the shipped schemes for a message.
func schemeNames() string {
	names := make([]string, 0, len(Schemes()))
	for _, s := range Schemes() {
		names = append(names, string(s))
	}
	return strings.Join(names, ", ")
}

// schemeKeys are the actions each scheme rebinds away from the default. Every
// other action keeps its default keys, so no scheme can leave one unbound.
var schemeKeys = map[Scheme]map[string][]string{
	SchemeVim: {
		"New":        {"o"},
		"NewProject": {"o"},
		"EditTitle":  {"i"},
		"EditBody":   {"I"},
		"Filter":     {"/", ":"},
		"Comment":    {"a"},
		"Refresh":    {"e"},
	},
	SchemeEmacs: {
		"Up":        {"up", "ctrl+p"},
		"Down":      {"down", "ctrl+n"},
		"Left":      {"left", "ctrl+b", "shift+tab"},
		"Right":     {"right", "ctrl+f", "tab"},
		"Top":       {"home", "ctrl+a"},
		"Bottom":    {"end", "ctrl+e"},
		"Back":      {"esc", "ctrl+g"},
		"Cancel":    {"esc", "ctrl+g"},
		"Filter":    {"ctrl+s"},
		"Refresh":   {"ctrl+l"},
		"EditTitle": {"ctrl+t"},
	},
	// nano's keys are its own footer, from the GNU nano manual: ^W searches
	// ("Where Is"), ^G shows help, ^X exits, ^O writes out, ^L redraws, and
	// ^P/^N/^B/^F/^A/^E move. ^K, which cuts a line, is deliberately left
	// alone: tix has no action that removes the selected task, and Release
	// gives up a lease rather than deleting anything, so binding it would be
	// inventing a meaning nano does not have. ^C is left alone too, because
	// Interrupt already owns it.
	SchemeNano: {
		"Up":      {"up", "ctrl+p"},
		"Down":    {"down", "ctrl+n"},
		"Left":    {"left", "ctrl+b", "shift+tab"},
		"Right":   {"right", "ctrl+f", "tab"},
		"Top":     {"home", "ctrl+a"},
		"Bottom":  {"end", "ctrl+e"},
		"Filter":  {"ctrl+w", "/"},
		"Help":    {"ctrl+g", "?"},
		"Quit":    {"ctrl+x", "q"},
		"Refresh": {"ctrl+l", "r"},
		"Accept":  {"ctrl+o", "enter"},
	},
	// helix is modal but selection first, so its keys are not vim's: from the
	// helix keymap, x selects the line under the cursor, d deletes the
	// selection, o opens a line below, i inserts before it, a appends after
	// it, : enters command mode, / searches, and space opens the menu layer.
	// x taking the row means Release moves to d, helix's own discard key,
	// rather than keeping the default x and meaning two things. Space is
	// listed second for Settings because the first key of a binding is the one
	// the footer prints, and a footer cannot show a space.
	SchemeHelix: {
		"Enter":      {"x", "enter"},
		"Release":    {"d"},
		"New":        {"o"},
		"NewProject": {"o"},
		"EditTitle":  {"i"},
		"EditBody":   {"I"},
		"Comment":    {"a"},
		"Filter":     {"/", ":"},
		"Settings":   {",", " "},
	},
}

// ActionNames lists every action a binding can be attached to, in a stable
// order, taken from the KeyMap itself so the list cannot drift from it.
func ActionNames() []string {
	t := reflect.TypeOf(KeyMap{})
	out := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		out = append(out, t.Field(i).Name)
	}
	return out
}

// Binding returns the binding an action carries.
func (k KeyMap) Binding(action string) (key.Binding, bool) {
	v := reflect.ValueOf(k).FieldByName(action)
	if !v.IsValid() {
		return key.Binding{}, false
	}
	b, ok := v.Interface().(key.Binding)
	return b, ok
}

// WithBinding returns a copy of the map with one action bound to other keys,
// keeping the help text so the footer keeps describing what the action does.
func (k KeyMap) WithBinding(action string, keys []string) (KeyMap, error) {
	current, ok := k.Binding(action)
	if !ok {
		return k, core.Invalid("%q is not an action the interface offers", action)
	}
	if len(keys) == 0 {
		return k, core.Invalid("action %q cannot be bound to no keys", action)
	}
	next := key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], current.Help().Desc))
	out := k
	reflect.ValueOf(&out).Elem().FieldByName(action).Set(reflect.ValueOf(next))
	return out, nil
}

// KeyMapFor builds the bindings of a scheme. A scheme naming an action the
// KeyMap does not have is a typo in a compiled-in table, not a condition a
// run can recover from, so it panics rather than dropping the binding: a
// discarded error here costs the user a key with nothing failing anywhere.
// Actions are applied in name order so a table with two faults always reports
// the same one.
func KeyMapFor(scheme Scheme) KeyMap {
	k := DefaultKeyMap()
	table := schemeKeys[scheme]
	actions := make([]string, 0, len(table))
	for action := range table {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	for _, action := range actions {
		next, err := k.WithBinding(action, table[action])
		if err != nil {
			panic(fmt.Sprintf("keybinding scheme %q: %v", scheme, err))
		}
		k = next
	}
	return k
}

// viewActions names the actions each view listens for, which is the set a
// collision has to be looked for within: the same key may mean two things in
// two views, and often should.
func viewActions(v viewKind) []string {
	global := []string{"Help", "Refresh", "Projects", "Activity", "Quit", "Interrupt"}
	switch v {
	case viewProjects:
		return append([]string{"Up", "Down", "Top", "Bottom", "Enter", "NewProject", "Back"}, global...)
	case viewBoard:
		return append([]string{
			"Up", "Down", "Left", "Right", "Top", "Bottom", "Enter", "Filter", "ClearFltr",
			"Claim", "Release", "Transition", "New", "EditTitle", "EditBody", "Priority",
			"Assign", "Comment", "Tag", "Untag", "Depend", "ClaimNext", "Renew", "Settings", "Back",
		}, global...)
	case viewDetail:
		return append([]string{
			"Up", "Down", "Claim", "Release", "Transition", "New", "EditTitle", "EditBody",
			"Priority", "Assign", "Comment", "Tag", "Untag", "Depend", "Renew", "Settings", "Back",
		}, global...)
	case viewSettings:
		return append([]string{"Up", "Down", "Top", "Bottom", "Enter", "Back"}, global...)
	case viewActivity:
		return append([]string{"Up", "Down", "Top", "Bottom", "Back"}, global...)
	default:
		return append([]string{"Up", "Down", "Top", "Back"}, global...)
	}
}

// Collision is one key bound to two actions in the same view.
type Collision struct {
	View    viewKind
	Key     string
	Actions []string
}

// Error renders a collision as the message the interface reports.
func (c Collision) Error() string {
	sort.Strings(c.Actions)
	return "key " + c.Key + " is bound to " + strings.Join(c.Actions, " and ") +
		" in the " + viewName(c.View) + " view"
}

// Validate reports every key bound twice within one view. A rebinding that
// collides is refused rather than applied, because the alternative is a key
// that quietly stops doing what its owner expects.
func (k KeyMap) Validate() []Collision {
	var out []Collision
	for _, v := range []viewKind{viewProjects, viewBoard, viewDetail, viewSettings, viewActivity, viewHelp} {
		owners := map[string][]string{}
		for _, action := range viewActions(v) {
			b, ok := k.Binding(action)
			if !ok {
				continue
			}
			for _, name := range b.Keys() {
				owners[name] = append(owners[name], action)
			}
		}
		names := make([]string, 0, len(owners))
		for name, actions := range owners {
			if len(actions) > 1 {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			out = append(out, Collision{View: v, Key: name, Actions: owners[name]})
		}
	}
	return out
}

// KeyMapFrom builds the bindings for a scheme with per-action overrides laid
// over it, refusing the result when it would leave a key meaning two things.
func KeyMapFrom(scheme Scheme, overrides map[string]string) (KeyMap, error) {
	k := KeyMapFor(scheme)
	actions := make([]string, 0, len(overrides))
	for action := range overrides {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	for _, action := range actions {
		next, err := k.WithBinding(action, splitKeys(overrides[action]))
		if err != nil {
			return DefaultKeyMap(), err
		}
		k = next
	}
	if clashes := k.Validate(); len(clashes) > 0 {
		return DefaultKeyMap(), core.Invalid("%s", clashes[0].Error())
	}
	return k, nil
}

// splitKeys parses a comma separated list of key names.
func splitKeys(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// viewName names a view for a message.
func viewName(v viewKind) string {
	switch v {
	case viewProjects:
		return "projects"
	case viewBoard:
		return "board"
	case viewDetail:
		return "detail"
	case viewSettings:
		return "settings"
	case viewActivity:
		return "activity"
	default:
		return "help"
	}
}
