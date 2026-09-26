// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
)

func TestEveryShippedSchemeIsUsableAndCollisionFree(t *testing.T) {
	for _, scheme := range Schemes() {
		t.Run(string(scheme), func(t *testing.T) {
			k := KeyMapFor(scheme)
			if clashes := k.Validate(); len(clashes) > 0 {
				t.Fatalf("%s binds a key twice: %s", scheme, clashes[0].Error())
			}
			for _, action := range ActionNames() {
				b, ok := k.Binding(action)
				if !ok || len(b.Keys()) == 0 {
					t.Fatalf("%s leaves %q unbound", scheme, action)
				}
				if b.Help().Key == "" || b.Help().Desc == "" {
					t.Fatalf("%s leaves %q undocumented: %+v", scheme, action, b.Help())
				}
			}
			if SchemeDescription(scheme) == "" {
				t.Fatalf("%s describes itself as nothing", scheme)
			}
		})
	}
}

func TestParseScheme(t *testing.T) {
	tests := []struct {
		in      string
		want    Scheme
		wantErr bool
	}{
		{"", SchemeDefault, false},
		{"default", SchemeDefault, false},
		{"vim", SchemeVim, false},
		{"  EMACS  ", SchemeEmacs, false},
		{"kakoune", SchemeDefault, true},
	}
	for _, tc := range tests {
		got, err := ParseScheme(tc.in)
		if (err != nil) != tc.wantErr {
			t.Fatalf("ParseScheme(%q) err = %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseScheme(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if tc.wantErr && !strings.Contains(err.Error(), "vim") {
			t.Fatalf("the refusal does not name the alternatives: %v", err)
		}
	}
}

func TestAnOverrideIsAppliedAndKeepsItsHelpText(t *testing.T) {
	k, err := KeyMapFrom(SchemeDefault, map[string]string{"New": "o"})
	if err != nil {
		t.Fatalf("KeyMapFrom: %v", err)
	}
	b, _ := k.Binding("New")
	if len(b.Keys()) != 1 || b.Keys()[0] != "o" {
		t.Fatalf("keys = %v", b.Keys())
	}
	if b.Help().Desc != "new task" {
		t.Fatalf("the override lost the action's description: %q", b.Help().Desc)
	}
	if b.Help().Key != "o" {
		t.Fatalf("the footer would still advertise %q", b.Help().Key)
	}
}

// TestACollidingOverrideIsRefusedRatherThanSwallowed holds the rule that a key
// must never quietly stop working: a rebinding that would make one key mean
// two things in one view is reported and rejected.
func TestACollidingOverrideIsRefusedRatherThanSwallowed(t *testing.T) {
	_, err := KeyMapFrom(SchemeDefault, map[string]string{"New": "c"})
	if err == nil {
		t.Fatal("binding new task over claim was accepted")
	}
	if !strings.Contains(err.Error(), "c") || !strings.Contains(err.Error(), "board") {
		t.Fatalf("the refusal does not say what collided or where: %v", err)
	}
}

func TestOverridesAreRefusedForUnknownActionsAndEmptyKeys(t *testing.T) {
	if _, err := KeyMapFrom(SchemeDefault, map[string]string{"Teleport": "z"}); err == nil {
		t.Fatal("an action the interface does not have was accepted")
	}
	if _, err := KeyMapFrom(SchemeDefault, map[string]string{"New": "  "}); err == nil {
		t.Fatal("an action was allowed to be bound to no keys")
	}
}

func TestMultipleOverridesApplyTogether(t *testing.T) {
	k, err := KeyMapFrom(SchemeVim, map[string]string{"Comment": "z", "Depend": "Z"})
	if err != nil {
		t.Fatalf("KeyMapFrom: %v", err)
	}
	for action, want := range map[string]string{"Comment": "z", "Depend": "Z"} {
		b, _ := k.Binding(action)
		if b.Keys()[0] != want {
			t.Fatalf("%s = %v, want %q", action, b.Keys(), want)
		}
	}
	if clashes := k.Validate(); len(clashes) > 0 {
		t.Fatalf("collision after overrides: %s", clashes[0].Error())
	}
}

// TestTheFooterRendersTheActiveSchemesKeys is the rule that help text and the
// real bindings can never disagree: the footer is built from the key map.
func TestTheFooterRendersTheActiveSchemesKeys(t *testing.T) {
	ctx := ActionContext{May: permitAll, HasProject: true, HasTask: true, CanTransition: true}
	def := DefaultKeyMap()
	vim := KeyMapFor(SchemeVim)

	advertised := func(k KeyMap) string {
		var keys []string
		for _, e := range k.ShortHelp(viewBoard, ctx) {
			keys = append(keys, e.Keys)
		}
		return strings.Join(keys, " ")
	}
	if !strings.Contains(advertised(def), "n") {
		t.Fatalf("the default footer does not advertise n: %q", advertised(def))
	}
	if !strings.Contains(advertised(vim), "o") {
		t.Fatalf("the vim footer does not advertise o: %q", advertised(vim))
	}
	if advertised(def) == advertised(vim) {
		t.Fatal("the footer is the same under two schemes, so it is hardcoded")
	}
}

func TestSwitchingSchemeInTheSettingsViewTakesEffectAtOnce(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m, _ = m.reduce(pressKey(","))
	if m.view != viewSettings {
		t.Fatalf("the settings view did not open: %v", m.view)
	}
	m, _ = m.reduce(pressKey("l"))
	if m.scheme != SchemeVim {
		t.Fatalf("scheme = %q", m.scheme)
	}
	b, _ := m.keys.Binding("New")
	if b.Keys()[0] != "o" {
		t.Fatalf("the vim keys were not installed: %v", b.Keys())
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view != viewBoard {
		t.Fatalf("leaving settings landed on %v", m.view)
	}
	frame := m.View()
	if !strings.Contains(frame, "o new task") {
		t.Fatalf("the footer did not follow the scheme:\n%s", frame)
	}
}

func TestTheSettingsViewNamesTheActiveScheme(t *testing.T) {
	m := boardModel(t)
	m = m.useScheme(SchemeEmacs)
	m = m.openSettings()
	row := settingRow(t, m, SettingKeymap)
	if !strings.Contains(row, "emacs") {
		t.Fatalf("the keys row does not name the active scheme: %q", row)
	}
	preview := strings.Join(m.settingsLines(LayoutFor(m.width, m.height, 0)), "\n")
	if !strings.Contains(preview, "ctrl+t") {
		t.Fatalf("the emacs bindings are not previewed:\n%s", preview)
	}
	if m.settingSel != SettingKeymap {
		t.Fatalf("the settings view opened on row %d rather than the keys row", m.settingSel)
	}
}

func TestAConfiguredSchemeIsInstalledAtStartup(t *testing.T) {
	m := New(Config{Access: fullAccess(), Scheme: "vim"})
	if m.scheme != SchemeVim {
		t.Fatalf("scheme = %q", m.scheme)
	}
	b, _ := m.keys.Binding("EditTitle")
	if b.Keys()[0] != "i" {
		t.Fatalf("keys = %v", b.Keys())
	}
}

func TestABadConfiguredSchemeIsReportedRatherThanIgnored(t *testing.T) {
	m := New(Config{Access: fullAccess(), Scheme: "kakoune"})
	if m.scheme != SchemeDefault {
		t.Fatalf("scheme = %q", m.scheme)
	}
	if !strings.Contains(m.err, "kakoune") {
		t.Fatalf("the interface started on the defaults without saying why: %q", m.err)
	}
}

func TestABadConfiguredOverrideIsReportedRatherThanIgnored(t *testing.T) {
	m := New(Config{Access: fullAccess(), Overrides: map[string]string{"New": "c"}})
	if !strings.Contains(m.err, "keybindings") {
		t.Fatalf("a colliding override started in silence: %q", m.err)
	}
	b, _ := m.keys.Binding("New")
	if b.Keys()[0] != "n" {
		t.Fatalf("a refused override was applied anyway: %v", b.Keys())
	}
}

func TestActionNamesCoverTheWholeKeyMap(t *testing.T) {
	names := ActionNames()
	if len(names) < 20 {
		t.Fatalf("only %d actions were found", len(names))
	}
	k := DefaultKeyMap()
	for _, name := range names {
		if _, ok := k.Binding(name); !ok {
			t.Fatalf("%q is listed but carries no binding", name)
		}
	}
	if _, ok := k.Binding("NotAnAction"); ok {
		t.Fatal("an action the map does not have reported a binding")
	}
}

func TestCollisionNamesTheViewAndBothActions(t *testing.T) {
	k, err := DefaultKeyMap().WithBinding("Comment", []string{"c"})
	if err != nil {
		t.Fatalf("WithBinding: %v", err)
	}
	clashes := k.Validate()
	if len(clashes) == 0 {
		t.Fatal("a duplicate binding was not detected")
	}
	msg := clashes[0].Error()
	for _, want := range []string{"c", "Claim", "Comment"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("%q is missing from %q", want, msg)
		}
	}
}

// TestNoSchemeNamesAnActionTheKeyMapDoesNotHave is the guard the discarded
// error in KeyMapFor used to hide: a mistyped action name in schemeKeys used
// to drop that scheme's binding silently, so the only symptom was a user
// losing a key. It is checked here as well as by the panic, because a table
// scan names the scheme and the bad action instead of a stack trace.
func TestNoSchemeNamesAnActionTheKeyMapDoesNotHave(t *testing.T) {
	known := map[string]bool{}
	for _, action := range ActionNames() {
		known[action] = true
	}
	for scheme, table := range schemeKeys {
		for action, keys := range table {
			if !known[action] {
				t.Errorf("scheme %q rebinds %q, which is not an action the interface offers", scheme, action)
			}
			if len(keys) == 0 {
				t.Errorf("scheme %q binds %q to no keys", scheme, action)
			}
			for _, k := range keys {
				if k == "" {
					t.Errorf("scheme %q binds %q to an empty key name", scheme, action)
				}
			}
		}
	}
}

// TestKeyMapForRefusesAnActionThatDoesNotExist proves the refusal is the
// panic and not a dropped binding, so a typo cannot reach a release however
// it arrives in the table.
func TestKeyMapForRefusesAnActionThatDoesNotExist(t *testing.T) {
	const probe Scheme = "probe-not-shipped"
	schemeKeys[probe] = map[string][]string{"EditTitl": {"ctrl+t"}}
	defer delete(schemeKeys, probe)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("a scheme naming an action the interface does not have was accepted")
		}
		msg, _ := r.(string)
		for _, want := range []string{string(probe), "EditTitl"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("the panic does not say %q: %v", want, r)
			}
		}
	}()
	_ = KeyMapFor(probe)
}

