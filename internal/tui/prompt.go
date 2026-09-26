// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// promptKind names the single-line input the interface has open, if any. The
// filter bar was the first of these; every later action that needs one line of
// text reuses the same input rather than growing a second mechanism.
type promptKind int

// The single-line inputs.
const (
	promptNone promptKind = iota
	promptFilter
	promptNewTask
	promptTitle
	promptBody
	promptAssignee
	promptComment
	promptCommentEdit
	promptTag
	promptUntag
	promptDependency
	promptNewProject
	promptActivityFilter
	promptTenant
	promptProjectName
	promptProjectDesc
	promptProjectIcon
	promptNewField
)

// PromptSpec is how one input introduces itself.
type PromptSpec struct {
	Prompt      string
	Placeholder string
	Limit       int
}

// promptSpecs describes every input. An input that is not listed is not open.
var promptSpecs = map[promptKind]PromptSpec{
	promptFilter:      {"filter: ", "status:todo is:unclaimed text", 512},
	promptNewTask:     {"new task: ", "title of the task to create", core.MaxTitleLength},
	promptTitle:       {"title: ", "new title", core.MaxTitleLength},
	promptBody:        {"body: ", "new body", 4096},
	promptAssignee:    {"assignee: ", "actor id to assign to", 128},
	promptComment:     {"comment: ", "what you want to record", 4096},
	promptCommentEdit: {"edit comment: ", "what the comment should say", 4096},
	promptTag:         {"add tag: ", "tag to attach", 128},
	promptUntag:       {"remove tag: ", "tag to detach", 128},
	promptDependency:  {"depends on: ", "task ref, such as infra-42", 128},
	promptNewProject:  {"new project: ", "key and name, such as: infra Infrastructure", 256},

	// The activity bar takes the audit filter's own grammar rather than the
	// task filter's: an event has a kind and an actor, and no status, tag or
	// due date to ask about.
	promptActivityFilter: {"activity: ", "kind:task actor:ada -action:task.updated word", 512},
	promptTenant:         {"tenant: ", "tenant key, such as acme", 128},

	// The project screen's own inputs. Each is seeded with the value it would
	// replace, so an edit starts from what is there rather than from blank.
	promptProjectName: {"project name: ", "what this project is called", 256},
	promptProjectDesc: {"description: ", "what this project is for", 1024},
	promptProjectIcon: {"icon: ", "one or two characters, such as \u25b2", core.MaxProjectIconRunes},
	promptNewField:    {"new field: ", "key and label, such as: severity Severity", 256},
}

// Spec describes an input, reporting whether the kind names one at all.
func (k promptKind) Spec() (PromptSpec, bool) {
	spec, ok := promptSpecs[k]
	return spec, ok
}

// NeedsTask reports whether an input acts on the selected task rather than on
// the board as a whole.
func (k promptKind) NeedsTask() bool {
	switch k {
	case promptNone, promptFilter, promptActivityFilter, promptTenant, promptNewTask, promptNewProject,
		promptProjectName, promptProjectDesc, promptProjectIcon, promptNewField:
		return false
	default:
		return true
	}
}

// choiceKind names the numbered picker the interface has open, if any.
type choiceKind int

// The numbered pickers.
const (
	choiceNone choiceKind = iota
	choiceTransition
	choicePriority
)

// Prompt introduces a picker.
func (k choiceKind) Prompt() string {
	switch k {
	case choiceTransition:
		return "transition to: "
	case choicePriority:
		return "set priority: "
	default:
		return ""
	}
}

// Choice is one numbered option, with the value it stands for.
type Choice struct {
	Label string
	Value string
}

// TransitionChoices offers the states a workflow permits from here.
func TransitionChoices(def *core.WorkflowDefinition, from string) []Choice {
	states := NextStates(def, from)
	out := make([]Choice, 0, len(states))
	for _, s := range states {
		label := s.Label
		if label == "" {
			label = s.Key
		}
		out = append(out, Choice{Label: label, Value: s.Key})
	}
	return out
}

// PriorityChoices offers every priority, named the way the CLI names them.
func PriorityChoices() []Choice {
	out := make([]Choice, 0, 5)
	for p := core.PriorityHighest; p <= core.PriorityLowest; p++ {
		out = append(out, Choice{Label: PriorityLabel(p), Value: strconv.Itoa(int(p))})
	}
	return out
}

// ChoiceLabels numbers the options on offer.
func ChoiceLabels(choices []Choice) []string {
	out := make([]string, 0, len(choices))
	for i, c := range choices {
		out = append(out, strconv.Itoa(i+1)+") "+c.Label)
	}
	return out
}

// ChoiceAt resolves a key press onto the option it picks.
func ChoiceAt(choices []Choice, press string) (Choice, bool) {
	n := digit(press)
	if n < 1 || n > len(choices) {
		return Choice{}, false
	}
	return choices[n-1], true
}

// SplitProjectEntry separates a project's key from its name, the way the CLI
// takes them as two arguments. A name is optional; the key then stands alone.
func SplitProjectEntry(text string) (key, name string) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", ""
	}
	return fields[0], strings.Join(fields[1:], " ")
}
