// SPDX-License-Identifier: AGPL-3.0-or-later

package ical

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

var todoNow = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func todoTask() core.Task {
	created := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	return core.Task{
		ID: "01JTASK00000000000000000AB", Ref: "INFRA-12", Title: "Rotate keys", Status: "todo",
		Priority: core.PriorityNormal, Version: 4, CreatedAt: created, UpdatedAt: created.Add(time.Hour),
	}
}

// vtodo renders and parses back the one VTODO of an object.
func vtodo(t *testing.T, in Todo) *Component {
	t.Helper()
	data := RenderTodo(in)
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\r\n"), "\r\n") {
		if len(line) > maxLineOctets {
			t.Fatalf("line of %d octets: %q", len(line), line)
		}
	}
	cal, err := Parse(data)
	if err != nil {
		t.Fatalf("parsing what was rendered: %v\n%s", err, data)
	}
	if cal.Name != "VCALENDAR" || len(cal.Children) != 1 || cal.Children[0].Name != "VTODO" {
		t.Fatalf("not one VTODO in a VCALENDAR:\n%s", data)
	}
	if _, ok := cal.Prop("METHOD"); ok {
		t.Error("a CalDAV object carries METHOD")
	}
	return cal.Children[0]
}

func value(c *Component, name string) string {
	p, _ := c.Prop(name)
	return p.Value
}

func TestRenderTodoIsTheMappingTable(t *testing.T) {
	want := strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//tix//caldav//EN",
		"BEGIN:VTODO",
		"UID:task-01jtask00000000000000000ab@tix",
		"DTSTAMP:20261101T090000Z",
		"CREATED:20261101T080000Z",
		"LAST-MODIFIED:20261101T090000Z",
		"SEQUENCE:4",
		"SUMMARY:Rotate keys",
		"DESCRIPTION:Old ones leak\\; see the runbook\\, step 2.\\nThen check.",
		"STATUS:NEEDS-ACTION",
		"DUE;VALUE=DATE:20261105",
		"PRIORITY:0",
		"CATEGORIES:ops,security",
		"RELATED-TO;RELTYPE=PARENT:task-01jparent0000000000000000@tix",
		"URL:https://tix.example/tasks/INFRA-12",
		"X-TIX-REF:INFRA-12",
		"X-TIX-STATE:todo",
		"END:VTODO",
		"END:VCALENDAR",
	}, "\r\n") + "\r\n"
	task := todoTask()
	task.Body = "Old ones leak; see the runbook, step 2.\nThen check."
	due := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	task.DueAt = &due
	task.Tags = []string{"ops", "security"}
	got := RenderTodo(Todo{Task: task, Category: core.CategoryTodo, URL: "https://tix.example/tasks/INFRA-12",
		ParentUID: UID("01JPARENT0000000000000000"), Now: todoNow})
	if string(got) != want {
		t.Errorf("rendered\n%s\nwant\n%s", got, want)
	}
}

func TestRenderTodoStatusFollowsTheCategory(t *testing.T) {
	completed := time.Date(2026, 11, 20, 9, 15, 0, 0, time.UTC)
	for _, tc := range []struct {
		category core.StateCategory
		status   string
	}{
		{core.CategoryTodo, "NEEDS-ACTION"},
		{core.CategoryBlocked, "NEEDS-ACTION"},
		{core.CategoryWaiting, "NEEDS-ACTION"},
		{core.CategoryInProgress, "IN-PROCESS"},
		{core.CategoryDone, "COMPLETED"},
		{core.CategoryCancelled, "NEEDS-ACTION"},
	} {
		t.Run(string(tc.category), func(t *testing.T) {
			task := todoTask()
			task.Status = "waiting-on-vendor"
			task.CompletedAt = &completed
			v := vtodo(t, Todo{Task: task, Category: tc.category, Now: todoNow})
			if got := value(v, "STATUS"); got != tc.status {
				t.Errorf("STATUS %q, want %q", got, tc.status)
			}
			if got := value(v, "X-TIX-STATE"); got != "waiting-on-vendor" {
				t.Errorf("X-TIX-STATE %q", got)
			}
			done := tc.category == core.CategoryDone
			if got := value(v, "COMPLETED"); (got == "20261120T091500Z") != done {
				t.Errorf("COMPLETED %q for %s", got, tc.category)
			}
			if got := value(v, "PERCENT-COMPLETE"); (got == "100") != done {
				t.Errorf("PERCENT-COMPLETE %q for %s", got, tc.category)
			}
		})
	}
}

