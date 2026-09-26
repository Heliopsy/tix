// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import "github.com/heliopsy/tix/internal/core"

// confirmKind names the destructive action a confirmation stands in front of.
type confirmKind int

// The confirmations.
const (
	confirmNone confirmKind = iota
	confirmDeleteTask
	confirmDeleteComment
	confirmArchiveProject
	confirmDeleteProject
	confirmDeleteField
)

// confirmVerbs are what each confirmation says it will do, in the words the
// question asks it in.
var confirmVerbs = map[confirmKind]string{
	confirmDeleteTask:     "delete task",
	confirmDeleteComment:  "delete",
	confirmArchiveProject: "archive project",
	confirmDeleteProject:  "delete project",
	confirmDeleteField:    "delete custom field",
}

// Confirm is one destructive action held back until the reader agrees to it.
//
// Target is what the action will affect, spelled the way the reader sees it on
// screen. It is not optional: a confirmation that asks "are you sure?" names
// nothing, so agreeing to it is agreeing to whatever happened to be selected,
// which is how somebody deletes the wrong row. Question renders nothing without
// it, and the interface refuses to open a confirmation that renders nothing.
type Confirm struct {
	Kind   confirmKind
	Target string
	// Note is what the action will do beyond its subject, such as taking the
	// subtasks with it, which is the part a reader cannot see from the target.
	Note string

	// ref, commentID, hard and cascade address the subject for the service
	// call the agreement releases.
	ref       core.TaskRef
	label     string
	commentID string
	hard      bool
	cascade   bool
	// projectRef and fieldKey address the subjects that are not a task: the
	// project a removal acts on, and the custom field a deletion names.
	projectRef string
	fieldKey   string
}

// Open reports whether a confirmation is waiting for an answer.
func (c Confirm) Open() bool { return c.Kind != confirmNone }

// Question states what the action will do, naming its subject.
func (c Confirm) Question() string {
	verb := confirmVerbs[c.Kind]
	if verb == "" || c.Target == "" {
		return ""
	}
	q := verb + " " + c.Target
	if c.Note != "" {
		q += ", " + c.Note
	}
	return q + "?"
}

// DeleteNote says how far a task deletion will reach, which neither the ref nor
// the title can say. It renders nothing for the ordinary case, so the question
// only grows for the deletions that do more than they look like.
func DeleteNote(hard, cascade bool) string {
	switch {
	case hard && cascade:
		return "permanently, with its subtasks"
	case hard:
		return "permanently"
	case cascade:
		return "with its subtasks"
	default:
		return ""
	}
}

// ConfirmHelp is the instruction the confirmation carries, naming the keys that
// answer it rather than assuming the reader knows them, because the answer keys
// are the two a scheme is most likely to have moved.
func ConfirmHelp(agree, cancel string) string {
	return "(" + agree + " confirms, " + cancel + " cancels)"
}
