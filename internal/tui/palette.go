// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// PaletteEntry is one row of the command palette: what the action is called,
// the key that performs it, and whether it is the view the reader is already on.
type PaletteEntry struct {
	Name string
	Keys string
	// Current marks the entry for the open view, so a reader can see where
	// they are in a list whose whole purpose is going somewhere else.
	Current bool
	id      actionID
}

// PaletteContext is what the palette needs in order to decide what to offer:
// the selection facts the footer already computes, which views this reader is
// offered, and the view they are in.
//
// Both predicates are the ones the rest of the interface filters with rather
// than a second opinion formed here. Nil offers nothing, the closed default
// taken everywhere else.
type PaletteContext struct {
	ActionContext
	Offered func(viewKind) bool
	Current viewKind
}

// offers reports whether an action may be listed for this reader right now.
//
// An action the reader cannot run is left out rather than listed and disabled.
// That is the decision GlobalHelp already takes about a view a reader is
// refused, for the reason it gives: an entry that leads to a refusal has told
// the reader the refusal was their mistake.
func (c PaletteContext) offers(a Action) bool {
	switch {
	case a.hidden:
		return false
	case a.needsTask && !c.HasTask:
		return false
	case a.needsProject && !c.HasProject:
		return false
	case a.always:
		return true
	case a.opensView():
		return c.Offered != nil && c.Offered(a.opens)
	default:
		return a.gate.permitted(c.May)
	}
}

// PaletteActions lists the actions this reader may perform now, in the order
// Actions declares them: the cross-view keys as the help overlay prints them,
// then the ones that act on the selection. Filtering is stable, so narrowing
// never moves a row out from under the key that would run it.
func (k KeyMap) PaletteActions(ctx PaletteContext) []PaletteEntry {
	all := k.Actions()
	out := make([]PaletteEntry, 0, len(all))
	for _, a := range all {
		if !ctx.offers(a) {
			continue
		}
		out = append(out, PaletteEntry{
			Name:    a.Name(),
			Keys:    a.Keys(),
			Current: a.opensView() && a.opens == ctx.Current,
			id:      a.id,
		})
	}
	return out
}

// PaletteFor is PaletteActions narrowed by a query.
func (k KeyMap) PaletteFor(ctx PaletteContext, query string) []PaletteEntry {
	all := k.PaletteActions(ctx)
	out := make([]PaletteEntry, 0, len(all))
	for _, e := range all {
		if MatchAction(e.Name, query) {
			out = append(out, e)
		}
	}
	return out
}

// MatchAction reports whether a query names an action. Every word of the query
// must appear in the name, ignoring case, and the words may be given in any
// order, so "task new" finds "new task". An empty query matches everything.
//
// It is a pure function of two strings so that it is tested directly rather
// than through a rendered frame.
func MatchAction(name, query string) bool {
	folded := strings.ToLower(name)
	for _, word := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(folded, word) {
			return false
		}
	}
	return true
}

// PaletteRows is the most entries the palette draws at once. Its panel grows
// into the body, so a list of every action would leave no board behind it; the
// rest is reached by scrolling and the hint below says how much there is.
const PaletteRows = 8

// PaletteCurrentMark says which entry is the open view, in a word rather than a
// colour, because nothing in this interface is carried by colour alone.
const PaletteCurrentMark = "· here"

// NoPaletteMatch is what the palette says when a query names nothing. An empty
// panel reads as a palette that has broken rather than a query that is wrong.
const NoPaletteMatch = "no action matches"

// PaletteTitle and PaletteField name the panel and its query row. The title is
// the word the footer hint uses, so the key and what it opens read as one thing.
const (
	PaletteTitle = "commands"
	PaletteField = "match"
	PaletteNote  = "type to narrow; every entry shows the key that does the same thing"
)

// PaletteQueryLimit bounds the query. An action's name is a handful of words, so
// anything longer is a paste rather than a search.
const PaletteQueryLimit = 64

// FooterHint is the footer's standing advertisement of the palette, and empty
// for a terminal too narrow to carry the whole of it: a hint cut to
// "ctrl+k comm…" names a key nobody can press, which is worse than no hint.
func FooterHint(k KeyMap, width int) string {
	hint := k.Palette.Help().Key + " " + k.Palette.Help().Desc
	if width > 0 && width < lipgloss.Width(hint) {
		return ""
	}
	return hint
}

// paletteContext is what the open palette filters itself with.
func (m Model) paletteContext() PaletteContext {
	return PaletteContext{ActionContext: m.actionContext(), Offered: m.offersView(), Current: m.view}
}

// paletteEntries are the rows the open palette is showing.
func (m Model) paletteEntries() []PaletteEntry {
	return m.keys.PaletteFor(m.paletteContext(), m.input.Value())
}

// paletteRows is how many entries this terminal has room for.
func (m Model) paletteRows(total int) int {
	return VisibleRows(max(1, min(PaletteRows, m.height-MinHeight)), total)
}

// openPalette focuses the query field on an empty query, which is the whole
// list: a palette that opened narrowed by whatever was typed last would answer
// a question the reader did not ask.
func (m Model) openPalette() Model {
	m.paletteOpen, m.paletteSel, m.paletteOff, m.err = true, 0, 0, ""
	// The panel says what the field is for on its own note line, the way every
	// prompt does; the field carries only what was typed.
	m.input.Prompt, m.input.Placeholder, m.input.CharLimit = "", "", PaletteQueryLimit
	m.input.SetValue("")
	m.input.Focus()
	return m
}