func TestRenderTodoDatesFollowTheBareDateRule(t *testing.T) {
	at := func(s string) *time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return &v
	}
	cases := []struct {
		name          string
		due, snoozed  *time.Time
		wantDue       string
		wantDueParam  string
		wantStart     string
		wantStartDate bool
	}{
		{name: "no due date"},
		{name: "a bare date", due: at("2026-11-05T00:00:00Z"), wantDue: "20261105", wantDueParam: "DATE"},
		{name: "a time", due: at("2026-11-05T14:30:00Z"), wantDue: "20261105T143000Z"},
		{name: "a time in another zone", due: at("2026-11-05T16:30:00+02:00"), wantDue: "20261105T143000Z"},
		{name: "snoozed before a timed due", due: at("2026-11-25T14:30:00Z"), snoozed: at("2026-11-23T08:00:00Z"),
			wantDue: "20261125T143000Z", wantStart: "20261123T080000Z"},
		{name: "snoozed before a bare due", due: at("2026-11-25T00:00:00Z"), snoozed: at("2026-11-23T08:00:00Z"),
			wantDue: "20261125", wantDueParam: "DATE", wantStart: "20261123", wantStartDate: true},
		{name: "snoozed past the due date", due: at("2026-11-22T14:30:00Z"), snoozed: at("2026-11-23T08:00:00Z"),
			wantDue: "20261122T143000Z"},
		{name: "snoozed to the due day", due: at("2026-11-23T00:00:00Z"), snoozed: at("2026-11-23T08:00:00Z"),
			wantDue: "20261123", wantDueParam: "DATE"},
		{name: "a snooze that has passed", due: at("2026-11-25T14:30:00Z"), snoozed: at("2026-11-19T08:00:00Z"),
			wantDue: "20261125T143000Z"},
		{name: "snoozed with no due date", snoozed: at("2026-11-23T08:00:00Z"), wantStart: "20261123T080000Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := todoTask()
			task.DueAt, task.SnoozedUntil = tc.due, tc.snoozed
			v := vtodo(t, Todo{Task: task, Category: core.CategoryTodo, Now: todoNow})
			due, hasDue := v.Prop("DUE")
			if hasDue != (tc.wantDue != "") || due.Value != tc.wantDue || due.Param("VALUE") != tc.wantDueParam {
				t.Errorf("DUE %+v (%v), want %q with VALUE %q", due, hasDue, tc.wantDue, tc.wantDueParam)
			}
			start, hasStart := v.Prop("DTSTART")
			if hasStart != (tc.wantStart != "") || start.Value != tc.wantStart ||
				(start.Param("VALUE") == "DATE") != tc.wantStartDate {
				t.Errorf("DTSTART %+v (%v), want %q", start, hasStart, tc.wantStart)
			}
			if _, ok := TodoStart(task, todoNow); ok != hasStart {
				t.Errorf("TodoStart says %v, the object %v", ok, hasStart)
			}
		})
	}
}

func TestRenderTodoPriorityRoundTrips(t *testing.T) {
	want := map[core.Priority]string{
		core.PriorityHighest: "1", core.PriorityHigh: "3", core.PriorityNormal: "0",
		core.PriorityLow: "7", core.PriorityLowest: "9",
	}
	for p, written := range want {
		task := todoTask()
		task.Priority = p
		v := vtodo(t, Todo{Task: task, Now: todoNow})
		if got := value(v, "PRIORITY"); got != written {
			t.Errorf("priority %d written %q, want %q", p, got, written)
		}
		if back := PriorityFromTodo(TodoPriority(p)); back != p {
			t.Errorf("priority %d reads back as %d", p, back)
		}
	}
	read := map[int]core.Priority{
		0: core.PriorityNormal, 1: core.PriorityHighest, 2: core.PriorityHighest, 3: core.PriorityHigh,
		4: core.PriorityHigh, 5: core.PriorityNormal, 6: core.PriorityLow, 7: core.PriorityLow,
		8: core.PriorityLowest, 9: core.PriorityLowest,
	}
	for n, p := range read {
		if got := PriorityFromTodo(n); got != p {
			t.Errorf("PRIORITY:%d reads as %d, want %d", n, got, p)
		}
	}
}

