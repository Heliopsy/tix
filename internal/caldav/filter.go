// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/ical"
)

// The collations a text-match may name, RFC 4790's two that RFC 4791
// requires.
const (
	collationCasemap = "i;ascii-casemap"
	collationOctet   = "i;octet"
)

// compFilter is a CALDAV:comp-filter.
type compFilter struct {
	name       string
	notDefined bool
	timeRange  *timeRange
	props      []propFilter
	comps      []compFilter
}

// propFilter is a CALDAV:prop-filter.
type propFilter struct {
	name       string
	notDefined bool
	timeRange  *timeRange
	text       *textMatch
	params     []paramFilter
}

// paramFilter is a CALDAV:param-filter.
type paramFilter struct {
	name       string
	notDefined bool
	text       *textMatch
}

// textMatch is a CALDAV:text-match.
type textMatch struct {
	value   string
	casemap bool
	negate  bool
}

// timeRange is a CALDAV:time-range; a missing bound is unbounded.
type timeRange struct {
	start, end time.Time
}

// filter is a calendar-query's filter; a nil root matches every object.
type filter struct {
	root *compFilter
}

// Bounds standing in for a missing time-range start or end.
var (
	minTime = time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	maxTime = time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
)

// invalidFilter is the 403 a filter RFC 4791 does not allow answers.
func invalidFilter() error {
	return precondition(http.StatusForbidden, calName("valid-filter"))
}

// parseFilter reads a CALDAV:filter: one comp-filter for VCALENDAR.
func parseFilter(e *element) (filter, error) {
	if e == nil {
		return filter{}, nil
	}
	comps := e.all(nsCalDAV, "comp-filter")
	if len(comps) != 1 {
		return filter{}, invalidFilter()
	}
	root, err := parseCompFilter(comps[0])
	if err != nil {
		return filter{}, err
	}
	if !strings.EqualFold(root.name, "VCALENDAR") || root.timeRange != nil {
		return filter{}, invalidFilter()
	}
	return filter{root: &root}, nil
}

func parseCompFilter(e *element) (compFilter, error) {
	name, ok := e.attr("name")
	if !ok || name == "" {
		return compFilter{}, invalidFilter()
	}
	f := compFilter{name: strings.ToUpper(name), notDefined: e.child(nsCalDAV, "is-not-defined") != nil}
	var err error
	if tr := e.child(nsCalDAV, "time-range"); tr != nil {
		if f.timeRange, err = parseTimeRange(tr); err != nil {
			return f, err
		}
	}
	for _, c := range e.all(nsCalDAV, "prop-filter") {
		p, err := parsePropFilter(c)
		if err != nil {
			return f, err
		}
		f.props = append(f.props, p)
	}
	for _, c := range e.all(nsCalDAV, "comp-filter") {
		k, err := parseCompFilter(c)
		if err != nil {
			return f, err
		}
		f.comps = append(f.comps, k)
	}
	return f, nil
}

func parsePropFilter(e *element) (propFilter, error) {
	name, ok := e.attr("name")
	if !ok || name == "" {
		return propFilter{}, invalidFilter()
	}
	f := propFilter{name: strings.ToUpper(name), notDefined: e.child(nsCalDAV, "is-not-defined") != nil}
	var err error
	if tr := e.child(nsCalDAV, "time-range"); tr != nil {
		if f.timeRange, err = parseTimeRange(tr); err != nil {
			return f, err
		}
	}
	if tm := e.child(nsCalDAV, "text-match"); tm != nil {
		if f.text, err = parseTextMatch(tm); err != nil {
			return f, err
		}
	}
	for _, c := range e.all(nsCalDAV, "param-filter") {
		pname, ok := c.attr("name")
		if !ok || pname == "" {
			return f, invalidFilter()
		}
		pf := paramFilter{name: strings.ToUpper(pname), notDefined: c.child(nsCalDAV, "is-not-defined") != nil}
		if tm := c.child(nsCalDAV, "text-match"); tm != nil {
			if pf.text, err = parseTextMatch(tm); err != nil {
				return f, err
			}
		}
		f.params = append(f.params, pf)
	}
	return f, nil
}

func parseTextMatch(e *element) (*textMatch, error) {
	collation, ok := e.attr("collation")
	if !ok {
		collation = collationCasemap
	}
	if collation != collationCasemap && collation != collationOctet {
		return nil, precondition(http.StatusForbidden, calName("supported-collation"))
	}
	negate, _ := e.attr("negate-condition")
	return &textMatch{value: e.text, casemap: collation == collationCasemap, negate: negate == "yes"}, nil
}

func parseTimeRange(e *element) (*timeRange, error) {
	tr := &timeRange{start: minTime, end: maxTime}
	start, hasStart := e.attr("start")
	end, hasEnd := e.attr("end")
	if !hasStart && !hasEnd {
		return nil, invalidFilter()
	}
	var err error
	if hasStart {
		if tr.start, err = time.Parse("20060102T150405Z", start); err != nil {
			return nil, invalidFilter()
		}
	}
	if hasEnd {
		if tr.end, err = time.Parse("20060102T150405Z", end); err != nil {
			return nil, invalidFilter()
		}
	}
	return tr, nil
}

// matches evaluates the filter on one VCALENDAR.
func (f filter) matches(cal *ical.Component) bool {
	if f.root == nil {
		return true
	}
	if f.root.notDefined || !strings.EqualFold(cal.Name, f.root.name) {
		return false
	}
	return f.root.matchesSelf(cal)
}

// matchesSelf is whether a component of the filter's name satisfies the rest
// of the filter.
func (f compFilter) matchesSelf(c *ical.Component) bool {
	if f.timeRange != nil && !componentInRange(c, *f.timeRange) {
		return false
	}
	for _, p := range f.props {
		if !p.matches(c) {
			return false
		}
	}
	for _, k := range f.comps {
		if !k.matchesIn(c) {
			return false
		}
	}
	return true
}

