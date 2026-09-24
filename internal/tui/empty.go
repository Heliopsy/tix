// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

// EmptyState is what a view says when it has nothing to show. Five columns
// reading "(0)" over blank space is indistinguishable from a failure, so every
// view that can be empty says so in words and says what to do about it.
type EmptyState struct {
	Title string
	Hint  string
}

// Zero reports whether the state names nothing.
func (e EmptyState) Zero() bool { return e.Title == "" }

// Lines renders an empty state as the lines a body is filled with.
func (e EmptyState) Lines() []string {
	if e.Zero() {
		return nil
	}
	lines := []string{"", "  " + e.Title}
	if e.Hint != "" {
		lines = append(lines, "", "  "+e.Hint)
	}
	return append(lines, "")
}

// ProjectsEmptyState decides what the project list says when it is empty.
func ProjectsEmptyState(projects int) EmptyState {
	if projects > 0 {
		return EmptyState{}
	}
	return EmptyState{
		Title: "No projects yet.",
		Hint:  `Create one with:  tix project create KEY "Name"`,
	}
}

// BoardEmptyState decides what a board says when no card is drawn on it. It
// distinguishes a board with no workflow, a board every task was filtered out
// of, and a project that genuinely holds no tasks, because the three call for
// three different actions.
func BoardEmptyState(projectKey string, columns []Column, filtered bool) EmptyState {
	if len(columns) == 0 {
		return EmptyState{
			Title: "This project has no workflow states, so it has no board.",
			Hint:  "Define one with:  tix workflow put",
		}
	}
	if countTasks(columns) > 0 {
		return EmptyState{}
	}
	if filtered {
		return EmptyState{
			Title: "No task matches the filter.",
			Hint:  "Press C to clear the filter, or / to change it.",
		}
	}
	hint := "Press n to add the first one."
	if projectKey != "" {
		hint = "Press n to add the first one to " + projectKey + "."
	}
	return EmptyState{Title: "This board is empty.", Hint: hint}
}

// ActivityEmptyState decides what the activity view says when it draws no
// line. It distinguishes a quiet tenant, a dropped subscription and a tail
// every event was filtered out of, because a long silence must never look
// like the interface has stalled, and a filter that hides everything must
// never look like a tenant that has gone quiet. shown counts the events the
// filter keeps, kept counts the events the tail holds at all.
func ActivityEmptyState(shown, kept int, connected bool) EmptyState {
	if shown > 0 {
		return EmptyState{}
	}
	if kept > 0 {
		return EmptyState{
			Title: "No event matches the filter.",
			Hint:  "Press C to clear the filter, or / to change it.",
		}
	}
	if !connected {
		return EmptyState{Title: "Not connected to the event stream.", Hint: "Retrying..."}
	}
	return EmptyState{Title: "Nothing yet.", Hint: "Waiting for events to arrive."}
}

// countTasks totals the cards a board would draw.
func countTasks(columns []Column) int {
	n := 0
	for _, c := range columns {
		n += len(c.Tasks)
	}
	return n
}
