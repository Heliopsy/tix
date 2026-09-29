// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// promptKind names the free-text input the interface has open, if any. The
// filter bar was the first of these; every later action that needs text reuses
// the same input rather than growing a second mechanism. A prompt whose spec
// is multi-line draws a taller field inside the same panel.
type promptKind int

// The free-text inputs.
const (
	promptNone promptKind = iota
	promptFilter
	promptNewTask
	promptTitle
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
	promptArtifact
	promptTenantName
	promptDomain
)

// PromptSpec is how one input introduces itself.
//
// Field is what the one answer is called, so a single-value prompt renders as
// a form with one field rather than as a second idiom the reader has to learn
// separately. Title names the panel, and is the Prompt without its colon.
//
// Multiline marks a field holding prose rather than a value. Enter inserts a
// newline there instead of applying, so the panel's legend names the key that
// does apply.
type PromptSpec struct {
	Prompt      string
	Placeholder string
	Limit       int
	Field       string
	Multiline   bool
}

// Title names the panel an input opens in.
func (s PromptSpec) Title() string { return strings.TrimSuffix(strings.TrimSpace(s.Prompt), ":") }

// promptSpecs describes every input. An input that is not listed is not open.
var promptSpecs = map[promptKind]PromptSpec{
	promptFilter:  {Prompt: "filter: ", Placeholder: "status:todo is:unclaimed text", Limit: 512, Field: "expression"},
	promptNewTask: {Prompt: "new task: ", Placeholder: "title of the task to create", Limit: core.MaxTitleLength, Field: "title"},
	promptTitle:   {Prompt: "title: ", Placeholder: "new title", Limit: core.MaxTitleLength, Field: "title"},

	// A comment is prose and often runs to paragraphs, so it takes the tall
	// field rather than one clipped line.
	promptComment:     {Prompt: "comment: ", Placeholder: "what you want to record", Limit: 4096, Field: "comment", Multiline: true},
	promptCommentEdit: {Prompt: "edit comment: ", Placeholder: "what the comment should say", Limit: 4096, Field: "comment", Multiline: true},

	promptTag:        {Prompt: "add tag: ", Placeholder: "tag to attach", Limit: 128, Field: "tag"},
	promptUntag:      {Prompt: "remove tag: ", Placeholder: "tag to detach", Limit: 128, Field: "tag"},
	promptDependency: {Prompt: "depends on: ", Placeholder: "task ref, such as infra-42", Limit: 128, Field: "task ref"},
	promptNewProject: {Prompt: "new project: ", Placeholder: "key and name, such as: infra Infrastructure", Limit: 256, Field: "key and name"},

	// The activity bar takes the audit filter's own grammar rather than the
	// task filter's: an event has a kind and an actor, and no status, tag or
	// due date to ask about.
	promptActivityFilter: {Prompt: "activity: ", Placeholder: "kind:task actor:ada -action:task.updated word", Limit: 512, Field: "expression"},
	promptTenant:         {Prompt: "tenant: ", Placeholder: "tenant key, such as acme", Limit: 128, Field: "tenant key"},

	// The project screen's own inputs. Each is seeded with the value it would
	// replace, so an edit starts from what is there rather than from blank.
	promptProjectName: {Prompt: "project name: ", Placeholder: "what this project is called", Limit: 256, Field: "name"},
	// A project's description is prose too, and the one prose field that screen
	// has.
	promptProjectDesc: {Prompt: "description: ", Placeholder: "what this project is for", Limit: 1024, Field: "description", Multiline: true},
	promptProjectIcon: {Prompt: "icon: ", Placeholder: "one or two characters, such as \u25b2", Limit: core.MaxProjectIconRunes, Field: "icon"},
	promptNewField:    {Prompt: "new field: ", Placeholder: "key and label, such as: severity Severity", Limit: 256, Field: "key and label"},

	// The artifact's name. Its kind is the one answer drawn from a fixed set,
	// so the form asks that and this asks the part no fixed list can hold.
	promptArtifact: {Prompt: "artifact name: ", Placeholder: "what this output is called", Limit: 256, Field: "name"},

	// The tenant screen's own inputs. A tenant's name is seeded with the name
	// it would replace; a hostname is new every time and is seeded with
	// nothing.
	promptTenantName: {Prompt: "tenant name: ", Placeholder: "what this tenant is called", Limit: 256, Field: "name"},
	promptDomain:     {Prompt: "hostname: ", Placeholder: "hostname that resolves here, such as acme.example", Limit: 253, Field: "hostname"},
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
		promptProjectName, promptProjectDesc, promptProjectIcon, promptNewField,
		promptTenantName, promptDomain:
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

// Field is what a picker's one answer is called, so the numbered picker reads
// as the same label-and-value shape every other input mode uses.
func (k choiceKind) Field() string {
	switch k {
	case choiceTransition:
		return "state"
	case choicePriority:
		return "priority"
	default:
		return "value"
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
