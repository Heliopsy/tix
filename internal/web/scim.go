// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

// scimTokenCookie carries a freshly minted provisioning token to the one
// screen that shows it.
const scimTokenCookie = "tix_scim_token" // #nosec G101 -- a cookie name, not a credential

// scimActivityShown is how many recent provider changes the screen lists.
const scimActivityShown = 10

// scimRoutes are the SCIM provisioning screen and its actions.
func (h *handler) scimRoutes() []route {
	return []route{
		get(RouteSCIM, "scim.html", h.showSCIM, "GetSCIM", "ListAudit"),
		post(RouteSCIMEnable, h.enableSCIM, "EnableSCIM"),
		post(RouteSCIMDisable, h.disableSCIM, "DisableSCIM"),
		post(RouteSCIMToken, h.rotateSCIMToken, "RotateSCIMToken"),
		post(RouteSCIMGroups, h.mapSCIMGroup, "MapSCIMGroup"),
		post(RouteSCIMUnmap, h.unmapSCIMGroup, "UnmapSCIMGroup"),
	}
}

// scimView is what the SCIM screen draws.
type scimView struct {
	Settings *core.SCIMSettings
	Secret   *oneTimeSecret
	Roles    []core.Role
	Activity []core.AuditEntry
}

// showSCIM draws the tenant's SCIM provisioning settings and what a provider
// changed lately.
func (h *handler) showSCIM(w http.ResponseWriter, r *http.Request) error {
	settings, err := h.svc.GetSCIM(r.Context())
	if err != nil {
		return err
	}
	if settings.URL == "" {
		settings.URL = httpapi.Origin(r) + settings.Path
	}
	view := scimView{Settings: settings, Roles: core.Roles,
		Secret: issuedTokenSecret(readOneTimeCookie(w, r, scimTokenCookie, RouteSCIM))}
	activity, _, err := h.svc.ListAudit(r.Context(), core.AuditFilter{Vias: []core.ViaKind{core.ViaSCIM},
		Page: core.Page{Limit: scimActivityShown, Direction: core.Descending}})
	if err == nil {
		view.Activity = activity
	}
	return h.render(w, r, "scim.html", "SCIM provisioning", view)
}

// scimMinted keeps a minted token for the screen to show once.
func (h *handler) scimMinted(w http.ResponseWriter, r *http.Request, s *core.SCIMSettings) {
	if s.Token != "" {
		// #nosec G124 -- one-time value, HttpOnly, cleared by the screen that shows it
		http.SetCookie(w, oneTimeCookie(scimTokenCookie, RouteSCIM,
			issuedValue(s.Token, []core.Scope{core.ScopeSCIMProvision}), h.secureCookie(r)))
	}
}

// enableSCIM turns provisioning on, minting the token the first time.
func (h *handler) enableSCIM(w http.ResponseWriter, r *http.Request) error {
	s, err := h.svc.EnableSCIM(r.Context())
	if err != nil {
		return err
	}
	h.scimMinted(w, r, s)
	redirect(w, r, RouteSCIM, "SCIM provisioning is on")
	return nil
}

// disableSCIM turns provisioning off.
func (h *handler) disableSCIM(w http.ResponseWriter, r *http.Request) error {
	if _, err := h.svc.DisableSCIM(r.Context()); err != nil {
		return err
	}
	redirect(w, r, RouteSCIM, "SCIM provisioning is off")
	return nil
}

// rotateSCIMToken mints a new provisioning token and retires the old one.
func (h *handler) rotateSCIMToken(w http.ResponseWriter, r *http.Request) error {
	s, err := h.svc.RotateSCIMToken(r.Context())
	if err != nil {
		return err
	}
	h.scimMinted(w, r, s)
	redirect(w, r, RouteSCIM, "token issued")
	return nil
}

// mapSCIMGroup maps a group to a role.
func (h *handler) mapSCIMGroup(w http.ResponseWriter, r *http.Request) error {
	in := core.SCIMGroupRoleInput{Group: field(r, "group"), Role: core.Role(field(r, "role"))}
	if _, err := h.svc.MapSCIMGroup(r.Context(), in); err != nil {
		return h.refuseIn(w, r, "scim-map", RouteSCIM, err, h.showSCIM)
	}
	redirect(w, r, RouteSCIM, "group mapped")
	return nil
}

// unmapSCIMGroup removes a group's mapping.
func (h *handler) unmapSCIMGroup(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.UnmapSCIMGroup(r.Context(), field(r, "group")); err != nil {
		return err
	}
	redirect(w, r, RouteSCIM, "mapping removed")
	return nil
}