// TestEveryViewListsOnlyRealActions keeps the collision guard from going
// blind: Validate skips a name no binding answers to, so a typo in
// viewActions would stop that action being checked for collisions at all.
func TestEveryViewListsOnlyRealActions(t *testing.T) {
	k := DefaultKeyMap()
	for _, v := range []viewKind{viewProjects, viewBoard, viewDetail, viewSettings, viewActivity,
		viewProject, viewHelp} {
		for _, action := range viewActions(v) {
			if _, ok := k.Binding(action); !ok {
				t.Errorf("the %s view lists %q, which carries no binding", viewName(v), action)
			}
		}
	}
}

// TestNoSchemeCollidesAcceptWithCancel covers the one pair Validate cannot:
// prompts are not a viewKind, so the two keys that drive every prompt are
// checked here. Cancel is matched first, so sharing a key would make a prompt
// impossible to accept.
func TestNoSchemeCollidesAcceptWithCancel(t *testing.T) {
	for _, scheme := range Schemes() {
		k := KeyMapFor(scheme)
		accept, _ := k.Binding("Accept")
		cancel, _ := k.Binding("Cancel")
		for _, a := range accept.Keys() {
			for _, c := range cancel.Keys() {
				if a == c {
					t.Errorf("scheme %q binds %q to both Accept and Cancel, so no prompt could be accepted", scheme, a)
				}
			}
		}
	}
}

