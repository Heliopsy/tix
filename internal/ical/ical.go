// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ical renders a calendar feed as an RFC 5545 VCALENDAR. It is a pure
// function of the entries it is given, so the same tasks always render the
// same bytes and an ETag over them changes only when the tasks do.
package ical

import (
	"bytes"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/core"
)

// ContentType is the media type a feed is served as.
const ContentType = "text/calendar; charset=utf-8"

// maxLineOctets is the longest content line RFC 5545 allows, excluding CRLF.
const maxLineOctets = 75

// refresh is the polling interval the calendar suggests to clients that read
// REFRESH-INTERVAL (RFC 7986) or the older X-PUBLISHED-TTL.
const refresh = "PT1H"

// DoneMark leads the summary of a finished task. VEVENT has no COMPLETED
// property, and STATUS:CANCELLED is drawn as struck out or hidden, which is
// wrong for work that was done.
const DoneMark = "✓ "

// Options carries what the renderer needs from the surface serving it.
type Options struct {
	// TaskURL returns the absolute address of a task's page.
	TaskURL func(ref string) string
	// Name is the calendar's display name.
	Name string
}

// Render writes the feed as a VCALENDAR with one all-day VEVENT per entry.
func Render(in core.CalendarEntries, opt Options) []byte {
	w := &writer{}
	w.prop("BEGIN", "VCALENDAR")
	w.prop("VERSION", "2.0")
	w.prop("PRODID", "-//tix//calendar feed//EN")
	w.prop("CALSCALE", "GREGORIAN")
	w.prop("METHOD", "PUBLISH")
	w.prop("X-WR-CALNAME", Text(opt.Name))
	w.prop("REFRESH-INTERVAL;VALUE=DURATION", refresh)
	w.prop("X-PUBLISHED-TTL", refresh)
	for _, e := range in.Entries {
		w.event(e, opt)
	}
	w.prop("END", "VCALENDAR")
	return w.buf.Bytes()
}

// event writes one task as an all-day event on its due date.
func (w *writer) event(e core.CalendarEntry, opt Options) {
	t := e.Task
	day := t.DueAt.UTC()
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	url := ""
	if opt.TaskURL != nil {
		url = opt.TaskURL(t.Ref)
	}
	summary := t.Ref + ": " + t.Title
	status := "CONFIRMED"
	switch e.Category {
	case core.CategoryDone:
		summary = DoneMark + summary
	case core.CategoryCancelled:
		status = "CANCELLED"
	}
	stamp := t.UpdatedAt.UTC().Format("20060102T150405Z")

	w.prop("BEGIN", "VEVENT")
	w.prop("UID", Text(UID(t.ID)))
	w.prop("DTSTAMP", stamp)
	w.prop("LAST-MODIFIED", stamp)
	w.prop("CREATED", t.CreatedAt.UTC().Format("20060102T150405Z"))
	w.prop("SEQUENCE", strconv.Itoa(t.Version))
	w.prop("DTSTART;VALUE=DATE", start.Format("20060102"))
	w.prop("DTEND;VALUE=DATE", start.AddDate(0, 0, 1).Format("20060102"))
	w.prop("SUMMARY", Text(summary))
	w.prop("DESCRIPTION", Text(description(e, url)))
	if url != "" {
		w.prop("URL", url)
	}
	if len(t.Tags) > 0 {
		tags := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			tags[i] = Text(tag)
		}
		w.prop("CATEGORIES", strings.Join(tags, ","))
	}
	w.prop("STATUS", status)
	w.prop("TRANSP", "TRANSPARENT")
	w.prop("END", "VEVENT")
}

// UID is the identifier a task's event keeps for as long as the task exists,
// across renames, new due dates and moves to another project.
func UID(taskID string) string { return "task-" + strings.ToLower(taskID) + "@tix" }

// description is the event body: the state, the priority and the link back.
func description(e core.CalendarEntry, url string) string {
	var b strings.Builder
	b.WriteString(e.Task.Ref + " · " + e.State + " · priority " + priorityName(e.Task.Priority))
	if url != "" {
		b.WriteString("\n" + url)
	}
	return b.String()
}

// priorityName is a priority as the filter language spells it.
func priorityName(p core.Priority) string {
	names := map[core.Priority]string{
		core.PriorityHighest: "highest", core.PriorityHigh: "high", core.PriorityNormal: "normal",
		core.PriorityLow: "low", core.PriorityLowest: "lowest",
	}
	if n, ok := names[p]; ok {
		return n
	}
	return strconv.Itoa(int(p))
}

// Text escapes a TEXT value: backslash, semicolon, comma and line breaks.
func Text(s string) string {
	r := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`)
	return r.Replace(s)
}

// writer accumulates folded content lines.
type writer struct{ buf bytes.Buffer }

// prop writes one content line, folded.
func (w *writer) prop(name, value string) {
	w.buf.WriteString(Fold(name + ":" + stripControls(value)))
}

// stripControls drops control characters RFC 5545 forbids in a value; line
// breaks have already been escaped by Text where a value may hold them.
func stripControls(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r >= 0x20 && r != 0x7f {
			return r
		}
		return -1
	}, s)
}

// Fold splits a content line into lines of at most 75 octets, each
// continuation starting with a space, never inside a UTF-8 sequence, and ends
// every line with CRLF.
func Fold(line string) string {
	var b strings.Builder
	limit := maxLineOctets
	for len(line) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		b.WriteString(line[:cut])
		b.WriteString("\r\n ")
		line = line[cut:]
		limit = maxLineOctets - 1
	}
	b.WriteString(line)
	b.WriteString("\r\n")
	return b.String()
}
