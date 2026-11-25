// SPDX-License-Identifier: AGPL-3.0-or-later

package ical

import (
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// ReasonInvalidObject is the reason detail of every error ReadTodo returns,
// which CalDAV answers with CALDAV:valid-calendar-object-resource.
const ReasonInvalidObject = "invalid_calendar_object"

// Status values a VTODO carries.
const (
	StatusNeedsAction = "NEEDS-ACTION"
	StatusInProcess   = "IN-PROCESS"
	StatusCompleted   = "COMPLETED"
	StatusCancelled   = "CANCELLED"
)

// Due is a DUE or DTSTART as read: the instant, and whether it was a bare date.
type Due struct {
	At   time.Time
	Date bool
}

// TodoValues is what a client sent in one VTODO, in tix's terms. A nil
// pointer or a false Has field is a property the client left out, which the
// caller reads as "unchanged" for every field but DUE and the status.
type TodoValues struct {
	// UID is the object's identifier, required.
	UID string
	// Summary and Description are unescaped.
	Summary     *string
	Description *string
	// Status is STATUS upper-cased, empty when absent or not one of the four.
	Status string
	// Completed reports a COMPLETED property, whatever its value.
	Completed bool
	// PercentComplete is PERCENT-COMPLETE, -1 when absent or unreadable.
	PercentComplete int
	// Due and Start are nil when absent.
	Due   *Due
	Start *Due
	// Priority is PRIORITY, nil when absent or not 0 to 9.
	Priority *int
	// Categories are every CATEGORIES value, trimmed and without repeats,
	// and HasCategories whether any CATEGORIES property was sent.
	Categories    []string
	HasCategories bool
	// Kept are the content lines tix has no field for, verbatim, in the
	// order sent: properties first, then whole components such as VALARM.
	Kept []string
}

// mapped are the VTODO properties tix reads, or writes itself, or drops:
// none of them is kept for the client.
var mapped = map[string]bool{
	"UID": true, "DTSTAMP": true, "CREATED": true, "LAST-MODIFIED": true, "SEQUENCE": true,
	"SUMMARY": true, "DESCRIPTION": true, "STATUS": true, "COMPLETED": true, "DUE": true, "DTSTART": true,
	"DURATION": true, "PRIORITY": true, "CATEGORIES": true, "RELATED-TO": true, "URL": true,
	"X-TIX-REF": true, "X-TIX-STATE": true,
	"RRULE": true, "RDATE": true, "EXDATE": true, "EXRULE": true, "RECURRENCE-ID": true,
}

// invalidObject is an error ReadTodo returns.
func invalidObject(format string, args ...any) error {
	return core.Invalid(format, args...).WithDetail("reason", ReasonInvalidObject)
}

// ReadTodo reads a CalDAV object a client sent: one VCALENDAR holding
// exactly one VTODO, and any VTIMEZONE its dates name. zone is what a
// floating time is read in, and a TZID neither the zone database nor the
// object's VTIMEZONE explains.
func ReadTodo(data []byte, zone *time.Location) (*TodoValues, error) {
	cal, err := Parse(data)
	if err != nil {
		return nil, invalidObject("%s", err.Error())
	}
	if cal.Name != "VCALENDAR" {
		return nil, invalidObject("a calendar object is a VCALENDAR, not %s", cal.Name)
	}
	var todo *Component
	zones := map[string]*Component{}
	for _, c := range cal.Children {
		switch c.Name {
		case "VTODO":
			if todo != nil {
				return nil, invalidObject("a calendar object holds exactly one VTODO")
			}
			todo = c
		case "VTIMEZONE":
			if id, ok := c.Prop("TZID"); ok {
				zones[id.Value] = c
			}
		default:
			return nil, invalidObject("tix stores tasks, and a %s is not one", c.Name)
		}
	}
	if todo == nil {
		return nil, invalidObject("a calendar object holds exactly one VTODO")
	}
	if zone == nil {
		zone = time.UTC
	}
	r := dateReader{zones: zones, floating: zone}
	out := &TodoValues{PercentComplete: -1}
	for _, p := range todo.Properties {
		if err := out.read(p, r); err != nil {
			return nil, err
		}
	}
	if out.UID == "" {
		return nil, invalidObject("a VTODO needs a UID")
	}
	for _, c := range todo.Children {
		out.Kept = append(out.Kept, componentLines(c)...)
	}
	return out, nil
}

// read takes one VTODO property into the values.
func (v *TodoValues) read(p Property, r dateReader) error {
	switch p.Name {
	case "UID":
		v.UID = strings.TrimSpace(Unescape(p.Value))
	case "SUMMARY":
		s := Unescape(p.Value)
		v.Summary = &s
	case "DESCRIPTION":
		s := Unescape(p.Value)
		v.Description = &s
	case "STATUS":
		switch s := strings.ToUpper(strings.TrimSpace(p.Value)); s {
		case StatusNeedsAction, StatusInProcess, StatusCompleted, StatusCancelled:
			v.Status = s
		}
	case "COMPLETED":
		v.Completed = true
	case "PERCENT-COMPLETE":
		n, err := strconv.Atoi(strings.TrimSpace(p.Value))
		if err == nil && n >= 0 && n <= 100 {
			v.PercentComplete = n
		}
		if n != 100 {
			v.Kept = append(v.Kept, p.Line)
		}
	case "DUE", "DTSTART":
		d, err := r.read(p)
		if err != nil {
			return err
		}
		if p.Name == "DUE" {
			v.Due = &d
		} else {
			v.Start = &d
		}
	case "PRIORITY":
		if n, err := strconv.Atoi(strings.TrimSpace(p.Value)); err == nil && n >= 0 && n <= 9 {
			v.Priority = &n
		}
	case "CATEGORIES":
		v.HasCategories = true
		for _, c := range SplitText(p.Value) {
			if c = strings.TrimSpace(c); c != "" && !containsFold(v.Categories, c) {
				v.Categories = append(v.Categories, c)
			}
		}
	default:
		if !mapped[p.Name] {
			v.Kept = append(v.Kept, p.Line)
		}
	}
	return nil
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// componentLines writes a component back as the lines it was read from.
func componentLines(c *Component) []string {
	out := []string{"BEGIN:" + c.Name}
	for _, p := range c.Properties {
		out = append(out, p.Line)
	}
	for _, k := range c.Children {
		out = append(out, componentLines(k)...)
	}
	return append(out, "END:"+c.Name)
}

// Layouts of the date-times a client sends: UTC, and local to a zone or floating.
const localLayout = "20060102T150405"

// dateReader reads DUE and DTSTART against the object's time zones.
type dateReader struct {
	zones    map[string]*Component
	floating *time.Location
}

// read reads a DATE or DATE-TIME: a date is midnight UTC, a UTC time is
// itself, a TZID is the zone database's or else the object's VTIMEZONE's,
// and anything else is read in the floating zone.
func (r dateReader) read(p Property) (Due, error) {
	v := strings.TrimSpace(p.Value)
	if strings.EqualFold(p.Param("VALUE"), "DATE") || len(v) == len(dateLayout) {
		t, err := time.Parse(dateLayout, v)
		if err != nil {
			return Due{}, invalidObject("%s %q is not a date", p.Name, v)
		}
		return Due{At: t, Date: true}, nil
	}
	if strings.HasSuffix(v, "Z") {
		t, err := time.Parse(dateTimeLayout, v)
		if err != nil {
			return Due{}, invalidObject("%s %q is not a date and time", p.Name, v)
		}
		return Due{At: t}, nil
	}
	wall, err := time.ParseInLocation(localLayout, v, time.UTC)
	if err != nil {
		return Due{}, invalidObject("%s %q is not a date and time", p.Name, v)
	}
	tzid := strings.TrimPrefix(p.Param("TZID"), "/")
	if tzid == "" {
		return Due{At: inZone(wall, r.floating)}, nil
	}
	if loc, err := time.LoadLocation(tzid); err == nil {
		return Due{At: inZone(wall, loc)}, nil
	}
	if tz, ok := r.zones[p.Param("TZID")]; ok {
		if offset, ok := vtimezoneOffset(tz, wall); ok {
			return Due{At: wall.Add(-time.Duration(offset) * time.Second)}, nil
		}
	}
	return Due{At: inZone(wall, r.floating)}, nil
}

// inZone reads a wall clock time, held as UTC, as a time in loc.
func inZone(wall time.Time, loc *time.Location) time.Time {
	return time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), 0, loc).UTC()
}

