// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// ProjectLine is one rendered row of the project screen, tagged with what it is
// so the frame can style it without parsing its text back.
type ProjectLine struct {
	Text    string
	Heading bool
	Dim     bool
}

// ProjectState is everything the project screen renders from. The workflow is a
// pointer because a reader who may not read workflows still gets the rest of the
// screen rather than nothing.
type ProjectState struct {
	Project   core.Project
	Workflow  *core.Workflow
	Fields    []core.FieldDef
	TimeStyle output.TimeStyle
	// WorkflowErr is why the workflow could not be read, said out loud rather
	// than drawn as a project running on no workflow at all.
	WorkflowErr string
}

// projectLabelWidth aligns the attribute names, wide enough for the longest of
// them so no value starts in a different column from the one above it.
const projectLabelWidth = 14

// ProjectView renders the project, the workflow it runs on and the custom fields
// its tasks carry. It is the screen `workflow.put` points away from: a state
// machine is read here and changed with the command line, which the closing line
// says rather than leaving a reader to look for an editor that is not there.
func ProjectView(state ProjectState) []ProjectLine {
	out := []ProjectLine{{Text: "project:", Heading: true}}
	out = append(out, projectAttributes(state)...)
	out = append(out, ProjectLine{})
	out = append(out, workflowLines(state)...)
	out = append(out, ProjectLine{})
	return append(out, fieldLines(state.Fields)...)
}

// projectAttributes states the project's own mutable attributes, which are
// exactly the ones the edit form offers to change.
func projectAttributes(state ProjectState) []ProjectLine {
	p := state.Project
	out := []ProjectLine{
		projectRow("key", p.Key),
		projectRow("name", p.Name),
		projectRow("description", orNone(p.Description)),
		projectRow("colour", orNone(p.Color.String())),
		projectRow("icon", ProjectGlyph(p)),
	}
	return append(out, projectRow("state", projectStanding(p, state.TimeStyle)))
}

// projectStanding says whether the project is live, and when it was archived if
// it is not, because "archived" without a date reads as a permanent property.
func projectStanding(p core.Project, style output.TimeStyle) string {
	if !p.Archived() {
		return "active"
	}
	if when := style.FormatPtr(p.ArchivedAt); when != "" {
		return "archived " + when
	}
	return "archived"
}

// workflowLines render the state machine the project's tasks move through: its
// states with the category each belongs to, the edges out of them, and the lease
// a claim takes by default.
func workflowLines(state ProjectState) []ProjectLine {
	if state.Workflow == nil {
		reason := state.WorkflowErr
		if reason == "" {
			reason = "this session cannot read it"
		}
		return []ProjectLine{
			{Text: "workflow:", Heading: true},
			{Text: "  not shown: " + reason, Dim: true},
		}
	}
	w := *state.Workflow
	out := []ProjectLine{{Text: "workflow: " + workflowTitle(w), Heading: true}}
	out = append(out, projectRow("key", w.Key), projectRow("initial", w.Definition.Initial))
	if lease := w.Definition.DefaultLease; lease != 0 {
		out = append(out, projectRow("default lease", lease.String()))
	}
	out = append(out, ProjectLine{Text: "  states (" + strconv.Itoa(len(w.Definition.States)) + "):"})
	for _, s := range w.Definition.States {
		out = append(out, ProjectLine{Text: "    " + pad(s.Key, projectLabelWidth-2) + stateNote(s)})
	}
	out = append(out, ProjectLine{Text: "  transitions (" + strconv.Itoa(len(w.Definition.Transitions)) + "):"})
	for _, tr := range w.Definition.Transitions {
		out = append(out, ProjectLine{Text: "    " + transitionNote(tr)})
	}
	return append(out, ProjectLine{Text: "  " + WorkflowEditHint, Dim: true})
}

// WorkflowEditHint is where a workflow is changed. The terminal reads a state
// machine and does not edit one, so the screen names the command that does
// rather than leaving a reader hunting for a key that was never bound.
const WorkflowEditHint = "states and transitions are edited with tix workflow put, or in the browser"

// workflowTitle names a workflow the way a reader recognises it, falling back to
// the key when it carries no name.
func workflowTitle(w core.Workflow) string {
	if w.Name != "" {
		return w.Name
	}
	return w.Key
}

// stateNote says what a state is beyond its key: the category it reports under,
// and whether a task that reaches it stops there.
func stateNote(s core.State) string {
	parts := []string{string(s.Category)}
	if s.Category == "" {
		parts = []string{"uncategorised"}
	}
	if s.Terminal {
		parts = append(parts, "terminal")
	}
	if s.RevertOnLeaseExpiry {
		to := s.RevertTo
		if to == "" {
			to = "the initial state"
		}
		parts = append(parts, "reverts to "+to+" when a lease expires")
	}
	return strings.Join(parts, ", ")
}

