// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// ssoRoutes are signing in through a provider and the screen that manages
// the tenant's providers.
func (h *handler) ssoRoutes() []route {
	return []route{
		public(get(RouteSSOStart, "", h.startSSO, "BeginSSOLogin")),
		public(get(RouteSSOCallback, totpTemplate, h.finishSSO, "CompleteSSOLogin")),
		get(RouteSSO, ssoTemplate, h.showSSO, "ListSSOProviders", "SignInOptions"),
		post(RouteSSO, h.createSSO, "CreateSSOProvider"),
		post(RouteSSOUpdate, h.updateSSO, "UpdateSSOProvider"),
		post(RouteSSODelete, h.deleteSSO, "DeleteSSOProvider"),
		post(RouteSSOPassword, h.setPasswordLogin, "UpdateTenant"),
	}
}

// ssoTemplate is the Single sign-on screen.
const ssoTemplate = "sso.html"

// ssoFlowPath scopes the flow cookie to the two addresses that read it.
const ssoFlowPath = "/login/sso"

// startSSO sends the browser to a provider. The sealed flow travels in an
// HttpOnly cookie, which is what binds the provider's answer to this browser:
// a callback carrying somebody else's code and state finds no flow, or one
// whose state does not match.
func (h *handler) startSSO(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	begin, err := h.svc.BeginSSOLogin(r.Context(), core.BeginSSOInput{
		Provider:    r.PathValue("key"),
		RedirectURI: baseURL(r) + RouteSSOCallback,
		Surface:     core.SurfaceWeb,
		Remember:    checkedValue(q.Get("remember")),
		Next:        safeNext(orElse(q.Get("next"), RouteRoot)),
	})
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- HttpOnly and SameSite always; Secure tracks TLS, as the session cookie does
		Name: auth.SSOFlowCookieName, Value: begin.Flow, Path: ssoFlowPath, HttpOnly: true,
		Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: int(auth.SSOFlowTTL.Seconds()),
	})
	// #nosec G710 -- the provider's own authorization address, built by the
	// service from the provider's configuration, not from this request.
	http.Redirect(w, r, begin.AuthURL, http.StatusSeeOther)
	return nil
}

