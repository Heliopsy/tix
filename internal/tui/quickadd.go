// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/quickadd"
)

// quickInput reads the new-task prompt's line, counting relative dates in the
// local zone. The open project is the default a line naming none goes to.
func (m Model) quickInput(line string) (core.CreateTaskInput, error) {
	parsed, err := quickadd.Parse(line, m.now())
	if err != nil {
		return core.CreateTaskInput{}, err
	}
	in, err := parsed.Apply(core.CreateTaskInput{})
	if err != nil {
		return in, err
	}
	if in.ProjectRef == "" {
		in.ProjectRef = m.project.Key
	}
	return in, nil
}

// QuickSummary names what a parsed line sets, in the order it was typed, for
// the row under the new-task prompt. Empty when the line sets nothing.
func QuickSummary(r quickadd.Result) string {
	parts := make([]string, 0, len(r.Tokens))
	for _, tok := range r.Tokens {
		parts = append(parts, string(tok.Kind)+" "+tok.Value)
	}
	return strings.Join(parts, ", ")
}

// quickRow is the row under the new-task prompt: the refusal when the line
// was refused, otherwise what the line typed so far will set.
func (m Model) quickRow() (string, bool) {
	if m.quickErr != "" {
		return m.theme.Error.Render("✗ " + m.quickErr), true
	}
	parsed, err := quickadd.Parse(m.input.Value(), m.now())
	if err != nil {
		return "", false
	}
	summary := QuickSummary(parsed)
	if summary == "" {
		return "", false
	}
	return m.theme.Dim.Render("sets " + summary), true
}
