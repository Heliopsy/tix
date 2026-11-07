// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/demo"
	"github.com/heliopsy/tix/internal/output"
)

// guardFormat and guardZone are the reader's preference every screen is
// rendered under: a layout no other format writes, in a zone that is not the
// host's, so a timestamp that skipped the reader's style shows up either as
// another layout or as another hour.
const (
	guardFormat = output.TimeDotted
	guardZone   = "Asia/Tokyo"
)

// guardFragments are the htmx fragments the screenshot manifest skips as
// parts of a screen, rendered here too because they draw timestamps of their
// own.
var guardFragments = map[string]bool{
	"/activity/feed": true, "/tasks/{ref}/history": true, "/tasks/live": true,
	"/mine/live": true, "/projects/{key}/live": true, "/tasks/{ref}/live": true,
}

var (
	timeElementRe = regexp.MustCompile(`(?s)<time\b([^>]*)>(.*?)</time>`)
	attrRe        = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)
	dropRe        = regexp.MustCompile(`(?s)<(script|style|code|pre|kbd|samp|textarea|option)\b[^>]*>.*?</(?:script|style|code|pre|kbd|samp|textarea|option)>`)
	tagRe         = regexp.MustCompile(`(?s)<[^>]*>`)
	humanAttrRe   = regexp.MustCompile(`\s(?:title|aria-label|placeholder|data-full)="([^"]*)"`)
	relativeRe    = regexp.MustCompile(`^(just now|in a moment|\d+ (?:minute|hour|day|month|year)s? ago|in \d+ (?:minute|hour|day|month|year)s?)$`)
	// foreignDates are timestamps written in a layout other than the
	// reader's: ISO and RFC 3339, US and day-first slashes, the long and
	// short month-name forms, and Go's own time.Time.String().
	foreignDates = []*regexp.Regexp{
		regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`),
		regexp.MustCompile(`\b\d{1,2}/\d{1,2}/\d{4}\b`),
		regexp.MustCompile(`\b\d{1,2} (?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)(?: \d{4})? \d{2}:\d{2}\b`),
		regexp.MustCompile(`\d{2}:\d{2}:\d{2}(?:\.\d+)? [+-]\d{4}`),
	}
)

// guardExamples are dates the screens write on purpose in another layout: the
// typed-date examples that explain how each format reads a date, the instant
// each format is demonstrated with in a chooser, and the until date of the
// repeat rule grammar, which is always typed as ISO. They are demonstrations
// of a layout, not instants.
func guardExamples() []string {
	day := time.Date(2026, time.November, 6, 0, 0, 0, 0, time.UTC)
	return []string{
		day.Format(time.DateOnly), "06/11/2026", "11/06/2026",
		"2026-11-20", "20/11/2026", "11/20/2026",
		"2026-09-21", "21/09/2026", "09/21/2026",
		"2027-06-30",
	}
}

// TestEveryScreenWritesDatesInTheReadersFormat renders every screen the
// screenshot manifest lists, on a seeded database, as a reader who reads
// dotted dates in Tokyo, and fails on any timestamp written another way. It
// also holds every <time> to the one helper's shape: machine datetime, the
// visible text in the reader's style, and the full instant on hover, to a
// screen reader and on focus.
func TestEveryScreenWritesDatesInTheReadersFormat(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.makeGlobalAdmin()
	seedGuardData(t, f)

	b := f.as("alice")
	resp := b.post("/timeformat", map[string][]string{"time_format": {guardFormat}, "next": {"/tasks"}})
	_ = resp.Body.Close()
	resp = b.post("/timezone", map[string][]string{"timezone": {guardZone}, "next": {"/tasks"}})
	_ = resp.Body.Close()
	style, err := output.NewTimeStyle(guardFormat, guardZone)
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, path := range guardRoutes(t, b) {
		page := guardPage(t, b, path)
		checked++
		for _, problem := range dateProblems(page, style) {
			t.Errorf("%s: %s", path, problem)
		}
	}
	if checked < 30 {
		t.Fatalf("only %d screens were rendered; the route list did not resolve", checked)
	}
	for _, problem := range rawDialogDateProblems(t, b, style) {
		t.Errorf("raw dialog: %s", problem)
	}
}

var (
	rawOpenRe   = regexp.MustCompile(`<a class="[^"]*\brawopen\b[^"]*" href="([^"]+)"`)
	rawDialogRe = regexp.MustCompile(`(?s)<dialog [^>]*class="[^"]*\brawdlg\b[^"]*"[^>]*>.*?</dialog>`)
)

