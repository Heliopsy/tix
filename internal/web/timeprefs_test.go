// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/web"
)

// makePerson makes actorID a person of the fixture tenant: an account, which
// is where their own date preference is kept, and an actor with that ID
// linked to it. The fixture's alice is one already; its other actors have no
// account, the way an agent has none.
func makePerson(t *testing.T, f *fixture, actorID, email string) {
	t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, core.TenantScope{TenantID: f.tenantA.ID}, func(tx store.Tx) error {
		switch a, err := tx.GetActor(ctx, actorID); {
		case err == nil && a.UserID != "":
			return nil
		case err == nil:
			t.Fatalf("actor %q exists with no account; seed it with seedPerson", actorID)
		case !core.IsKind(err, core.KindNotFound):
			return err
		}
		user := &core.User{Email: email}
		if err := tx.CreateUser(ctx, user, "unused-hash"); err != nil {
			return err
		}
		return tx.CreateActor(ctx, &core.Actor{ID: actorID, UserID: user.ID, Kind: core.ActorUser, Handle: actorID})
	}); err != nil {
		t.Fatalf("creating user %q: %v", email, err)
	}
}

// personFixture is a fixture whose alice is a person, which every fixture's
// alice now is.
func personFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixture(t)
}

// setTenantTime sets the tenant's default date format and timezone through
// the tenant screen, the way an administrator does.
func setTenantTime(t *testing.T, b *browser, format, zone string) {
	t.Helper()
	page := b.page("/admin/tenant")
	name := between(t, page, `id="name" name="name" value="`, `"`)
	resp := b.post("/admin/tenant", url.Values{"name": {name}, "theme": {""},
		"time_format": {format}, "timezone": {zone}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
}

// A date preference chosen in one browser is kept with the account, so a
// second browser signed in as the same person reads it without choosing.
func TestDatePreferencesFollowThePersonToAnotherBrowser(t *testing.T) {
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
			f := personFixture(t)
			first := f.as("alice")
			if page := first.page("/settings"); !hasPlainForm(page, c.action) {
				t.Fatalf("the settings page offers no %s control:\n%s", c.name, page)
			}
			resp := first.post(c.action, url.Values{c.field: {c.value}, "next": {"/tasks"}})
			_ = resp.Body.Close()
			wantStatus(t, resp, http.StatusSeeOther)

			second := f.as("alice")
			if page := second.page("/settings"); !strings.Contains(page, c.option) {
				t.Errorf("another browser does not read the %s the person chose:\n%s", c.name, page)
			}
			for _, cookie := range []string{web.TimeFormatCookie, web.TimezoneCookie} {
				if v := first.cookie(cookie); v != "" {
					t.Errorf("the choice was also kept in the %s cookie as %q", cookie, v)
				}
			}
		})
	}
}

// A value the renderer does not know is refused and nothing is stored.
func TestUnknownDatePreferenceIsRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, action, field, value string
	}{
		{"unknown zone", "/timezone", "timezone", "Mars/Olympus"},
		{"server zone", "/timezone", "timezone", "local"},
		{"unknown format", "/timeformat", "time_format", "klingon"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := personFixture(t)
			b := f.as("alice")
			resp := b.post(c.action, url.Values{c.field: {c.value}, "next": {"/tasks"}})
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusSeeOther {
				t.Fatalf("%s %q was accepted", c.field, c.value)
			}
			page := b.page("/settings")
			control := between(t, page, `<select id="`+c.field+`"`, "</select>")
			if !strings.Contains(control, `<option value="" selected>`) || strings.Contains(control, c.value+`" selected`) {
				t.Errorf("%s did not stay on inherit:\n%s", c.name, control)
			}
		})
	}
}

