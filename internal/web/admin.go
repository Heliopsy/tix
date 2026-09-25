// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// adminRoutes are the tenant, domain, user, token and key administration screens.
func (h *handler) adminRoutes() []route {
	return []route{
		get(RouteTenant, "tenant.html", h.showTenant,
			"GetTenant", "ListTenants", "ListMembers", "GetRetention", "GetActor"),
		post(RouteTenant, h.updateTenant, "UpdateTenant"),
		post(RouteMembers, h.addMember, "AddMember"),
		post(RouteMemberRemove, h.removeMember, "RemoveMember"),
		post(RouteRetention, h.putRetention, "PutRetention"),
		post(RoutePrune, h.prune, "Prune"),
		get(RouteDomains, "domains.html", h.showDomains, "ListDomains"),
		post(RouteDomains, h.addDomain, "AddDomain"),
		post(RouteDomainRemove, h.removeDomain, "RemoveDomain"),
		get(RouteUsers, "users.html", h.showUsers, "ListUsers", "ListMembers"),
		post(RouteUsers, h.createUser, "CreateUser"),
		post(RouteUserUpdate, h.updateUser, "UpdateUser"),
		post(RouteUserDelete, h.deleteUser, "DeleteUser"),
		get(RouteTokens, "tokens.html", h.showTokens, "ListTokens"),
		post(RouteTokens, h.createToken, "CreateToken"),
		post(RouteTokenRevoke, h.revokeToken, "RevokeToken"),
		get(RouteSSHKeys, "sshkeys.html", h.showSSHKeys, "ListSSHKeys"),
		post(RouteSSHKeys, h.enrolSSHKey, "EnrolSSHKey"),
		post(RouteSSHKeyRevoke, h.revokeSSHKey, "RevokeSSHKey"),
	}
}

// tenantView is what the tenant administration screen renders.
type tenantView struct {
	Tenant core.Tenant
	// Themes are the palettes this deployment resolves, for the picker. The
	// command line could set one from the start and the browser could not,
	// which made a tenant-wide setting reachable only by whoever had shell
	// access to the server.
	Themes    []core.Theme
	Tenants   []core.Tenant
	Members   []core.Membership
	Retention core.RetentionPolicy
	Roles     []core.Role
	Shape     []shapeNode
	// Names labels each member by the handle its actor carries, because a
	// membership row holds an actor identifier and nothing else. Without
	// this the table read as three 26-character ULIDs under a heading
	// saying "people", which is the opposite of what the diagram above it
	// promises.
	Names actorNames
}

// shapeNode is one row of the diagram showing what sits under what.
//
// It carries a live count rather than being a static picture, because the
// question somebody actually has on this screen is "where does my stuff
// live", and a drawing of an empty model does not answer it. A count of zero
// is worth as much as any other: it is how you find out that a tenant has no
// domains, which is the usual reason a hostname is not resolving.
type shapeNode struct {
	Depth int
	Name  string
	Count int
	// Href links to where that kind is managed, empty when there is nowhere
	// to go.
	Href string
	// Note says what the thing is, in one clause.
	Note string
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
	shape, err := h.tenantShape(r, len(members))
	if err != nil {
		return err
	}
	actors := make([]string, 0, len(members))
	for _, m := range members {
		actors = append(actors, m.ActorID)
	}
	return h.render(w, r, "tenant.html", "Tenant", tenantView{
		Tenant: *tenant, Tenants: tenants, Members: members, Themes: h.themes.List(),
		Retention: *retention, Roles: core.Roles, Shape: shape,
		Names: h.resolveActors(r, actors...)})
}

// tenantShape counts what hangs off this tenant, for the diagram.
//
// Every count is a listing this screen's reader is already allowed to make,
// so the diagram shows nothing that a walk of the navigation would not. A
// listing that fails is not fatal: the diagram is an explanation, and losing
// the tenant page because a count errored would be a poor trade.
func (h *handler) tenantShape(r *http.Request, members int) ([]shapeNode, error) {
	ctx := r.Context()
	count := func(n int, err error) int {
		if err != nil {
			return -1
		}
		return n
	}

	projects, _, err := h.svc.ListProjects(ctx, core.ProjectFilter{})
	if err != nil {
		return nil, err
	}
	workflows, wErr := h.svc.ListWorkflows(ctx)
	domains, dErr := h.svc.ListDomains(ctx)
	tokens, tErr := h.svc.ListTokens(ctx, "")

	return []shapeNode{
		{Depth: 0, Name: "Tenant", Count: -1, Note: "everything below belongs to it and is invisible from any other"},
		{Depth: 1, Name: "Members", Count: members, Href: RouteUsers, Note: "people, each with a role here"},
		{Depth: 1, Name: "Domains", Count: count(len(domains), dErr), Href: RouteDomains, Note: "hostnames that resolve to this tenant"},
		{Depth: 1, Name: "API tokens", Count: count(len(tokens), tErr), Href: RouteTokens, Note: "what an agent authenticates with"},
		{Depth: 1, Name: "Workflows", Count: count(len(workflows), wErr), Href: RouteWorkflows, Note: "the states a task moves between"},
		{Depth: 1, Name: "Projects", Count: len(projects), Href: RouteProjects, Note: "each one picks a workflow"},
		{Depth: 2, Name: "Tasks", Count: -1, Note: "the work, and its comments, artifacts and dependencies"},
		{Depth: 2, Name: "Field definitions", Count: -1, Note: "the custom fields a task in that project carries"},
	}, nil
}

