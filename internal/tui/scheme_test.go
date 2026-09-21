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
	ctx := ActionContext{HasProject: true, HasTask: true, CanTransition: true}
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
	m, _ = m.reduce(pressKey("j"))
	m, _ = m.reduce(pressKey("enter"))
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

func TestTheSettingsViewMarksTheActiveScheme(t *testing.T) {
	m := boardModel(t)
	m = m.useScheme(SchemeEmacs)
	m = m.openSettings()
	lines := strings.Join(m.settingsLines(), "\n")
	if !strings.Contains(lines, "✓ emacs") {
		t.Fatalf("the active scheme is not marked:\n%s", lines)
	}
	for _, scheme := range Schemes() {
		if !strings.Contains(lines, string(scheme)) {
			t.Fatalf("%s is not offered:\n%s", scheme, lines)
		}
	}
	if m.schemeSel != 2 {
		t.Fatalf("the settings view opened on row %d rather than the active scheme", m.schemeSel)
	}
}

func TestAConfiguredSchemeIsInstalledAtStartup(t *testing.T) {
	m := New(Config{Scheme: "vim"})
	if m.scheme != SchemeVim {
		t.Fatalf("scheme = %q", m.scheme)
	}
	b, _ := m.keys.Binding("EditTitle")
	if b.Keys()[0] != "i" {
		t.Fatalf("keys = %v", b.Keys())
	}
}

func TestABadConfiguredSchemeIsReportedRatherThanIgnored(t *testing.T) {
	m := New(Config{Scheme: "kakoune"})
	if m.scheme != SchemeDefault {
		t.Fatalf("scheme = %q", m.scheme)
	}
	if !strings.Contains(m.err, "kakoune") {
		t.Fatalf("the interface started on the defaults without saying why: %q", m.err)
	}
}

func TestABadConfiguredOverrideIsReportedRatherThanIgnored(t *testing.T) {
	m := New(Config{Overrides: map[string]string{"New": "c"}})
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
