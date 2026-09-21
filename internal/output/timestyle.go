package output

import (
	"fmt"
	"strings"
	"time"
)

// Named time layouts a person can choose between. A layout string is not
// offered: Go's reference-time layouts are a footgun in a configuration file,
// and every format anybody actually asked for is here.
const (
	TimeISO      = "iso"      // 2026-09-21 14:05
	TimeRFC3339  = "rfc3339"  // 2026-09-21T14:05:09+03:00
	TimeShort    = "short"    // 21 Sep 14:05
	TimeUS       = "us"       // 09/21/2026 2:05 PM
	TimeRelative = "relative" // 18 minutes ago
)

// TimeFormats lists the named layouts this build implements, in the order a
// chooser should offer them.
var TimeFormats = []string{TimeISO, TimeRFC3339, TimeShort, TimeUS, TimeRelative}

// TimeStyle renders an instant the way a person configured. It is the one
// place that decides, so the table, the terminal interface, the live stream and
// the browser cannot disagree about what "two o'clock" means.
//
// Machine-readable output never goes through here. JSON, YAML and NDJSON always
// carry RFC 3339 in UTC, because a consumer parsing a timestamp must not have to
// discover which zone a deployment happened to configure.
type TimeStyle struct {
	format string
	loc    *time.Location
	// now is injected so a relative rendering is testable. Nil means time.Now.
	now func() time.Time
}

// NewTimeStyle resolves a named format and an IANA timezone. The timezone is
// "local" for the machine's own zone, "utc", or any name the system zone
// database carries. An unknown format or zone is refused rather than silently
// falling back, because a timestamp quietly in the wrong zone is worse than an
// error at startup.
func NewTimeStyle(format, timezone string) (TimeStyle, error) {
	f := strings.ToLower(strings.TrimSpace(format))
	if f == "" {
		f = TimeISO
	}
	if !slicesContains(TimeFormats, f) {
		return TimeStyle{}, fmt.Errorf("time format %q is not one of %s", format, strings.Join(TimeFormats, ", "))
	}
	loc, err := loadLocation(timezone)
	if err != nil {
		return TimeStyle{}, err
	}
	return TimeStyle{format: f, loc: loc}, nil
}

// loadLocation resolves a timezone name, treating the empty string as local.
func loadLocation(name string) (*time.Location, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "local":
		return time.Local, nil
	case "utc":
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("timezone %q is not a name this system knows: %w", name, err)
	}
	return loc, nil
}

// Format renders t, or the empty string for the zero time. A zero time means
// "never happened", and "never happened" prints as nothing rather than as the
// first instant of 1970, which every reader misreads as a real date.
func (s TimeStyle) Format(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	loc := s.loc
	if loc == nil {
		loc = time.Local
	}
	in := t.In(loc)
	switch s.format {
	case TimeRFC3339:
		return in.Format(time.RFC3339)
	case TimeShort:
		return in.Format("2 Jan 15:04")
	case TimeUS:
		return in.Format("01/02/2006 3:04 PM")
	case TimeRelative:
		return s.relative(t)
	default:
		return in.Format(CompactTimeLayout)
	}
}

// FormatPtr renders t, or the empty string for nil or the zero time.
func (s TimeStyle) FormatPtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return s.Format(*t)
}

// Absolute renders t in the ISO layout whatever the configured format, for the
// title of a relative timestamp. A reader who hovers "3 hours ago" wants the
// instant, and a second relative string would tell them nothing new.
func (s TimeStyle) Absolute(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	loc := s.loc
	if loc == nil {
		loc = time.Local
	}
	return t.In(loc).Format(CompactTimeLayout)
}

// Clock renders t as a time of day in the configured zone, for a line in a
// live stream. A running stream has no need for the date on every line, only
// the clock; the zone still has to be the configured one; otherwise a live
// tail and a table of the same instants would read as different times.
func (s TimeStyle) Clock(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	loc := s.loc
	if loc == nil {
		loc = time.Local
	}
	return t.In(loc).Format(ClockTimeLayout)
}

// Relative renders t as a distance from now ("18 minutes ago"), regardless of
// the configured named format. It is the one implementation of that
// rendering: the "relative" named format and any caller that always wants a
// relative rendering, such as the web's history view, both go through it, so
// there is exactly one algorithm for what "a while ago" means.
func (s TimeStyle) Relative(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return s.relative(t)
}

// relative renders the distance from now in the coarsest unit that still says
// something useful. Past and future both read naturally, because a due date is
// as common a thing to render as a created date.
func (s TimeStyle) relative(t time.Time) string {
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	d := now().Sub(t)
	future := d < 0
	if future {
		d = -d
	}
	unit := func(n int, name string) string {
		if n != 1 {
			name += "s"
		}
		if future {
			return fmt.Sprintf("in %d %s", n, name)
		}
		return fmt.Sprintf("%d %s ago", n, name)
	}
	switch {
	case d < 45*time.Second:
		if future {
			return "in a moment"
		}
		return "just now"
	case d < time.Hour:
		// Truncation would say "0 minutes ago" for the gap between the
		// just-now cutoff and a full minute, so every bucket floors at one.
		return unit(atLeastOne(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return unit(atLeastOne(d.Hours()), "hour")
	case d < 30*24*time.Hour:
		return unit(atLeastOne(d.Hours()/24), "day")
	case d < 365*24*time.Hour:
		return unit(atLeastOne(d.Hours()/(24*30)), "month")
	default:
		return unit(atLeastOne(d.Hours()/(24*365)), "year")
	}
}

// slicesContains avoids importing slices for one call in a package that other
// packages depend on being cheap.
func slicesContains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// atLeastOne truncates to a whole unit without ever reaching zero, because
// "0 minutes ago" is not something anybody means.
func atLeastOne(v float64) int {
	if n := int(v); n > 0 {
		return n
	}
	return 1
}
