// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// Named time layouts a person can choose between. A layout string is not
// offered: Go's reference-time layouts are a footgun in a configuration file,
// and every format anybody actually asked for is here.
const (
	TimeISO      = "iso"      // 2026-09-21 14:05
	TimeRFC3339  = "rfc3339"  // 2026-09-21T14:05:09+03:00
	TimeShort    = "short"    // 21 Sep 14:05
	TimeUS       = "us"       // 09/21/2026 2:05 PM
	TimeDotted   = "dotted"   // 21.09.2026 14:05
	TimeDayFirst = "dayfirst" // 21/09/2026 14:05
	TimeLong     = "long"     // 21 Sep 2026 14:05
	TimeRelative = "relative" // 18 minutes ago
)

// TimeFormats lists the named layouts this build implements, in the order a
// chooser should offer them.
// It is core.TimeFormatNames, so a tenant or a person can be refused a name
// this package would not render.
var TimeFormats = core.TimeFormatNames

// DateOrder is how a date a person types with digits alone is read. YYYY-MM-DD
// is read the same way under every order; a date such as 06/11/2026 is not,
// so it is read only under an order that names which number is the day.
type DateOrder int

// Date orders.
const (
	// DateISO reads YYYY-MM-DD only and refuses any other run of digits,
	// because guessing which number is the day is how a task lands a month
	// early.
	DateISO DateOrder = iota
	// DateDayFirst reads 06.11.2026 and 06/11/2026 as the 6th of November.
	DateDayFirst
	// DateMonthFirst reads 11/06/2026 as the 6th of November.
	DateMonthFirst
)

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

// Name is the named format this style renders, iso for a zero style.
func (s TimeStyle) Name() string {
	if s.format == "" {
		return TimeISO
	}
	return s.format
}

// Location is the zone instants are rendered in. A zero style renders in the
// machine's own zone, as Format does, so a caller that counts days in
// Location and a screen that prints with Format agree.
func (s TimeStyle) Location() *time.Location {
	if s.loc == nil {
		return time.Local
	}
	return s.loc
}

// CacheKey names the style for callers that keep work derived from it, such
// as parsed templates. It is false for a style with an injected clock, which
// no two callers can be assumed to share.
func (s TimeStyle) CacheKey() (string, bool) {
	if s.now != nil {
		return "", false
	}
	return s.format + "|" + s.Location().String(), true
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
	case TimeDotted:
		return in.Format("02.01.2006 15:04")
	case TimeDayFirst:
		return in.Format("02/01/2006 15:04")
	case TimeLong:
		return in.Format("2 Jan 2006 15:04")
	case TimeRelative:
		return s.relative(t)
	default:
		return in.Format(CompactTimeLayout)
	}
}

// DateOrder is how this style reads a date typed with digits alone: the order
// its own dates are written in, or ISO only for a style whose dates do not
// say.
func (s TimeStyle) DateOrder() DateOrder {
	switch s.format {
	case TimeDotted, TimeDayFirst:
		return DateDayFirst
	case TimeUS:
		return DateMonthFirst
	default:
		return DateISO
	}
}

// Date writes t's calendar date, as it stands, the way a person using this
// style would type it into a date field, so a field filled with it saves back
// to the same day.
func (s TimeStyle) Date(t time.Time) string {
	switch s.format {
	case TimeDotted:
		return t.Format("02.01.2006")
	case TimeDayFirst:
		return t.Format("02/01/2006")
	case TimeUS:
		return t.Format("01/02/2006")
	case TimeLong, TimeShort:
		return t.Format("2 Jan 2006")
	default:
		return t.Format(time.DateOnly)
	}
}

// Full renders t with everything a reader could ask of it: the date in this
// style's order with the year, the time to the second, and the zone by name
// and by offset, such as "06.11.2026 14:05:32 EET (UTC+02:00)". It is what a
// timestamp's tooltip says, so a relative "2 hours ago" or a short "6 Nov
// 14:05" can always be pinned down to an instant.
func (s TimeStyle) Full(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	in := t.In(s.Location())
	var layout string
	switch s.format {
	case TimeUS:
		layout = "01/02/2006 3:04:05 PM"
	case TimeDotted:
		layout = "02.01.2006 15:04:05"
	case TimeDayFirst:
		layout = "02/01/2006 15:04:05"
	case TimeLong, TimeShort:
		layout = "2 Jan 2006 15:04:05"
	default:
		layout = "2006-01-02 15:04:05"
	}
	return in.Format(layout) + " " + zoneSuffix(in)
}

// zoneSuffix names t's zone and offset, "EET (UTC+02:00)", without repeating
// a zone whose abbreviation is only its offset.
func zoneSuffix(t time.Time) string {
	offset := "UTC" + t.Format("-07:00")
	if t.Format("-07:00") == "+00:00" {
		offset = "UTC+00:00"
	}
	name := t.Format("MST")
	if name == "" || name[0] == '+' || name[0] == '-' {
		return "(" + offset + ")"
	}
	return name + " (" + offset + ")"
}

// DayMonth writes t's calendar day without the year, in this style's order,
// for a label in a series whose year every column shares.
func (s TimeStyle) DayMonth(t time.Time) string {
	switch s.format {
	case TimeDotted:
		return t.Format("02.01")
	case TimeDayFirst:
		return t.Format("02/01")
	case TimeUS:
		return t.Format("01/02")
	case TimeLong, TimeShort:
		return t.Format("2 Jan")
	default:
		return t.Format("01-02")
	}
}

// FormatPtr renders t, or the empty string for nil or the zero time.
func (s TimeStyle) FormatPtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return s.Format(*t)
}

// Absolute renders t as an instant, for the title of a relative timestamp.
//
// "relative" is the one format it escapes, falling back to ISO, because a
// second relative string would tell a reader who hovers "3 hours ago" nothing
// new. Every other format is the reader's own choice and is honoured: forcing
// ISO on all of them, as this used to, left a reader who asked for US dates
// reading them everywhere on a page except in this one tooltip.
func (s TimeStyle) Absolute(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if s.format != TimeRelative {
		return s.Format(t)
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

// TimeSettingsReader is anything that can say how a caller's dates are shown,
// which every core.Service can.
type TimeSettingsReader interface {
	GetTimeSettings(ctx context.Context) (*core.TimeSettings, error)
}

// StyleFor renders a resolved core.TimeSettings as a style.
func StyleFor(s core.TimeSettings) (TimeStyle, error) {
	return NewTimeStyle(s.Effective.TimeFormat, s.Effective.Timezone)
}

// ForCaller resolves the style the caller in ctx reads dates in: their own
// choice, else their tenant's, else the deployment's. A service that cannot
// answer, such as an older server, leaves fallback in force, because a date in
// the wrong layout is a smaller failure than a screen that will not open.
func ForCaller(ctx context.Context, src TimeSettingsReader, fallback TimeStyle) TimeStyle {
	if src == nil {
		return fallback
	}
	settings, err := src.GetTimeSettings(ctx)
	if err != nil || settings == nil {
		return fallback
	}
	style, err := StyleFor(*settings)
	if err != nil {
		return fallback
	}
	return style
}