// rawDialogDateProblems opens the first Raw dialog on the activity feed, as
// its no-script link draws it open, and checks the dialog alone. The manifest
// reaches it only by a click, so the route walk above never renders it.
func rawDialogDateProblems(t *testing.T, b *browser, style output.TimeStyle) []string {
	t.Helper()
	open := rawOpenRe.FindStringSubmatch(b.page("/activity"))
	if open == nil {
		t.Fatal("the activity feed has no Raw control to open")
	}
	dlg := rawDialogRe.FindString(guardPage(t, b, html.UnescapeString(open[1])))
	if dlg == "" {
		t.Fatalf("%s drew no raw dialog", open[1])
	}
	if n := len(timeElementRe.FindAllString(dlg, -1)); n < 2 {
		t.Fatalf("the raw dialog has %d <time> elements, want its heading's and each entry's", n)
	}
	return dateProblems(dlg, style)
}

// dateProblems lists every timestamp on a page that is not in the reader's
// style, and every <time> missing part of its tooltip.
func dateProblems(page string, style output.TimeStyle) []string {
	var out []string
	for _, m := range timeElementRe.FindAllStringSubmatch(page, -1) {
		attrs := map[string]string{}
		for _, a := range attrRe.FindAllStringSubmatch(m[1], -1) {
			attrs[a[1]] = html.UnescapeString(a[2])
		}
		at, err := time.Parse(time.RFC3339, attrs["datetime"])
		if err != nil {
			out = append(out, "a <time> without an RFC 3339 datetime: "+m[0])
			continue
		}
		visible := html.UnescapeString(strings.TrimSpace(tagRe.ReplaceAllString(m[2], "")))
		if visible != style.Format(at) && !relativeRe.MatchString(visible) {
			out = append(out, "a <time> reads "+visible+", want "+style.Format(at)+" or a relative time")
		}
		full := style.Full(at)
		for _, name := range []string{"title", "data-full"} {
			if !strings.HasPrefix(attrs[name], full) {
				out = append(out, "a <time>'s "+name+" is "+attrs[name]+", want "+full)
			}
		}
		if !strings.Contains(attrs["aria-label"], full) {
			out = append(out, "a <time>'s aria-label does not carry "+full)
		}
		if attrs["tabindex"] != "0" {
			out = append(out, "a <time> that cannot take focus, so its tip is out of reach of a tap or a key: "+m[0])
		}
	}

	text := timeElementRe.ReplaceAllString(page, " ")
	text = dropRe.ReplaceAllString(text, " ")
	var human []string
	for _, a := range humanAttrRe.FindAllStringSubmatch(text, -1) {
		human = append(human, a[1])
	}
	body := html.UnescapeString(tagRe.ReplaceAllString(text, " "))
	body = strings.ReplaceAll(body, style.Format(formatDemo), "")
	for _, hit := range ownLayout(style).FindAllString(body, -1) {
		out = append(out, "a timestamp written as text rather than through when, so it has no tooltip: "+hit)
	}
	human = append(human, body)
	visible := html.UnescapeString(strings.Join(human, "\n"))
	for _, ex := range guardExamples() {
		visible = strings.ReplaceAll(visible, ex, "")
	}
	for _, re := range foreignDates {
		for _, hit := range re.FindAllString(visible, -1) {
			out = append(out, "a timestamp outside the reader's format: "+hit)
		}
	}
	return out
}

// formatDemo is the instant a format chooser demonstrates each layout with.
var formatDemo = time.Date(2026, time.September, 21, 14, 5, 9, 0, time.UTC)

// ownLayout matches an instant in the guard's own layout, which outside a
// <time> means a template printed it with compact or stamp instead of when.
// An attribute may (a title has no room for markup); text may not.
func ownLayout(style output.TimeStyle) *regexp.Regexp {
	if style.Name() != output.TimeDotted {
		panic("ownLayout knows only the dotted layout")
	}
	return regexp.MustCompile(`\b\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}\b`)
}

