// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// paletteKey is the press that opens the palette, built the way pressKey builds
// the rest but for a chord it does not name.
func paletteKey() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl} }

// openPaletteOn opens the palette over a model and fails if it did not take.
func openPaletteOn(t *testing.T, m Model) Model {
	t.Helper()
	next, _ := m.reduce(paletteKey())
	if !next.paletteOpen {
		t.Fatal("ctrl+k did not open the palette")
	}
	return next
}

// typeQuery sends a query one rune at a time, which is what a reader does.
func typeQuery(m Model, query string) Model {
	for _, r := range query {
		m, _ = m.reduce(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// names lists the entry names of a palette listing.
func names(entries []PaletteEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

// fullPalette is the context of a reader who may do everything, with a task
// selected and a project open, so a missing entry is never explained by
// permission or by context.
func fullPalette() PaletteContext {
	return PaletteContext{
		ActionContext: ActionContext{May: permitAll, HasTask: true, HasProject: true},
		Offered:       allViews,
	}
}

func TestMatchActionIsCaseInsensitiveAndWordwise(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "new task", query: "", want: true},
		{name: "new task", query: "   ", want: true},
		{name: "new task", query: "new", want: true},
		{name: "new task", query: "NEW", want: true},
		{name: "New Task", query: "new", want: true},
		{name: "new task", query: "task new", want: true},
		{name: "new task", query: "ew ta", want: true},
		{name: "new task", query: "new project", want: false},
		{name: "new task", query: "zzz", want: false},
		{name: "statistics", query: "stat", want: true},
		{name: "record artifact", query: "art", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name+"/"+tc.query, func(t *testing.T) {
			t.Parallel()
			if got := MatchAction(tc.name, tc.query); got != tc.want {
				t.Errorf("MatchAction(%q, %q) = %v, want %v", tc.name, tc.query, got, tc.want)
			}
		})
	}
}

// TestTheEmptyQueryListsEverythingTheReaderIsOffered is the half of the matcher
// a table cannot state: an empty query is not "match nothing", it is the whole
// list, because that is what a palette opens showing.
func TestTheEmptyQueryListsEverythingTheReaderIsOffered(t *testing.T) {
	t.Parallel()
	k := DefaultKeyMap()
	ctx := fullPalette()
	if got, all := len(k.PaletteFor(ctx, "")), len(k.PaletteActions(ctx)); got != all {
		t.Fatalf("an empty query listed %d of %d entries", got, all)
	}
	if len(k.PaletteFor(ctx, "")) == 0 {
		t.Fatal("the palette offers nothing at all, so this guard is watching nothing")
	}
}

// TestAQueryMatchingNothingSaysSoRatherThanRenderingAnEmptyBox holds the case a
// list is most likely to get wrong. An empty panel reads as a palette that has
// broken rather than a query that is wrong.
func TestAQueryMatchingNothingSaysSoRatherThanRenderingAnEmptyBox(t *testing.T) {
	m := openPaletteOn(t, boardModel(t))
	m = typeQuery(m, "zzzz")
	if len(m.paletteEntries()) != 0 {
		t.Fatalf("the query matched %v", names(m.paletteEntries()))
	}
	rows := strings.Join(m.palettePanel().Rows, "\n")
	if !strings.Contains(rows, NoPaletteMatch) {
		t.Fatalf("a query matching nothing rendered no explanation:\n%s", rows)
	}
	if !strings.Contains(rows, `"zzzz"`) {
		t.Fatalf("the no-match line does not name the query:\n%s", rows)
	}
}

// TestThePaletteOffersNothingTheReaderHasNoAuthorityFor is the promise the help
// overlay already makes, made by the palette: an entry the service would refuse
// has told the reader the refusal was their mistake.
func TestThePaletteOffersNothingTheReaderHasNoAuthorityFor(t *testing.T) {
	t.Parallel()
	k := DefaultKeyMap()

	// A token that may read tasks and projects and nothing else: no event
	// subscription, so no activity view, and no write scope, so no claim.
	actor := &core.Actor{ID: "p", TenantID: "t", Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}}
	m := New(Config{Actor: actor, Access: capability.TUIAccess(actor)})
	narrowed := names(k.PaletteActions(PaletteContext{
		ActionContext: ActionContext{May: m.permits(), HasTask: true, HasProject: true},
		Offered:       m.offersView(),
	}))

	for _, refused := range []string{
		k.Activity.Help().Desc, k.Claim.Help().Desc, k.Delete.Help().Desc, k.Comment.Help().Desc,
	} {
		if slices.Contains(narrowed, refused) {
			t.Errorf("a reader refused it is offered %q: %v", refused, narrowed)
		}
	}
	for _, kept := range []string{k.Help.Help().Desc, k.Quit.Help().Desc, k.Stats.Help().Desc} {
		if !slices.Contains(narrowed, kept) {
			t.Errorf("a narrowed palette dropped %q, which the reader may still reach: %v", kept, narrowed)
		}
	}
	if nothing := k.PaletteActions(PaletteContext{}); len(nothing) >= len(narrowed) {
		t.Errorf("a palette with no predicates at all offered %d entries", len(nothing))
	}
}

// TestThePaletteAndTheHelpOverlayOfferTheSameActions is the guard the whole
// change rests on. Two lists maintained apart will disagree, and a palette
// offering an action the overlay does not document, or omitting one it does, is
// the drift this was built to make impossible.
func TestThePaletteAndTheHelpOverlayOfferTheSameActions(t *testing.T) {
	t.Parallel()
	k := DefaultKeyMap()
	ctx := fullPalette()
	offered := names(k.PaletteActions(ctx))

	documented := make([]string, 0, 32)
	for _, e := range append(k.GlobalHelp(allViews), k.taskActions(permitAll)...) {
		documented = append(documented, e.Desc)
	}

	for _, want := range documented {
		// The palette's own key is documented by the overlay and deliberately
		// not listed by the palette, which is the one asymmetry.
		if want == k.Palette.Help().Desc {
			continue
		}
		if !slices.Contains(offered, want) {
			t.Errorf("the overlay documents %q and the palette does not offer it", want)
		}
	}
	for _, got := range offered {
		if !slices.Contains(documented, got) {
			t.Errorf("the palette offers %q and the overlay documents no such action", got)
		}
	}
}

// TestAnActionOnTheSelectionIsHiddenWithNothingSelected pins the decision that
// such an entry is omitted rather than shown disabled, which is the decision
// GlobalHelp already takes about a view a reader is refused.
func TestAnActionOnTheSelectionIsHiddenWithNothingSelected(t *testing.T) {
	t.Parallel()
	k := DefaultKeyMap()
	with := names(k.PaletteActions(fullPalette()))
	without := names(k.PaletteActions(PaletteContext{
		ActionContext: ActionContext{May: permitAll}, Offered: allViews,
	}))

	for _, onTask := range []string{k.Claim.Help().Desc, k.Edit.Help().Desc, k.Delete.Help().Desc} {
		if !slices.Contains(with, onTask) {
			t.Errorf("%q is not offered with a task selected", onTask)
		}
		if slices.Contains(without, onTask) {
			t.Errorf("%q is offered with nothing selected: %v", onTask, without)
		}
	}
	// A new task needs an open project rather than a selection, and claiming
	// the next one needs neither.
	if slices.Contains(without, k.New.Help().Desc) {
		t.Errorf("a new task is offered with no project open: %v", without)
	}
	if !slices.Contains(without, k.ClaimNext.Help().Desc) {
		t.Errorf("claiming the next task needs no selection and was dropped: %v", without)
	}
	if !slices.Contains(without, k.Settings.Help().Desc) {
		t.Errorf("a navigation entry was dropped with nothing selected: %v", without)
	}
}

// TestTheOpenViewIsMarkedOnItsOwnEntry is the "where am I" half of a list whose
// whole purpose is going somewhere else.
func TestTheOpenViewIsMarkedOnItsOwnEntry(t *testing.T) {
	t.Parallel()
	k := DefaultKeyMap()
	for _, tc := range []struct {
		view viewKind
		want string
	}{
		{view: viewActivity, want: k.Activity.Help().Desc},
		{view: viewProjects, want: k.Projects.Help().Desc},
		{view: viewTenant, want: k.Tenant.Help().Desc},
	} {
		t.Run(viewName(tc.view), func(t *testing.T) {
			t.Parallel()
			ctx := fullPalette()
			ctx.Current = tc.view
			marked := 0
			for _, e := range k.PaletteActions(ctx) {
				if !e.Current {
					continue
				}
				marked++
				if e.Name != tc.want {
					t.Errorf("the %s view marked %q", viewName(tc.view), e.Name)
				}
			}
			if marked != 1 {
				t.Errorf("%d entries are marked as the open view", marked)
			}
		})
	}
}

// TestThePaletteRendersEachEntrysOwnKey is why the palette is not a replacement
// for the bindings: a reader who uses it learns the key and eventually stops
// needing it.
func TestThePaletteRendersEachEntrysOwnKey(t *testing.T) {
	m := openPaletteOn(t, boardModel(t))
	m = typeQuery(m, "settings")
	rows := strings.Join(m.palettePanel().Rows, "\n")
	if !strings.Contains(rows, m.keys.Settings.Help().Key) {
		t.Fatalf("the settings entry does not carry its key:\n%s", rows)
	}
	if !strings.Contains(rows, m.keys.Settings.Help().Desc) {
		t.Fatalf("the settings entry does not carry its name:\n%s", rows)
	}
}

// TestEveryActionIsPerformed is the guard that keeps the one list honest from the
// other end. An action listed with no case in performAction is a palette row that
// does nothing and a key that has silently stopped working.
func TestEveryActionIsPerformed(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	for _, a := range m.keys.Actions() {
		if _, _, handled := m.performAction(a.id); !handled {
			t.Errorf("action %q (%s) is in the list and nothing performs it",
				a.Name(), a.Keys())
		}
	}
}

// TestEveryActionResolvesFromItsOwnKey is the other half: a descriptor whose key
// nothing dispatches is a palette entry the reader cannot learn a key from.
func TestEveryActionResolvesFromItsOwnKey(t *testing.T) {
	t.Parallel()
	k := DefaultKeyMap()
	all := k.Actions()
	if len(all) < 20 {
		t.Fatalf("the action list holds %d entries; the walk is broken", len(all))
	}
	for i, a := range all {
		first := a.binding.Keys()[0]
		got, ok := ResolveAction(pressFor(first), all)
		if !ok {
			t.Errorf("%q, the first key of %q, resolves to no action", first, a.Name())
			continue
		}
		if got.id != a.id {
			// The position is in the message because two actions sharing a key
			// can also share a name, and "tenant resolves to tenant" reads as a
			// passing test rather than as two entries fighting over one key.
			t.Errorf("%q, the first key of the action at position %d (%q), resolves to the "+
				"action at position %d (%q) instead", first, i, a.Name(),
				slices.IndexFunc(all, func(x Action) bool { return x.id == got.id }), got.Name())
		}
	}
}

// pressFor builds the press a terminal sends for one key name, including the
// chords pressKey does not spell.
func pressFor(name string) tea.KeyPressMsg {
	if chord, ok := strings.CutPrefix(name, "ctrl+"); ok {
		return tea.KeyPressMsg{Code: []rune(chord)[0], Mod: tea.ModCtrl}
	}
	return pressKey(name)
}

// TestChoosingAnEntryTakesTheKeysOwnPath is the rule that a palette entry is not
// a second implementation of its action. The palette and the key press are each
// driven through a real Model from the same starting state, and the whole frame
// is compared as well as the state the action was for.
//
// The frame is what makes this a guard rather than a gesture. Checking only
// "the settings view opened" passed while the palette entered the view directly
// instead of through openSettings, because a fresh model's cursor is on row zero
// either way; checking only "the comment prompt opened" passed while the palette
// seeded that prompt with its own query. Both show up in the frame.
func TestChoosingAnEntryTakesTheKeysOwnPath(t *testing.T) {
	tests := []struct {
		name  string
		query string
		press string
		// prepare puts the model into a state where the action's own work is
		// observable, so a shortcut that skips it cannot look identical.
		prepare func(Model) Model
		check   func(t *testing.T, viaKey, viaPalette Model)
	}{
		{
			name: "opening a view", query: "settings", press: ",",
			// The cursor is left down the list, because opening the settings
			// screen puts it back at the top and entering the view does not.
			prepare: func(m Model) Model { m.settingSel, m.settingsOff = 3, 2; return m },
			check: func(t *testing.T, viaKey, viaPalette Model) {
				t.Helper()
				if viaKey.view != viewSettings || viaPalette.view != viewSettings {
					t.Fatalf("key landed on %v, palette on %v", viaKey.view, viaPalette.view)
				}
				if viaKey.settingSel != viaPalette.settingSel || viaKey.settingsOff != viaPalette.settingsOff {
					t.Errorf("the cursor is at %d/%d via the key and %d/%d via the palette",
						viaKey.settingSel, viaKey.settingsOff, viaPalette.settingSel, viaPalette.settingsOff)
				}
			},
		},
		{
			name: "acting on the selection", query: "comment", press: "m",
			check: func(t *testing.T, viaKey, viaPalette Model) {
				t.Helper()
				if viaKey.prompt != promptComment || viaPalette.prompt != promptComment {
					t.Fatalf("key opened %v, palette opened %v", viaKey.prompt, viaPalette.prompt)
				}
				if viaPalette.area.Value() != viaKey.area.Value() {
					t.Errorf("the field holds %q via the palette and %q via the key",
						viaPalette.area.Value(), viaKey.area.Value())
				}
			},
		},
		{
			name: "opening a picker", query: "transition", press: "t",
			check: func(t *testing.T, viaKey, viaPalette Model) {
				t.Helper()
				if viaKey.choice != choiceTransition || viaPalette.choice != choiceTransition {
					t.Fatalf("key chose %v, palette chose %v", viaKey.choice, viaPalette.choice)
				}
				if len(viaKey.choices) != len(viaPalette.choices) {
					t.Errorf("choices differ: %d and %d", len(viaKey.choices), len(viaPalette.choices))
				}
			},
		},
		{
			// The palette's query lives in the same field the prompt it opens
			// gathers into, so this is the case where a second code path would
			// have left the query behind as the prompt's seed.
			name: "opening a prompt the palette typed into", query: "artifact", press: "O",
			check: func(t *testing.T, viaKey, viaPalette Model) {
				t.Helper()
				if viaKey.prompt != promptArtifact || viaPalette.prompt != promptArtifact {
					t.Fatalf("key opened %v, palette opened %v", viaKey.prompt, viaPalette.prompt)
				}
				if viaPalette.input.Value() != viaKey.input.Value() {
					t.Errorf("the prompt is seeded with %q via the palette and %q via the key",
						viaPalette.input.Value(), viaKey.input.Value())
				}
			},
		},
		{
			name: "returning to the root", query: "projects", press: "W",
			check: func(t *testing.T, viaKey, viaPalette Model) {
				t.Helper()
				if viaKey.view != viewProjects || viaPalette.view != viewProjects {
					t.Fatalf("key landed on %v, palette on %v", viaKey.view, viaPalette.view)
				}
				if len(viaKey.stack) != 0 || len(viaPalette.stack) != 0 {
					t.Errorf("stacks differ: %d and %d", len(viaKey.stack), len(viaPalette.stack))
				}
			},
		},
		{
			name: "cycling a priority, which sends at once", query: "cycle", press: "p",
			check: func(t *testing.T, viaKey, viaPalette Model) {
				t.Helper()
				if viaKey.err != "" || viaPalette.err != "" {
					t.Fatalf("key reported %q, palette reported %q", viaKey.err, viaPalette.err)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := boardModel(t)
			base.svc = newFakeService()
			if tc.prepare != nil {
				base = tc.prepare(base)
			}

			viaKey, _ := base.reduce(pressKey(tc.press))

			viaPalette := typeQuery(openPaletteOn(t, base), tc.query)
			if len(viaPalette.paletteEntries()) == 0 {
				t.Fatalf("the query %q matched nothing", tc.query)
			}
			viaPalette, _ = viaPalette.reduce(pressKey("enter"))
			if viaPalette.paletteOpen {
				t.Fatal("running an entry left the palette open")
			}
			tc.check(t, viaKey, viaPalette)
			// The frame, last and widest: an action reached two ways that draws
			// two screens has been implemented twice, whatever the fields above
			// happen to agree about.
			if key, palette := viaKey.Frame(), viaPalette.Frame(); key != palette {
				t.Errorf("the key and the palette draw different frames.\nvia the key:\n%s\nvia the palette:\n%s",
					key, palette)
			}
		})
	}
}

// TestCancellingThePaletteLeavesNothingBehind is the promise every other input
// mode already makes: escape puts the screen back exactly as it was.
func TestCancellingThePaletteLeavesNothingBehind(t *testing.T) {
	before := boardModel(t)
	m := typeQuery(openPaletteOn(t, before), "settings")
	m, cmd := m.reduce(pressKey("esc"))
	if m.paletteOpen || cmd != nil {
		t.Fatalf("esc left the palette open=%v cmd=%v", m.paletteOpen, cmd)
	}
	if m.view != before.view || m.sel != before.sel {
		t.Fatalf("esc moved the view to %v and the selection to %+v", m.view, m.sel)
	}
	if m.input.Value() != "" {
		t.Fatalf("the query survived the cancel: %q", m.input.Value())
	}
}

// TestALetterBoundToAMovementKeyIsTypedIntoTheQuery is the collision the palette
// cannot avoid by rebinding: Up is bound to the letter k, and inside a query
// field a letter is text.
func TestALetterBoundToAMovementKeyIsTypedIntoTheQuery(t *testing.T) {
	m := openPaletteOn(t, boardModel(t))
	if !slices.Contains(m.keys.Up.Keys(), "k") {
		t.Skip("the up key is no longer bound to a letter")
	}
	m = typeQuery(m, "k")
	if m.input.Value() != "k" {
		t.Fatalf("k moved the highlight instead of typing: query = %q", m.input.Value())
	}
	moved, _ := m.reduce(pressKey("down"))
	if moved.paletteSel == m.paletteSel && len(moved.paletteEntries()) > 1 {
		t.Fatal("the arrow key did not move the highlight")
	}
}

// TestThePaletteScrollsRatherThanEatingTheBoard holds the panel to a window, so
// a list of every action does not leave the view it was opened over invisible.
func TestThePaletteScrollsRatherThanEatingTheBoard(t *testing.T) {
	m := openPaletteOn(t, boardModel(t))
	entries := m.paletteEntries()
	if len(entries) <= PaletteRows {
		t.Fatalf("only %d entries are offered; the window is not exercised", len(entries))
	}
	rows := m.palettePanel().Rows
	// The query row, at most PaletteRows entries, and the scroll hint.
	if len(rows) > PaletteRows+2 {
		t.Fatalf("the panel drew %d rows for %d entries", len(rows), len(entries))
	}
	if !strings.Contains(strings.Join(rows, "\n"), "more") {
		t.Fatalf("a windowed list gave no sign of what it is hiding:\n%s", strings.Join(rows, "\n"))
	}
}

// TestThePaletteOwnsTheFooterWhileItIsOpen is the rule every other input mode
// obeys: a legend offering "n new task" over a query field describes a keystroke
// that types the letter n.
func TestThePaletteOwnsTheFooterWhileItIsOpen(t *testing.T) {
	m := openPaletteOn(t, boardModel(t))
	footer := m.footerLines()
	if !strings.Contains(footer, PaletteTitle) {
		t.Fatalf("the palette is open and its panel is not in the footer:\n%s", footer)
	}
	if strings.Contains(footer, m.keys.New.Help().Desc) {
		t.Fatalf("the board's own keys are still advertised under the palette:\n%s", footer)
	}
}

// TestTheFooterNamesThePaletteKeyInEveryView is the complaint this change
// answers, stated as a guard: the keys existed and nothing advertised them, so a
// palette nobody is told about would repeat the failure exactly.
func TestTheFooterNamesThePaletteKeyInEveryView(t *testing.T) {
	for _, v := range everyView {
		t.Run(viewName(v), func(t *testing.T) {
			m := boardModel(t)
			m.view = v
			footer := m.footerLines()
			if !strings.Contains(footer, m.keys.Palette.Help().Key) {
				t.Errorf("the %s footer does not name the palette key:\n%s", viewName(v), footer)
			}
			if !strings.Contains(footer, m.keys.Palette.Help().Desc) {
				t.Errorf("the %s footer names the key without saying what it opens:\n%s",
					viewName(v), footer)
			}
		})
	}
}

// TestThePaletteOpensFromEveryViewThatAdvertisesIt is the other half of the
// footer's promise. The settings screen and the help overlay take every other
// key for themselves, so the key they advertise has to be answered before they
// get the chance: the footer named it on both while pressing it did nothing.
func TestThePaletteOpensFromEveryViewThatAdvertisesIt(t *testing.T) {
	for _, v := range everyView {
		t.Run(viewName(v), func(t *testing.T) {
			m := boardModel(t)
			m.view = v
			if !strings.Contains(m.footerLines(), m.keys.Palette.Help().Key) {
				t.Skipf("the %s footer does not advertise the palette", viewName(v))
			}
			next, _ := m.reduce(paletteKey())
			if !next.paletteOpen {
				t.Errorf("the %s footer advertises the palette key and pressing it did nothing",
					viewName(v))
			}
			if next.view != v {
				t.Errorf("opening the palette moved the view from %s to %s",
					viewName(v), viewName(next.view))
			}
		})
	}
}

// TestAnOpenPanelSwallowsThePaletteKey is the exception, stated rather than
// discovered: while a prompt holds the keyboard the footer is that panel's own
// legend, it promises nothing about the palette, and ctrl+k there is text.
func TestAnOpenPanelSwallowsThePaletteKey(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m, _ = m.reduce(pressKey("m"))
	if m.prompt != promptComment {
		t.Fatal("the comment prompt did not open")
	}
	if strings.Contains(m.footerLines(), m.keys.Palette.Help().Key) {
		t.Fatal("an open panel still advertises the palette key")
	}
	next, _ := m.reduce(paletteKey())
	if next.paletteOpen {
		t.Fatal("the palette opened over an input that was holding the keyboard")
	}
}

// TestTheFooterHintSurvivesTheNarrowestTerminalAndIsDroppedBelowIt is the
// boundary. The hint is placed first so truncation eats the view's own keys, and
// below the width its own text needs it is dropped whole: "ctrl+k comm…" names a
// key nobody can press.
func TestTheFooterHintSurvivesTheNarrowestTerminalAndIsDroppedBelowIt(t *testing.T) {
	k := DefaultKeyMap()
	whole := k.Palette.Help().Key + " " + k.Palette.Help().Desc

	tests := []struct {
		name  string
		width int
		want  string
	}{
		{name: "unbounded", width: 0, want: whole},
		{name: "wide", width: 120, want: whole},
		{name: "the floor the board draws at", width: MinWidth, want: whole},
		{name: "exactly the hint", width: len(whole), want: whole},
		{name: "one cell short", width: len(whole) - 1, want: ""},
		{name: "unusably narrow", width: 4, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FooterHint(k, tc.width); got != tc.want {
				t.Errorf("FooterHint(width %d) = %q, want %q", tc.width, got, tc.want)
			}
		})
	}

	// And the placement: at the floor the footer keeps the hint and loses the
	// view's own bindings rather than the other way round.
	m := boardModel(t)
	m.width, m.height = MinWidth, MinHeight
	footer := m.footerLines()
	if !strings.Contains(footer, k.Palette.Help().Key) {
		t.Errorf("a %d-cell footer dropped the palette hint:\n%s", MinWidth, footer)
	}
}