// TestNoTwoSchemesShipTheSameBindings keeps the picker honest: a scheme whose
// keys are another scheme's keys is a second name for one set of bindings,
// which is why there is no mac scheme. macOS sends no command key to a
// terminal and its system-wide text chords are the emacs ones, so a mac entry
// would fail here against emacs rather than mislead anyone into choosing it.
func TestNoTwoSchemesShipTheSameBindings(t *testing.T) {
	schemes := Schemes()
	for i, a := range schemes {
		for _, b := range schemes[i+1:] {
			ka, kb := KeyMapFor(a), KeyMapFor(b)
			same := true
			for _, action := range ActionNames() {
				ba, _ := ka.Binding(action)
				bb, _ := kb.Binding(action)
				if strings.Join(ba.Keys(), ",") != strings.Join(bb.Keys(), ",") {
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

// TestNanoAndHelixCarryTheKeysTheirEditorsUse pins the bindings each new
// scheme claims, so a later edit cannot quietly turn "nano reflexes" into
// something nano does not do. Sources: the GNU nano manual (^W Where Is,
// ^G help, ^X exit, ^O write out, ^L redraw, ^P/^N/^B/^F/^A/^E movement) and
// the helix keymap (x select line, d delete selection, o open below,
// i insert, a append, : command mode, / search, space menu layer).
func TestNanoAndHelixCarryTheKeysTheirEditorsUse(t *testing.T) {
	tests := []struct {
		scheme Scheme
		action string
		key    string
	}{
		{SchemeNano, "Filter", "ctrl+w"},
		{SchemeNano, "Help", "ctrl+g"},
		{SchemeNano, "Quit", "ctrl+x"},
		{SchemeNano, "Accept", "ctrl+o"},
		{SchemeNano, "Refresh", "ctrl+l"},
		{SchemeNano, "Up", "ctrl+p"},
		{SchemeNano, "Down", "ctrl+n"},
		{SchemeNano, "Top", "ctrl+a"},
		{SchemeNano, "Bottom", "ctrl+e"},
		{SchemeHelix, "Enter", "x"},
		{SchemeHelix, "Release", "d"},
		{SchemeHelix, "New", "o"},
		{SchemeHelix, "EditTitle", "i"},
		{SchemeHelix, "Comment", "a"},
		{SchemeHelix, "Filter", ":"},
		{SchemeHelix, "Settings", " "},
	}
	for _, tc := range tests {
		k := KeyMapFor(tc.scheme)
		b, ok := k.Binding(tc.action)
		if !ok {
			t.Fatalf("%s has no action %q", tc.scheme, tc.action)
		}
		found := false
		for _, name := range b.Keys() {
			if name == tc.key {
				found = true
			}
		}
		if !found {
			t.Errorf("%s binds %s to %v, which does not include %q", tc.scheme, tc.action, b.Keys(), tc.key)
		}
	}
	// nano and helix must still keep the shipped key for everything they do
	// not name, which is what stops a scheme leaving an action unreachable.
	for _, scheme := range []Scheme{SchemeNano, SchemeHelix} {
		k := KeyMapFor(scheme)
		b, _ := k.Binding("Claim")
		if len(b.Keys()) != 1 || b.Keys()[0] != "c" {
			t.Errorf("%s disturbed an action it does not rebind: Claim = %v", scheme, b.Keys())
		}
	}
}
