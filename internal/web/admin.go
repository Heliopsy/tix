package web

import (
	"net/http"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

// adminRoutes are the tenant, domain, user and token administration screens.
func (h *handler) adminRoutes() []route {
	return []route{
		get(RouteTenant, "tenant.html", h.showTenant,
			"GetTenant", "ListTenants", "ListMembers", "GetRetention"),
		post(RouteTenant, h.updateTenant, "UpdateTenant"),
		post(RouteMembers, h.addMember, "AddMember"),
		post(RouteMemberRemove, h.removeMember, "RemoveMember"),
		post(RouteRetention, h.putRetention, "PutRetention"),
		post(RoutePrune, h.prune, "Prune"),
		get(RouteDomains, "domains.html", h.showDomains, "ListDomains"),
		post(RouteDomains, h.addDomain, "AddDomain"),
		post(RouteDomainRemove, h.removeDomain, "RemoveDomain"),
		get(RouteUsers, "users.html", h.showUsers, "ListUsers"),
		post(RouteUsers, h.createUser, "CreateUser"),
		post(RouteUserUpdate, h.updateUser, "UpdateUser"),
		post(RouteUserDelete, h.deleteUser, "DeleteUser"),
		get(RouteTokens, "tokens.html", h.showTokens, "ListTokens"),
		post(RouteTokens, h.createToken, "CreateToken"),
		post(RouteTokenRevoke, h.revokeToken, "RevokeToken"),
	}
}

// roles is the role vocabulary the administration forms offer.
var roles = []core.Role{core.RoleViewer, core.RoleMember, core.RoleAdmin}

// certModes is the certificate vocabulary the domain form offers.
var certModes = []core.CertMode{core.CertNone, core.CertFile}

// tenantView is what the tenant administration screen renders.
type tenantView struct {
	Tenant    core.Tenant
	Tenants   []core.Tenant
	Members   []core.Membership
	Retention core.RetentionPolicy
	Roles     []core.Role
}

// showTenant renders the tenant record, its members and its retention policy.
func (h *handler) showTenant(w http.ResponseWriter, r *http.Request) error {
	tenant, err := h.svc.GetTenant(r.Context(), "")
	if err != nil {
		return err
	}
	tenants, _, err := h.svc.ListTenants(r.Context(), core.Page{})
	if err != nil {
		return err
	}
	members, err := h.svc.ListMembers(r.Context())
	if err != nil {
		return err
	}
	retention, err := h.svc.GetRetention(r.Context())
	if err != nil {
		return err
	}
	return h.render(w, r, "tenant.html", "Tenant", tenantView{
		Tenant: *tenant, Tenants: tenants, Members: members,
		Retention: *retention, Roles: roles})
}

// updateTenant saves the tenant's display name.
func (h *handler) updateTenant(w http.ResponseWriter, r *http.Request) error {
	name := field(r, "name")
	if _, err := h.svc.UpdateTenant(r.Context(), "", core.UpdateTenantInput{Name: &name}); err != nil {
		return err
	}
	redirect(w, r, RouteTenant, "tenant saved")
	return nil
}

// addMember grants an actor a role in this tenant.
func (h *handler) addMember(w http.ResponseWriter, r *http.Request) error {
	if _, err := h.svc.AddMember(r.Context(), field(r, "actor_id"), core.Role(field(r, "role"))); err != nil {
		return err
	}
	redirect(w, r, RouteTenant, "member added")
	return nil
}

// removeMember revokes an actor's membership.
func (h *handler) removeMember(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RemoveMember(r.Context(), field(r, "actor_id")); err != nil {
		return err
	}
	redirect(w, r, RouteTenant, "member removed")
	return nil
}

// putRetention saves how long the append-only tables are kept.
func (h *handler) putRetention(w http.ResponseWriter, r *http.Request) error {
	policy := core.RetentionPolicy{}
	for name, target := range map[string]*core.Duration{
		"events":             &policy.Events,
		"audit_entries":      &policy.AuditEntries,
		"webhook_deliveries": &policy.WebhookDeliveries,
	} {
		value, err := durationField(r, name)
		if err != nil {
			return err
		}
		*target = value
	}
	if _, err := h.svc.PutRetention(r.Context(), policy); err != nil {
		return err
	}
	redirect(w, r, RouteTenant, "retention saved")
	return nil
}

// durationField reads a retention window expressed as a Go duration.
func durationField(r *http.Request, name string) (core.Duration, error) {
	raw := field(r, name)
	if raw == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed < 0 {
		return 0, core.Invalid("%s %q must be a duration such as 720h", name, raw)
	}
	return core.Duration(parsed), nil
}

// prune removes records past their retention window.
func (h *handler) prune(w http.ResponseWriter, r *http.Request) error {
	result, err := h.svc.Prune(r.Context(), core.PruneInput{DryRun: checked(r, "dry_run")})
	if err != nil {
		return err
	}
	flash := "pruning removed records"
	if result.DryRun {
		flash = "dry run: nothing was removed"
	}
	redirect(w, r, RouteTenant, flash)
	return nil
}

// domainsView is what the domain administration screen renders.
type domainsView struct {
	Domains   []core.Domain
	CertModes []core.CertMode
}

// showDomains renders the hostnames mapped to this tenant.
func (h *handler) showDomains(w http.ResponseWriter, r *http.Request) error {
	domains, err := h.svc.ListDomains(r.Context())
	if err != nil {
		return err
	}
	return h.render(w, r, "domains.html", "Domains",
		domainsView{Domains: domains, CertModes: certModes})
}

// addDomain maps a hostname to this tenant.
func (h *handler) addDomain(w http.ResponseWriter, r *http.Request) error {
	in := core.AddDomainInput{
		Hostname: field(r, "hostname"),
		CertMode: core.CertMode(field(r, "cert_mode")),
		CertPath: field(r, "cert_path"),
		KeyPath:  field(r, "key_path"),
	}
	if _, err := h.svc.AddDomain(r.Context(), in); err != nil {
		return err
	}
	redirect(w, r, RouteDomains, "domain added")
	return nil
}

// removeDomain unmaps a hostname.
func (h *handler) removeDomain(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RemoveDomain(r.Context(), field(r, "hostname")); err != nil {
		return err
	}
	redirect(w, r, RouteDomains, "domain removed")
	return nil
}

// usersView is what the user administration screen renders.
type usersView struct {
	Users      []core.User
	Roles      []core.Role
	NextCursor string
}

// showUsers renders the tenant's users.
func (h *handler) showUsers(w http.ResponseWriter, r *http.Request) error {
	users, next, err := h.svc.ListUsers(r.Context(), core.Page{Cursor: r.URL.Query().Get("cursor")})
	if err != nil {
		return err
	}
	return h.render(w, r, "users.html", "Users",
		usersView{Users: users, Roles: roles, NextCursor: next})
}

// createUser creates a credentialed user.
func (h *handler) createUser(w http.ResponseWriter, r *http.Request) error {
	in := core.CreateUserInput{
		Email:       field(r, "email"),
		Password:    r.PostFormValue("password"),
		DisplayName: field(r, "display_name"),
		Role:        core.Role(field(r, "role")),
	}
	if _, err := h.svc.CreateUser(r.Context(), in); err != nil {
		return err
	}
	redirect(w, r, RouteUsers, "user created")
	return nil
}

// updateUser changes a user's name, role or disabled state.
func (h *handler) updateUser(w http.ResponseWriter, r *http.Request) error {
	in := core.UpdateUserInput{}
	if name := field(r, "display_name"); name != "" {
		in.DisplayName = &name
	}
	if role := field(r, "role"); role != "" {
		value := core.Role(role)
		in.Role = &value
	}
	disabled := checked(r, "disabled")
	in.Disabled = &disabled
	if _, err := h.svc.UpdateUser(r.Context(), field(r, "id"), in); err != nil {
		return err
	}
	redirect(w, r, RouteUsers, "user updated")
	return nil
}

// deleteUser removes a user.
func (h *handler) deleteUser(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteUser(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteUsers, "user deleted")
	return nil
}

// tokensView is what the token administration screen renders.
type tokensView struct {
	Tokens []core.APIToken
	Scopes []core.Scope
	Issued string
}

// showTokens renders the caller's API tokens, and a newly issued value once.
func (h *handler) showTokens(w http.ResponseWriter, r *http.Request) error {
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return err
	}
	tokens, err := h.svc.ListTokens(r.Context(), actor.ID)
	if err != nil {
		return err
	}
	return h.render(w, r, "tokens.html", "Tokens", tokensView{
		Tokens: tokens, Scopes: core.AllScopes, Issued: issuedToken(w, r)})
}