// updateTenant saves the tenant's display name and theme.
func (h *handler) updateTenant(w http.ResponseWriter, r *http.Request) error {
	name := field(r, "name")
	// The empty option clears the theme, so the field is always sent and
	// always applied: treating "" as unset would make the picker one-way.
	theme := field(r, "theme")
	in := core.UpdateTenantInput{Name: &name, Theme: &theme}
	if _, err := h.svc.UpdateTenant(r.Context(), "", in); err != nil {
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
		domainsView{Domains: domains, CertModes: core.CertModes})
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

// userRow is one account as the administration screen shows it: the record
// itself, the role its membership grants, and whether it is the account the
// reader is signed in as.
//
// Role does not live on core.User -- it is a property of the membership that
// joins an account to this tenant -- which is why this screen has to bring
// the two together. It did not, and the consequence was not cosmetic: the
// edit form's role control listed every role with none of them selected, so
// a browser submitted the first one. Opening a user to tick "disabled" and
// pressing Update silently demoted an administrator to a viewer.
type userRow struct {
	User core.User
	Role core.Role
	Self bool
}

// Label is what the row is called: the name they chose, or their email.
func (u userRow) Label() string { return userLabel(u.User) }

// Monogram is the one or two letters the avatar shows, taken from whatever
// the row is called so it changes with the name rather than with the id.
func (u userRow) Monogram() string {
	label := strings.TrimSpace(u.Label())
	if label == "" {
		return "?"
	}
	fields := strings.Fields(label)
	if len(fields) > 1 {
		return strings.ToUpper(fields[0][:1] + fields[1][:1])
	}
	return strings.ToUpper(label[:1])
}

// Disabled reports whether the account has been switched off.
func (u userRow) Disabled() bool { return u.User.DisabledAt != nil }

// usersView is what the user administration screen renders.
type usersView struct {
	Users      []userRow
	Roles      []core.Role
	NextCursor string
	Active     int
	Off        int
}

// showUsers renders the tenant's users, each with the role its membership
// grants. A membership listing the caller may not read leaves every row's
// role empty rather than failing the screen, and the edit control then says
// so instead of quietly proposing a role nobody chose.
func (h *handler) showUsers(w http.ResponseWriter, r *http.Request) error {
	users, next, err := h.svc.ListUsers(r.Context(), core.Page{Cursor: r.URL.Query().Get("cursor")})
	if err != nil {
		return err
	}
	roles := h.memberRoles(r)
	actor, _ := core.ActorFrom(r.Context())
	data := usersView{Users: make([]userRow, 0, len(users)), Roles: core.Roles, NextCursor: next}
	for _, u := range users {
		row := userRow{User: u, Role: roles[u.ID]}
		if actor != nil && actor.ID == u.ID {
			row.Self = true
		}
		if row.Disabled() {
			data.Off++
		} else {
			data.Active++
		}
		data.Users = append(data.Users, row)
	}
	return h.render(w, r, "users.html", "Users", data)
}

// memberRoles maps each actor to the role their membership of this tenant
// grants them.
func (h *handler) memberRoles(r *http.Request) map[string]core.Role {
	members, err := h.svc.ListMembers(r.Context())
	if err != nil {
		return nil
	}
	out := make(map[string]core.Role, len(members))
	for _, m := range members {
		out[m.ActorID] = m.Role
	}
	return out
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
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteStrictMode, MaxAge: 60,
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

// sshKeysView is what the enrolled key screen renders.
type sshKeysView struct {
	Keys []core.SSHKey
}

// showSSHKeys renders the caller's own enrolled keys, revoked ones included,
// because a key that stopped working is the one somebody came here about.
func (h *handler) showSSHKeys(w http.ResponseWriter, r *http.Request) error {
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return err
	}
	keys, err := h.svc.ListSSHKeys(r.Context(), actor.ID)
	if err != nil {
		return err
	}
	return h.render(w, r, "sshkeys.html", "SSH keys", sshKeysView{Keys: keys})
}

// enrolSSHKey enrols a public key against the signed-in actor. The submission
// is passed through untouched: the service is the authority on what an ssh
// public key is, and parsing one here would be a second, divergent opinion.
func (h *handler) enrolSSHKey(w http.ResponseWriter, r *http.Request) error {
	in := core.EnrolSSHKeyInput{
		PublicKey: field(r, "public_key"),
		Label:     field(r, "label"),
	}
	if _, err := h.svc.EnrolSSHKey(r.Context(), in); err != nil {
		return err
	}
	redirect(w, r, RouteSSHKeys, "key enrolled")
	return nil
}

// revokeSSHKey stops an enrolled key authenticating.
func (h *handler) revokeSSHKey(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RevokeSSHKey(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteSSHKeys, "key revoked")
	return nil
}