// guardRoutes resolves every screen the screenshot manifest captures, plus
// the fragments in guardFragments, to a path on the seeded database.
func guardRoutes(t *testing.T, b *browser) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "scripts", "screenshots", "routes.json"))
	if err != nil {
		t.Fatalf("reading the screenshot manifest: %v", err)
	}
	var m struct {
		Params map[string]struct {
			From   string `json:"from"`
			Match  string `json:"match"`
			Prefer string `json:"prefer"`
		} `json:"params"`
		Routes []struct {
			Pattern string            `json:"pattern"`
			Skip    string            `json:"skip"`
			Public  bool              `json:"public"`
			Params  map[string]string `json:"params"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parsing the screenshot manifest: %v", err)
	}
	resolved := map[string]string{}
	resolve := func(name string) string {
		if v, ok := resolved[name]; ok {
			return v
		}
		p := m.Params[name]
		hrefs := regexp.MustCompile(`href="([^"]+)"`).FindAllStringSubmatch(b.page(p.From), -1)
		re := regexp.MustCompile(p.Match)
		value := ""
		for _, h := range hrefs {
			if sm := re.FindStringSubmatch(html.UnescapeString(h[1])); sm != nil {
				if value == "" || sm[1] == p.Prefer {
					value = sm[1]
				}
			}
		}
		resolved[name] = value
		return value
	}
	refParams := map[string]string{"{ref}": "task", "{key}": "project"}
	var out []string
	for _, r := range m.Routes {
		if r.Public || (r.Skip != "" && !guardFragments[r.Pattern]) {
			continue
		}
		path, ok := r.Pattern, true
		for _, ph := range placeholderRe.FindAllStringSubmatch(r.Pattern, -1) {
			from := r.Params[ph[1]]
			if from == "" {
				from = refParams[ph[0]]
			}
			v := resolve(from)
			if v == "" {
				t.Errorf("%s: nothing on %s resolves {%s}; seed one", r.Pattern, m.Params[from].From, ph[1])
				ok = false
				break
			}
			path = strings.Replace(path, ph[0], v, 1)
		}
		if ok {
			out = append(out, path)
		}
	}
	return out
}

// seedGuardData writes a demo tenant's worth of history and one of each thing
// the seed does not write, so every screen that can show a timestamp has one.
func seedGuardData(t *testing.T, f *fixture) {
	t.Helper()
	admin := copyActor(f.actorA, []core.Scope{core.ScopeAll}, core.RoleAdmin)
	ctx := core.WithActor(context.Background(), admin)
	now := f.clock.Now()
	if _, err := demo.Seed(ctx, f.svc, f.clock, admin, demo.Options{Days: demo.MinDays, Now: now}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	expires := now.Add(72 * time.Hour)
	steps := []struct {
		what string
		do   func() error
	}{
		{"membership", func() error {
			_, err := f.svc.AddMember(ctx, f.actorA.ID, core.RoleAdmin)
			return err
		}},
		{"token", func() error {
			_, err := f.svc.CreateToken(ctx, core.CreateTokenInput{Name: "deploy", Scopes: []core.Scope{core.ScopeTaskRead}, ExpiresAt: &expires})
			return err
		}},
		{"filter", func() error {
			_, err := f.svc.CreateSavedFilter(ctx, core.SavedFilterInput{Name: "Urgent", Query: "priority:1", Shared: true})
			return err
		}},
		{"channel", func() error {
			_, err := f.svc.CreateInboxChannel(ctx, core.InboxChannelInput{Name: "Phone", URL: "https://ntfy.invalid/tix", Format: string(core.ChannelNtfy)})
			return err
		}},
		{"calendar", func() error {
			_, err := f.svc.CreateCalendarFeed(ctx, core.CalendarFeedInput{})
			return err
		}},
		{"webhook", func() error {
			_, err := f.svc.PutWebhook(ctx, core.WebhookInput{URL: "https://hooks.invalid/tix", EventTypes: []string{"task.created"}, Active: true})
			return err
		}},
	}
	for _, s := range steps {
		if err := s.do(); err != nil {
			t.Fatalf("seeding a %s: %v", s.what, err)
		}
	}
	tasks, err := f.svc.ListTasks(ctx, core.TaskFilter{})
	if err != nil || len(tasks.Tasks) == 0 {
		t.Fatalf("listing seeded tasks: %v", err)
	}
	if _, err := f.svc.SaveTemplate(ctx, core.SaveTemplateInput{Name: "Release checklist", TaskRef: tasks.Tasks[0].Ref}); err != nil {
		t.Fatalf("seeding a template: %v", err)
	}
}

// guardPage fetches a screen, following one redirect on this origin, as a
// saved filter's address leads to the task list it opens.
func guardPage(t *testing.T, b *browser, path string) string {
	t.Helper()
	resp := b.get(path)
	if loc := resp.Header.Get("Location"); resp.StatusCode == http.StatusSeeOther && strings.HasPrefix(loc, "/") {
		_ = resp.Body.Close()
		return b.page(loc)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", path, resp.StatusCode)
	}
	return readAll(t, resp)
}
