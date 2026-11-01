// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/query"
)

// SnoozeAtLayout is the value a datetime-local input posts.
const SnoozeAtLayout = "2006-01-02T15:04"

// snoozePreset is one named wake time a snooze menu offers.
type snoozePreset struct {
	Value string
	Label string
}

// snoozePresets are the menu's choices, in the order they are drawn. The
// values are the presets query.Preset resolves, so the browser and the
// command line mean the same moment by the same word.
var snoozePresets = []snoozePreset{
	{Value: query.UntilLater, Label: "Later today"},
	{Value: query.UntilTomorrow, Label: "Tomorrow morning"},
	{Value: query.UntilNextWeek, Label: "Next week"},
}

// snoozeForm is what the "snooze-choices" partial draws: one task's presets
// and date picker, where they return the reader, and, on a lane card, the
// card the answer is swapped into.
type snoozeForm struct {
	Ref  string
	Next string
	CSRF string
	ID   string
	Card string
}

// Presets are the named wake times the form offers.
func (snoozeForm) Presets() []snoozePreset { return snoozePresets }

// newSnoozeForm builds the snooze form for a template. card is empty except
// on a lane card, whose own forms swap only the card.
func newSnoozeForm(ref, next, csrf, id, card string) snoozeForm {
	return snoozeForm{Ref: ref, Next: next, CSRF: csrf, ID: id, Card: card}
}

// readerStyle is the time style this request's pages render in, built the
// way templatesFor builds it, so a toast and the page around it agree on the
// zone and the layout.
func (h *handler) readerStyle(r *http.Request) output.TimeStyle {
	format, zone := timeFormatOf(r), timezoneOf(r)
	if format == "" && zone == "" {
		return h.style
	}
	style, err := output.NewTimeStyle(format, zone)
	if err != nil {
		return h.style
	}
	return style
}

// snoozeUntil reads when the reader asked the task to wake: a preset posted
// as until, or a picked date and time posted as at, both in the reader's zone.
func snoozeUntil(r *http.Request, loc *time.Location, now time.Time) (time.Time, error) {
	if at := strings.TrimSpace(field(r, "at")); at != "" && field(r, "until") == "" {
		t, err := time.ParseInLocation(SnoozeAtLayout, at, loc)
		if err != nil {
			return time.Time{}, core.Invalid("pick a date and a time to wake the task")
		}
		return t, nil
	}
	name := strings.TrimSpace(field(r, "until"))
	if t, ok := query.Preset(name, now, loc); ok {
		return t, nil
	}
	if name == "" {
		return time.Time{}, core.Invalid("pick when the task should wake")
	}
	return time.Time{}, core.Invalid("%q is not a time this menu offers", name)
}

// snoozeTask hides a task until the time the reader picked and sends them
// back where they were. On a listing the task leaves it, so the toast names
// the task and its wake time.
func (h *handler) snoozeTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	style := h.readerStyle(r)
	until, err := snoozeUntil(r, style.Location(), time.Now())
	if err != nil {
		return err
	}
	task, err := h.svc.SnoozeTask(r.Context(), ref, core.SnoozeInput{Until: until})
	if err != nil {
		return err
	}
	redirectTo(w, r, h.backFrom(r), "t-"+ref.String(),
		"snoozed "+task.Ref+" until "+style.FormatPtr(task.SnoozedUntil))
	return nil
}

// unsnoozeTask brings a snoozed task back now.
func (h *handler) unsnoozeTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	if _, err := h.svc.UnsnoozeTask(r.Context(), ref); err != nil {
		return err
	}
	redirectTo(w, r, h.backFrom(r), "t-"+ref.String(), "unsnoozed "+ref.String())
	return nil
}

// backFrom is the listing or screen a posted next names, or the task's own
// screen, the way the star sends a reader back.
func (h *handler) backFrom(r *http.Request) string {
	back := safeNext(field(r, "next"))
	if back == RouteProjects && field(r, "next") != RouteProjects {
		back = taskPath(r)
	}
	return back
}

// snoozedNow reports whether a task is snoozed at the moment it is drawn.
func snoozedNow(t core.Task) bool { return t.SnoozedAt(time.Now()) }
