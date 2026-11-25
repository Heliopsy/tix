// SPDX-License-Identifier: AGPL-3.0-or-later

package ical

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// Todo is what one task's CalDAV object is rendered from.
type Todo struct {
	// Task is the task rendered.
	Task core.Task
	// Category is the category of the task's state in its workflow.
	Category core.StateCategory
	// ParentUID is the parent's UID, empty when there is no parent or the
	// reader cannot read it.
	ParentUID string
	// URL is the task's page, absolute.
	URL string
	// Now decides whether a snooze is still in the future.
	Now time.Time
	// UID replaces the task's own UID, for a task a client created.
	UID string
	// Kept are content lines a client sent that tix has no field for, kept
	// for the one person whose object this is, written back verbatim.
	Kept []string
}

// TodoUID is the UID a task's object carries: the client's, or the task's own.
func TodoUID(taskID, clientUID string) string {
	if clientUID != "" {
		return clientUID
	}
	return UID(taskID)
}

// Formats of iCalendar dates and UTC date-times.
const (
	dateLayout     = "20060102"
	dateTimeLayout = "20060102T150405Z"
)

// RenderTodo writes a task as a VCALENDAR holding one VTODO, by the mapping in
// docs/caldav.md. It never writes STATUS:CANCELLED or a recurrence rule.
func RenderTodo(in Todo) []byte {
	t := in.Task
	w := &writer{}
	w.prop("BEGIN", "VCALENDAR")
	w.prop("VERSION", "2.0")
	w.prop("PRODID", "-//tix//caldav//EN")
	w.prop("BEGIN", "VTODO")
	w.prop("UID", Text(TodoUID(t.ID, in.UID)))
	w.prop("DTSTAMP", t.UpdatedAt.UTC().Format(dateTimeLayout))
	w.prop("CREATED", t.CreatedAt.UTC().Format(dateTimeLayout))
	w.prop("LAST-MODIFIED", t.UpdatedAt.UTC().Format(dateTimeLayout))
	w.prop("SEQUENCE", strconv.Itoa(t.Version))
	w.prop("SUMMARY", Text(t.Title))
	if t.Body != "" {
		w.prop("DESCRIPTION", Text(t.Body))
	}
	w.prop("STATUS", todoStatus(in.Category))
	if in.Category == core.CategoryDone {
		done := t.UpdatedAt
		if t.CompletedAt != nil {
			done = *t.CompletedAt
		}
		w.prop("COMPLETED", done.UTC().Format(dateTimeLayout))
		w.prop("PERCENT-COMPLETE", "100")
	}
	if start, ok := TodoStart(t, in.Now); ok {
		if dateOnly(t.DueAt) {
			w.prop("DTSTART;VALUE=DATE", start.UTC().Format(dateLayout))
		} else {
			w.prop("DTSTART", start.UTC().Format(dateTimeLayout))
		}
	}
	if t.DueAt != nil {
		if dateOnly(t.DueAt) {
			w.prop("DUE;VALUE=DATE", t.DueAt.UTC().Format(dateLayout))
		} else {
			w.prop("DUE", t.DueAt.UTC().Format(dateTimeLayout))
		}
	}
	w.prop("PRIORITY", strconv.Itoa(TodoPriority(t.Priority)))
	if len(t.Tags) > 0 {
		tags := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			tags[i] = Text(tag)
		}
		w.prop("CATEGORIES", strings.Join(tags, ","))
	}
	if in.ParentUID != "" {
		w.prop("RELATED-TO;RELTYPE=PARENT", Text(in.ParentUID))
	}
	if in.URL != "" {
		w.prop("URL", in.URL)
	}
	w.prop("X-TIX-REF", Text(output.ShowRef(t.Ref)))
	w.prop("X-TIX-STATE", Text(t.Status))
	for _, line := range in.Kept {
		if in.Category == core.CategoryDone && strings.HasPrefix(strings.ToUpper(line), "PERCENT-COMPLETE") {
			continue
		}
		w.buf.WriteString(Fold(stripControls(line)))
	}
	w.prop("END", "VTODO")
	w.prop("END", "VCALENDAR")
	return w.buf.Bytes()
}

