// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// requireMFA turns the tenant's second factor policy on, with a grace.
func (f *fixture) requireMFA(grace time.Duration) {
	f.t.Helper()
	ctx := context.Background()
	if err := f.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		tenant, err := u.GetTenantByID(ctx, f.tenantA.ID)
		if err != nil {
			return err
		}
		tenant.MFAPolicy = core.MFAPolicyRequired
		tenant.MFAGrace = core.Duration(grace)
		at := f.clock.Now()
		tenant.MFARequiredAt = &at
		return u.UpdateTenant(ctx, tenant)
	}); err != nil {
		f.t.Fatalf("requiring a second factor: %v", err)
	}
}

// htmx asks for a page the way an htmx swap does.
func (b *browser) htmx(path string) *http.Response {
	b.t.Helper()
	req, err := http.NewRequest(http.MethodGet, b.fix.server.URL+path, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("HX-Request", "true")
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	return resp
}

// Under the policy a person without a factor signs in to a session that
// reaches enrolment and nothing else, and the same session works once they
// enrol.
func TestAGatedSessionCanOnlyEnrolThenWorks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("gated@example.test", "correct-horse-battery")
	f.requireMFA(0)

	signIn := f.as("").post("/login", url.Values{
		"email": {"gated@example.test"}, "password": {"correct-horse-battery"}, "next": {"/tasks"}})
	_ = signIn.Body.Close()
	if signIn.StatusCode != http.StatusSeeOther ||
		!strings.Contains(signIn.Header.Get("Set-Cookie"), auth.SessionCookieName) {
		t.Fatalf("the password step answered %d without a session, want one the gate confines", signIn.StatusCode)
	}

	b := f.asActor(user.ID)
	tasks := b.get("/tasks")
	_ = tasks.Body.Close()
	if tasks.StatusCode != http.StatusSeeOther || tasks.Header.Get("Location") != "/settings/totp" {
		t.Fatalf("GET /tasks = %d to %q, want a redirect to /settings/totp",
			tasks.StatusCode, tasks.Header.Get("Location"))
	}
	swap := b.htmx("/tasks")
	_ = swap.Body.Close()
	if swap.StatusCode != http.StatusForbidden || swap.Header.Get("HX-Redirect") != "/settings/totp" {
		t.Fatalf("an htmx GET /tasks = %d, HX-Redirect %q; want 403 to /settings/totp",
			swap.StatusCode, swap.Header.Get("HX-Redirect"))
	}

	screen := b.page("/settings/totp")
	totpForm(t, screen, "/settings/totp")
	totpForm(t, screen, "/logout")
	if !strings.Contains(screen, "Until you add one you can only set it up") {
		t.Errorf("the enrolment screen does not say why the reader is there:\n%s", screen)
	}
	if got := b.get("/account"); got.StatusCode != http.StatusOK {
		t.Errorf("the account history = %d under the gate, want 200", got.StatusCode)
	}

	enrol(t, f, b)
	if got := b.get("/tasks"); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /tasks after enrolling = %d, want 200", got.StatusCode)
	}
}

// During the grace a reader without a factor works as before, with a banner
// that names the deadline and leads to enrolment.
func TestTheGraceIsAnnouncedOnEveryPage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("grace@example.test", "correct-horse-battery")
	f.requireMFA(7 * 24 * time.Hour)

	page := f.asActor(user.ID).page("/tasks")
	banner := between(t, page, `<p class="flash factorbanner"`, "</p>")
	if !strings.Contains(banner, `href="/settings/totp"`) || !strings.Contains(banner, "requires a second factor from") {
		t.Fatalf("the grace banner = %q, want the deadline and a link to enrolment", banner)
	}
}

// The Users screen can be narrowed to people without a factor.
func TestTheUserListFiltersPeopleWithoutAFactor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	enrolled := f.person("has@example.test", "correct-horse-battery")
	f.person("lacks@example.test", "correct-horse-battery")
	enrol(t, f, f.asActor(enrolled.ID))

	page := f.asActor(f.actorA.ID).page("/admin/users?without_totp=1")
	list := between(t, page, `<ul class="peoplelist rows">`, "</ul>")
	if !strings.Contains(list, "lacks@example.test") || strings.Contains(list, "has@example.test") {
		t.Fatalf("the filtered list = %s, want lacks alone", list)
	}
}
