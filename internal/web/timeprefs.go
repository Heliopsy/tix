// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// readerTimeKey carries the reader's resolved date settings through a request.
type readerTimeKey struct{}

// readerTime is what one request resolved about how its reader reads dates.
type readerTime struct {
	settings core.TimeSettings
	style    output.TimeStyle
}

// withReaderTime resolves how the signed-in reader reads dates, once per
// request, so every screen, partial and toast it draws agrees.
//
// A browser still carrying the cookies the per-browser preference used to
// live in has them moved onto the account first, where they follow the
// person to every other browser, and then dropped. A field the account
// already holds is not overwritten: a choice made on purpose somewhere else
// is newer than a cookie from before the account could hold one.
func (h *handler) withReaderTime(w http.ResponseWriter, r *http.Request) *http.Request {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return r
	}
	settings, err := h.svc.GetTimeSettings(r.Context())
	if err != nil {
		return r
	}
	if migrated := h.migrateTimeCookies(w, r, settings); migrated != nil {
		settings = migrated
	}
	style, err := output.StyleFor(*settings)
	if err != nil {
		h.logger.Error("rendering in the deployment default", "format", settings.Effective.TimeFormat,
			"zone", settings.Effective.Timezone, "error", err)
		style = h.style
	}
	return r.WithContext(context.WithValue(r.Context(), readerTimeKey{}, readerTime{settings: *settings, style: style}))
}

// migrateTimeCookies moves a legacy per-browser date preference onto the
// account and expires the cookie. It returns the settings after the move, or
// nil when there was nothing to move.
func (h *handler) migrateTimeCookies(w http.ResponseWriter, r *http.Request, s *core.TimeSettings) *core.TimeSettings {
	format, hasFormat := legacyCookie(r, TimeFormatCookie)
	zone, hasZone := legacyCookie(r, TimezoneCookie)
	if !hasFormat && !hasZone {
		return nil
	}
	for _, name := range []string{TimeFormatCookie, TimezoneCookie} {
		// #nosec G124 -- expiring a display cookie, readable by no script.
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode})
	}
	if !s.Personal {
		return nil
	}
	var in core.TimePrefsInput
	if v := resolvedTimeFormat(format); v != "" && s.User.TimeFormat == "" {
		in.TimeFormat = &v
	}
	if v := resolvedZone(zone); v != "" && s.User.Timezone == "" {
		in.Timezone = &v
	}
	if in.TimeFormat == nil && in.Timezone == nil {
		return nil
	}
	out, err := h.svc.SetTimePrefs(r.Context(), in)
	if err != nil {
		h.logger.Warn("moving a browser's date preference onto the account", "error", err)
		return nil
	}
	return out
}

// legacyCookie reads one of the cookies the per-browser date preference used
// to be kept in.
func legacyCookie(r *http.Request, name string) (string, bool) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", false
	}
	return c.Value, true
}

// readerStyle is the time style this request renders in: the reader's own
// choice, else their tenant's, else the deployment's.
func (h *handler) readerStyle(r *http.Request) output.TimeStyle {
	if r != nil {
		if rt, ok := r.Context().Value(readerTimeKey{}).(readerTime); ok {
			return rt.style
		}
	}
	return h.style
}

// readerTimeSettings is the reader's resolved settings, and whether the
// request resolved any.
func readerTimeSettings(r *http.Request) (core.TimeSettings, bool) {
	rt, ok := r.Context().Value(readerTimeKey{}).(readerTime)
	return rt.settings, ok
}

// timeSourceLabels name each level the way the settings screens say where a
// value came from.
var timeSourceLabels = map[string]string{
	core.TimeSourceGlobal: "server default",
	core.TimeSourceTenant: "tenant default",
	core.TimeSourceUser:   "your choice",
}

// timeLevel is one date preference as a settings screen offers it: the value
// held at this level, what holding nothing would inherit, and what is in
// force.
type timeLevel struct {
	// Name is the form field.
	Name string
	// Held is this level's own value, empty when it inherits.
	Held string
	// Choices are the options, the inherit option first.
	Choices []timeChoice
	// Effective is the value in force, as its label.
	Effective string
	// Source says where Effective came from.
	Source string
}

// formatLevel offers the date format at one level. inherited is what an
// empty value would fall back to and from where.
func formatLevel(held, inherited, inheritedFrom, zone string, effective core.TimeSettings) timeLevel {
	choices := timeFormatChoices(zone)
	choices[0].Inherits, choices[0].From = timeFormatLabel(inherited, zone), timeSourceLabels[inheritedFrom]
	return timeLevel{Name: "time_format", Held: held, Choices: choices,
		Effective: timeFormatLabel(effective.Effective.TimeFormat, zone), Source: timeSourceLabels[effective.FormatSource]}
}