// todoStatus is the VTODO STATUS of a state category. Blocked and waiting have
// no counterpart and read as needing action; cancelled is never written,
// because a cancelled task is not a member of any collection.
func todoStatus(c core.StateCategory) string {
	switch c {
	case core.CategoryDone:
		return "COMPLETED"
	case core.CategoryInProgress:
		return "IN-PROCESS"
	default:
		return "NEEDS-ACTION"
	}
}

// dateOnly reports whether a due instant is a bare date: midnight UTC, which
// is how every surface stores a date given without a time.
func dateOnly(due *time.Time) bool {
	if due == nil {
		return false
	}
	u := due.UTC()
	return u.Hour() == 0 && u.Minute() == 0 && u.Second() == 0 && u.Nanosecond() == 0
}

// TodoStart is the DTSTART a task carries: its snooze, while that is still in
// the future and before the due date. A start on or after the due date would
// be refused by ical4j, so it is left out.
func TodoStart(t core.Task, now time.Time) (time.Time, bool) {
	if t.SnoozedUntil == nil || !t.SnoozedUntil.After(now) {
		return time.Time{}, false
	}
	start := *t.SnoozedUntil
	if t.DueAt == nil {
		return start, true
	}
	if dateOnly(t.DueAt) {
		day := start.UTC().Truncate(24 * time.Hour)
		return day, day.Before(t.DueAt.UTC())
	}
	return start, start.Before(*t.DueAt)
}

// TodoPriority is the iCalendar PRIORITY of a tix priority. Normal is 0,
// undefined, so a normal task shows no marker in a reminders list.
func TodoPriority(p core.Priority) int {
	switch p {
	case core.PriorityHighest:
		return 1
	case core.PriorityHigh:
		return 3
	case core.PriorityLow:
		return 7
	case core.PriorityLowest:
		return 9
	default:
		return 0
	}
}

// PriorityFromTodo reads an iCalendar PRIORITY back: 0 and 5 normal, 1 and 2
// highest, 3 and 4 high, 6 and 7 low, 8 and 9 lowest. Every value TodoPriority
// writes reads back to itself.
func PriorityFromTodo(n int) core.Priority {
	switch n {
	case 1, 2:
		return core.PriorityHighest
	case 3, 4:
		return core.PriorityHigh
	case 6, 7:
		return core.PriorityLow
	case 8, 9:
		return core.PriorityLowest
	default:
		return core.PriorityNormal
	}
}

// ETagInput is everything a task's object depends on, so its entity tag can
// be computed without rendering it.
type ETagInput struct {
	TaskID    string
	Version   int
	UpdatedAt time.Time
	// ProjectKey changes X-TIX-REF and URL.
	ProjectKey string
	// Categories is the task's workflow, state key to category, which
	// decides STATUS.
	Categories map[string]core.StateCategory
	// Origin is what URL is built under.
	Origin string
	// ParentUID is the RELATED-TO the reader is shown.
	ParentUID string
	// Start is the DTSTART shown, zero for none: a snooze that passes changes
	// the object without changing the task.
	Start time.Time
	// UID is a client's UID for a task it created, empty for the task's own.
	UID string
	// Kept is what is kept for the reader, as KeptDigest names it; empty for
	// nothing.
	Kept string
}

// KeptDigest names a set of kept lines for an entity tag, empty for none.
func KeptDigest(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	h := sha256.Sum256([]byte(strings.Join(lines, "\r\n")))
	return hex.EncodeToString(h[:16])
}

// ETag is a strong entity tag over an object's rendering inputs.
func ETag(in ETagInput) string {
	h := sha256.New()
	field := func(s string) {
		h.Write([]byte(strconv.Itoa(len(s))))
		h.Write([]byte{':'})
		h.Write([]byte(s))
	}
	field(in.TaskID)
	field(strconv.Itoa(in.Version))
	field(in.UpdatedAt.UTC().Format(time.RFC3339Nano))
	field(in.ProjectKey)
	keys := make([]string, 0, len(in.Categories))
	for k := range in.Categories {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		field(k)
		field(string(in.Categories[k]))
	}
	field(in.Origin)
	field(in.ParentUID)
	if !in.Start.IsZero() {
		field(in.Start.UTC().Format(time.RFC3339Nano))
	}
	if in.UID != "" {
		field("uid:" + in.UID)
	}
	if in.Kept != "" {
		field("kept:" + in.Kept)
	}
	return `"` + hex.EncodeToString(h.Sum(nil)[:16]) + `"`
}
