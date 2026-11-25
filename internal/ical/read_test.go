// SPDX-License-Identifier: AGPL-3.0-or-later

package ical

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// object wraps VTODO lines, and any components after it, in a VCALENDAR.
func object(todo []string, after ...string) []byte {
	lines := append([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN", "BEGIN:VTODO"}, todo...)
	lines = append(lines, "END:VTODO")
	lines = append(lines, after...)
	lines = append(lines, "END:VCALENDAR")
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

func mustRead(t *testing.T, data []byte, zone *time.Location) *TodoValues {
	t.Helper()
	v, err := ReadTodo(data, zone)
	if err != nil {
		t.Fatalf("reading: %v\n%s", err, data)
	}
	return v
}

func TestReadTodoRoundTripsWhatRenderTodoWrites(t *testing.T) {
	due := time.Date(2026, 12, 1, 14, 30, 0, 0, time.UTC)
	snooze := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	task := todoTask()
	task.Title = "Rotate keys, then; tell \\ ops\nnow"
	task.Body = "line one\nline two, with a comma"
	task.Tags = []string{"ops", "security, urgent"}
	task.Priority = core.PriorityHigh
	task.DueAt, task.SnoozedUntil = &due, &snooze
	v := mustRead(t, RenderTodo(Todo{Task: task, Category: core.CategoryInProgress, ParentUID: "task-p@tix",
		URL: "https://tix.example/tasks/INFRA-12", Now: todoNow}), nil)
	if v.UID != UID(task.ID) || *v.Summary != task.Title || *v.Description != task.Body {
		t.Errorf("text fields %q %q %q", v.UID, *v.Summary, *v.Description)
	}
	if v.Status != StatusInProcess || v.Completed || v.PercentComplete != -1 {
		t.Errorf("status %q completed %v percent %d", v.Status, v.Completed, v.PercentComplete)
	}
	if v.Due == nil || !v.Due.At.Equal(due) || v.Due.Date || v.Start == nil || !v.Start.At.Equal(snooze) {
		t.Errorf("due %+v start %+v", v.Due, v.Start)
	}
	if v.Priority == nil || PriorityFromTodo(*v.Priority) != task.Priority {
		t.Errorf("priority %v", v.Priority)
	}
	if !v.HasCategories || !slices.Equal(v.Categories, task.Tags) {
		t.Errorf("categories %q", v.Categories)
	}
	if len(v.Kept) != 0 {
		t.Errorf("tix's own object kept %q", v.Kept)
	}
	task.DueAt = new(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	v = mustRead(t, RenderTodo(Todo{Task: task, Category: core.CategoryDone, Now: todoNow}), nil)
	if !v.Due.Date || !v.Due.At.Equal(*task.DueAt) || v.Status != StatusCompleted || !v.Completed ||
		v.PercentComplete != 100 || len(v.Kept) != 0 {
		t.Errorf("a done task with a bare date read back as %+v", v)
	}
}

func TestReadTodoKeepsWhatTixHasNoFieldFor(t *testing.T) {
	v := mustRead(t, object([]string{"UID:abc", "SUMMARY:Call the bank", "LOCATION:Main St\\, 4",
		"X-APPLE-SORT-ORDER:12", "RRULE:FREQ=WEEKLY", "EXDATE:20261201T000000Z", "PERCENT-COMPLETE:40",
		"CLASS:PRIVATE", "URL:https://elsewhere.example", "RELATED-TO:parent", "DURATION:PT1H",
		"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER;RELATED=START:-PT15M", "END:VALARM"}), nil)
	want := []string{"LOCATION:Main St\\, 4", "X-APPLE-SORT-ORDER:12", "PERCENT-COMPLETE:40", "CLASS:PRIVATE",
		"BEGIN:VALARM", "ACTION:DISPLAY", "TRIGGER;RELATED=START:-PT15M", "END:VALARM"}
	if !slices.Equal(v.Kept, want) {
		t.Errorf("kept %q\nwant %q", v.Kept, want)
	}
	if v.PercentComplete != 40 {
		t.Errorf("percent %d", v.PercentComplete)
	}
	out := string(RenderTodo(Todo{Task: todoTask(), Now: todoNow, UID: "abc", Kept: v.Kept}))
	if !strings.Contains(out, "UID:abc\r\n") || !strings.Contains(out, "BEGIN:VALARM\r\nACTION:DISPLAY\r\n") ||
		strings.Contains(out, "RRULE") || strings.Contains(out, "elsewhere") {
		t.Errorf("rendered with what was kept:\n%s", out)
	}
	done := string(RenderTodo(Todo{Task: todoTask(), Category: core.CategoryDone, Now: todoNow, Kept: v.Kept}))
	if strings.Count(done, "PERCENT-COMPLETE") != 1 || !strings.Contains(done, "PERCENT-COMPLETE:100") {
		t.Errorf("a done task carries a kept percentage:\n%s", done)
	}
}

func TestReadTodoRefusesWhatIsNotOneTask(t *testing.T) {
	for name, data := range map[string][]byte{
		"an event": []byte("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:x\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
		"two tasks": object([]string{"UID:a"}, "BEGIN:VTODO", "UID:a", "RECURRENCE-ID:20261201T000000Z",
			"END:VTODO"),
		"no task":   []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"),
		"no uid":    object([]string{"SUMMARY:x"}),
		"bare todo": []byte("BEGIN:VTODO\r\nUID:x\r\nEND:VTODO\r\n"),
		"bad due":   object([]string{"UID:x", "DUE:tomorrow"}),
		"unclosed":  []byte("BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:x\r\n"),
		"too large": object([]string{"UID:x", "DESCRIPTION:" + strings.Repeat("a", MaxObjectBytes)}),
		"too deep": []byte("BEGIN:VCALENDAR\r\n" + strings.Repeat("BEGIN:X\r\n", maxDepth) +
			strings.Repeat("END:X\r\n", maxDepth) + "END:VCALENDAR\r\n"),
	} {
		_, err := ReadTodo(data, nil)
		var e *core.Error
		if !core.IsKind(err, core.KindInvalid) || !asError(err, &e) || e.Details["reason"] != ReasonInvalidObject {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func asError(err error, out **core.Error) bool { return errors.As(err, out) }

const berlinZone = `BEGIN:VTIMEZONE
TZID:Custom/Berlin
BEGIN:DAYLIGHT
TZOFFSETFROM:+0100
TZOFFSETTO:+0200
DTSTART:19700329T020000
RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU
END:DAYLIGHT
BEGIN:STANDARD
TZOFFSETFROM:+0200
TZOFFSETTO:+0100
DTSTART:19701025T030000
RRULE:FREQ=YEARLY;BYMONTH=10;BYDAY=-1SU
END:STANDARD
END:VTIMEZONE`

// Each client's way of writing a due date, from their published behaviour.
func TestReadTodoReadsEveryClientsDates(t *testing.T) {
	sofia, _ := time.LoadLocation("Europe/Sofia")
	zone := strings.Split(berlinZone, "\n")
	tests := []struct {
		name  string
		due   string
		after []string
		want  time.Time
		date  bool
	}{
		{"Apple, all day", "DUE;VALUE=DATE:20261205", nil, time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC), true},
		{"Apple, a time in a known zone", "DUE;TZID=Europe/Sofia:20261205T090000", nil,
			time.Date(2026, 12, 5, 7, 0, 0, 0, time.UTC), false},
		{"Thunderbird, a known zone with its VTIMEZONE", "DUE;TZID=Europe/Berlin:20260710T090000", zone,
			time.Date(2026, 7, 10, 7, 0, 0, 0, time.UTC), false},
		{"tasks.org through DAVx5, UTC", "DUE:20261205T090000Z", nil, time.Date(2026, 12, 5, 9, 0, 0, 0, time.UTC), false},
		{"DAVx5, a date without VALUE", "DUE:20261205", nil, time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC), true},
		{"an unknown zone in summer, from its VTIMEZONE", "DUE;TZID=Custom/Berlin:20260710T090000", zone,
			time.Date(2026, 7, 10, 7, 0, 0, 0, time.UTC), false},
		{"an unknown zone in winter, from its VTIMEZONE", "DUE;TZID=Custom/Berlin:20260110T090000", zone,
			time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC), false},
		{"Evolution's prefixed zone", "DUE;TZID=/Europe/Sofia:20261205T090000", nil,
			time.Date(2026, 12, 5, 7, 0, 0, 0, time.UTC), false},
		{"an unknown zone with no VTIMEZONE, in the person's zone", "DUE;TZID=Nowhere:20261205T090000", nil,
			time.Date(2026, 12, 5, 7, 0, 0, 0, time.UTC), false},
		{"floating, in the person's zone", "DUE:20261205T090000", nil, time.Date(2026, 12, 5, 7, 0, 0, 0, time.UTC), false},
	}
	for _, tc := range tests {
		v := mustRead(t, object([]string{"UID:x", tc.due}, tc.after...), sofia)
		if v.Due == nil || !v.Due.At.Equal(tc.want) || v.Due.Date != tc.date {
			t.Errorf("%s: read %+v, want %s date=%v", tc.name, v.Due, tc.want, tc.date)
		}
	}
	if v := mustRead(t, object([]string{"UID:x", "DUE:20261205T090000"}), nil); !v.Due.At.Equal(time.Date(2026, 12, 5, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("a floating time with no zone is %s, want UTC", v.Due.At)
	}
}

func TestParseBoundsHowDeeplyComponentsNest(t *testing.T) {
	deep := func(n int) []byte {
		return []byte(strings.Repeat("BEGIN:X\r\n", n) + strings.Repeat("END:X\r\n", n))
	}
	if _, err := Parse(deep(maxDepth)); err != nil {
		t.Errorf("%d levels: %v", maxDepth, err)
	}
	if _, err := Parse(deep(maxDepth + 1)); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("%d levels: %v", maxDepth+1, err)
	}
}

func TestObservanceOnsetFindsTheNthWeekday(t *testing.T) {
	first := time.Date(1970, 3, 29, 2, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		rule string
		want time.Time
	}{
		{"FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU", time.Date(2026, 3, 29, 2, 0, 0, 0, time.UTC)},
		{"FREQ=YEARLY;BYMONTH=3;BYDAY=2SU", time.Date(2026, 3, 8, 2, 0, 0, 0, time.UTC)},
		{"FREQ=YEARLY;BYMONTH=11;BYDAY=1SU", time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC)},
	} {
		if got, ok := observanceOnset(first, tc.rule, 2026); !ok || !got.Equal(tc.want) {
			t.Errorf("%s: %s %v, want %s", tc.rule, got, ok, tc.want)
		}
	}
	if _, ok := observanceOnset(first, "FREQ=MONTHLY", 2026); ok {
		t.Error("a monthly rule was read")
	}
}

func FuzzReadTodo(f *testing.F) {
	f.Add(RenderTodo(Todo{Task: todoTask(), Now: todoNow}))
	f.Add(object([]string{"UID:x", "DUE;TZID=Custom/Berlin:20260710T090000"}, strings.Split(berlinZone, "\n")...))
	f.Add(object([]string{"UID:x", "BEGIN:VALARM", "TRIGGER:-PT5M", "END:VALARM"}))
	f.Fuzz(func(t *testing.T, data []byte) {
		v, err := ReadTodo(data, time.UTC)
		if err == nil && (v == nil || v.UID == "") {
			t.Fatal("a task with no UID and no error")
		}
	})
}
