// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RecurrenceUnit is the calendar step a recurrence rule counts in.
type RecurrenceUnit string

// Recurrence units.
const (
	UnitDay   RecurrenceUnit = "day"
	UnitWeek  RecurrenceUnit = "week"
	UnitMonth RecurrenceUnit = "month"
	UnitYear  RecurrenceUnit = "year"
)

// MaxRecurrenceNumber bounds every number a rule carries: the interval and
// the occurrence count. A rule beyond it is a typing mistake, not a plan.
const MaxRecurrenceNumber = 1000

// RecurrenceDateLayout is the spelling of a rule's until date.
const RecurrenceDateLayout = "2006-01-02"

// Recurrence is a parsed repeat rule. A task stores its canonical String, not
// this value, so the text a person typed and the text every surface shows are
// the same thing.
type Recurrence struct {
	Interval int
	Unit     RecurrenceUnit
	// Weekdays, in Monday-first order, select days within each counted week.
	// Only a weekly rule carries them.
	Weekdays []time.Weekday
	// MonthDay pins a monthly rule to one day of the month, clamped to the end
	// of a shorter month. Zero takes the day from the date counted from.
	MonthDay int
	// AfterDone counts from the completion date rather than the due date.
	AfterDone bool
	// Until is the last calendar date an occurrence may fall on.
	Until *time.Time
	// Count is how many occurrences are left including the current one, or
	// zero for a rule with no count.
	Count int
}

var shorthands = map[string]RecurrenceUnit{
	"daily": UnitDay, "weekly": UnitWeek, "monthly": UnitMonth, "yearly": UnitYear,
}

var unitWords = map[string]RecurrenceUnit{
	"day": UnitDay, "days": UnitDay,
	"week": UnitWeek, "weeks": UnitWeek,
	"month": UnitMonth, "months": UnitMonth,
	"year": UnitYear, "years": UnitYear,
}

var weekdayWords = map[string]time.Weekday{
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
	"sun": time.Sunday, "sunday": time.Sunday,
}

var weekdayNames = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

var commaSpace = regexp.MustCompile(`\s*,\s*`)

// RecurrenceSyntax is the one-line reminder of the grammar a refusal points at.
const RecurrenceSyntax = `every [N] day|week|month|year [on mon,thu | on day D] [after done] [until YYYY-MM-DD | for N times]`

