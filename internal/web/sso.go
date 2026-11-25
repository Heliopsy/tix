// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"errors"
	"html/template"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ssoRoutes are signing in through a provider and the screen that manages
// the tenant's providers.
func (h *handler) ssoRoutes() []route {
	return []route{
		public(get(RouteSSOStart, "", h.startSSO, "BeginSSOLogin")),
		public(get(RouteSSOCallback, totpTemplate, h.finishSSO, "CompleteSSOLogin")),
		crossSite(public(post(RouteSAMLACS, h.finishSAML, "CompleteSSOLogin"))),
		public(post(RouteSSOApp, h.openApp, "Login")),
		public(getFile(RouteSAMLMetadata, h.samlMetadata, "SAMLMetadata")),
		get(RouteSSO, ssoTemplate, h.showSSO, "ListSSOProviders", "SignInOptions"),
		post(RouteSSO, h.createSSO, "CreateSSOProvider"),
		post(RouteSSOUpdate, h.updateSSO, "UpdateSSOProvider"),
		post(RouteSSODelete, h.deleteSSO, "DeleteSSOProvider"),
		post(RouteSSORefresh, h.refreshSSO, "RefreshSAMLMetadata"),
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

		AppRedirect:         q.Get("app_redirect"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
		AppState:            q.Get("app_state"),
	})
	if err != nil {
		return h.refusedForApp(w, r, err)
	}
	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- HttpOnly and SameSite always; Secure tracks TLS, as the session cookie does
		Name: auth.SSOFlowCookieName, Value: begin.Flow, Path: ssoFlowPath, HttpOnly: true,
		Secure: h.secureCookie(r), SameSite: flowSameSite(begin.Kind, h.secureCookie(r)),
		MaxAge: int(auth.SSOFlowTTL.Seconds()),
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
		return h.refusedForApp(w, r, err)
	}
	return h.signedInThroughSSO(w, r, result)
}