// finishSSO is where every provider returns the browser.
func (h *handler) finishSSO(w http.ResponseWriter, r *http.Request) error {
	// #nosec G124 -- clears the flow cookie; HttpOnly and SameSite as it was set.
	http.SetCookie(w, &http.Cookie{Name: auth.SSOFlowCookieName, Value: "", Path: ssoFlowPath, HttpOnly: true,
		Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	flow, err := r.Cookie(auth.SSOFlowCookieName)
	if err != nil || flow.Value == "" {
		return core.Unauthenticated("this single sign-on was not started in this browser, or it took too long; " +
			"start again from the sign-in screen")
	}
	q := r.URL.Query()
	result, err := h.svc.CompleteSSOLogin(r.Context(), core.CompleteSSOInput{
		Flow: flow.Value, Code: q.Get("code"), State: q.Get("state"),
		Error: strings.TrimSpace(q.Get("error") + " " + q.Get("error_description")),
	})
	if err != nil {
		return err
	}
	next := safeNext(result.Next)
	if challenge := result.Challenge; challenge != nil {
		uri := ""
		if challenge.Enrolment != nil {
			uri = challenge.Enrolment.URI
		}
		v := signInChallenge(challenge.Ticket, next, uri, nil)
		v.Remember = result.Remember
		return h.renderTOTP(w, r, http.StatusOK, v)
	}
	http.SetCookie(w, auth.SessionCookie(*result.Session, h.secureCookie(r)))
	if len(result.RecoveryCodes) > 0 {
		return h.renderRecoveryCodes(w, r, result.RecoveryCodes, next,
			"You are signed in. Keep these codes somewhere safe: each one signs you in "+
				"once if you lose the authenticator, and they are not shown again.")
	}
	// #nosec G710 -- safeNext keeps only a relative path on this origin.
	http.Redirect(w, r, next, http.StatusSeeOther)
	return nil
}

// ssoView is what the Single sign-on screen renders.
type ssoView struct {
	Providers []ssoRow
	// PasswordLogin is the tenant's password policy.
	PasswordLogin core.PasswordLogin
	// RedirectURL is the address to register with every provider.
	RedirectURL string
	Roles       []core.Role
	Kinds       []core.SSOKind
}

// ssoRow is one provider with its settings as the screen lists them.
type ssoRow struct {
	core.SSOProvider
	Mapping string
}

// mappingText renders a role mapping one group=role per line, sorted.
func mappingText(m map[string]core.Role) string {
	lines := make([]string, 0, len(m))
	for group, role := range m {
		lines = append(lines, group+"="+string(role))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// showSSO renders the providers and the password policy.
func (h *handler) showSSO(w http.ResponseWriter, r *http.Request) error {
	providers, err := h.svc.ListSSOProviders(r.Context())
	if err != nil {
		return err
	}
	v := ssoView{PasswordLogin: core.PasswordLoginAllowed, RedirectURL: baseURL(r) + RouteSSOCallback,
		Roles: core.Roles, Kinds: core.SSOKinds}
	if options, err := h.svc.SignInOptions(r.Context()); err == nil {
		v.PasswordLogin = options.PasswordLogin
	}
	for _, p := range providers {
		v.Providers = append(v.Providers, ssoRow{SSOProvider: p, Mapping: mappingText(p.RoleMapping)})
	}
	return h.render(w, r, ssoTemplate, "Single sign-on", v)
}

// parseMapping reads the role mapping box: one group=role per line.
func parseMapping(raw string) (map[string]core.Role, error) {
	out := map[string]core.Role{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		group, role, ok := strings.Cut(line, "=")
		if !ok {
			return nil, core.OnField(core.Invalid("role mapping line %q is not group=role", line), "role_mapping")
		}
		out[strings.TrimSpace(group)] = core.Role(strings.TrimSpace(role))
	}
	return out, nil
}

// createSSO adds a provider from the new provider dialog.
func (h *handler) createSSO(w http.ResponseWriter, r *http.Request) error {
	mapping, err := parseMapping(r.PostFormValue("role_mapping"))
	if err != nil {
		return h.refuseIn(w, r, "sso-new", RouteSSO, err, h.showSSO)
	}
	in := core.CreateSSOProviderInput{
		Key: field(r, "key"), Kind: core.SSOKind(field(r, "kind")), DisplayName: field(r, "display_name"),
		Disabled: !checked(r, "enabled"), AutoCreate: checked(r, "auto_create"), TrustMFA: checked(r, "trust_mfa"),
		DefaultRole: core.Role(field(r, "default_role")), AllowedDomains: fieldList(r, "allowed_domains"),
		RoleMapping: mapping, Issuer: field(r, "issuer"), ClientID: field(r, "client_id"),
		ClientSecret: r.PostFormValue("client_secret"), Scopes: fieldList(r, "scopes"),
		EmailClaim: field(r, "email_claim"), NameClaim: field(r, "name_claim"),
		UsernameClaim: field(r, "username_claim"), GroupsClaim: field(r, "groups_claim"),
		MFAValues: fieldList(r, "mfa_values"), AllowedOrgs: fieldList(r, "allowed_orgs"),
		WebURL: field(r, "web_url"), APIURL: field(r, "api_url"),
	}
	if _, err := h.svc.CreateSSOProvider(r.Context(), in); err != nil {
		return h.refuseIn(w, r, "sso-new", RouteSSO, err, h.showSSO)
	}
	redirect(w, r, RouteSSO, "provider added")
	return nil
}

// updateSSO saves a provider's edit dialog. Every setting the dialog draws is
// applied; a blank secret keeps the stored one.
func (h *handler) updateSSO(w http.ResponseWriter, r *http.Request) error {
	id := field(r, "id")
	dialog := "sso-edit-" + id
	mapping, err := parseMapping(r.PostFormValue("role_mapping"))
	if err != nil {
		return h.refuseIn(w, r, dialog, RouteSSO, err, h.showSSO)
	}
	str := func(name string) *string { v := field(r, name); return &v }
	list := func(name string) *[]string { v := fieldList(r, name); return &v }
	flag := func(name string) *bool { v := checked(r, name); return &v }
	role := core.Role(field(r, "default_role"))
	in := core.UpdateSSOProviderInput{
		DisplayName: str("display_name"), Enabled: flag("enabled"), AutoCreate: flag("auto_create"),
		TrustMFA: flag("trust_mfa"), DefaultRole: &role, AllowedDomains: list("allowed_domains"),
		RoleMapping: &mapping, ClientID: str("client_id"), Scopes: list("scopes"),
		MFAValues: list("mfa_values"), AllowedOrgs: list("allowed_orgs"),
	}
	if field(r, "kind") == string(core.SSOKindOIDC) {
		in.Issuer, in.EmailClaim, in.NameClaim = str("issuer"), str("email_claim"), str("name_claim")
		in.UsernameClaim, in.GroupsClaim = str("username_claim"), str("groups_claim")
	} else {
		in.WebURL, in.APIURL = str("web_url"), str("api_url")
	}
	if secret := r.PostFormValue("client_secret"); secret != "" {
		in.ClientSecret = &secret
	}
	if _, err := h.svc.UpdateSSOProvider(r.Context(), id, in); err != nil {
		return h.refuseIn(w, r, dialog, RouteSSO, err, h.showSSO)
	}
	redirect(w, r, RouteSSO, "provider saved")
	return nil
}

// deleteSSO removes a provider from its confirm dialog.
func (h *handler) deleteSSO(w http.ResponseWriter, r *http.Request) error {
	id := field(r, "id")
	if err := h.svc.DeleteSSOProvider(r.Context(), id); err != nil {
		return h.refuseIn(w, r, "sso-delete-"+id, RouteSSO, err, h.showSSO)
	}
	redirect(w, r, RouteSSO, "provider deleted")
	return nil
}

// setPasswordLogin saves the tenant's password sign-in policy.
func (h *handler) setPasswordLogin(w http.ResponseWriter, r *http.Request) error {
	policy := core.PasswordLogin(field(r, "password_login"))
	if _, err := h.svc.UpdateTenant(r.Context(), "", core.UpdateTenantInput{PasswordLogin: &policy}); err != nil {
		return err
	}
	redirect(w, r, RouteSSO, "password sign-in saved")
	return nil
}

// ssoFormView is what the settings the new and edit dialogs share draw from.
type ssoFormView struct {
	Dialog   dialogState
	ID       string
	Roles    []core.Role
	Provider core.SSOProvider
	Mapping  string
}

// newSSOForm builds the shared settings of one dialog: a listed provider's,
// or a new provider's defaults when row is not one.
func newSSOForm(d dialogState, id string, roles []core.Role, row any) ssoFormView {
	out := ssoFormView{Dialog: d, ID: id, Roles: roles,
		Provider: core.SSOProvider{Enabled: true, DefaultRole: core.RoleMember}}
	if r, ok := row.(ssoRow); ok {
		out.Provider, out.Mapping = r.SSOProvider, r.Mapping
	}
	return out
}
