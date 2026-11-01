// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// savedFiltersMsg carries the reader's saved filters, which the picker
// offers to apply.
type savedFiltersMsg struct {
	filters []core.SavedFilter
	err     error
}

// SavedFilterLabel names one saved filter the way the picker offers it: its
// name and the expression it applies, and whether somebody shared it.
func SavedFilterLabel(f core.SavedFilter) string {
	label := f.Name
	if f.Shared && !f.Mine {
		label += " (shared)"
	}
	if f.Query != "" {
		label += "  " + f.Query
	}
	return label
}

// SavedFilterForm offers the reader's saved filters, their own first, in the
// order the service lists them.
func SavedFilterForm(filters []core.SavedFilter) Form {
	if len(filters) == 0 {
		return Form{}
	}
	options := make([]string, 0, len(filters))
	for _, f := range filters {
		options = append(options, SavedFilterLabel(f))
	}
	return Form{
		Kind: formSavedFilter, Title: "apply a saved filter",
		Note: "the filter, with its sort, replaces the one on the board",
		Fields: []FormField{
			{Key: "filter", Label: "saved filter", Options: options, Value: options[0]},
		},
	}
}

// SavedFilterFor finds the filter a picker option names.
func SavedFilterFor(filters []core.SavedFilter, option string) (core.SavedFilter, bool) {
	for _, f := range filters {
		if SavedFilterLabel(f) == option {
			return f, true
		}
	}
	return core.SavedFilter{}, false
}

// SavedExpression is a saved filter as the board's filter bar takes it: its
// sort first, then its expression, the same reading tix task ls --saved
// gives it.
func SavedExpression(f core.SavedFilter) string {
	parts := make([]string, 0, 2)
	if f.Sort != "" {
		parts = append(parts, "sort:"+f.Sort)
	}
	parts = append(parts, f.Query)
	return strings.TrimSpace(strings.Join(parts, " "))
}

// BindMe reads "me" in a filter's actor terms as the reader, so assignee:me
// keeps the reader's own cards in the board's own matching as it does on the
// service. The lists are copied, never written through.
func BindMe(f core.TaskFilter, actorID string) core.TaskFilter {
	if actorID == "" {
		return f
	}
	bind := func(refs []string) []string {
		if refs == nil {
			return nil
		}
		out := make([]string, len(refs))
		for i, ref := range refs {
			if strings.EqualFold(strings.TrimSpace(ref), core.MeRef) {
				ref = actorID
			}
			out[i] = ref
		}
		return out
	}
	f.AssigneeIDs, f.Exclude.AssigneeIDs = bind(f.AssigneeIDs), bind(f.Exclude.AssigneeIDs)
	f.CreatorIDs, f.Exclude.CreatorIDs = bind(f.CreatorIDs), bind(f.Exclude.CreatorIDs)
	f.ClaimedBy, f.Exclude.ClaimedBy = bind(f.ClaimedBy), bind(f.Exclude.ClaimedBy)
	return f
}

// loadSavedFilters reads the reader's saved filters for the picker.
func (m Model) loadSavedFilters() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		filters, err := svc.ListSavedFilters(ctx)
		return savedFiltersMsg{filters: filters, err: err}
	}
}

// onSavedFilters opens the picker, or says how to save a first filter rather
// than offering a picker with nothing in it.
func (m Model) onSavedFilters(msg savedFiltersMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "listing saved filters: " + msg.err.Error()
		return m, nil
	}
	form := SavedFilterForm(msg.filters)
	if !form.Open() {
		m.err = "no saved filters yet; filter the board and press " + m.keys.SaveFilter.Help().Key +
			" to keep one, or use tix filter save"
		return m, nil
	}
	m.savedFilters = msg.filters
	return m.openForm(form)
}

// applySavedFilter puts the picked filter on the board.
func (m Model) applySavedFilter(form Form) (Model, tea.Cmd) {
	f, ok := SavedFilterFor(m.savedFilters, form.Value("filter"))
	m.savedFilters = nil
	if !ok {
		return m, nil
	}
	next := m.applyFilterText(SavedExpression(f))
	if next.filterErr != "" {
		next.err = "saved filter " + f.Name + ": " + next.filterErr
		next.filterErr = ""
		return next, nil
	}
	next.status = "applied saved filter " + f.Name
	return next, next.loadTasks()
}

// openSaveFilter asks for the name to keep the board's filter under. It is
// reached only while a filter is in force: the action needs one to be
// offered, on the key and in the palette alike.
func (m Model) openSaveFilter() (Model, tea.Cmd) {
	return m.openPrompt(promptSaveFilter)
}

// saveFilter keeps the board's filter under a name, for the reader alone.
func (m Model) saveFilter(name string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, expr := m.svc, m.ctx, strings.TrimSpace(m.filterText)
	return func() tea.Msg {
		f, err := svc.CreateSavedFilter(ctx, core.SavedFilterInput{Name: name, Query: expr})
		if err != nil {
			return actionMsg{kind: actionSaveFilter, label: name, err: err}
		}
		return actionMsg{kind: actionSaveFilter, label: f.Name, sentence: "saved filter " + f.Name}
	}
}
