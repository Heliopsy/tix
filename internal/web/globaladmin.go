// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// initialCredentialsCookie carries the passwords a tenant creation generated
// to the screen that shows them once.
const initialCredentialsCookie = "tix_initial_credentials" // #nosec G101 -- a cookie name, not a credential

// globalRoutes are the global admin area: every tenant, the global admins,
// and the users of any tenant.
func (h *handler) globalRoutes() []route {
	return []route{
		get(RouteGlobal, "globaladmin.html", h.showGlobal, "ListTenants", "ListGlobalAdmins"),
		post(RouteGlobalTenants, h.createTenant, "CreateTenant"),
		post(RouteGlobalTenantRename, h.renameTenant, "UpdateTenant"),
		post(RouteGlobalTenantDelete, h.deleteTenant, "DeleteTenant"),
		post(RouteGlobalAdmins, h.grantGlobalAdmin, "GrantGlobalAdmin"),
		post(RouteGlobalAdminRevoke, h.revokeGlobalAdmin, "RevokeGlobalAdmin"),
		get(RouteGlobalTenant, "globalusers.html", h.showGlobalUsers, "ListTenantUsers", "GetTenant"),
		post(RouteGlobalUsers, h.createGlobalUser, "CreateTenantUser"),
		post(RouteGlobalMembers, h.addGlobalMember, "AddTenantMember"),
		post(RouteGlobalUserUpdate, h.updateGlobalUser, "UpdateTenantUser"),
		post(RouteGlobalUserDelete, h.deleteGlobalUser, "DeleteTenantUser"),
		get(RouteGlobalAccount, accountLogTemplate, h.showGlobalAccountLog, "GetAccountLogFor"),
	}
}

// globalAdmin reports whether the reader may act as a global admin, which
// decides whether the sidebar offers the area at all.
func (h *handler) globalAdmin(r *http.Request) bool {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return false
	}
	me, err := h.svc.WhoAmI(r.Context())
	return err == nil && me.GlobalAdmin
}

// globalView is what the global admin screen renders.
type globalView struct {
	Tenants []globalTenantRow
	Pager   pager
	Admins  []core.User
	// Credentials are the passwords the last tenant creation generated, shown
	// once and gone on the next load.
	Credentials []oneTimeSecret
	// Own is the reader's own tenant, which is marked and cannot be deleted
	// from here: doing that would end the reader's own access mid-click.
	Own string
}

// globalTenantRow is one tenant of the listing.
type globalTenantRow struct {
	Tenant core.Tenant
	Own    bool
}

// showGlobal renders every tenant and the global admins. A reader who is not
// a global admin is refused by ListGlobalAdmins, which the service decides.
func (h *handler) showGlobal(w http.ResponseWriter, r *http.Request) error {
	admins, err := h.svc.ListGlobalAdmins(r.Context())
	if err != nil {
		return err
	}
	tenants, next, err := h.svc.ListTenants(r.Context(), core.Page{
		Cursor: r.URL.Query().Get(CursorParam), Limit: h.rowsPerPage(r),
	})
	if err != nil {
		return err
	}
	actor, _ := core.ActorFrom(r.Context())
	data := globalView{Admins: admins, Pager: newPager(r, RouteGlobal, next, len(tenants), "tenants", SizeParam),
		Credentials: credentialSecrets(readOneTimeCookie(w, r, initialCredentialsCookie, RouteGlobal))}
	for _, t := range tenants {
		row := globalTenantRow{Tenant: t, Own: actor != nil && t.ID == actor.TenantID}
		data.Tenants = append(data.Tenants, row)
	}
	return h.render(w, r, "globaladmin.html", "Global admin", data)
}

// credentialSecrets turns the cookie's value back into one secret per
// generated password, each with its own copy button.
func credentialSecrets(raw string) []oneTimeSecret {
	if raw == "" {
		return nil
	}
	body, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil
	}
	var rows [][3]string
	if json.Unmarshal(body, &rows) != nil {
		return nil
	}
	out := make([]oneTimeSecret, 0, len(rows))
	for i, row := range rows {
		out = append(out, oneTimeSecret{
			ID:      "initial-password-" + string(rune('a'+i)),
			Heading: row[0],
			Note:    row[1],
			Value:   row[2],
		})
	}
	return out
}

