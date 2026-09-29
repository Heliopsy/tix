// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// formKind names the multi-field input the interface has open, if any. The
// single-line prompt takes one piece of free text; a form takes several
// answers, each drawn from the values the operation actually accepts.
type formKind int

// The forms.
const (
	formNone formKind = iota
	formDelete
	formTag
	formDependency
	formProject
	formProjectRemove
	formFieldPick
	formFieldDef
	formAssignee
	formArtifact
	formTenant
	formTenantAdd
	formMember
	formTaskEdit
)

// FieldKind says what a form field gathers. A field drawn from a fixed set is
// cycled; the other two are typed into, and the widget that gathers them owns
// the arrows while it has the cursor.
type FieldKind int

// The field kinds.
const (
	// FieldChoice is the zero value, so every field written before free text
	// existed keeps cycling its options.
	FieldChoice FieldKind = iota
	FieldText
	FieldProse
)

// FieldCondition keeps a field off screen until another field holds a value.
// The delete form's reach switches mean nothing once its subject is a comment,
// and a field that is shown but ignored is a question answered for nothing.
type FieldCondition struct {
	Key   string
	Value string
}

// Met reports whether the condition holds for a form.
func (c FieldCondition) Met(f Form) bool {
	return c.Key == "" || f.value(c.Key) == c.Value
}

// FormField is one answer a form gathers: a value cycled out of a fixed set, a
// line of text, or prose.
//
// Limit caps a typed field, and is ignored by a field with options.
type FormField struct {
	Key     string
	Label   string
	Options []string
	Value   string
	Needs   FieldCondition
	Kind    FieldKind
	Limit   int
}

// Typed reports whether a field is gathered by typing rather than by cycling.
func (f FormField) Typed() bool { return f.Kind == FieldText || f.Kind == FieldProse }

// Form is a set of answers gathered on one screen. It is moved through and
// cycled with the bindings the settings screen already uses, so every
// keybinding scheme drives it without rebinding anything.
type Form struct {
	Kind   formKind
	Title  string
	Fields []FormField
	// Note is what the reader needs to know to answer, such as which of the
	// offered tags the task already carries.
	Note string
	// Sel indexes the visible fields, not Fields, so a hidden field is never
	// the selected one.
	Sel int
}

// Open reports whether a form is gathering answers.
func (f Form) Open() bool { return f.Kind != formNone }

// Visible are the fields whose conditions the current answers meet, in order.
func (f Form) Visible() []FormField {
	out := make([]FormField, 0, len(f.Fields))
	for _, field := range f.Fields {
		if field.Needs.Met(f) {
			out = append(out, field)
		}
	}
	return out
}

// value reads a field's answer by key, ignoring visibility, which is what a
// condition has to ask about.
func (f Form) value(key string) string {
	for _, field := range f.Fields {
		if field.Key == key {
			return field.Value
		}
	}
	return ""
}

// Value is the answer a visible field holds. A hidden field answers nothing,
// so a caller cannot act on a switch the reader was never shown.
func (f Form) Value(key string) string {
	for _, field := range f.Visible() {
		if field.Key == key {
			return field.Value
		}
	}
	return ""
}

// Typing reports whether the selected field is one the reader types into, which
// is what decides who owns the arrows and whether enter applies the form.
func (f Form) Typing() bool {
	selected, ok := f.Selected()
	return ok && selected.Typed()
}

// Prose reports whether the selected field is the multi-line one, where enter
// inserts a newline rather than applying the form.
func (f Form) Prose() bool {
	selected, ok := f.Selected()
	return ok && selected.Kind == FieldProse
}

// SetValue records an answer against a field, which is how a typed field's
// widget writes what the reader typed back into the form.
func (f Form) SetValue(key, value string) Form {
	fields := make([]FormField, len(f.Fields))
	copy(fields, f.Fields)
	for i := range fields {
		if fields[i].Key == key {
			fields[i].Value = value
		}
	}
	f.Fields = fields
	return f
}