// NormalizeRecurrence returns the canonical spelling of a rule, or the empty
// string for an empty one.
func NormalizeRecurrence(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	r, err := ParseRecurrence(text)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

// ParseRecurrence reads a rule strictly. It refuses rather than guesses,
// because a rule read differently from how it was meant produces the wrong
// task weeks after anybody could notice.
func ParseRecurrence(text string) (Recurrence, error) {
	words := strings.Fields(commaSpace.ReplaceAllString(strings.ToLower(strings.TrimSpace(text)), ","))
	if len(words) == 0 {
		return Recurrence{}, Invalid("recurrence rule is empty; write it as %s", RecurrenceSyntax)
	}
	p := &ruleParser{words: words, text: text}
	r, err := p.parse()
	if err != nil {
		return Recurrence{}, err
	}
	return r, nil
}

type ruleParser struct {
	words []string
	pos   int
	text  string
}

func (p *ruleParser) next() (string, bool) {
	if p.pos >= len(p.words) {
		return "", false
	}
	w := p.words[p.pos]
	p.pos++
	return w, true
}

func (p *ruleParser) fail(format string, args ...any) error {
	return Invalid("recurrence rule %q: "+format, append([]any{p.text}, args...)...)
}

func (p *ruleParser) parse() (Recurrence, error) {
	r, every, err := p.head()
	if err != nil {
		return Recurrence{}, err
	}
	seen := map[string]bool{}
	for {
		w, ok := p.next()
		if !ok {
			break
		}
		if seen[w] {
			return Recurrence{}, p.fail("%q is given twice", w)
		}
		seen[w] = true
		if err := p.clause(&r, w); err != nil {
			return Recurrence{}, err
		}
	}
	if !every && !r.AfterDone {
		return Recurrence{}, p.fail(`a rule starts with "every", as in %q`, r.String())
	}
	if r.Until != nil && r.Count > 0 {
		return Recurrence{}, p.fail("a rule takes one end, until or for, not both")
	}
	return r, nil
}

func (p *ruleParser) head() (Recurrence, bool, error) {
	first, _ := p.next()
	if unit, ok := shorthands[first]; ok {
		return Recurrence{Interval: 1, Unit: unit}, true, nil
	}
	every := first == "every"
	w := first
	if every {
		var ok bool
		if w, ok = p.next(); !ok {
			return Recurrence{}, false, p.fail("say what it repeats every: day, week, month or year")
		}
	}
	r := Recurrence{Interval: 1}
	if n, err := strconv.Atoi(w); err == nil {
		if n < 1 || n > MaxRecurrenceNumber {
			return Recurrence{}, false, p.fail("the interval must be between 1 and %d", MaxRecurrenceNumber)
		}
		r.Interval = n
		var ok bool
		if w, ok = p.next(); !ok {
			return Recurrence{}, false, p.fail("say what it repeats every: day, week, month or year")
		}
	} else if !every {
		return Recurrence{}, false, p.fail("unknown word %q; write it as %s", w, RecurrenceSyntax)
	}
	unit, ok := unitWords[w]
	if !ok {
		return Recurrence{}, false, p.fail("unknown unit %q; use day, week, month or year", w)
	}
	r.Unit = unit
	return r, every, nil
}

func (p *ruleParser) clause(r *Recurrence, w string) error {
	switch w {
	case "on":
		return p.on(r)
	case "after":
		if v, _ := p.next(); v != "done" {
			return p.fail(`"after" must be followed by "done"`)
		}
		r.AfterDone = true
	case "until":
		v, _ := p.next()
		d, err := time.Parse(RecurrenceDateLayout, v)
		if err != nil {
			return p.fail("until %q must be a date written YYYY-MM-DD", v)
		}
		r.Until = &d
	case "for":
		v, _ := p.next()
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > MaxRecurrenceNumber {
			return p.fail("for %q must be a count between 1 and %d", v, MaxRecurrenceNumber)
		}
		if t, _ := p.next(); t != "time" && t != "times" {
			return p.fail(`"for %d" must be followed by "times"`, n)
		}
		r.Count = n
	default:
		return p.fail("unknown word %q; write it as %s", w, RecurrenceSyntax)
	}
	return nil
}

func (p *ruleParser) on(r *Recurrence) error {
	v, ok := p.next()
	if !ok {
		return p.fail(`"on" must name weekdays, as in "on mon,thu", or a day, as in "on day 15"`)
	}
	if v == "day" {
		if r.Unit != UnitMonth {
			return p.fail("a day of the month applies only to a monthly rule")
		}
		d, _ := p.next()
		n, err := strconv.Atoi(d)
		if err != nil || n < 1 || n > 31 {
			return p.fail("day %q must be between 1 and 31", d)
		}
		r.MonthDay = n
		return nil
	}
	if r.Unit != UnitWeek {
		return p.fail("weekdays apply only to a weekly rule")
	}
	var set [7]bool
	for _, name := range strings.Split(v, ",") {
		wd, ok := weekdayWords[name]
		if !ok {
			return p.fail("unknown weekday %q; use mon, tue, wed, thu, fri, sat or sun", name)
		}
		set[wd] = true
	}
	for i := 1; i <= 7; i++ {
		if wd := time.Weekday(i % 7); set[wd] {
			r.Weekdays = append(r.Weekdays, wd)
		}
	}
	return nil
}

// String renders the rule in its canonical spelling, which parses back to
// the same rule.
func (r Recurrence) String() string {
	if r.Unit == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("every ")
	if r.Interval > 1 {
		b.WriteString(strconv.Itoa(r.Interval) + " " + string(r.Unit) + "s")
	} else {
		b.WriteString(string(r.Unit))
	}
	if len(r.Weekdays) > 0 {
		names := make([]string, len(r.Weekdays))
		for i, wd := range r.Weekdays {
			names[i] = weekdayNames[wd]
		}
		b.WriteString(" on " + strings.Join(names, ","))
	}
	if r.MonthDay > 0 {
		b.WriteString(" on day " + strconv.Itoa(r.MonthDay))
	}
	if r.AfterDone {
		b.WriteString(" after done")
	}
	if r.Until != nil {
		b.WriteString(" until " + r.Until.Format(RecurrenceDateLayout))
	}
	switch {
	case r.Count == 1:
		b.WriteString(" for 1 time")
	case r.Count > 1:
		b.WriteString(" for " + strconv.Itoa(r.Count) + " times")
	}
	return b.String()
}

// Successor is the rule the next occurrence carries: the same rule with one
// fewer occurrence left.
func (r Recurrence) Successor() Recurrence {
	if r.Count > 1 {
		r.Count--
	}
	return r
}

// Next returns the due date of the occurrence that follows one due at due and
// completed at done, and false when the rule has run out.
//
// On the schedule anchor it is the first occurrence counted from the due date
// that falls after both the due date and the completion, so finishing late
// skips the missed occurrences rather than producing a pile of overdue
// copies. Counting from the completion date, which an "after done" rule and an
// undated task both do, keeps the due date's time of day, or midnight.
func (r Recurrence) Next(due *time.Time, done time.Time) (time.Time, bool) {
	if r.Unit == "" || r.Count == 1 {
		return time.Time{}, false
	}
	origin, bound := r.anchor(due, done)
	next := r.firstAfter(origin, bound)
	if r.Until != nil && afterDate(next, *r.Until) {
		return time.Time{}, false
	}
	return next, true
}

func (r Recurrence) anchor(due *time.Time, done time.Time) (origin, bound time.Time) {
	if due != nil && !r.AfterDone {
		if done.After(*due) {
			return *due, done
		}
		return *due, *due
	}
	y, m, d := done.Date()
	h, mi, s, ns := 0, 0, 0, 0
	if due != nil {
		at := due.In(done.Location())
		h, mi, s = at.Clock()
		ns = at.Nanosecond()
	}
	origin = time.Date(y, m, d, h, mi, s, ns, done.Location())
	return origin, origin
}

// afterDate reports whether t falls on a later calendar date than limit.
func afterDate(t, limit time.Time) bool {
	ty, tm, td := t.Date()
	ly, lm, ld := limit.Date()
	return time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC).After(time.Date(ly, lm, ld, 0, 0, 0, 0, time.UTC))
}