func TestRenderTodoNeverWritesRecurrenceOrCancelled(t *testing.T) {
	task := todoTask()
	task.Recurrence = "every week"
	for _, c := range []core.StateCategory{core.CategoryCancelled, core.CategoryDone, core.CategoryTodo} {
		data := RenderTodo(Todo{Task: task, Category: c, Now: todoNow})
		for _, banned := range []string{"RRULE", "RDATE", "EXDATE", "STATUS:CANCELLED"} {
			if bytes.Contains(data, []byte(banned)) {
				t.Errorf("%s object carries %s", c, banned)
			}
		}
	}
}

func TestRenderTodoFoldsAndEscapesALongTitle(t *testing.T) {
	task := todoTask()
	task.Title = strings.Repeat("Ротация ключей, этап; ", 8) + `a\b`
	task.Tags = []string{"a,b", "c;d"}
	v := vtodo(t, Todo{Task: task, Now: todoNow})
	if got := Unescape(value(v, "SUMMARY")); got != task.Title {
		t.Errorf("SUMMARY reads back %q, want %q", got, task.Title)
	}
	if got := SplitText(value(v, "CATEGORIES")); !slices.Equal(got, task.Tags) {
		t.Errorf("CATEGORIES reads back %q", got)
	}
	if got := value(v, "UID"); got != UID(task.ID) {
		t.Errorf("UID %q", got)
	}
}

// The entity tag moves with every input that changes the rendering, and with
// nothing else: two calls on the same inputs agree, whatever map order.
func TestETagChangesWithItsInputsAndNothingElse(t *testing.T) {
	base := ETagInput{TaskID: "01JTASK", Version: 4, UpdatedAt: todoNow, ProjectKey: "infra",
		Categories: map[string]core.StateCategory{"todo": core.CategoryTodo, "done": core.CategoryDone},
		Origin:     "https://tix.example"}
	tag := ETag(base)
	if !strings.HasPrefix(tag, `"`) || !strings.HasSuffix(tag, `"`) || len(tag) != 34 {
		t.Fatalf("entity tag %s is not a strong quoted tag", tag)
	}
	same := base
	same.Categories = map[string]core.StateCategory{"done": core.CategoryDone, "todo": core.CategoryTodo}
	if ETag(same) != tag {
		t.Error("the same inputs gave another tag")
	}
	for name, change := range map[string]func(*ETagInput){
		"version":     func(in *ETagInput) { in.Version++ },
		"update time": func(in *ETagInput) { in.UpdatedAt = in.UpdatedAt.Add(time.Second) },
		"project key": func(in *ETagInput) { in.ProjectKey = "ops" },
		"category map": func(in *ETagInput) {
			in.Categories = map[string]core.StateCategory{"todo": core.CategoryTodo, "done": core.CategoryCancelled}
		},
		"origin": func(in *ETagInput) { in.Origin = "https://tasks.example" },
		"parent": func(in *ETagInput) { in.ParentUID = "task-x@tix" },
		"start":  func(in *ETagInput) { in.Start = todoNow.Add(time.Hour) },
	} {
		in := base
		in.Categories = map[string]core.StateCategory{"todo": core.CategoryTodo, "done": core.CategoryDone}
		change(&in)
		if ETag(in) == tag {
			t.Errorf("changing the %s left the tag alone", name)
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add(RenderTodo(Todo{Task: todoTask(), Now: todoNow}))
	f.Add([]byte("BEGIN:VCALENDAR\r\nX;A=\"b,c\";D=e,f:g\r\nEND:VCALENDAR\r\n"))
	f.Add([]byte("BEGIN:A\r\nBEGIN:B\r\nEND:A\r\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := Parse(data)
		if err == nil && c == nil {
			t.Fatal("no component and no error")
		}
	})
}