// Yes reports whether a switch is set.
func (f Form) Yes(key string) bool { return f.Value(key) == yesValue }

// The values a switch takes, named once so the builders and the readers cannot
// spell them differently.
const (
	yesValue = "yes"
	noValue  = "no"
)

// switchOptions are what a yes-or-no field offers, refused first.
func switchOptions() []string { return []string{noValue, yesValue} }

// Selected is the field the cursor is on.
func (f Form) Selected() (FormField, bool) {
	visible := f.Visible()
	if f.Sel < 0 || f.Sel >= len(visible) {
		return FormField{}, false
	}
	return visible[f.Sel], true
}

// Move steps the cursor between the visible fields, stopping at the ends rather
// than wrapping, so holding a key cannot walk off one end onto the other.
func (f Form) Move(delta int) Form {
	f.Sel = clamp(f.Sel+delta, 0, len(f.Visible())-1)
	return f
}

// Cycle steps the selected field's answer through its values, wrapping, and
// clamps the cursor afterwards because an answer can hide the field below it.
// A typed field cycles nothing: its arrows move the cursor through the text.
func (f Form) Cycle(delta int) Form {
	selected, ok := f.Selected()
	if !ok || selected.Typed() || len(selected.Options) == 0 {
		return f
	}
	next := CycleValue(selected.Options, selected.Value, delta)
	fields := make([]FormField, len(f.Fields))
	copy(fields, f.Fields)
	for i := range fields {
		if fields[i].Key == selected.Key {
			fields[i].Value = next
		}
	}
	f.Fields = fields
	f.Sel = clamp(f.Sel, 0, len(f.Visible())-1)
	return f
}

// FormLine is one rendered row of a form.
type FormLine struct {
	Label    string
	Value    string
	Hint     string
	Selected bool
	Kind     FieldKind
}

// Lines renders the visible fields, each with the values it could hold instead,
// because a field whose alternatives are invisible reads as a label rather than
// as a question.
func (f Form) Lines() []FormLine {
	visible := f.Visible()
	out := make([]FormLine, 0, len(visible))
	for i, field := range visible {
		out = append(out, FormLine{
			Label: field.Label, Value: FieldSummary(field),
			Hint: FieldHint(field), Selected: i == f.Sel, Kind: field.Kind,
		})
	}
	return out
}

// FieldSummary is what a field shows while the cursor is elsewhere. Prose is
// summarised to its first line and a count of the rest, because a form row is
// one line and several paragraphs drawn into it is the defect this field
// exists to fix.
func FieldSummary(field FormField) string {
	if field.Kind != FieldProse {
		return field.Value
	}
	lines := strings.Split(field.Value, "\n")
	if len(lines) == 1 {
		return field.Value
	}
	return lines[0] + " (+" + strconv.Itoa(len(lines)-1) + " more lines)"
}

// hintWidth is how much room the alternatives get before a field states its
// position in the list instead of listing it.
const hintWidth = 44

// FieldHint says what else a field could hold: the whole list while it is short
// enough to read, and where in the list this answer sits once it is not.
func FieldHint(field FormField) string {
	if field.Typed() {
		return ""
	}
	if len(field.Options) < 2 {
		return ""
	}
	joined := strings.Join(field.Options, " / ")
	if len(joined) <= hintWidth {
		return joined
	}
	at := 0
	for i, o := range field.Options {
		if o == field.Value {
			at = i + 1
			break
		}
	}
	return strconv.Itoa(at) + " of " + strconv.Itoa(len(field.Options))
}