// issuedTokenCookie carries a freshly minted token to the screen that shows it
// once, so the value never appears in a URL or in the audit trail.
const issuedTokenCookie = "tix_issued_token"

// issuedToken reads and clears the one-time token cookie.
func issuedToken(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(issuedTokenCookie)
	if err != nil || cookie.Value == "" {
		return ""
	}
	// #nosec G124 -- this clears the cookie (MaxAge -1). HttpOnly and SameSite
	// are set; Secure follows the deployment and is applied where it is issued.
	http.SetCookie(w, &http.Cookie{Name: issuedTokenCookie, Path: RouteTokens,
		MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	return cookie.Value
}

// createToken mints an API token and shows its value exactly once.
func (h *handler) createToken(w http.ResponseWriter, r *http.Request) error {
	scopes := make([]core.Scope, 0, len(r.PostForm["scopes"]))
	for _, raw := range r.PostForm["scopes"] {
		scopes = append(scopes, core.Scope(raw))
	}
	issued, err := h.svc.CreateToken(r.Context(), core.CreateTokenInput{
		Name: field(r, "name"), Scopes: scopes})
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- one-time value, HttpOnly, cleared by the screen that shows it
		Name: issuedTokenCookie, Value: issued.Token, Path: RouteTokens,
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode, MaxAge: 60,
	})
	redirect(w, r, RouteTokens, "token issued")
	return nil
}

// revokeToken revokes an API token.
func (h *handler) revokeToken(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RevokeToken(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteTokens, "token revoked")
	return nil
}