// zoneLevel offers the timezone at one level.
func zoneLevel(held, inherited, inheritedFrom string, effective core.TimeSettings) timeLevel {
	choices := timezoneChoicesWith(held)
	choices[0].Inherits, choices[0].From = zoneLabel(inherited), timeSourceLabels[inheritedFrom]
	return timeLevel{Name: "timezone", Held: held, Choices: choices,
		Effective: zoneLabel(effective.Effective.Timezone), Source: timeSourceLabels[effective.TimezoneSource]}
}

// timeFormatLabel names a format with an example of it.
func timeFormatLabel(name, zone string) string {
	style, err := output.NewTimeStyle(name, zone)
	if err != nil {
		return name
	}
	sample := timeFormatExample
	if style.Name() == output.TimeRelative {
		sample = time.Now().Add(-18 * time.Minute)
	}
	return timeFormatNames[style.Name()] + " (" + style.Format(sample) + ")"
}

// zoneLabel names a zone as a reader would.
func zoneLabel(zone string) string {
	switch zone {
	case "", "local", "Local":
		return "the server's own zone"
	}
	return zoneDisplay(zone)
}

// userTimeLevels are the reader's own date format and timezone controls.
func userTimeLevels(s core.TimeSettings) (timeLevel, timeLevel) {
	below := core.ResolveTimePrefs(s.Global, s.Tenant, core.TimePrefs{})
	zone := s.Effective.Timezone
	return formatLevel(s.User.TimeFormat, below.Effective.TimeFormat, below.FormatSource, zone, s),
		zoneLevel(s.User.Timezone, below.Effective.Timezone, below.TimezoneSource, s)
}

// tenantTimeLevels are the tenant default's controls, which inherit from the
// deployment. Effective here is the tenant's own result, not the reader's.
func tenantTimeLevels(s core.TimeSettings) (timeLevel, timeLevel) {
	tenant := core.ResolveTimePrefs(s.Global, s.Tenant, core.TimePrefs{})
	zone := tenant.Effective.Timezone
	return formatLevel(s.Tenant.TimeFormat, s.Global.TimeFormat, core.TimeSourceGlobal, zone, tenant),
		zoneLevel(s.Tenant.Timezone, s.Global.Timezone, core.TimeSourceGlobal, tenant)
}

// readerSettings is the reader's resolved settings, or the deployment's alone
// for a request that resolved none.
func (h *handler) readerSettings(r *http.Request) core.TimeSettings {
	if s, ok := readerTimeSettings(r); ok {
		return s
	}
	global := core.TimePrefs{TimeFormat: h.style.Name(), Timezone: h.style.Location().String()}
	return core.ResolveTimePrefs(global, core.TimePrefs{}, core.TimePrefs{})
}

// timeElement renders an instant for a person: visible text in their style,
// inside a <time> whose datetime stays RFC 3339 for machines, with the full
// instant (seconds, zone and offset, and how long ago) on hover as a title,
// to a screen reader as its label, and on keyboard focus or a tap as a tip.
//
// It is the one way a template shows a timestamp, so every screen answers
// "when exactly?" the same way. A nil or zero instant renders nothing.
func timeElement(style output.TimeStyle, v any, relative bool) template.HTML {
	t, ok := instant(v)
	if !ok {
		return ""
	}
	visible := style.Format(t)
	full := style.Full(t)
	if relative {
		visible = style.Relative(t)
	} else if style.Name() != output.TimeRelative {
		full += " · " + style.Relative(t)
	}
	machine := t.UTC().Format(time.RFC3339)
	// #nosec G203 -- every piece is escaped here; none is markup.
	return template.HTML(`<time class="ts" datetime="` + machine + `" tabindex="0" title="` +
		template.HTMLEscapeString(full) + `" aria-label="` + template.HTMLEscapeString(visible+", "+full) +
		`" data-full="` + template.HTMLEscapeString(full) + `">` + template.HTMLEscapeString(visible) + `</time>`)
}

// instant reads a template value as an instant, reporting false for nil and
// for the zero time, which mean "never happened".
func instant(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, !t.IsZero()
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, !t.IsZero()
	}
	return time.Time{}, false
}

// calendarDay writes a calendar date the service keys by "2006-01-02" in the
// reader's order. A day is not an instant, so no zone applies.
func calendarDay(d string, write func(time.Time) string) string {
	t, err := time.Parse(time.DateOnly, d)
	if err != nil {
		return d
	}
	return write(t)
}
