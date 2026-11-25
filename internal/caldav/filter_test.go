// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/heliopsy/tix/internal/ical"
)

func vtodo(lines ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:task-1@tix\r\n" + strings.Join(lines, "\r\n") +
		"\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
}

var (
	openTodo = vtodo("CREATED:20261101T000000Z", "DTSTART:20261104T090000Z", "DUE:20261105T143000Z",
		"SUMMARY:Rotate Keys", "CATEGORIES:ops,security", "RELATED-TO;RELTYPE=PARENT:task-0@tix", "STATUS:NEEDS-ACTION")
	dueOnly   = vtodo("CREATED:20261101T000000Z", "DUE;VALUE=DATE:20261105", "SUMMARY:Pay rent")
	doneTodo  = vtodo("CREATED:20261101T000000Z", "COMPLETED:20261110T091500Z", "STATUS:COMPLETED", "SUMMARY:Done")
	bareTodo  = vtodo("CREATED:20261101T000000Z", "SUMMARY:Someday")
	startDur  = vtodo("DTSTART:20261104T090000Z", "DURATION:PT2H", "SUMMARY:Call")
	startOnly = vtodo("DTSTART:20261104T090000Z", "SUMMARY:Start")
)

func todoFilter(inner string) string {
	return `<c:comp-filter name="VCALENDAR"><c:comp-filter name="VTODO">` + inner + `</c:comp-filter></c:comp-filter>`
}

func evalFilter(t *testing.T, inner, object string) (bool, error) {
	t.Helper()
	root, err := decodeBody([]byte(`<c:filter xmlns:c="urn:ietf:params:xml:ns:caldav">` + inner + `</c:filter>`))
	if err != nil {
		t.Fatalf("decoding the filter: %v", err)
	}
	f, err := parseFilter(root)
	if err != nil {
		return false, err
	}
	cal, err := ical.Parse([]byte(object))
	if err != nil {
		t.Fatalf("parsing the object: %v", err)
	}
	return f.matches(cal), nil
}

