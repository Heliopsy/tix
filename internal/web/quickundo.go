// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"
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
	undoSnoozePrefix   = "snooze:"
	undoUnsnoozePrefix = "unsnooze:"
)

// undoKinds maps each prefix to the route that reverses it.
var undoKinds = []undoKind{
	{undoPriorityPrefix, RouteTaskPriority, "priority"},
	{undoSnoozePrefix, RouteTaskSnooze, "at"},
	{undoUnsnoozePrefix, RouteTaskUnsnooze, ""},
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

// snoozeAtValue is a wake time as the snooze form's picker posts it, in the
// reader's zone, so an undo can put it back.
func snoozeAtValue(t *time.Time, loc *time.Location) string {
	if t == nil {
		return ""
	}
	return t.In(loc).Format(SnoozeAtLayout)
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
