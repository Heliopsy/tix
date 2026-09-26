// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"sort"
	"strconv"
	"strings"
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

// FormField is one answer a form gathers, drawn from a fixed set of values.
//
// There is deliberately no free-text field kind. None of the operations this
// form was built for wants one: a delete chooses among its own switches, and
// picking a tag or a dependency chooses among the ones that exist. Free text
// already has the single-line prompt, and a field kind with no caller would be
// a guess about the operations that come next.
type FormField struct {
	Key     string
	Label   string
	Options []string
	Value   string
	Needs   FieldCondition
}

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
func (f Form) Cycle(delta int) Form {
	selected, ok := f.Selected()
	if !ok || len(selected.Options) == 0 {
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
}

// Lines renders the visible fields, each with the values it could hold instead,
// because a field whose alternatives are invisible reads as a label rather than
// as a question.
func (f Form) Lines() []FormLine {
	visible := f.Visible()
	out := make([]FormLine, 0, len(visible))
	for i, field := range visible {
		out = append(out, FormLine{
			Label: field.Label, Value: field.Value,
			Hint: FieldHint(field), Selected: i == f.Sel,
		})
	}
	return out
}

// hintWidth is how much room the alternatives get before a field states its
// position in the list instead of listing it.
const hintWidth = 44

// FieldHint says what else a field could hold: the whole list while it is short
// enough to read, and where in the list this answer sits once it is not.
func FieldHint(field FormField) string {
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