// vtimezoneOffset is the UTC offset, in seconds, a VTIMEZONE gives a wall
// clock time: that of the observance whose latest onset is at or before it.
// An observance repeats yearly when its RRULE says FREQ=YEARLY with BYMONTH
// and BYDAY, which is how every client writes one; anything else is its
// DTSTART alone.
func vtimezoneOffset(tz *Component, wall time.Time) (int, bool) {
	var (
		best   time.Time
		offset int
		found  bool
	)
	for _, o := range tz.Children {
		if o.Name != "STANDARD" && o.Name != "DAYLIGHT" {
			continue
		}
		to, ok := o.Prop("TZOFFSETTO")
		start, okStart := o.Prop("DTSTART")
		if !ok || !okStart {
			continue
		}
		secs, ok := parseOffset(to.Value)
		if !ok {
			continue
		}
		first, err := time.ParseInLocation(localLayout, strings.TrimSpace(start.Value), time.UTC)
		if err != nil {
			continue
		}
		rule, _ := o.Prop("RRULE")
		for _, year := range []int{wall.Year(), wall.Year() - 1} {
			onset, ok := observanceOnset(first, rule.Value, year)
			if !ok || onset.After(wall) || onset.Before(first) {
				continue
			}
			if !found || onset.After(best) {
				best, offset, found = onset, secs, true
			}
			break
		}
	}
	return offset, found
}

