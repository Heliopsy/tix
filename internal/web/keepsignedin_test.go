// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
)

// sessionCookie is the session cookie a response set, or nil.
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	return nil
}

func TestKeepMeSignedInDecidesTheCookieLifetime(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	created := f.as("alice").post("/admin/users", url.Values{
		"email": {"kept@example.test"}, "password": {"correct-horse-battery"},
		"display_name": {"Kept"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := body(t, f.as("").get("/login"))
	form := formAt(t, page, "/login")
	if !strings.Contains(form, `name="remember"`) || !strings.Contains(form, "Keep me signed in") {
		t.Fatalf("the sign-in form offers no Keep me signed in:\n%s", form)
	}

	plain := f.as("").post("/login", url.Values{
		"email": {"kept@example.test"}, "password": {"correct-horse-battery"}})
	_ = plain.Body.Close()
	if c := sessionCookie(plain); c == nil || c.MaxAge != 0 || !c.Expires.IsZero() {
		t.Errorf("an ordinary sign-in's cookie = %+v, want one that ends with the browser", c)
	}

	kept := f.as("").post("/login", url.Values{
		"email": {"kept@example.test"}, "password": {"correct-horse-battery"}, "remember": {"on"}})
	_ = kept.Body.Close()
	if c := sessionCookie(kept); c == nil || c.MaxAge < 29*24*3600 {
		t.Errorf("a remembered sign-in's cookie = %+v, want a Max-Age of about 30 days", c)
	}
}

func TestATenantThatTurnsKeepMeSignedInOffHidesTheBox(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	saved := f.as("alice").post("/admin/tenant", url.Values{
		"name": {"Acme Works"}, "session_remember_ttl": {"off"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	form := formAt(t, body(t, f.as("").get("/login")), "/login")
	if strings.Contains(form, `name="remember"`) {
		t.Errorf("the box is drawn on a tenant that turned it off:\n%s", form)
	}
	tenant := body(t, f.as("alice").get("/admin/tenant"))
	if !strings.Contains(tenant, `name="session_remember_ttl"`) || !strings.Contains(tenant, `value="off"`) {
		t.Errorf("Tenant settings does not show keep me signed in as off")
	}
}

func TestTenantSettingsSaveAndRefuseSessionDurations(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	saved := f.as("alice").post("/admin/tenant", url.Values{
		"name": {"Acme Works"}, "session_ttl": {"8h"}, "session_idle_timeout": {"30m"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)
	page := body(t, f.as("alice").get("/admin/tenant"))
	if !strings.Contains(page, `value="8h"`) || !strings.Contains(page, `value="30m"`) {
		t.Errorf("Tenant settings does not show the saved 8h and 30m")
	}
	bad := f.as("alice").post("/admin/tenant", url.Values{"name": {"Acme Works"}, "session_idle_timeout": {"soon"}})
	_ = bad.Body.Close()
	if bad.StatusCode == http.StatusSeeOther {
		t.Error("an unreadable idle timeout was saved")
	}
}

func TestKeepMeSignedInNoLongerThanASignInIsNotOffered(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	saved := f.as("alice").post("/admin/tenant", url.Values{
		"name": {"Acme Works"}, "session_remember_ttl": {"6h"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	form := formAt(t, body(t, f.as("").get("/login")), "/login")
	if strings.Contains(form, `name="remember"`) {
		t.Errorf("a 6h keep me signed in under a 12h sign-in is offered, though it grants nothing:\n%s", form)
	}
}
