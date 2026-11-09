// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/tasklink"
)

// NoLinkNote is what the share picker says when it has no address to link
// a task under.
const NoLinkNote = "no link: the server sets no web address (server.public_url, TIX_SERVER_PUBLIC_URL)"

// sharedText is what the share key copied, kept on screen until the next key.
type sharedText struct {
	label string
	text  string
}

// TaskLink is a task's address in the browser interface: its web_url, else
// its screen under base, else "" when there is neither.
func TaskLink(t core.Task, base string) string {
	switch {
	case t.WebURL != "":
		return t.WebURL
	case base != "":
		return tasklink.URL(base, t.Ref)
	default:
		return ""
	}
}

// ShareChoices are the forms the share picker offers, the ones the web Share
// menu copies. Without an address there is no link to copy, so it offers the
// key and title and the Markdown label alone.
func ShareChoices(t core.Task, address string) []Choice {
	ref := output.ShowRef(t.Ref)
	text := Choice{Label: "key and title", Value: tasklink.Text(ref, t.Title)}
	if address == "" {
		return []Choice{text, {Label: "Markdown label", Value: tasklink.Label(ref, t.Title)}}
	}
	return []Choice{
		{Label: "link", Value: address},
		{Label: "Markdown", Value: tasklink.Markdown(ref, t.Title, address)},
		text,
	}
}

// startShare opens the share picker over the selected task.
func (m Model) startShare() Model {
	task, ok := m.selectedTask()
	if !ok {
		m.err = "no task is selected"
		return m
	}
	address := TaskLink(task, m.webBase)
	m.choice, m.choices, m.err, m.choiceNote = choiceShare, ShareChoices(task, address), "", ""
	if address == "" {
		m.choiceNote = NoLinkNote
	}
	return m
}

// copyShare sends the picked form to the terminal's clipboard with OSC 52 and
// draws it too, since nothing reports whether the terminal took it.
func (m Model) copyShare(picked Choice) (Model, tea.Cmd) {
	m.shared = &sharedText{label: picked.Label, text: picked.Value}
	m.err, m.status = "", "copied the "+picked.Label+" to the clipboard"
	return m, tea.SetClipboard(picked.Value)
}

// sharedPanel draws what was copied, wrapped rather than cut, so a reader
// whose terminal ignores OSC 52 can still select it.
func (m Model) sharedPanel() Panel {
	width := max(1, m.width-2)
	rows := make([]string, 0, 2)
	for _, line := range wrapCells(m.shared.text, width) {
		rows = append(rows, "  "+m.theme.Selected.Render(line))
	}
	return Panel{
		Title: "copied " + m.shared.label,
		Note:  "sent to the terminal's clipboard (OSC 52); if it did not arrive, select the text below",
		Rows:  rows,
		Keys:  "any key closes",
	}
}

// wrapCells breaks s into lines of at most width runes, keeping every
// character: a link is one word, so wrapping at spaces would cut nothing.
func wrapCells(s string, width int) []string {
	runes := []rune(s)
	if width < 1 || len(runes) <= width {
		return []string{s}
	}
	var out []string
	for len(runes) > width {
		out = append(out, string(runes[:width]))
		runes = runes[width:]
	}
	return append(out, string(runes))
}