// observanceOnset is when an observance starts in a year.
func observanceOnset(first time.Time, rule string, year int) (time.Time, bool) {
	at := func(month time.Month, day int) time.Time {
		return time.Date(year, month, day, first.Hour(), first.Minute(), first.Second(), 0, time.UTC)
	}
	if rule == "" {
		return first, first.Year() == year
	}
	parts := map[string]string{}
	for _, kv := range strings.Split(strings.ToUpper(rule), ";") {
		k, v, _ := strings.Cut(kv, "=")
		parts[k] = v
	}
	if parts["FREQ"] != "YEARLY" {
		return time.Time{}, false
	}
	month := first.Month()
	if m, err := strconv.Atoi(parts["BYMONTH"]); err == nil && m >= 1 && m <= 12 {
		month = time.Month(m)
	}
	byday := parts["BYDAY"]
	if byday == "" {
		return at(month, first.Day()), true
	}
	if len(byday) < 3 {
		return time.Time{}, false
	}
	weekday, ok := weekdays[byday[len(byday)-2:]]
	n, err := strconv.Atoi(byday[:len(byday)-2])
	if !ok || err != nil || n == 0 {
		return time.Time{}, false
	}
	if n > 0 {
		d := at(month, 1)
		d = d.AddDate(0, 0, (int(weekday)-int(d.Weekday())+7)%7+7*(n-1))
		return d, d.Month() == month
	}
	d := at(month+1, 1).AddDate(0, 0, -1)
	d = d.AddDate(0, 0, -((int(d.Weekday())-int(weekday)+7)%7 + 7*(-n-1)))
	return d, d.Month() == month
}

var weekdays = map[string]time.Weekday{"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday,
	"WE": time.Wednesday, "TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday}

// parseOffset reads a UTC-OFFSET such as +0200, -0500 or +053000.
func parseOffset(v string) (int, bool) {
	v = strings.TrimSpace(v)
	if len(v) != 5 && len(v) != 7 || v[0] != '+' && v[0] != '-' {
		return 0, false
	}
	h, errH := strconv.Atoi(v[1:3])
	m, errM := strconv.Atoi(v[3:5])
	s := 0
	var errS error
	if len(v) == 7 {
		s, errS = strconv.Atoi(v[5:7])
	}
	if errH != nil || errM != nil || errS != nil {
		return 0, false
	}
	secs := h*3600 + m*60 + s
	if v[0] == '-' {
		secs = -secs
	}
	return secs, true
}
