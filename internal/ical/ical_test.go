// SPDX-License-Identifier: AGPL-3.0-or-later

package ical

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/core"
)

// component is one parsed BEGIN/END block: its properties by name, each with
// its parameters and unescaped-on-demand raw value, and its children.
type component struct {
	name     string
	props    map[string][]property
	children []*component
}

type property struct {
	params string
	value  string
}

// one returns the single value of a property, failing when there is not
// exactly one.
func (c *component) one(t *testing.T, name string) property {
	t.Helper()
	got := c.props[name]
	if len(got) != 1 {
		t.Fatalf("%s carries %d %s properties, want 1", c.name, len(got), name)
	}
	return got[0]
}

// parse is a minimal RFC 5545 reader: it checks the physical line rules,
// unfolds, and builds the component tree. Anything it cannot read is a
// failure, so a document that passes is one a strict client accepts.
func parse(t *testing.T, doc []byte) *component {
	t.Helper()
	if !bytes.HasSuffix(doc, []byte("\r\n")) {
		t.Fatal("document does not end with CRLF")
	}
	physical := strings.Split(strings.TrimSuffix(string(doc), "\r\n"), "\r\n")
	var logical []string
	for i, line := range physical {
		if strings.ContainsAny(line, "\r\n") {
			t.Fatalf("line %d holds a bare CR or LF: %q", i+1, line)
		}
		if len(line) > maxLineOctets {
			t.Fatalf("line %d is %d octets, over %d: %q", i+1, len(line), maxLineOctets, line)
		}
		if !utf8.ValidString(line) {
			t.Fatalf("line %d splits a UTF-8 sequence: %q", i+1, line)
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if len(logical) == 0 {
				t.Fatal("document starts with a continuation line")
			}
			logical[len(logical)-1] += line[1:]
			continue
		}
		logical = append(logical, line)
	}
	root := &component{name: "root", props: map[string][]property{}}
	stack := []*component{root}
	for _, line := range logical {
		head, value, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("content line has no colon: %q", line)
		}
		name, params, _ := strings.Cut(head, ";")
		top := stack[len(stack)-1]
		switch name {
		case "BEGIN":
			c := &component{name: value, props: map[string][]property{}}
			top.children = append(top.children, c)
			stack = append(stack, c)
		case "END":
			if top.name != value {
				t.Fatalf("END:%s closes %s", value, top.name)
			}
			stack = stack[:len(stack)-1]
		default:
			top.props[name] = append(top.props[name], property{params: params, value: value})
		}
	}
	if len(stack) != 1 || len(root.children) != 1 || root.children[0].name != "VCALENDAR" {
		t.Fatalf("document is not exactly one closed VCALENDAR")
	}
	return root.children[0]
}

// unescape reverses TEXT escaping, refusing an escape RFC 5545 does not define.
func unescape(t *testing.T, s string) string {
	t.Helper()
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			if s[i] == ',' || s[i] == ';' {
				t.Fatalf("unescaped %q in TEXT value %q", s[i], s)
			}
			b.WriteByte(s[i])
			continue
		}
		i++
		if i == len(s) {
			t.Fatalf("TEXT value %q ends in a lone backslash", s)
		}
		switch s[i] {
		case '\\', ';', ',':
			b.WriteByte(s[i])
		case 'n', 'N':
			b.WriteByte('\n')
		default:
			t.Fatalf("TEXT value %q holds the undefined escape \\%c", s, s[i])
		}
	}
	return b.String()
}

func day(y int, m time.Month, d, h int) *time.Time {
	v := time.Date(y, m, d, h, 0, 0, 0, time.UTC)
	return &v
}

func entries() core.CalendarEntries {
	updated := time.Date(2026, 10, 30, 9, 15, 0, 0, time.UTC)
	return core.CalendarEntries{Entries: []core.CalendarEntry{
		{Task: core.Task{
			ID: "01JABCDEFGHJKMNPQRSTVWXYZ0", Ref: "infra-12",
			Title:   `Ünïcödé, semi; back\slash and a` + "\nnewline " + strings.Repeat("ж", 60),
			DueAt:   day(2026, 11, 5, 0),
			Version: 4, Priority: core.PriorityHigh, Tags: []string{"ops", "a,b"},
			CreatedAt: updated.AddDate(0, 0, -3), UpdatedAt: updated,
		}, Category: core.CategoryInProgress, State: "Doing"},
		{Task: core.Task{
			ID: "01JABCDEFGHJKMNPQRSTVWXYZ1", Ref: "infra-13", Title: "Shipped",
			DueAt: day(2026, 12, 31, 23), Version: 1, Priority: core.PriorityNormal,
			CreatedAt: updated, UpdatedAt: updated,
		}, Category: core.CategoryDone, State: "Done"},
		{Task: core.Task{
			ID: "01JABCDEFGHJKMNPQRSTVWXYZ2", Ref: "infra-14", Title: "Dropped",
			DueAt: day(2026, 11, 1, 12), Version: 2, Priority: core.PriorityLow,
			CreatedAt: updated, UpdatedAt: updated,
		}, Category: core.CategoryCancelled, State: "Cancelled"},
	}}
}

func render() []byte {
	return Render(entries(), Options{
		Name:    "tix: due dates, alice",
		TaskURL: func(ref string) string { return "https://tix.example/tasks/" + ref },
	})
}