// flowSameSite is the flow cookie's SameSite. A SAML response comes back as
// a cross-site POST, which a Lax cookie does not survive, so over TLS a SAML
// flow is None; over plain HTTP, where None is refused without Secure, it
// stays Lax and works only with an IdP on the same site.
func flowSameSite(kind core.SSOKind, secure bool) http.SameSite {
	if kind == core.SSOKindSAML && secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

// crossSite marks a route that takes another site's POST without the CSRF
// token.
func crossSite(rt route) route {
	rt.CrossSite = true
	return rt
}

// finishSAML is a SAML provider's assertion consumer service. The flow
// cookie, when this browser started the sign-in, travels with the IdP's
// form; without one the response is IdP-initiated, which the provider has
// to allow.
func (h *handler) finishSAML(w http.ResponseWriter, r *http.Request) error {
	flow := ""
	if c, err := r.Cookie(auth.SSOFlowCookieName); err == nil {
		flow = c.Value
		// #nosec G124 -- clears the flow cookie; HttpOnly as it was set.
		http.SetCookie(w, &http.Cookie{Name: auth.SSOFlowCookieName, Value: "", Path: ssoFlowPath, HttpOnly: true,
			Secure: h.secureCookie(r), SameSite: flowSameSite(core.SSOKindSAML, h.secureCookie(r)), MaxAge: -1})
	}
	key := r.PathValue("key")
	result, err := h.svc.CompleteSSOLogin(r.Context(), core.CompleteSSOInput{
		Flow: flow, State: r.PostFormValue("RelayState"), SAMLResponse: r.PostFormValue("SAMLResponse"),
		Provider: key, RedirectURI: baseURL(r) + core.SAMLACSPath(key),
	})
	if err != nil {
		return h.refusedForApp(w, r, err)
	}
	return h.signedInThroughSSO(w, r, result)
}

// samlMetadata answers a SAML provider's service provider metadata.
func (h *handler) samlMetadata(w http.ResponseWriter, r *http.Request) error {
	md, err := h.svc.SAMLMetadata(r.Context(), core.SAMLMetadataInput{Provider: r.PathValue("key"), BaseURL: baseURL(r)})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", wire.ContentSAMLMetadata)
	_, err = io.WriteString(w, md.XML)
	return err
}

// signedInThroughSSO answers a completed provider sign-in: the second factor
// still to pass, the recovery codes of a first enrolment, or the screen to
// return to.
func (h *handler) signedInThroughSSO(w http.ResponseWriter, r *http.Request, result *core.LoginResult) error {
	next := safeNext(result.Next)
	if result.Challenge == nil && (result.AppRedirect != "" || result.AppHandoff != "") {
		return h.backToApp(w, r, result)
	}
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

// ssoAppTemplate is the page that sends the browser back to a native app.
const ssoAppTemplate = "ssoapp.html"

// ssoAppView is the page that hands a sign-in back to the app: the address
// with its code or refusal, and the recovery codes an enrolment issued.
type ssoAppView struct {
	// Target is the app's own redirect, an address from
	// auth.sso_app_redirects with the code or the refusal and the app's
	// state in its query.
	Target template.URL
	Codes  []string
	Failed bool
	// Handoff is the signed handoff of a sign-in that completed by
	// enrolling. Open the app posts it to mint the app's code, so the code's
	// two minutes start when the reader has kept the recovery codes.
	Handoff string
	// Action is where Open the app posts the handoff.
	Action string
}

// Joined is the recovery codes one to a line.
func (v ssoAppView) Joined() string { return strings.Join(v.Codes, "\n") }

// refusedForApp answers a refused provider sign-in: the app's redirect
// carrying the reason when an app started it, the error screen otherwise.
func (h *handler) refusedForApp(w http.ResponseWriter, r *http.Request, err error) error {
	target := appTarget(err)
	if target == "" {
		return err
	}
	return h.toApp(w, r, ssoAppView{Target: appURL(target), Failed: true})
}

// appTarget is the app redirect a refusal names, or "" for a browser's own
// sign-in.
func appTarget(err error) string {
	var e *core.Error
	if !errors.As(err, &e) {
		return ""
	}
	target, _ := e.Details[core.DetailAppRedirect].(string)
	return target
}

// backToApp hands a completed sign-in to the app that started it. No
// session cookie is set: the app exchanges the code for its own session.
func (h *handler) backToApp(w http.ResponseWriter, r *http.Request, result *core.LoginResult) error {
	if result.AppHandoff != "" {
		return h.toApp(w, r, ssoAppView{Codes: result.RecoveryCodes, Handoff: result.AppHandoff, Action: RouteSSOApp})
	}
	return h.toApp(w, r, ssoAppView{Target: appURL(result.AppRedirect), Codes: result.RecoveryCodes})
}

// openApp is Open the app under an enrolment's recovery codes: it mints the
// app's code now and sends the browser to the app with it. The tap that
// posted the form is the gesture a browser asks for before it opens an app.
func (h *handler) openApp(w http.ResponseWriter, r *http.Request) error {
	result, err := h.svc.Login(r.Context(), core.LoginInput{Surface: core.SurfaceWeb,
		AppHandoff: r.PostFormValue("handoff")})
	if err != nil {
		return h.refusedForApp(w, r, err)
	}
	if result.AppRedirect == "" {
		return core.Invalid("this sign-in has nothing to hand to an app; start again from the app")
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// #nosec G710 -- an address auth.sso_app_redirects lists exactly; see appURL.
	http.Redirect(w, r, result.AppRedirect, http.StatusFound)
	return nil
}

// appURL marks an app redirect safe to draw as a link. The service built it
// from an address the configuration lists exactly, with a query it encoded.
func appURL(target string) template.URL {
	return template.URL(target) // #nosec G203 -- an allowlisted app redirect with an encoded query
}

// toApp sends the browser to the app. A GET, the provider's redirect back,
// is answered with a 302. A POST, the SAML ACS or the second factor's form,
// is answered with a page that navigates on its own and links the address,
// because a browser may refuse to follow a redirect to an app's scheme that
// no gesture started; the same page shows the recovery codes of an
// enrolment before the reader goes on.
func (h *handler) toApp(w http.ResponseWriter, r *http.Request, v ssoAppView) error {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method == http.MethodGet && len(v.Codes) == 0 {
		// #nosec G710 -- an address auth.sso_app_redirects lists exactly; see appURL.
		http.Redirect(w, r, string(v.Target), http.StatusFound)
		return nil
	}
	return h.render(w, r, ssoAppTemplate, "Back to the app", v)
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
	// ACSURL, MetadataURL and EntityID are a SAML provider's addresses on
	// this screen's origin, to give the IdP.
	ACSURL, MetadataURL, EntityID string
	// Certificates is the IdP's certificates, one PEM block after another.
	Certificates string
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
		row := ssoRow{SSOProvider: p, Mapping: mappingText(p.RoleMapping)}
		if p.Kind == core.SSOKindSAML {
			row.ACSURL, row.MetadataURL = baseURL(r)+core.SAMLACSPath(p.Key), baseURL(r)+core.SAMLMetadataPath(p.Key)
			row.EntityID = orElse(p.SPEntityID, row.MetadataURL)
			row.Certificates = strings.Join(p.IdPCertificates, "\n")
		}
		v.Providers = append(v.Providers, row)
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
		IdPMetadataURL: field(r, "idp_metadata_url"), IdPMetadataXML: r.PostFormValue("idp_metadata_xml"),
		IdPEntityID: field(r, "idp_entity_id"), IdPSSOURL: field(r, "idp_sso_url"),
		IdPCertificates: core.SplitCertificates(r.PostFormValue("idp_certificates")),
		SPEntityID:      field(r, "sp_entity_id"), NameIDFormat: field(r, "name_id_format"),
		AllowIdPInitiated: checked(r, "allow_idp_initiated"),
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
	switch core.SSOKind(field(r, "kind")) {
	case core.SSOKindOIDC:
		in.Issuer, in.EmailClaim, in.NameClaim = str("issuer"), str("email_claim"), str("name_claim")
		in.UsernameClaim, in.GroupsClaim = str("username_claim"), str("groups_claim")
	case core.SSOKindSAML:
		in.EmailClaim, in.NameClaim = str("email_claim"), str("name_claim")
		in.UsernameClaim, in.GroupsClaim = str("username_claim"), str("groups_claim")
		samlEdit(r, &in)
	default:
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

// samlEdit sets a SAML provider's settings from its edit dialog. The IdP
// metadata is read again when its address changed, a document was pasted or
// the box asking for it is ticked; the entity id, sign-in address and
// certificates drawn in the dialog are then left for the metadata to fill.
func samlEdit(r *http.Request, in *core.UpdateSSOProviderInput) {
	str := func(name string) *string { v := field(r, name); return &v }
	in.SPEntityID, in.NameIDFormat = str("sp_entity_id"), str("name_id_format")
	idpInitiated := checked(r, "allow_idp_initiated")
	in.AllowIdPInitiated = &idpInitiated
	pasted := strings.TrimSpace(r.PostFormValue("idp_metadata_xml"))
	if checked(r, "refresh_metadata") || pasted != "" || field(r, "idp_metadata_url") != field(r, "idp_metadata_url_was") {
		in.IdPMetadataURL = str("idp_metadata_url")
		if pasted != "" {
			in.IdPMetadataXML = &pasted
		}
		return
	}
	certs := core.SplitCertificates(r.PostFormValue("idp_certificates"))
	in.IdPEntityID, in.IdPSSOURL, in.IdPCertificates = str("idp_entity_id"), str("idp_sso_url"), &certs
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

// refreshSSO reads a SAML provider's IdP metadata again. A read that failed
// is recorded on the provider, whose row then says why.
func (h *handler) refreshSSO(w http.ResponseWriter, r *http.Request) error {
	p, err := h.svc.RefreshSAMLMetadata(r.Context(), field(r, "id"))
	switch {
	case core.IsKind(err, core.KindUpstream) || core.IsKind(err, core.KindInvalid):
		redirect(w, r, RouteSSO, "metadata refresh failed")
	case err != nil:
		return err
	case p.MetadataRefresh != nil && p.MetadataRefresh.Result == core.SAMLRefreshUpdated:
		redirect(w, r, RouteSSO, "metadata refreshed, certificates updated")
	default:
		redirect(w, r, RouteSSO, "metadata refreshed, nothing changed")
	}
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
	Dialog       dialogState
	ID           string
	Roles        []core.Role
	Provider     core.SSOProvider
	Mapping      string
	Certificates string
}

// newSSOForm builds the shared settings of one dialog: a listed provider's,
// or a new provider's defaults when row is not one.
func newSSOForm(d dialogState, id string, roles []core.Role, row any) ssoFormView {
	out := ssoFormView{Dialog: d, ID: id, Roles: roles,
		Provider: core.SSOProvider{Enabled: true, DefaultRole: core.RoleMember}}
	if r, ok := row.(ssoRow); ok {
		out.Provider, out.Mapping, out.Certificates = r.SSOProvider, r.Mapping, r.Certificates
	}
	return out
}
