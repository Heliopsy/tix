// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/web"
)

// The date format and the timezone are per-browser preferences, the same way
// the theme and the keyboard scheme are, so the settings page has to offer
// both as plain forms and remember what was chosen.
func TestDatePreferencesPersistPerBrowser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		action string
		field  string
		value  string
		option string
	}{
		{"timezone", "/timezone", "timezone", "Asia/Tokyo",
			`<option value="Asia/Tokyo" selected>Asia/Tokyo</option>`},
		{"format", "/timeformat", "time_format", "us",
			`<option value="us" selected>US (`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			b := f.as("alice")

			page := b.page("/settings")
			if !hasPlainForm(page, c.action) {
				t.Fatalf("the settings page offers no %s control:\n%s", c.name, page)
			}

			resp := b.post(c.action, url.Values{c.field: {c.value}, "next": {"/tasks"}})
			_ = resp.Body.Close()
			wantStatus(t, resp, http.StatusSeeOther)

			settings := b.page("/settings")
			if !strings.Contains(settings, c.option) {
				t.Errorf("the %s choice did not stick across a reload:\n%s", c.name, settings)
			}
		})
	}
}

// A value submitted by hand, or left behind in a cookie by something else,
// falls back to the deployment default rather than being stored verbatim, the
// same defence setKeyScheme applies to an unknown scheme.
func TestUnknownDatePreferenceFallsBackToTheDefault(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		action string
		field  string
		value  string
		cookie string
	}{
		{"unknown zone", "/timezone", "timezone", "Mars/Olympus", "tix_timezone"},
		{"zone off the list", "/timezone", "timezone", "Antarctica/Troll", "tix_timezone"},
		{"unknown format", "/timeformat", "time_format", "klingon", "tix_time_format"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			b := f.as("alice")

			resp := b.post(c.action, url.Values{c.field: {c.value}, "next": {"/tasks"}})
			_ = resp.Body.Close()

			if stored := b.cookie(c.cookie); stored != "" {
				t.Errorf("%s was stored as %q, want the default", c.name, stored)
			}
			page := b.page("/settings")
			if !strings.Contains(page, `<option value="" selected>Server default</option>`) {
				t.Errorf("%s did not fall back to the server default:\n%s", c.name, page)
			}
			if strings.Contains(page, c.value) {
				t.Errorf("%s reached the page verbatim:\n%s", c.name, page)
			}
		})
	}

	// A cookie planted directly, never through the form, is refused the same
	// way: validation is on the way out as well as on the way in.
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie("tix_timezone", "Mars/Olympus")
	b.setCookie("tix_time_format", "klingon")
	created, ref := seedTimedTask(t, f, b)
	page := b.page("/tasks/" + ref)
	if want := formatIn(t, "", "", created); !strings.Contains(page, want) {
		t.Errorf("a planted cookie displaced the default rendering %q:\n%s", want, page)
	}
}

// The zone a browser chose is the zone its own timestamps render in, while the
// deployment's configured zone stays the default for a browser that chose
// nothing.
func TestTimezonePreferenceRendersItsOwnZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	chooser := f.as("alice")
	created, ref := seedTimedTask(t, f, chooser)

	plain := f.as("alice")
	if want := formatIn(t, "", "", created); !strings.Contains(plain.page("/tasks/"+ref), want) {
		t.Errorf("a browser that chose nothing does not read the deployment default %q", want)
	}

	resp := chooser.post("/timezone", url.Values{"timezone": {"Asia/Tokyo"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	page := chooser.page("/tasks/" + ref)
	want := formatIn(t, "", "Asia/Tokyo", created)
	if !strings.Contains(page, want) {
		t.Errorf("the chosen zone did not reach the task screen, want %q:\n%s", want, page)
	}
}

// The failure this whole design guards against: the formatting helpers are
// bound into a template at parse time, so a style shared between requests
// would serve one browser the zone another one chose. Two browsers hammer the
// same task at once, each holding a different preference, and each response
// has to carry that browser's own rendering and none of the other's.
func TestConcurrentBrowsersNeverSeeAnotherZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seeder := f.as("alice")
	created, ref := seedTimedTask(t, f, seeder)

	// The last reader chose nothing, so it reads the deployment default,
	// which here is whatever zone the host runs in. The zones the other two
	// read in are therefore picked against that rather than named outright:
	// hardcoding them made this suite go red on a machine set to one of them,
	// which says nothing about the product.
	zones := zonesDistinctFromTheHost(t, created, 2)
	readers := []struct {
		zone   string
		format string
	}{
		{zones[0], ""},
		{zones[1], ""},
		{"UTC", "us"},
		{"", ""},
	}
	stamps := make([]string, len(readers))
	browsers := make([]*browser, len(readers))
	for i, r := range readers {
		browsers[i] = f.as("alice")
		stamps[i] = formatIn(t, r.format, r.zone, created)
		if r.zone != "" {
			resp := browsers[i].post("/timezone", url.Values{"timezone": {r.zone}, "next": {"/tasks"}})
			_ = resp.Body.Close()
		}
		if r.format != "" {
			resp := browsers[i].post("/timeformat", url.Values{"time_format": {r.format}, "next": {"/tasks"}})
			_ = resp.Body.Close()
		}
	}
	for i, mine := range stamps {
		for j, other := range stamps {
			if i != j && mine == other {
				t.Fatalf("readers %d and %d render the same stamp %q, so the test proves nothing", i, j, mine)
			}
		}
	}

	const rounds = 25
	var wg sync.WaitGroup
	fail := make(chan string, len(readers)*rounds)
	start := make(chan struct{})
	for i := range readers {
		for range rounds {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				page := browsers[i].page("/tasks/" + ref)
				if !strings.Contains(page, stamps[i]) {
					fail <- "reader " + stamps[i] + " did not get its own rendering"
					return
				}
				for j, other := range stamps {
					if j != i && strings.Contains(page, other) {
						fail <- "reader " + stamps[i] + " was served " + other
						return
					}
				}
			}(i)
		}
	}
	close(start)
	wg.Wait()
	close(fail)
	for msg := range fail {
		t.Error(msg)
	}
}

// seedTimedTask creates one task and reports the instant it carries, which the
// fixture's fake clock makes exact.
func seedTimedTask(t *testing.T, f *fixture, b *browser) (time.Time, string) {
	t.Helper()
	created := f.clock.Now()
	return created, b.createTask("infra", "timezone subject")
}

// zonesDistinctFromTheHost picks n offered zones that read differently
// from each other and from the host's own zone, so every reader in the test
// above carries a stamp no other reader can also be holding.
func zonesDistinctFromTheHost(t *testing.T, at time.Time, n int) []string {
	t.Helper()
	seen := map[string]bool{formatIn(t, "", "", at): true}
	out := make([]string, 0, n)
	for _, zone := range web.Timezones {
		if zone == "" {
			continue
		}
		if _, err := time.LoadLocation(zone); err != nil {
			continue
		}
		stamp := formatIn(t, "", zone, at)
		if seen[stamp] {
			continue
		}
		seen[stamp] = true
		if out = append(out, zone); len(out) == n {
			return out
		}
	}
	t.Fatalf("this host offers fewer than %d zones that read differently from its own", n)
	return nil
}

// formatIn renders an instant the way a browser holding those preferences
// reads it, through the renderer the screens themselves use.
func formatIn(t *testing.T, format, zone string, at time.Time) string {
	t.Helper()
	style, err := output.NewTimeStyle(format, zone)
	if err != nil {
		t.Fatalf("NewTimeStyle(%q, %q): %v", format, zone, err)
	}
	return style.Format(at)
}