// DeleteForm asks what a delete will remove and how far it will reach. The
// subject is a field rather than a second destructive key, because the
// interface has one key to spend and a board has two things on it a reader may
// want gone, and a key whose subject depends on what is selected is the
// ambiguity a confirmation exists to remove.
//
// A subject the reader has no authority over is not offered, so the form never
// gathers an answer the service would refuse.
func DeleteForm(taskLabel, commentLabel string, mayTask, mayComment bool) Form {
	var subjects []string
	if mayTask && taskLabel != "" {
		subjects = append(subjects, "task")
	}
	if mayComment && commentLabel != "" {
		subjects = append(subjects, "comment")
	}
	if len(subjects) == 0 {
		return Form{}
	}
	fields := []FormField{
		{Key: "what", Label: "what", Options: subjects, Value: subjects[0]},
	}
	if subjects[0] == "task" || len(subjects) > 1 {
		fields = append(fields,
			FormField{Key: "hard", Label: "permanently", Options: switchOptions(), Value: noValue,
				Needs: FieldCondition{Key: "what", Value: "task"}},
			FormField{Key: "cascade", Label: "with subtasks", Options: switchOptions(), Value: noValue,
				Needs: FieldCondition{Key: "what", Value: "task"}},
		)
	}
	return Form{Kind: formDelete, Title: "delete", Fields: fields}
}

// TagForm offers the tags the tenant already has, which is what the typed
// prompts cannot: they take a name, and a reader who does not know the names
// has no way to learn them. The tags already on the task are named too, so
// attaching one that is there and detaching one that is not are both visible
// mistakes rather than silent ones.
func TagForm(available, attached []string, mayAttach, mayDetach bool) Form {
	names := uniqueSorted(available)
	var actions []string
	if mayAttach {
		actions = append(actions, "attach")
	}
	if mayDetach {
		actions = append(actions, "detach")
	}
	if len(names) == 0 || len(actions) == 0 {
		return Form{}
	}
	action := actions[0]
	if mayDetach && slices.Contains(attached, names[0]) {
		action = "detach"
	}
	return Form{
		Kind: formTag, Title: "tags",
		Note: tagNote(attached),
		Fields: []FormField{
			{Key: "tag", Label: "tag", Options: names, Value: names[0]},
			{Key: "action", Label: "action", Options: actions, Value: action},
		},
	}
}

// tagNote names what the task already carries.
func tagNote(attached []string) string {
	if len(attached) == 0 {
		return "this task carries no tags"
	}
	return "on this task: " + strings.Join(attached, ", ")
}

// DependencyForm picks one of the dependencies a task waits on. It is a form
// rather than the numbered picker because the picker reads a single digit, and a
// task waiting on more than nine others would have had the rest unreachable.
func DependencyForm(refs []string) Form {
	if len(refs) == 0 {
		return Form{}
	}
	return Form{
		Kind: formDependency, Title: "remove dependency",
		Fields: []FormField{
			{Key: "dependency", Label: "depends on", Options: refs, Value: refs[0]},
		},
	}
}

// uniqueSorted drops repeats and empties, in a stable order so a form offers
// its values the same way twice.
func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// unassignedOption is what the assignee picker calls having nobody. The service
// takes an empty identifier, which no list of alternatives can show.
const unassignedOption = "unassigned"

// ActorLabel names one actor the way a picker offers it: the handle a reader
// recognises, or a short identifier for an actor that carries none.
func ActorLabel(a core.Actor) string {
	if a.Handle != "" {
		return a.Handle
	}
	return shortID(a.ID)
}

// ActorOptions are the tenant's directory as a picker offers it, with
// "unassigned" first so clearing an assignment is a choice on the same list
// rather than a state with no way back.
func ActorOptions(actors []core.Actor) []string {
	out := []string{unassignedOption}
	for _, a := range actors {
		if label := ActorLabel(a); label != "" && !slices.Contains(out, label) {
			out = append(out, label)
		}
	}
	return out
}

// ActorIDFor resolves a picked option back to the identifier the service takes.
// The word for nobody resolves to the empty identifier, which is how an
// assignment is cleared.
func ActorIDFor(actors []core.Actor, option string) string {
	if option == unassignedOption {
		return ""
	}
	for _, a := range actors {
		if ActorLabel(a) == option {
			return a.ID
		}
	}
	return ""
}