// transitionNote renders one edge, naming what it asks of whoever takes it.
func transitionNote(t core.Transition) string {
	line := t.From + " -> " + t.To
	var needs []string
	if t.RequiresComment {
		needs = append(needs, "a comment")
	}
	if t.RequiresScope != "" {
		needs = append(needs, string(t.RequiresScope))
	}
	if len(needs) == 0 {
		return line
	}
	return line + " (requires " + strings.Join(needs, " and ") + ")"
}

// fieldLines render the custom field definitions the project's tasks carry.
func fieldLines(fields []core.FieldDef) []ProjectLine {
	out := []ProjectLine{{Text: "custom fields (" + strconv.Itoa(len(fields)) + "):", Heading: true}}
	if len(fields) == 0 {
		return append(out, ProjectLine{Text: "  none defined", Dim: true})
	}
	for _, f := range fields {
		out = append(out, ProjectLine{Text: "  " + pad(f.Key, projectLabelWidth) + FieldDefNote(f)})
	}
	return out
}

// FieldDefNote describes one custom field definition in the terms its own input
// takes, so what the screen says and what the form gathers are the same words.
func FieldDefNote(f core.FieldDef) string {
	parts := []string{string(f.Type)}
	if f.Required {
		parts = append(parts, "required")
	}
	if f.Indexed {
		parts = append(parts, "indexed")
	}
	note := strings.Join(parts, ", ")
	if len(f.EnumOptions) > 0 {
		note += "   " + strings.Join(f.EnumOptions, " / ")
	}
	if f.Label != "" && f.Label != f.Key {
		note += "   " + f.Label
	}
	return note
}

// orNone renders an unset attribute as a word rather than as nothing, so an
// empty description cannot be mistaken for a row that failed to draw.
func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}

// projectRow is one aligned attribute line.
func projectRow(label, value string) ProjectLine {
	return ProjectLine{Text: "  " + pad(label, projectLabelWidth) + value}
}

// The attributes the project edit form offers, named once so the form and the
// action it releases cannot spell one differently.
const (
	attrName        = "name"
	attrDescription = "description"
	attrIcon        = "icon"
	attrColour      = "colour"
	attrWorkflow    = "workflow"
)

// noColour is what the colour list calls the absence of one. The service takes
// an empty string, which no list of alternatives can show.
const noColour = "none"

// ColourOptions are the colours a project may be given, the palette plus the
// word for having none, so clearing a colour is a choice on the same list
// rather than an attribute with no way back.
func ColourOptions() []string {
	palette := core.ProjectColors()
	out := make([]string, 0, len(palette)+1)
	out = append(out, noColour)
	for _, c := range palette {
		out = append(out, c.String())
	}
	return out
}

// ColourValue turns a chosen colour back into what the service takes.
func ColourValue(option string) string {
	if option == noColour {
		return ""
	}
	return option
}

// ProjectEditForm offers the project attributes an edit can change. The three
// free-text ones are answered by the single-line prompt the form hands over to,
// and the two drawn from fixed sets are answered here, so one key reaches every
// field the operation accepts rather than reaching the enumerable ones only.
//
// A workflow is offered only when there is another one to move to: a tenant with
// one workflow has no choice to make, and a field whose list holds the current
// answer alone is a question with one answer.
func ProjectEditForm(p core.Project, workflows []string) Form {
	attributes := []string{attrName, attrDescription, attrIcon, attrColour}
	if len(workflows) > 1 {
		attributes = append(attributes, attrWorkflow)
	}
	colour := p.Color.String()
	if colour == "" {
		colour = noColour
	}
	fields := []FormField{
		{Key: "attribute", Label: "attribute", Options: attributes, Value: attributes[0]},
		{Key: attrColour, Label: "colour", Options: ColourOptions(), Value: colour,
			Needs: FieldCondition{Key: "attribute", Value: attrColour}},
	}
	if len(workflows) > 1 {
		fields = append(fields, FormField{Key: attrWorkflow, Label: "workflow",
			Options: workflows, Value: workflows[0],
			Needs: FieldCondition{Key: "attribute", Value: attrWorkflow}})
	}
	return Form{Kind: formProject, Title: "edit project " + p.Key, Fields: fields}
}