// credentialRows is the cookie's shape: email, handle and password, positional.
// The value lives for one redirect, so it carries no field names; it is the
// cookie's attributes, not its encoding, that keep it from leaking.
func credentialRows(creds []core.InitialCredential) [][3]string {
	out := make([][3]string, 0, len(creds))
	for _, c := range creds {
		out = append(out, [3]string{c.Email, c.Handle, c.Password})
	}
	return out
}

// adminEmailsOf reads the first admins from the form: one address per line,
// or separated by commas.
func adminEmailsOf(r *http.Request) []core.TenantAdminInput {
	raw := strings.NewReplacer(",", "\n", ";", "\n").Replace(r.PostFormValue("admins"))
	var out []core.TenantAdminInput
	for _, line := range strings.Split(raw, "\n") {
		if email := strings.TrimSpace(line); email != "" {
			out = append(out, core.TenantAdminInput{Email: email})
		}
	}
	return out
}

// createTenant creates a tenant with its first admins and hands the generated
// passwords to the next load of the screen, in a cookie that only that
// screen reads and that it clears as it reads it.
func (h *handler) createTenant(w http.ResponseWriter, r *http.Request) error {
	created, err := h.svc.CreateTenant(r.Context(), core.CreateTenantInput{
		Key: field(r, "key"), Name: field(r, "name"), Admins: adminEmailsOf(r),
	})
	if err != nil {
		return h.refuseIn(w, r, "tenant-new", RouteGlobal, err, h.showGlobal)
	}
	if len(created.InitialCredentials) > 0 {
		body, err := json.Marshal(credentialRows(created.InitialCredentials))
		if err != nil {
			return core.Internal("carrying the generated passwords: %v", err)
		}
		// #nosec G124 -- one-time value, HttpOnly, cleared by the screen that shows it
		http.SetCookie(w, oneTimeCookie(initialCredentialsCookie, RouteGlobal,
			base64.RawURLEncoding.EncodeToString(body), h.secureCookie(r)))
	}
	redirect(w, r, RouteGlobal, "tenant created")
	return nil
}

// Forms on these screens name the tenant they act on as "target". The
// "tenant" parameter is refused on every route, because a session's own
// tenant never comes from the request; this one is what a global admin is
// acting on, and the service decides whether they may.

// renameTenant changes any tenant's name.
func (h *handler) renameTenant(w http.ResponseWriter, r *http.Request) error {
	name := field(r, "name")
	key := field(r, "target")
	if _, err := h.svc.UpdateTenant(r.Context(), key, core.UpdateTenantInput{Name: &name}); err != nil {
		return h.refuseIn(w, r, "tenant-rename-"+key, RouteGlobal, err, h.showGlobal)
	}
	redirect(w, r, RouteGlobal, "tenant renamed")
	return nil
}

// deleteTenant soft deletes any tenant but the reader's own.
func (h *handler) deleteTenant(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteTenant(r.Context(), field(r, "target")); err != nil {
		return err
	}
	redirect(w, r, RouteGlobal, "tenant deleted")
	return nil
}

// grantGlobalAdmin makes a user a global admin.
func (h *handler) grantGlobalAdmin(w http.ResponseWriter, r *http.Request) error {
	if _, err := h.svc.GrantGlobalAdmin(r.Context(), core.GlobalAdminInput{Email: field(r, "email")}); err != nil {
		return h.refuseIn(w, r, "global-grant", RouteGlobal, err, h.showGlobal)
	}
	redirect(w, r, RouteGlobal, "global admin granted")
	return nil
}

// revokeGlobalAdmin takes global admin away.
func (h *handler) revokeGlobalAdmin(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RevokeGlobalAdmin(r.Context(), field(r, "email")); err != nil {
		return err
	}
	redirect(w, r, RouteGlobal, "global admin revoked")
	return nil
}

