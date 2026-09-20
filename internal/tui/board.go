package tui

import (
	"sort"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

// Column is one board column: a workflow state and the tasks sitting in it.
type Column struct {
	Key      string
	Label    string
	Terminal bool
	Category core.StateCategory
	Tasks    []core.Task
}

// Selection addresses one card on the board.
type Selection struct {
	Col int
	Row int
}

// BuildColumns groups tasks into the workflow's states in declared order.
func BuildColumns(def *core.WorkflowDefinition, tasks []core.Task) []Column {
	cols, index := declaredColumns(def)
	for _, t := range tasks {
		i, ok := index[t.Status]
		if !ok {
			cols = append(cols, Column{Key: t.Status, Label: t.Status})
			i = len(cols) - 1
			index[t.Status] = i
		}
		cols[i].Tasks = append(cols[i].Tasks, t)
	}
	for i := range cols {
		sortTasks(cols[i].Tasks)
	}
	return cols
}

// declaredColumns seeds the board with the workflow's own states.
func declaredColumns(def *core.WorkflowDefinition) ([]Column, map[string]int) {
	index := map[string]int{}
	if def == nil {
		return nil, index
	}
	cols := make([]Column, 0, len(def.States))
	for _, s := range def.States {
		label := s.Label
		if label == "" {
			label = s.Key
		}
		index[s.Key] = len(cols)
		cols = append(cols, Column{Key: s.Key, Label: label, Terminal: s.Terminal, Category: s.Category})
	}
	return cols, index
}

// sortTasks orders a column deterministically by priority, then sequence.
func sortTasks(tasks []core.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Seq != b.Seq {
			return a.Seq < b.Seq
		}
		return a.ID < b.ID
	})
}

// TaskAt returns the task a selection addresses.
func TaskAt(cols []Column, sel Selection) (core.Task, bool) {
	if sel.Col < 0 || sel.Col >= len(cols) {
		return core.Task{}, false
	}
	tasks := cols[sel.Col].Tasks
	if sel.Row < 0 || sel.Row >= len(tasks) {
		return core.Task{}, false
	}
	return tasks[sel.Row], true
}

// FindTask locates a task by identifier.
func FindTask(cols []Column, id string) (Selection, bool) {
	if id == "" {
		return Selection{}, false
	}
	for c := range cols {
		for r, t := range cols[c].Tasks {
			if t.ID == id {
				return Selection{Col: c, Row: r}, true
			}
		}
	}
	return Selection{}, false
}

// ClampSelection keeps a selection inside the board.
func ClampSelection(cols []Column, sel Selection) Selection {
	if len(cols) == 0 {
		return Selection{}
	}
	if sel.Col < 0 {
		sel.Col = 0
	}
	if sel.Col >= len(cols) {
		sel.Col = len(cols) - 1
	}
	n := len(cols[sel.Col].Tasks)
	if sel.Row < 0 || n == 0 {
		sel.Row = 0
	}
	if n > 0 && sel.Row >= n {
		sel.Row = n - 1
	}
	return sel
}

// PreserveSelection keeps the previously selected task selected after a rebuild.
func PreserveSelection(cols []Column, sel Selection, id string) Selection {
	if found, ok := FindTask(cols, id); ok {
		return found
	}
	return ClampSelection(cols, sel)
}

// MoveSelection shifts the selection by whole columns and rows.
func MoveSelection(cols []Column, sel Selection, dCol, dRow int) Selection {
	if len(cols) == 0 {
		return Selection{}
	}
	if dCol != 0 {
		sel.Col += dCol
		return ClampSelection(cols, sel)
	}
	sel = ClampSelection(cols, sel)
	sel.Row += dRow
	return ClampSelection(cols, sel)
}

// NextStates lists the states a task may legally move to under the workflow.
func NextStates(def *core.WorkflowDefinition, from string) []core.State {
	if def == nil {
		return nil
	}
	var out []core.State
	seen := map[string]bool{}
	for _, tr := range def.Transitions {
		if tr.From != from || seen[tr.To] {
			continue
		}
		if s, ok := def.State(tr.To); ok {
			seen[tr.To] = true
			out = append(out, s)
		}
	}
	return out
}

// TaskBadges renders the state markers of a task as text, never as colour alone.
func TaskBadges(t core.Task, now time.Time) []string {
	badges := []string{"P" + priorityDigit(t.Priority)}
	if t.ClaimedAtTime(now) {
		badges = append(badges, "@"+shortID(t.ClaimedByActorID))
	}
	if t.Blocked {
		badges = append(badges, "!blocked")
	}
	if len(t.DependsOn) > 0 {
		badges = append(badges, "deps")
	}
	if t.DueAt != nil {
		badges = append(badges, "due")
	}
	return badges
}

// priorityDigit renders a priority as a single character.
func priorityDigit(p core.Priority) string {
	if !p.Valid() {
		return "?"
	}
	// #nosec G115 -- p is a priority in 1..5; anything else returned above.
	return string(rune('0' + int(p)))
}

// shortID trims an identifier to a readable prefix.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