// AssigneeForm offers the people and agents this tenant holds. It replaces a
// prompt that took an actor identifier, which is a question nobody at a
// terminal can answer: a reader knows a colleague by handle and has never seen
// the identifier the service stores.
//
// The form opens on whoever holds the task now, so the list states the current
// answer rather than making the reader find it.
func AssigneeForm(actors []core.Actor, current string) Form {
	options := ActorOptions(actors)
	if len(actors) == 0 {
		return Form{}
	}
	value := unassignedOption
	for _, a := range actors {
		if a.ID == current {
			value = ActorLabel(a)
		}
	}
	return Form{
		Kind: formAssignee, Title: "assign",
		Fields: []FormField{
			{Key: "assignee", Label: "assignee", Options: options, Value: value},
		},
	}
}

// ArtifactKindOptions are the kinds an artifact may be recorded under, taken
// from core.ArtifactKinds so the terminal cannot offer one the service refuses.
func ArtifactKindOptions() []string {
	out := make([]string, 0, len(core.ArtifactKinds))
	for _, k := range core.ArtifactKinds {
		out = append(out, string(k))
	}
	return out
}

// ArtifactNote says what a terminal does not record, so a reader who needs a
// payload learns it here rather than by finding the artifact empty afterwards.
const ArtifactNote = "the payload, content type and inline blob are recorded with tix artifact put"

// ArtifactForm classifies a named artifact. The name is free text and arrives
// from the prompt; the kind is the one answer drawn from a fixed set, and it is
// the only field the service requires.
func ArtifactForm(name string) Form {
	return Form{
		Kind: formArtifact, Title: "artifact " + name,
		Note: ArtifactNote,
		Fields: []FormField{
			{Key: "kind", Label: "kind", Options: ArtifactKindOptions(),
				Value: string(core.ArtifactResult)},
		},
	}
}

// TaskEditNote says which keys move between the fields, because a reader who
// cannot leave the body field is stuck in a way that reads as a hang.
const TaskEditNote = "tab and shift+tab move between fields; the arrows belong to the field you are in"

// TaskEditForm gathers everything one update can change about a task on one
// screen. Editing a task cost six separate keys and six round trips, each
// asking one question and applying it before the next could be asked.
//
// The single-key actions are all still there. This is the trip that changes
// several things at once, and the only place the body is more than one line.
//
// Custom fields are deliberately absent: they are per project and typed, and a
// form that gathers a typed value it cannot validate here would be a worse
// answer than tix task edit --field, which already exists.
func TaskEditForm(t core.Task, actors []core.Actor) Form {
	fields := []FormField{
		{Key: "title", Label: "title", Value: t.Title, Kind: FieldText, Limit: core.MaxTitleLength},
		{Key: "body", Label: "body", Value: t.Body, Kind: FieldProse, Limit: 4096},
		{Key: "priority", Label: "priority", Options: PriorityOptions(),
			Value: PriorityLabel(t.Priority)},
	}
	if len(actors) > 0 {
		value := unassignedOption
		for _, a := range actors {
			if a.ID == t.AssigneeActorID {
				value = ActorLabel(a)
			}
		}
		fields = append(fields, FormField{Key: "assignee", Label: "assignee",
			Options: ActorOptions(actors), Value: value})
	}
	return Form{Kind: formTaskEdit, Title: "edit " + t.Ref, Note: TaskEditNote, Fields: fields}
}

// PriorityOptions names every priority the way the rest of the interface names
// them, highest first, which is the order the numbered picker offers.
func PriorityOptions() []string {
	out := make([]string, 0, 5)
	for _, c := range PriorityChoices() {
		out = append(out, c.Label)
	}
	return out
}

// PriorityFor resolves a named priority back to the value the service takes,
// reporting a label no priority carries rather than guessing at one.
func PriorityFor(label string) (core.Priority, bool) {
	for p := core.PriorityHighest; p <= core.PriorityLowest; p++ {
		if PriorityLabel(p) == label {
			return p, true
		}
	}
	return core.PriorityNormal, false
}