func TestRenderRoundTripsThroughAStrictParser(t *testing.T) {
	cal := parse(t, render())
	if got := cal.one(t, "VERSION").value; got != "2.0" {
		t.Fatalf("VERSION = %q", got)
	}
	if got := unescape(t, cal.one(t, "X-WR-CALNAME").value); got != "tix: due dates, alice" {
		t.Fatalf("calendar name = %q", got)
	}
	if len(cal.children) != 3 {
		t.Fatalf("calendar holds %d components, want 3 events", len(cal.children))
	}
	in := entries().Entries
	for i, ev := range cal.children {
		task := in[i].Task
		if ev.name != "VEVENT" {
			t.Fatalf("component %d is %s, want VEVENT", i, ev.name)
		}
		if got := unescape(t, ev.one(t, "UID").value); got != UID(task.ID) {
			t.Errorf("%s UID = %q", task.Ref, got)
		}
		for _, name := range []string{"DTSTAMP", "LAST-MODIFIED"} {
			if got := ev.one(t, name).value; got != "20261030T091500Z" {
				t.Errorf("%s %s = %q", task.Ref, name, got)
			}
		}
		start, end := ev.one(t, "DTSTART"), ev.one(t, "DTEND")
		if start.params != "VALUE=DATE" || end.params != "VALUE=DATE" {
			t.Errorf("%s dates are not all-day: %q %q", task.Ref, start.params, end.params)
		}
		s, err := time.Parse("20060102", start.value)
		if err != nil {
			t.Fatalf("%s DTSTART %q: %v", task.Ref, start.value, err)
		}
		e, err := time.Parse("20060102", end.value)
		if err != nil || !e.Equal(s.AddDate(0, 0, 1)) {
			t.Errorf("%s DTEND %q is not the day after DTSTART %q", task.Ref, end.value, start.value)
		}
		if want := task.DueAt.UTC().Format("20060102"); start.value != want {
			t.Errorf("%s DTSTART = %q, want the UTC due date %q", task.Ref, start.value, want)
		}
		if got := ev.one(t, "URL").value; got != "https://tix.example/tasks/"+task.Ref {
			t.Errorf("%s URL = %q", task.Ref, got)
		}
		if got := unescape(t, ev.one(t, "DESCRIPTION").value); !strings.HasSuffix(got, "\nhttps://tix.example/tasks/"+task.Ref) {
			t.Errorf("%s DESCRIPTION does not end with the link: %q", task.Ref, got)
		}
	}
}

func TestSummaryRoundTripsEveryAwkwardCharacter(t *testing.T) {
	ev := parse(t, render()).children[0]
	task := entries().Entries[0].Task
	if got := unescape(t, ev.one(t, "SUMMARY").value); got != task.Ref+": "+task.Title {
		t.Fatalf("SUMMARY round-tripped as %q, want %q", got, task.Ref+": "+task.Title)
	}
	if got := ev.one(t, "CATEGORIES").value; got != `ops,a\,b` {
		t.Fatalf("CATEGORIES = %q", got)
	}
	if got := ev.one(t, "SEQUENCE").value; got != "4" {
		t.Fatalf("SEQUENCE = %q", got)
	}
}

func TestStateMapsToStatusAndSummary(t *testing.T) {
	cal := parse(t, render())
	cases := []struct {
		status, summaryPrefix string
	}{
		{"CONFIRMED", "infra-12: "},
		{"CONFIRMED", DoneMark + "infra-13: "},
		{"CANCELLED", "infra-14: "},
	}
	for i, tc := range cases {
		ev := cal.children[i]
		if got := ev.one(t, "STATUS").value; got != tc.status {
			t.Errorf("event %d STATUS = %q, want %q", i, got, tc.status)
		}
		if got := unescape(t, ev.one(t, "SUMMARY").value); !strings.HasPrefix(got, tc.summaryPrefix) {
			t.Errorf("event %d SUMMARY = %q, want prefix %q", i, got, tc.summaryPrefix)
		}
		if got := ev.one(t, "TRANSP").value; got != "TRANSPARENT" {
			t.Errorf("event %d TRANSP = %q", i, got)
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	if !bytes.Equal(render(), render()) {
		t.Fatal("rendering the same entries twice gave different bytes, so the ETag would churn")
	}
}

func TestFoldNeverSplitsACharacter(t *testing.T) {
	for _, line := range []string{
		"SUMMARY:" + strings.Repeat("a", 200),
		"SUMMARY:" + strings.Repeat("ж", 100),
		"SUMMARY:" + strings.Repeat("😀", 50),
		"SUMMARY:a" + strings.Repeat("€", 70),
	} {
		folded := Fold(line)
		parts := strings.Split(strings.TrimSuffix(folded, "\r\n"), "\r\n")
		var joined strings.Builder
		for i, p := range parts {
			if len(p) > maxLineOctets || !utf8.ValidString(p) {
				t.Fatalf("part %d of %q is %d octets or splits a character", i, line[:20], len(p))
			}
			if i > 0 {
				if !strings.HasPrefix(p, " ") {
					t.Fatalf("continuation %d does not start with a space", i)
				}
				p = p[1:]
			}
			joined.WriteString(p)
		}
		if joined.String() != line {
			t.Fatalf("unfolding did not give the line back")
		}
	}
}

func TestEmptyFeedIsAValidCalendar(t *testing.T) {
	cal := parse(t, Render(core.CalendarEntries{}, Options{Name: "x"}))
	if len(cal.children) != 0 {
		t.Fatalf("an empty feed drew %d components", len(cal.children))
	}
}
