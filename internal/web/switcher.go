// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// RouteSwitch is the Choose a tenant screen and the form moving a session to
// another tenant on this host.
const RouteSwitch = "/switch"

// switcherDialog is the id of the header's tenant dialog, so ?dialog=tenants
// draws it in place without scripting.
const switcherDialog = "tenants"

// switchTemplate is the Choose a tenant screen.
const switchTemplate = "switch.html"

// switchRoutes are the tenant switcher's screen and form.
func (h *handler) switchRoutes() []route {
	return []route{
		get(RouteSwitch, switchTemplate, h.showSwitch, "WhoAmI"),
		post(RouteSwitch, h.doSwitch, "SwitchTenant"),
	}
}

// tenantSwitcher is the reader's tenants, for the header's dialog and the
// Choose a tenant screen. It is drawn only for somebody who belongs to more
// than one.
type tenantSwitcher struct {
	Current string
	Rows    []switcherRow
	// GlobalAdmin links the dialog to the Global admin area, where a global
	// admin reaches tenants they are no member of; those are not listed here.
	GlobalAdmin bool
	CSRF        string
	Here        string
	Next        string
	Opener      dialogOpener
}

// switcherRow is one of the reader's tenants.
type switcherRow struct {
	Key  string
	Name string
	Role core.Role
	// Current marks the tenant this session is in; it has no button.
	Current bool
	// Switch offers the Switch button: the tenant is served on this host.
	Switch bool
	// Link is the tenant's own sign-in page, for a tenant served at an
	// address of its own.
	Link string
}

// Many reports whether the reader has a tenant to move to at all.
func (s *tenantSwitcher) Many() bool { return s != nil && len(s.Rows) > 1 }

// Choices counts the tenants the reader could pick on this host.
func (s *tenantSwitcher) Choices() int {
	if s == nil {
		return 0
	}
	n := 0
	for _, r := range s.Rows {
		if r.Current || r.Switch {
			n++
		}
	}
	return n
}

// switcherFor builds the reader's switcher from their whoami answer, which
// lists memberships only for a session. A tenant this host serves is a
// Switch button; one at an address of its own is a link to its sign-in page;
// one this host cannot serve and that has no address is listed plainly.
func (h *handler) switcherFor(r *http.Request, me *core.Actor, here string) *tenantSwitcher {
	if me == nil || len(me.Memberships) < 2 {
		return nil
	}
	served, _ := core.ServedHostFrom(r.Context())
	scheme := "http"
	if h.secureCookie(r) {
		scheme = "https"
	}
	name := me.TenantName
	out := &tenantSwitcher{Current: me.TenantKey, GlobalAdmin: me.GlobalAdmin, CSRF: csrfFrom(r), Here: here}
	for _, m := range me.Memberships {
		row := switcherRow{Key: m.TenantKey, Name: m.TenantName, Role: m.Role, Current: m.TenantID == me.TenantID}
		switch {
		case row.Current:
			name = m.TenantName
		case served.Serves(m.TenantID, m.Host):
			row.Switch = true
		case m.Host != "":
			row.Link = (&url.URL{Scheme: scheme, Host: m.Host, Path: RouteLogin}).String()
		}
		out.Rows = append(out.Rows, row)
	}
	out.Opener = newOpener(switcherDialog, name, "tenant", "", here)
	return out
}

// whoAmI is the reader's identity for the page frame, or nil when there is
// none to show.
func (h *handler) whoAmI(r *http.Request) *core.Actor {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return nil
	}
	me, err := h.svc.WhoAmI(r.Context())
	if err != nil {
		return nil
	}
	return me
}

// switchView is the Choose a tenant screen, and the code step of a switch to
// a tenant that asks for a second factor this session did not pass.
type switchView struct {
	Switcher *tenantSwitcher
	Next     string
	// Code names the tenant a code is being asked for, on the code step.
	Code   string
	Errors fieldErrors
}

// showSwitch draws the Choose a tenant screen, which a sign-in on a shared
// host lands on when the account has more than one tenant here. With nothing
// to choose it goes straight on.
func (h *handler) showSwitch(w http.ResponseWriter, r *http.Request) error {
	next := safeNext(r.URL.Query().Get("next"))
	me, err := h.svc.WhoAmI(r.Context())
	if err != nil {
		return err
	}
	sw := h.switcherFor(r, me, RouteSwitch)
	if sw.Choices() < 2 && r.URL.Query().Get("next") != "" {
		// #nosec G710 -- safeNext keeps the target on this origin.
		http.Redirect(w, r, next, http.StatusSeeOther)
		return nil
	}
	return h.render(w, r, switchTemplate, "Choose a tenant", switchView{Switcher: sw, Next: next})
}

// doSwitch moves the session to the tenant the form names. The new session
// replaces the cookie, and the reader lands on the projects of the tenant
// they moved to, since where they were belongs to the tenant they left.
func (h *handler) doSwitch(w http.ResponseWriter, r *http.Request) error {
	key := field(r, "tenant")
	result, err := h.svc.SwitchTenant(r.Context(), core.SwitchTenantInput{
		Tenant: key, Code: field(r, "code"), Surface: core.SurfaceWeb,
	})
	if err != nil {
		if _, sentCode := sent(r, "code"); sentCode && refusable(err) {
			return h.renderSwitchCode(w, r, core.KindOf(err).HTTPStatus(), key,
				fieldErrors{"code": safeMessage(err, core.KindOf(err))})
		}
		return err
	}
	if result.Challenge != nil {
		return h.renderSwitchCode(w, r, http.StatusOK, key, nil)
	}
	session := result.Session
	http.SetCookie(w, auth.NewSessionCookie(session.Token, session.ExpiresAt, h.secureCookie(r)))
	http.Redirect(w, r, RouteProjects, http.StatusSeeOther)
	return nil
}

// renderSwitchCode draws the code step of a switch, refusal and all.
func (h *handler) renderSwitchCode(w http.ResponseWriter, r *http.Request, status int, key string, errs fieldErrors) error {
	return h.renderStatus(w, r, status, switchTemplate, "Second factor",
		switchView{Code: key, Next: RouteProjects, Errors: errs})
}