// closePalette dismisses the palette, leaving the view exactly as it was.
func (m Model) closePalette() Model {
	m.paletteOpen, m.paletteSel, m.paletteOff = false, 0, 0
	m.input.SetValue("")
	m.input.Blur()
	return m
}

// handlePaletteKey narrows the list, moves the highlight and runs an entry,
// under the interface's own bindings rather than literals of its own.
func (m Model) handlePaletteKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	entries := m.paletteEntries()
	switch {
	case key.Matches(msg, m.keys.Cancel):
		return m.closePalette(), nil
	case key.Matches(msg, m.keys.Accept):
		return m.runPaletteEntry(entries)
	case movesWithin(msg, m.keys.Up):
		m.paletteSel = clamp(m.paletteSel-1, 0, len(entries)-1)
	case movesWithin(msg, m.keys.Down):
		m.paletteSel = clamp(m.paletteSel+1, 0, len(entries)-1)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		// A narrowed list is a different list, so the highlight goes back to
		// its top rather than staying on a row number that now means something
		// else.
		m.paletteSel, m.paletteOff = 0, 0
		return m, cmd
	}
	m.paletteOff = ScrollWindow(m.paletteOff, m.paletteSel, m.paletteRows(len(entries)), len(entries))
	return m, nil
}

// movesWithin reports whether a press moves the highlight inside a control that
// is also taking text. Only the binding's non-typing keys count: Up is bound to
// "up" and to "k", and inside a query field k is the letter k.
func movesWithin(msg tea.KeyPressMsg, b key.Binding) bool {
	for _, name := range b.Keys() {
		if len([]rune(name)) == 1 {
			continue
		}
		if msg.String() == name {
			return true
		}
	}
	return false
}

// runPaletteEntry performs the highlighted action by handing its id to the one
// switch that performs it. That is the same call the key press makes: an action
// performed from here by a second route would be a second implementation of it,
// free to drift from the key that is supposed to be teaching it.
func (m Model) runPaletteEntry(entries []PaletteEntry) (Model, tea.Cmd) {
	if m.paletteSel < 0 || m.paletteSel >= len(entries) {
		return m.closePalette(), nil
	}
	id := entries[m.paletteSel].id
	next, cmd, _ := m.closePalette().performAction(id)
	return next, cmd
}

// palettePanel renders the palette as the panel every other input mode uses.
func (m Model) palettePanel() Panel {
	entries := m.paletteEntries()
	out := []string{m.fit("  " + pad(PaletteField, FormLabelWidth) + m.input.View())}
	if len(entries) == 0 {
		out = append(out, m.fit("  "+m.theme.Dim.Render(NoPaletteMatch+" "+quoted(m.input.Value()))))
		return Panel{Title: PaletteTitle, Note: PaletteNote, Rows: out, Keys: m.paletteHelp()}
	}
	rows := m.paletteRows(len(entries))
	offset := ScrollWindow(m.paletteOff, m.paletteSel, rows, len(entries))
	for i := offset; i < len(entries) && i < offset+rows; i++ {
		out = append(out, m.paletteRow(entries[i], i == m.paletteSel))
	}
	if hint := ScrollHint(offset, rows, len(entries)); hint != "" {
		out = append(out, m.fit("  "+m.theme.Dim.Render(hint)))
	}
	return Panel{Title: PaletteTitle, Note: PaletteNote, Rows: out, Keys: m.paletteHelp()}
}

// paletteRow draws one entry, its key in the column the panels line values up
// at so the keys read as a column rather than as part of the sentence.
func (m Model) paletteRow(e PaletteEntry, selected bool) string {
	name := e.Name
	if e.Current {
		name += " " + PaletteCurrentMark
	}
	row := "  " + pad(e.Keys, FormLabelWidth) + name
	if selected {
		return m.fit(m.selection().Render(row))
	}
	return m.fit(m.theme.Dim.Render(row))
}

// paletteHelp names the keys that drive the palette, from the bindings that
// drive every other input mode. The movement keys are named by the half of their
// bindings that works here, because the other half is a letter the query takes.
func (m Model) paletteHelp() string {
	return m.keys.Accept.Help().Key + " run   " +
		moveKey(m.keys.Up) + "/" + moveKey(m.keys.Down) + " move   " +
		m.keys.Cancel.Help().Key + " cancel"
}

// arrows render a movement key the way a terminal draws it, so a legend does not
// spell the word "up" at a reader looking for an arrow.
var arrows = map[string]string{"up": "↑", "down": "↓"}

// moveKey is the key a binding offers inside a field that is taking text: its
// first key that is not a single printable character.
func moveKey(b key.Binding) string {
	for _, name := range b.Keys() {
		if len([]rune(name)) == 1 {
			continue
		}
		if glyph, ok := arrows[name]; ok {
			return glyph
		}
		return name
	}
	return ""
}

// quoted renders a query for the no-match line.
func quoted(query string) string { return `"` + strings.TrimSpace(query) + `"` }