// firstAfter returns the earliest occurrence counted from origin that is
// strictly after bound. Each occurrence is computed from origin rather than
// from the one before it, so clamping a month end never accumulates.
func (r Recurrence) firstAfter(origin, bound time.Time) time.Time {
	if len(r.Weekdays) > 0 {
		return r.firstWeekdayAfter(origin, bound)
	}
	for k := r.estimate(origin, bound); ; k++ {
		if occ := r.occurrence(origin, k); occ.After(bound) {
			return occ
		}
	}
}

// estimate is a lower bound on the step whose occurrence first passes bound,
// so a task finished years late does not walk every missed occurrence.
func (r Recurrence) estimate(origin, bound time.Time) int {
	var steps int
	switch r.Unit {
	case UnitDay:
		steps = int(bound.Sub(origin).Hours()/24) / r.Interval
	case UnitWeek:
		steps = int(bound.Sub(origin).Hours()/(24*7)) / r.Interval
	case UnitMonth:
		steps = monthsBetween(origin, bound) / r.Interval
	case UnitYear:
		steps = (bound.Year() - origin.Year()) / r.Interval
	}
	return max(steps-1, 0)
}

func monthsBetween(a, b time.Time) int {
	return (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
}

func (r Recurrence) occurrence(origin time.Time, k int) time.Time {
	y, m, d := origin.Date()
	h, mi, s := origin.Clock()
	ns, loc := origin.Nanosecond(), origin.Location()
	switch r.Unit {
	case UnitDay:
		return time.Date(y, m, d+k*r.Interval, h, mi, s, ns, loc)
	case UnitWeek:
		return time.Date(y, m, d+7*k*r.Interval, h, mi, s, ns, loc)
	case UnitMonth:
		day := d
		if r.MonthDay > 0 {
			day = r.MonthDay
		}
		return clampedDate(y, int(m)+k*r.Interval, day, h, mi, s, ns, loc)
	default:
		return clampedDate(y, int(m)+12*k*r.Interval, d, h, mi, s, ns, loc)
	}
}

// clampedDate builds a date in the given month, holding the day to the last
// one that month has.
func clampedDate(y, month, day, h, mi, s, ns int, loc *time.Location) time.Time {
	first := time.Date(y, time.Month(month), 1, 0, 0, 0, 0, loc)
	last := first.AddDate(0, 1, -1).Day()
	return time.Date(first.Year(), first.Month(), min(day, last), h, mi, s, ns, loc)
}

func (r Recurrence) firstWeekdayAfter(origin, bound time.Time) time.Time {
	y, m, d := origin.Date()
	h, mi, s := origin.Clock()
	ns, loc := origin.Nanosecond(), origin.Location()
	monday := d - mondayOffset(origin.Weekday())
	cycle := 7 * r.Interval
	j := max(int(bound.Sub(origin).Hours()/24)/cycle-1, 0)
	for ; ; j++ {
		for _, wd := range r.Weekdays {
			occ := time.Date(y, m, monday+j*cycle+mondayOffset(wd), h, mi, s, ns, loc)
			if occ.After(bound) {
				return occ
			}
		}
	}
}

// mondayOffset is a weekday's distance from the Monday that starts its week.
func mondayOffset(wd time.Weekday) int { return (int(wd) + 6) % 7 }