// matchesIn is whether a parent component's children satisfy the filter: one
// of them does, or none exists for is-not-defined.
func (f compFilter) matchesIn(parent *ical.Component) bool {
	children := parent.Components(f.name)
	if f.notDefined {
		return len(children) == 0
	}
	for _, c := range children {
		if f.matchesSelf(c) {
			return true
		}
	}
	return false
}

// matches is whether a component has a property satisfying the filter, or
// none for is-not-defined.
func (f propFilter) matches(c *ical.Component) bool {
	props := c.Props(f.name)
	if f.notDefined {
		return len(props) == 0
	}
	for _, p := range props {
		if f.matchesProp(p) {
			return true
		}
	}
	return false
}

func (f propFilter) matchesProp(p ical.Property) bool {
	if f.timeRange != nil {
		at, ok := propTime(p)
		if !ok || at.Before(f.timeRange.start) || !f.timeRange.end.After(at) {
			return false
		}
	}
	if f.text != nil && !f.text.matches(ical.Unescape(p.Value)) {
		return false
	}
	for _, pf := range f.params {
		values, ok := p.Params[pf.name]
		if pf.notDefined {
			if ok {
				return false
			}
			continue
		}
		if !ok {
			return false
		}
		if pf.text != nil && !anyMatches(pf.text, values) {
			return false
		}
	}
	return true
}

func anyMatches(t *textMatch, values []string) bool {
	for _, v := range values {
		if t.matches(v) {
			return true
		}
	}
	return false
}

// matches is a substring test under the collation, inverted by
// negate-condition.
func (t *textMatch) matches(v string) bool {
	needle := t.value
	if t.casemap {
		v, needle = asciiLower(v), asciiLower(needle)
	}
	return strings.Contains(v, needle) != t.negate
}

// asciiLower folds ASCII letters alone, as i;ascii-casemap does.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// propTime reads a DATE or DATE-TIME value. A date is its UTC midnight; a
// local time is read in its TZID when that names a zone, in UTC otherwise.
func propTime(p ical.Property) (time.Time, bool) {
	v := strings.TrimSpace(p.Value)
	if strings.EqualFold(p.Param("VALUE"), "DATE") || len(v) == len("20060102") {
		t, err := time.Parse("20060102", v)
		return t, err == nil
	}
	if strings.HasSuffix(v, "Z") {
		t, err := time.Parse("20060102T150405Z", v)
		return t, err == nil
	}
	loc := time.UTC
	if tz := p.Param("TZID"); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	t, err := time.ParseInLocation("20060102T150405", v, loc)
	return t.UTC(), err == nil
}

// componentInRange is RFC 4791 section 9.9 for a VTODO. A time-range on any
// other component tix never holds matches nothing.
func componentInRange(c *ical.Component, tr timeRange) bool {
	if !strings.EqualFold(c.Name, "VTODO") {
		return false
	}
	at := func(name string) (time.Time, bool) {
		p, ok := c.Prop(name)
		if !ok {
			return time.Time{}, false
		}
		return propTime(p)
	}
	start, end := tr.start, tr.end
	dtstart, hasStart := at("DTSTART")
	due, hasDue := at("DUE")
	completed, hasCompleted := at("COMPLETED")
	created, hasCreated := at("CREATED")
	var (
		dur    time.Duration
		hasDur bool
	)
	if p, ok := c.Prop("DURATION"); ok {
		dur, hasDur = parseDuration(p.Value)
	}
	le := func(a, b time.Time) bool { return !a.After(b) }
	switch {
	case hasStart && hasDue:
		return (start.Before(due) || le(start, dtstart)) && (end.After(dtstart) || !end.Before(due))
	case hasStart && hasDur:
		finish := dtstart.Add(dur)
		return le(start, finish) && (end.After(dtstart) || !end.Before(finish))
	case hasStart:
		return le(start, dtstart) && end.After(dtstart)
	case hasDue:
		return start.Before(due) && !end.Before(due)
	case hasCompleted && hasCreated:
		return (le(start, created) || le(start, completed)) && (!end.Before(created) || !end.Before(completed))
	case hasCompleted:
		return le(start, completed) && !end.Before(completed)
	case hasCreated:
		return end.After(created)
	}
	return true
}

// parseDuration reads an RFC 5545 DURATION such as P1W, PT15M or -P1DT2H.
func parseDuration(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	sign := time.Duration(1)
	switch {
	case strings.HasPrefix(v, "-"):
		sign, v = -1, v[1:]
	case strings.HasPrefix(v, "+"):
		v = v[1:]
	}
	v, ok := strings.CutPrefix(v, "P")
	if !ok || v == "" || strings.HasSuffix(v, "T") {
		return 0, false
	}
	units := map[byte]time.Duration{'W': 7 * 24 * time.Hour, 'D': 24 * time.Hour}
	timeUnits := map[byte]time.Duration{'H': time.Hour, 'M': time.Minute, 'S': time.Second}
	var total time.Duration
	inTime := false
	num := ""
	for i := 0; i < len(v); i++ {
		ch := v[i]
		switch {
		case ch == 'T':
			if inTime || num != "" {
				return 0, false
			}
			inTime = true
		case '0' <= ch && ch <= '9':
			num += string(ch)
		default:
			unit, ok := units[ch]
			if inTime {
				unit, ok = timeUnits[ch]
			}
			n, err := strconv.Atoi(num)
			if !ok || err != nil {
				return 0, false
			}
			total += time.Duration(n) * unit
			num = ""
		}
	}
	if num != "" {
		return 0, false
	}
	return sign * total, true
}