// The settings screen says what is in use and where it came from, and what
// Inherit would give, so "inherit" never reads as an unknown.
func TestSettingsSayWhereEachDatePreferenceComesFrom(t *testing.T) {
	t.Parallel()
	f := personFixture(t)
	b := f.as("alice")
	setTenantTime(t, b, "dotted", "Europe/Sofia")

	page := b.page("/settings")
	help := between(t, page, `id="time-format-help">`, "</span></span>")
	if !strings.Contains(help, "Dotted") || !strings.Contains(help, "from tenant default") {
		t.Errorf("the format help does not say dotted from the tenant default: %q", help)
	}
	if got := optionLabel(t, page, "time_format", ""); !strings.Contains(got, "Inherit: Dotted") || !strings.Contains(got, "tenant default") {
		t.Errorf("the inherit option does not name what it inherits: %q", got)
	}

	resp := b.post("/timezone", url.Values{"timezone": {"Asia/Tokyo"}, "next": {"/settings"}})
	_ = resp.Body.Close()
	page = b.page("/settings")
	zone := between(t, page, `id="timezone-help">`, "</span></span>")
	if !strings.Contains(zone, "Asia/Tokyo") || !strings.Contains(zone, "from your choice") {
		t.Errorf("the zone help does not say Tokyo from the person's choice: %q", zone)
	}
	if got := optionLabel(t, page, "timezone", ""); !strings.Contains(got, "Europe/Sofia") {
		t.Errorf("the zone inherit option does not name the tenant's zone: %q", got)
	}
}

// The tenant screen sets the default every member reads until they choose,
// and says what the server would give when it inherits.
func TestTenantDefaultReachesAMemberWhoChoseNothing(t *testing.T) {
	t.Parallel()
	f := personFixture(t)
	admin := f.as("alice")
	created, ref := seedTimedTask(t, f, admin)

	page := admin.page("/admin/tenant")
	if got := optionLabel(t, page, "time_format", ""); !strings.Contains(got, "Inherit:") || !strings.Contains(got, "server default") {
		t.Errorf("the tenant's inherit option does not name the server default: %q", got)
	}
	setTenantTime(t, admin, "long", "Asia/Tokyo")
	page = admin.page("/admin/tenant")
	if !strings.Contains(between(t, page, `id="tenant-format-help">`, "</p>"), "from tenant default") {
		t.Errorf("the tenant screen does not say its own default is in use")
	}

	makePerson(t, f, "member-1", "m@example.com")
	member := f.as("member:member-1")
	want := formatIn(t, "long", "Asia/Tokyo", created)
	if got := member.page("/tasks/" + ref); !showsAt(got, created, want) {
		t.Errorf("a member who chose nothing does not read the tenant default %q", want)
	}
}

