// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/heliopsy/tix/internal/core"
)

// profileView is the Profile section of the settings screen: who the reader
// is, as WhoAmI answers it, email and memberships included where WhoAmI
// gives them, so the section never shows more than the API tells the reader.
type profileView struct {
	Actor      core.Actor
	TenantKey  string
	TenantName string
	Scopes     []core.Scope
	// ViaToken reports a request that authenticated with an API token, and
	// Token names it; anything else is the browser's session.
	ViaToken bool
	Token    string
}

// ActorHref is the reader's own actor screen.
func (p profileView) ActorHref() string { return actorHref(p.Actor.ID) }

// buildProfile asks the service who the reader is.
func (h *handler) buildProfile(r *http.Request) (profileView, error) {
	me, err := h.svc.WhoAmI(r.Context())
	if err != nil {
		return profileView{}, err
	}
	out := profileView{Actor: *me, Scopes: grantedScopes(me),
		ViaToken: me.TokenID != "", Token: me.TokenName}
	out.TenantKey, out.TenantName = me.TenantKey, me.TenantName
	return out, nil
}

// grantedScopes are the scopes an actor holds, granted directly or by its role,
// sorted and without repeats.
func grantedScopes(a *core.Actor) []core.Scope {
	out := slices.Clone(a.Scopes)
	if a.Role != "" {
		out = append(out, a.Role.Scopes()...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// actorHref is the actor screen of one identifier or handle.
func actorHref(ref string) string {
	if ref == "" {
		return ""
	}
	return "/actors/" + url.PathEscape(ref)
}