// globalUsersView is what the screen of one tenant's users renders.
type globalUsersView struct {
	Tenant core.Tenant
	Users  []userRow
	Roles  []core.Role
	Pager  pager
}

// globalUsersPath is the users screen of one tenant.
func globalUsersPath(tenant string) string {
	return strings.Replace(RouteGlobalTenant, "{key}", url.PathEscape(tenant), 1)
}

// showGlobalUsers renders the users of the tenant named in the path.
func (h *handler) showGlobalUsers(w http.ResponseWriter, r *http.Request) error {
	key := strings.TrimSpace(r.PathValue("key"))
	users, next, err := h.svc.ListTenantUsers(r.Context(), key, core.Page{
		Cursor: r.URL.Query().Get(CursorParam), Limit: h.rowsPerPage(r),
	})
	if err != nil {
		return err
	}
	tenant, err := h.svc.GetTenant(r.Context(), key)
	if err != nil {
		return err
	}
	data := globalUsersView{Tenant: *tenant, Roles: core.Roles,
		Pager: newPager(r, globalUsersPath(tenant.Key), next, len(users), "users", SizeParam)}
	for _, u := range users {
		data.Users = append(data.Users, userRow{User: u.User, Role: u.Role})
	}
	return h.render(w, r, "globalusers.html", "Users of a tenant", data)
}

// createGlobalUser creates a user in the named tenant.
func (h *handler) createGlobalUser(w http.ResponseWriter, r *http.Request) error {
	key := field(r, "target")
	in := core.CreateUserInput{
		Email:       field(r, "email"),
		Password:    r.PostFormValue("password"),
		DisplayName: field(r, "display_name"),
		Role:        core.Role(field(r, "role")),
	}
	if _, err := h.svc.CreateTenantUser(r.Context(), key, in); err != nil {
		return h.refuseIn(w, r, "user-new", globalUsersPath(key), err, h.showGlobalUsersFor(key))
	}
	redirect(w, r, globalUsersPath(key), "user created")
	return nil
}

// addGlobalMember adds an existing account to the named tenant.
func (h *handler) addGlobalMember(w http.ResponseWriter, r *http.Request) error {
	key := field(r, "target")
	in := core.AddTenantMemberInput{
		Email:  field(r, "email"),
		Handle: field(r, "handle"),
		Role:   core.Role(field(r, "role")),
	}
	if _, err := h.svc.AddTenantMember(r.Context(), key, in); err != nil {
		return h.refuseIn(w, r, "member-add", globalUsersPath(key), err, h.showGlobalUsersFor(key))
	}
	redirect(w, r, globalUsersPath(key), "person added")
	return nil
}

// showGlobalUsersFor draws one tenant's users for a refused form, whose
// request carries the tenant in the body rather than the address.
func (h *handler) showGlobalUsersFor(key string) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		r.SetPathValue("key", key)
		return h.showGlobalUsers(w, r)
	}
}

// updateGlobalUser changes a user of the named tenant.
func (h *handler) updateGlobalUser(w http.ResponseWriter, r *http.Request) error {
	key := field(r, "target")
	in := core.UpdateUserInput{}
	if name, ok := sent(r, "display_name"); ok {
		in.DisplayName = &name
	}
	if role := field(r, "role"); role != "" {
		value := core.Role(role)
		in.Role = &value
	}
	disabled := checked(r, "disabled")
	in.Disabled = &disabled
	if _, err := h.svc.UpdateTenantUser(r.Context(), key, field(r, "id"), in); err != nil {
		return err
	}
	redirect(w, r, globalUsersPath(key), "user updated")
	return nil
}

// deleteGlobalUser deletes a user of the named tenant.
func (h *handler) deleteGlobalUser(w http.ResponseWriter, r *http.Request) error {
	key := field(r, "target")
	if err := h.svc.DeleteTenantUser(r.Context(), key, field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, globalUsersPath(key), "user deleted")
	return nil
}