// ProjectRemoveForm asks whether a project should be hidden or destroyed. The
// two are one key because they have one subject and differ only in how far they
// reach, which is the distinction a confirmation is there to state.
//
// An archived project is not offered archiving again, because the service
// refuses it: an entry leading to a refusal is worse than no entry.
func ProjectRemoveForm(p core.Project, mayArchive, mayDelete bool) Form {
	var actions []string
	if mayArchive && !p.Archived() {
		actions = append(actions, "archive")
	}
	if mayDelete {
		actions = append(actions, "delete")
	}
	if len(actions) == 0 {
		return Form{}
	}
	return Form{
		Kind: formProjectRemove, Title: "remove project " + p.Key,
		Note:   projectRemoveNote(p),
		Fields: []FormField{{Key: "action", Label: "action", Options: actions, Value: actions[0]}},
	}
}

// ProjectDeleteNote says how far a project deletion reaches, which the key
// cannot: archiving and deleting differ only in that, so the confirmation has to
// state it out loud.
const ProjectDeleteNote = "with every task, comment and attachment in it"

// projectRemoveNote says what the reader cannot see from the key alone.
func projectRemoveNote(p core.Project) string {
	if p.Archived() {
		return "this project is already archived"
	}
	return "archiving hides the project and keeps its history"
}

// FieldPickForm picks one custom field definition and what to do with it. The
// field is chosen first and the change is gathered afterwards, by a second form
// seeded from that definition, because a form cannot reseed its own rows when an
// answer above them changes and a row showing another field's type would be a
// lie about the one the reader picked.
func FieldPickForm(fields []core.FieldDef, mayPut, mayDelete bool) Form {
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	var actions []string
	if mayPut {
		actions = append(actions, "redefine")
	}
	if mayDelete {
		actions = append(actions, "remove")
	}
	if len(keys) == 0 || len(actions) == 0 {
		return Form{}
	}
	return Form{
		Kind: formFieldPick, Title: "custom fields",
		Fields: []FormField{
			{Key: "field", Label: "field", Options: keys, Value: keys[0]},
			{Key: "action", Label: "action", Options: actions, Value: actions[0]},
		},
	}
}

// FieldDefForm gathers what a definition says about a field's values. The key
// and the label are free text and arrive from the prompt or from the definition
// being changed, so the form asks only the two answers drawn from fixed sets.
func FieldDefForm(key string, base core.FieldDef) Form {
	kind := base.Type
	if kind == "" {
		kind = core.FieldString
	}
	required := noValue
	if base.Required {
		required = yesValue
	}
	return Form{
		Kind: formFieldDef, Title: "field " + key,
		Note: FieldDefNote(base),
		Fields: []FormField{
			{Key: "type", Label: "type", Options: FieldTypeOptions(base.Type), Value: string(kind)},
			{Key: "required", Label: "required", Options: switchOptions(), Value: required},
		},
	}
}

// FieldTypeOptions are the types a terminal can set. An enum needs its options,
// which no fixed list of alternatives can gather, so enum is offered only to a
// field that already is one: the definition then keeps the options it has rather
// than being narrowed to an enum with nothing to choose from.
func FieldTypeOptions(current core.FieldType) []string {
	out := make([]string, 0, len(core.FieldTypes))
	for _, t := range core.FieldTypes {
		if t == core.FieldEnum && current != core.FieldEnum {
			continue
		}
		out = append(out, string(t))
	}
	return out
}

// FieldDefUpdate is the input a redefinition sends: the answers the form
// gathered, over everything the existing definition already held. A terminal
// that asked two questions must not silently drop the enum options, the default
// or the position, because a put replaces the whole definition.
func FieldDefUpdate(key, label string, base core.FieldDef, kind core.FieldType, required bool) core.FieldDefInput {
	if label == "" {
		label = base.Label
	}
	if label == "" {
		label = key
	}
	in := core.FieldDefInput{
		Key: key, Label: label, Type: kind, Required: required,
		Default: base.Default, Indexed: base.Indexed, Position: base.Position,
	}
	if kind == core.FieldEnum {
		in.EnumOptions = base.EnumOptions
	}
	return in
}

// SplitFieldEntry separates a custom field's key from its label, the way the
// project prompt separates a key from a name. A label is optional; the key then
// stands for both.
func SplitFieldEntry(text string) (key, label string) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", ""
	}
	return fields[0], strings.Join(fields[1:], " ")
}

// FieldDefFor returns the definition a key names, so a redefinition starts from
// what the project already holds rather than from zero.
func FieldDefFor(fields []core.FieldDef, key string) (core.FieldDef, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
	}
	return core.FieldDef{}, false
}