func TestCalendarQueryFilters(t *testing.T) {
	tr := func(start, end string) string {
		out := `<c:time-range`
		if start != "" {
			out += ` start="` + start + `"`
		}
		if end != "" {
			out += ` end="` + end + `"`
		}
		return out + `/>`
	}
	tests := []struct {
		name   string
		filter string
		object string
		want   bool
	}{
		{"any VTODO", todoFilter(""), openTodo, true},
		{"no VEVENT", `<c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"/></c:comp-filter>`, openTodo, false},
		{"VEVENT is not defined", `<c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"><c:is-not-defined/>` +
			`</c:comp-filter></c:comp-filter>`, openTodo, true},
		{"VTODO is not defined", todoFilter(`<c:is-not-defined/>`), openTodo, false},
		{"completed is not defined, open", todoFilter(`<c:prop-filter name="COMPLETED"><c:is-not-defined/></c:prop-filter>`), openTodo, true},
		{"completed is not defined, done", todoFilter(`<c:prop-filter name="COMPLETED"><c:is-not-defined/></c:prop-filter>`), doneTodo, false},
		{"status defined", todoFilter(`<c:prop-filter name="STATUS"/>`), bareTodo, false},
		{"casemap text", todoFilter(`<c:prop-filter name="SUMMARY"><c:text-match>rotate keys</c:text-match></c:prop-filter>`), openTodo, true},
		{"default collation folds ASCII", todoFilter(`<c:prop-filter name="SUMMARY"><c:text-match collation="i;ascii-casemap">KEYS</c:text-match></c:prop-filter>`), openTodo, true},
		{"octet text is exact", todoFilter(`<c:prop-filter name="SUMMARY"><c:text-match collation="i;octet">rotate</c:text-match></c:prop-filter>`), openTodo, false},
		{"octet text matches its case", todoFilter(`<c:prop-filter name="SUMMARY"><c:text-match collation="i;octet">Rotate</c:text-match></c:prop-filter>`), openTodo, true},
		{"negated text", todoFilter(`<c:prop-filter name="STATUS"><c:text-match negate-condition="yes">COMPLETED</c:text-match></c:prop-filter>`), openTodo, true},
		{"negated text, done", todoFilter(`<c:prop-filter name="STATUS"><c:text-match negate-condition="yes">COMPLETED</c:text-match></c:prop-filter>`), doneTodo, false},
		{"category text", todoFilter(`<c:prop-filter name="CATEGORIES"><c:text-match>security</c:text-match></c:prop-filter>`), openTodo, true},
		{"param text", todoFilter(`<c:prop-filter name="RELATED-TO"><c:param-filter name="RELTYPE"><c:text-match>parent</c:text-match></c:param-filter></c:prop-filter>`), openTodo, true},
		{"param text mismatch", todoFilter(`<c:prop-filter name="RELATED-TO"><c:param-filter name="RELTYPE"><c:text-match>child</c:text-match></c:param-filter></c:prop-filter>`), openTodo, false},
		{"param not defined", todoFilter(`<c:prop-filter name="RELATED-TO"><c:param-filter name="RELTYPE"><c:is-not-defined/></c:param-filter></c:prop-filter>`), openTodo, false},
		{"param missing", todoFilter(`<c:prop-filter name="DUE"><c:param-filter name="TZID"/></c:prop-filter>`), openTodo, false},
		{"prop time range", todoFilter(`<c:prop-filter name="DUE">` + tr("20261105T000000Z", "20261106T000000Z") + `</c:prop-filter>`), openTodo, true},
		{"prop time range misses", todoFilter(`<c:prop-filter name="DUE">` + tr("20261106T000000Z", "") + `</c:prop-filter>`), openTodo, false},
		{"start and due, overlapping", todoFilter(tr("20261105T000000Z", "20261106T000000Z")), openTodo, true},
		{"start and due, before", todoFilter(tr("20261001T000000Z", "20261104T090000Z")), openTodo, false},
		{"start and due, after", todoFilter(tr("20261106T000000Z", "")), openTodo, false},
		{"due only, inside", todoFilter(tr("20261104T000000Z", "20261105T000000Z")), dueOnly, true},
		{"due only, end before due", todoFilter(tr("20261101T000000Z", "20261104T235959Z")), dueOnly, false},
		{"due only, start at due", todoFilter(tr("20261105T000000Z", "")), dueOnly, false},
		{"start and duration, inside", todoFilter(tr("20261104T100000Z", "20261104T103000Z")), startDur, true},
		{"start and duration, after", todoFilter(tr("20261104T110001Z", "")), startDur, false},
		{"start only, inside", todoFilter(tr("20261104T000000Z", "20261105T000000Z")), startOnly, true},
		{"start only, end at start", todoFilter(tr("", "20261104T090000Z")), startOnly, false},
		{"completed and created, inside", todoFilter(tr("20261109T000000Z", "20261111T000000Z")), doneTodo, true},
		{"completed and created, before both", todoFilter(tr("20261001T000000Z", "20261002T000000Z")), doneTodo, false},
		{"created only, end after", todoFilter(tr("20261001T000000Z", "20261102T000000Z")), bareTodo, true},
		{"created only, end before", todoFilter(tr("20261001T000000Z", "20261101T000000Z")), bareTodo, false},
		{"a zoned time is read in its zone", todoFilter(`<c:prop-filter name="DTSTART">` + tr("20261104T085900Z", "20261104T090100Z") + `</c:prop-filter>`),
			vtodo("DTSTART;TZID=Europe/Sofia:20261104T110000"), true},
		{"a floating time is read as UTC", todoFilter(`<c:prop-filter name="DTSTART">` + tr("20261104T085900Z", "20261104T090100Z") + `</c:prop-filter>`),
			vtodo("DTSTART:20261104T090000"), true},
		{"a date is its UTC midnight", todoFilter(`<c:prop-filter name="DUE">` + tr("20261105T000000Z", "20261105T000001Z") + `</c:prop-filter>`), dueOnly, true},
		{"an unreadable time matches no range", todoFilter(`<c:prop-filter name="DUE">` + tr("20201105T000000Z", "") + `</c:prop-filter>`),
			vtodo("DUE:soon"), false},
		{"nothing to place, always", todoFilter(tr("20201001T000000Z", "20201002T000000Z")), vtodo("SUMMARY:x"), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := evalFilter(t, tc.filter, tc.object)
			if err != nil || got != tc.want {
				t.Fatalf("matches = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestCalendarQueryRefusesWhatItCannotEvaluate(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		want   string
	}{
		{"an unknown collation", todoFilter(`<c:prop-filter name="SUMMARY"><c:text-match collation="i;unicode-casemap">x</c:text-match></c:prop-filter>`), "supported-collation"},
		{"a param collation", todoFilter(`<c:prop-filter name="RELATED-TO"><c:param-filter name="RELTYPE"><c:text-match collation="x">p</c:text-match></c:param-filter></c:prop-filter>`), "supported-collation"},
		{"no VCALENDAR", `<c:comp-filter name="VTODO"/>`, "valid-filter"},
		{"two roots", `<c:comp-filter name="VCALENDAR"/><c:comp-filter name="VCALENDAR"/>`, "valid-filter"},
		{"a nameless comp-filter", `<c:comp-filter/>`, "valid-filter"},
		{"a bad time", todoFilter(`<c:time-range start="tomorrow"/>`), "valid-filter"},
		{"an empty time range", todoFilter(`<c:time-range/>`), "valid-filter"},
		{"a time range on VCALENDAR", `<c:comp-filter name="VCALENDAR"><c:time-range start="20260101T000000Z"/></c:comp-filter>`, "valid-filter"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evalFilter(t, tc.filter, openTodo)
			var de *davError
			if !errors.As(err, &de) || de.status != http.StatusForbidden || !strings.Contains(string(de.body), "<c:"+tc.want+"/>") {
				t.Fatalf("got %v, want 403 with %s", err, tc.want)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"PT15M", 15 * time.Minute, true},
		{"P1W", 7 * 24 * time.Hour, true},
		{"-P1DT2H", -26 * time.Hour, true},
		{"+PT1H30M5S", time.Hour + 30*time.Minute + 5*time.Second, true},
		{"P", 0, false},
		{"PT", 0, false},
		{"1H", 0, false},
		{"P1H", 0, false},
		{"PT1", 0, false},
		{"PTT1H", 0, false},
	}
	for _, tc := range tests {
		got, ok := parseDuration(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseDuration(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
