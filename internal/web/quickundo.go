// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// undoKind is one quick action's way back: the prefix its undo is written
// with, the task route the toast's Undo posts to, and the one field that
// carries the value after the prefix. An empty field posts nothing but the
// token and the page to return to.
type undoKind struct {
	prefix string
	route  string
	field  string
}

// The undos the quick actions offer, written as prefix and value in
// FlashUndoParam. A transition's undo is a bare route of states, which never
// starts with one of these.
const (
	undoStarPrefix     = "star:"
	undoUnstarPrefix   = "unstar:"
	undoSnoozePrefix   = "snooze:"
	undoUnsnoozePrefix = "unsnooze:"
	undoAssignPrefix   = "assign:"
	undoDuePrefix      = "due:"
	undoTagPrefix      = "tag:"
	undoUntagPrefix    = "untag:"
)

// undoKinds maps each prefix to the route that reverses it.
var undoKinds = []undoKind{
	{undoPriorityPrefix, RouteTaskPriority, "priority"},
	{undoStarPrefix, RouteTaskStar, ""},
	{undoUnstarPrefix, RouteTaskUnstar, ""},
	{undoSnoozePrefix, RouteTaskSnooze, "at"},
	{undoUnsnoozePrefix, RouteTaskUnsnooze, ""},
	{undoAssignPrefix, RouteTaskAssign, "assignee"},
	{undoDuePrefix, RouteTaskDue, "due"},
	{undoTagPrefix, RouteTaskTags, "tag"},
	{undoUntagPrefix, RouteTaskTagDel, "tag"},
}

// undoKindOf finds the kind an undo was written with, and the value after
// its prefix.
func undoKindOf(undo string) (undoKind, string, bool) {
	for _, k := range undoKinds {
		if value, ok := strings.CutPrefix(undo, k.prefix); ok {
			return k, value, true
		}
	}
	return undoKind{}, "", false
}

// assignTask sets or clears one task's assignee and nothing else: "me" is the
// reader, an empty value unassigns, anything else is a handle or an actor id.
// The toast offers the assignee it had before.
func (h *handler) assignTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	assignee := field(r, "assignee")
	toMe := assignee == "me"
	if toMe {
		actor, ok := core.ActorFrom(r.Context())
		if !ok {
			return core.Unauthenticated("sign in to take a task")
		}
		assignee = actor.ID
	}
	before, err := h.svc.GetTask(r.Context(), ref)
	if err != nil {
		return err
	}
	after, err := h.svc.UpdateTask(r.Context(), ref, core.UpdateTaskInput{AssigneeActorID: &assignee})
	if err != nil {
		return err
	}
	undo := ""
	if before.AssigneeActorID != after.AssigneeActorID {
		undo = undoAssignPrefix + before.AssigneeActorID
	}
	switch {
	case after.AssigneeActorID == "":
		h.afterTransition(w, r, ref, "assignee cleared", undo)
	case toMe:
		h.afterTransition(w, r, ref, "assigned to you", undo)
	default:
		h.afterTransition(w, r, ref, "assignee changed", undo)
	}
	return nil
}

// setDue sets or clears one task's due date and nothing else, read the way
// the task screen's Due field reads it. The toast offers the date it had.
func (h *handler) setDue(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	due, err := dateField(field(r, "due"), "due", h.readerNow(r), h.readerStyle(r).DateOrder())
	if err != nil {
		return err
	}
	before, err := h.svc.GetTask(r.Context(), ref)
	if err != nil {
		return err
	}
	if _, err := h.svc.UpdateTask(r.Context(), ref, core.UpdateTaskInput{DueAt: &due}); err != nil {
		return err
	}
	undo := ""
	if !sameInstant(before.DueAt, due) {
		undo = undoDuePrefix + instantValue(before.DueAt)
	}
	if due == nil {
		h.afterTransition(w, r, ref, "due date cleared", undo)
	} else {
		h.afterTransition(w, r, ref, "due date set", undo)
	}
	return nil
}

// sameInstant reports whether two optional instants are both absent or equal.
func sameInstant(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// instantValue writes an optional instant the way the due field reads it
// back exactly, or nothing for none.
func instantValue(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// snoozeAtValue is a wake time as the snooze form's picker posts it, in the
// reader's zone, so an undo can put it back.
func snoozeAtValue(t *time.Time, loc *time.Location) string {
	if t == nil {
		return ""
	}
	return t.In(loc).Format(SnoozeAtLayout)
}

// assignMeForm is what the "assign-me" partial draws: one press that makes
// the reader a task's assignee, where it returns them, and, on a lane card,
// the card the answer is swapped into.
type assignMeForm struct {
	Ref  string
	Show bool
	Next string
	CSRF string
	Card string
}

// newAssignMe builds the Assign to me button for a template. It is drawn only
// for a signed-in reader the task is not already assigned to.
func newAssignMe(ref, assignee string, actor *core.Actor, next, csrf, card string) assignMeForm {
	show := actor != nil && actor.ID != "" && actor.ID != assignee
	return assignMeForm{Ref: ref, Show: show, Next: next, CSRF: csrf, Card: card}
}

// dueFieldForm is what the "due-field" partial draws: one task's due date
// field, a Clear button while it has one, where both return the reader and,
// on a lane card, the card the answer is swapped into.
type dueFieldForm struct {
	Ref  string
	Due  *time.Time
	Next string
	CSRF string
	ID   string
	Card string
}

// newDueField builds the due date field for a template.
func newDueField(ref string, due *time.Time, next, csrf, id, card string) dueFieldForm {
	return dueFieldForm{Ref: ref, Due: due, Next: next, CSRF: csrf, ID: id, Card: card}
}

// redirectToUndo is redirectTo with an undo for the toast, written after the
// flash so the listing's own address still leads the redirect.
func redirectToUndo(w http.ResponseWriter, r *http.Request, path, fragment, flash, undo string) {
	target := path
	if flash != "" {
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + "flash=" + url.QueryEscape(flash)
	}
	redirectTo(w, r, withUndo(target, undo), fragment, "")
}