// A person's own choice wins over the tenant's, field by field: a zone of
// their own keeps the tenant's format.
func TestAPersonsChoiceOverridesTheTenantFieldByField(t *testing.T) {
	t.Parallel()
	f := personFixture(t)
	b := f.as("alice")
	created, ref := seedTimedTask(t, f, b)
	setTenantTime(t, b, "dotted", "Europe/Sofia")
	resp := b.post("/timezone", url.Values{"timezone": {"Asia/Tokyo"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	want := formatIn(t, "dotted", "Asia/Tokyo", created)
	if got := b.page("/tasks/" + ref); !showsAt(got, created, want) {
		t.Errorf("want the tenant's format in the person's zone, %q", want)
	}
}

// A browser still carrying the old per-browser cookies has them moved onto
// the account on its next visit and dropped, so the choice now follows the
// person; a field the account already holds is not overwritten.
func TestLegacyDateCookiesMoveOntoTheAccount(t *testing.T) {
	t.Parallel()
	f := personFixture(t)
	created, ref := seedTimedTask(t, f, f.as("alice"))

	elsewhere := f.as("alice")
	resp := elsewhere.post("/timeformat", url.Values{"time_format": {"long"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	old := f.as("alice")
	old.setCookie(web.TimeFormatCookie, "us")
	old.setCookie(web.TimezoneCookie, "Asia/Tokyo")
	want := formatIn(t, "long", "Asia/Tokyo", created)
	if got := old.page("/tasks/" + ref); !showsAt(got, created, want) {
		t.Errorf("the first visit does not read the moved zone with the account's format, want %q", want)
	}
	for _, name := range []string{web.TimeFormatCookie, web.TimezoneCookie} {
		if v := old.cookie(name); v != "" {
			t.Errorf("the %s cookie is still set to %q after the move", name, v)
		}
	}
	fresh := f.as("alice")
	if got := fresh.page("/tasks/" + ref); !showsAt(got, created, want) {
		t.Errorf("another browser does not read the moved zone, want %q", want)
	}
}

// A planted cookie naming something the renderer does not know is dropped
// rather than moved.
func TestABadLegacyCookieIsDroppedNotMoved(t *testing.T) {
	t.Parallel()
	f := personFixture(t)
	b := f.as("alice")
	b.setCookie(web.TimezoneCookie, "Mars/Olympus")
	b.setCookie(web.TimeFormatCookie, "klingon")
	page := b.page("/settings")
	if strings.Contains(page, "Mars") || strings.Contains(page, "klingon") {
		t.Errorf("a planted cookie reached the page")
	}
	if b.cookie(web.TimezoneCookie) != "" || b.cookie(web.TimeFormatCookie) != "" {
		t.Errorf("the planted cookies were kept")
	}
}

// The formatting helpers are bound into a template at parse time, so a style
// shared between requests would serve one person the zone another chose. Four
// people read the same task at once, each with their own preference.
func TestConcurrentPeopleNeverSeeAnotherZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	created, ref := seedTimedTask(t, f, f.as("alice"))

	zones := zonesDistinctFromTheHost(t, created, 2)
	readers := []struct{ zone, format string }{
		{zones[0], ""}, {zones[1], ""}, {"UTC", "us"}, {"", ""},
	}
	stamps := make([]string, len(readers))
	browsers := make([]*browser, len(readers))
	for i, r := range readers {
		id := "reader-" + string(rune('a'+i))
		makePerson(t, f, id, id+"@example.com")
		browsers[i] = f.as("member:" + id)
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
				if !showsAt(page, created, stamps[i]) {
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
// from each other and from the host's own zone.
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

// optionLabel is the words one option of a named select carries.
func optionLabel(t *testing.T, page, selectID, value string) string {
	t.Helper()
	control := between(t, page, `<select id="`+selectID+`"`, "</select>")
	if control == "" {
		t.Fatalf("the page has no %q control:\n%s", selectID, page)
	}
	return between(t, control, `<option value="`+value+`"`, "</option>")
}

// Every date format is offered with an example of itself.
func TestEachDateFormatIsOfferedWithAnExampleOfItself(t *testing.T) {
	t.Parallel()
	page := personFixture(t).as("alice").page("/settings")

	iso := optionLabel(t, page, "time_format", output.TimeISO)
	if !strings.Contains(iso, "2026-09-21") {
		t.Errorf("the ISO option is not demonstrated with the fixed example: %q", iso)
	}
	for format, want := range map[string]string{
		output.TimeDotted: "21.09.2026", output.TimeDayFirst: "21/09/2026",
		output.TimeLong: "21 Sep 2026", output.TimeUS: "09/21/2026",
	} {
		if got := optionLabel(t, page, "time_format", format); !strings.Contains(got, want) {
			t.Errorf("the %s option is not demonstrated as %s: %q", format, want, got)
		}
	}
	relative := optionLabel(t, page, "time_format", output.TimeRelative)
	if !strings.Contains(relative, "18 minutes ago") {
		t.Errorf("the relative option does not read as a moment in the past: %q", relative)
	}
}

// formatIn renders an instant the way a reader holding those preferences
// reads it, through the renderer the screens themselves use.
func formatIn(t *testing.T, format, zone string, at time.Time) string {
	t.Helper()
	style, err := output.NewTimeStyle(format, zone)
	if err != nil {
		t.Fatalf("NewTimeStyle(%q, %q): %v", format, zone, err)
	}
	return style.Format(at)
}

// showsAt reports whether the page shows the instant at as text, in a <time>
// whose machine value is that instant. Matching the text alone could be
// satisfied by some other instant on the page that happens to read the same.
func showsAt(page string, at time.Time, text string) bool {
	re := regexp.MustCompile(`<time [^>]*datetime="` + regexp.QuoteMeta(at.UTC().Format(time.RFC3339)) + `"[^>]*>` + regexp.QuoteMeta(text) + `</time>`)
	return re.MatchString(page)
}

// readers numbers the people reader creates, so each has its own handle.
var readers atomic.Int64

// reader returns a browser for a new administrator of the fixture tenant who
// reads dates in format and zone, either of which may be empty to inherit.
// Each call is a separate person, because a date preference is the
// account's, and two subtests sharing one account would race over it.
func (f *fixture) reader(t *testing.T, format, zone string) *browser {
	t.Helper()
	handle := "reader" + strconv.FormatInt(readers.Add(1), 10)
	actor := seedPerson(t, f.store, f.tenantA.ID, handle, handle+"@example.com", core.RoleAdmin)
	b := f.asActor(actor.ID)
	for _, set := range []struct{ route, field, value string }{
		{"/timeformat", "time_format", format}, {"/timezone", "timezone", zone},
	} {
		if set.value == "" {
			continue
		}
		resp := b.post(set.route, url.Values{set.field: {set.value}, "next": {"/tasks"}})
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
	}
	return b
}
