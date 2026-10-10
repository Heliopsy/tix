// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"slices"
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
		get(RouteTokens, "tokens.html", h.showTokens, "ListTokens", "ListActors"),
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
	// Count is the live figure, negative where the row has none.
	Count int
	// Uncounted takes the figure's place where there is none, so the row says
	// why rather than leaving a gap. A blank beside five numbers reads as a
	// count that broke, and a count that really did break read exactly like
	// one nobody attempted.
	Uncounted string
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

	projects, whole, err := h.allProjects(r, true)
	if err != nil {
		return nil, err
	}
	projectCount := len(projects)
	projectsUncounted := ""
	if !whole {
		projectCount, projectsUncounted = -1, countShort
	}
	workflows, wErr := h.svc.ListWorkflows(ctx)
	domains, dErr := h.svc.ListDomains(ctx)

	// Neither Tasks nor Field definitions is a tenant-wide figure the service
	// can produce: a task listing is keyset-paginated and carries no total,
	// and a field definition is declared on one project at a time. Each names
	// where its number actually lives instead of showing a gap.
	return []shapeNode{
		{Depth: 0, Name: "Tenant", Count: -1,
			Note: "everything below belongs to it and is invisible from any other"},
		{Depth: 1, Name: "Members", Count: members, Href: RouteUsers,
			Note: "people, each with a role here"},
		{Depth: 1, Name: "Domains", Count: count(len(domains), dErr), Href: RouteDomains, Uncounted: countFailed,
			Note: "hostnames that resolve to this tenant"},
		h.tokenShapeRow(r),
		{Depth: 1, Name: "Workflows", Count: count(len(workflows), wErr), Href: RouteWorkflows, Uncounted: countFailed,
			Note: "the states a task moves between"},
		{Depth: 1, Name: "Projects", Count: projectCount, Href: RouteProjects, Uncounted: projectsUncounted,
			Note: "each one picks a workflow"},
		{Depth: 2, Name: "Tasks", Count: -1, Uncounted: "on the task list", Href: RouteTasks,
			Note: "the work, and its comments, artifacts and dependencies"},
		{Depth: 2, Name: "Field definitions", Count: -1, Uncounted: "per project", Href: RouteProjects,
			Note: "the custom fields a task in that project carries"},
	}, nil
}

// countFailed is what a row shows where its listing errored, which is not the
// same thing as a row that carries no figure by design.
const countFailed = "unavailable"

// countShort is what a row shows where the walk behind its figure stopped at
// its bound, which is neither a count nor a failure: the figure would have
// been a floor presented as a total.
const countShort = "incomplete"

// tokenShapeRow is the API tokens row of the diagram.
//
// It counts the listing the token screen renders rather than ListTokens with
// an empty actor, which has always meant the caller's own: beside Members,
// Domains and Workflows, all tenant-wide, a reader's personal figure linking
// to a screen showing every token of the tenant disagreed with the screen it
// sits next to. Counting the same listing is what makes the two agree by
// construction rather than by both being maintained.
//
// The listing is always the tenant's here: this screen's own GetTenant needs
// authz.ActionTenantAdmin, which resolves to the scope tokenListing tests, so
// a reader who can read this row holds it. A reader holding it without
// token:admin cannot list tokens at all, and the row says so.
func (h *handler) tokenShapeRow(r *http.Request) shapeNode {
	row := shapeNode{Depth: 1, Name: "API tokens", Count: -1, Href: RouteTokens,
		Uncounted: countFailed, Note: "what an agent authenticates with"}
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return row
	}
	list, err := h.tokenListing(r, actor)
	if err != nil {
		return row
	}
	if !list.Whole {
		row.Uncounted = countShort
		return row
	}
	row.Count = len(list.Rows)
	return row
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

// durationField reads a retention window through the same grammar the screen
// prints it in. time.ParseDuration knows no day, so the field rejected the
// "30d" it had just rendered into itself.
func durationField(r *http.Request, name string) (core.Duration, error) {
	raw := field(r, name)
	if raw == "" {
		return 0, nil
	}
	parsed, err := core.ParseDuration(raw)
	if err != nil || parsed < 0 {
		return 0, core.Invalid("%s %q must be a duration such as 30d, 12h or 2h30m", name, raw)
	}
	return parsed, nil
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
		return strings.ToUpper(firstRune(fields[0]) + firstRune(fields[1]))
	}
	return strings.ToUpper(firstRune(label))
}

// firstRune is a string's first character, which is not its first byte: a
// name outside ASCII sliced at one byte leaves a fragment of a character,
// and what reaches the page is a replacement glyph rather than an initial.
func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return ""
}

// Disabled reports whether the account has been switched off.
func (u userRow) Disabled() bool { return u.User.DisabledAt != nil }

// usersView is what the user administration screen renders.
type usersView struct {
	Users  []userRow
	Roles  []core.Role
	Pager  pager
	Active int
	Off    int
}

// showUsers renders the tenant's users, each with the role its membership
// grants. A membership listing the caller may not read leaves every row's
// role empty rather than failing the screen, and the edit control then says
// so instead of quietly proposing a role nobody chose.
func (h *handler) showUsers(w http.ResponseWriter, r *http.Request) error {
	users, next, err := h.svc.ListUsers(r.Context(), core.Page{
		Cursor: r.URL.Query().Get(CursorParam), Limit: h.rowsPerPage(r),
	})
	if err != nil {
		return err
	}
	roles := h.memberRoles(r)
	actor, _ := core.ActorFrom(r.Context())
	data := usersView{Users: make([]userRow, 0, len(users)), Roles: core.Roles,
		Pager: newPager(r, RouteUsers, next, len(users), "users", SizeParam)}
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
	// Presence, not value: the input is rendered with the current name in it,
	// so emptying it and saving is the only gesture the screen offers for
	// "this account has no display name". A caller that omits the key
	// entirely is asking for no change, which is why the key is tested rather
	// than what it holds.
	if name, ok := sent(r, "display_name"); ok {
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

// tokenRow is one row of the token listing: a token and whose it is.
type tokenRow struct {
	core.APIToken
	// Owner is the handle, or the generated name, of the actor the token acts
	// as. A listing that spans more than one actor is unreadable without it:
	// the credential an operator came here to kill is identified by whose it
	// is at least as often as by what it is called.
	Owner string
	// Mine reports whether the token belongs to the reader, so a row that is
	// somebody else's reads as somebody else's.
	Mine bool
}

// tokensView is what the token administration screen renders.
type tokensView struct {
	Tokens []tokenRow
	Scopes []core.Scope
	Expiry []tokenExpiry
	Secret *oneTimeSecret
	Form   tokenForm
	Errors fieldErrors
	// Owners reports whether this listing spans actors other than the reader,
	// which is what puts the owner column on the table.
	Owners bool
	// Whole is false when the listing could not reach every actor's tokens, so
	// the screen says the table is short rather than presenting it as every
	// token of the tenant.
	Whole bool
}

// Fixed is the number of always-present columns, for the empty row's span.
func (v tokensView) Fixed() int {
	if v.Owners {
		return 4
	}
	return 3
}

// tokenExpiry is one option of the form's expiry control.
//
// Whole days rather than an instant, because "90 days" is the decision an
// operator is making and a datetime field makes them do the arithmetic, in a
// zone neither side has agreed on. The command line already takes an absolute
// date (tix token create --expires), so both grammars exist and neither
// surface has to parse the other's.
type tokenExpiry struct {
	Key   string
	Label string
	// Days is how long the token lasts; zero means it never expires.
	Days int
}

// tokenExpiryChoices are the expiries the form offers, and the only values it
// accepts: a key that is not one of these is refused rather than quietly read
// as no expiry.
var tokenExpiryChoices = []tokenExpiry{
	{Key: "7d", Label: "7 days", Days: 7},
	{Key: "30d", Label: "30 days", Days: 30},
	{Key: "90d", Label: "90 days", Days: 90},
	{Key: "365d", Label: "1 year", Days: 365},
	{Key: "never", Label: "Never expires", Days: 0},
}

// defaultTokenExpiry is what the form proposes before anybody chooses.
//
// An expiry, not "never". A token is normally handed to something outside the
// operator's control, and until this control existed every token the browser
// minted was immortal by omission rather than by anybody's decision, so there
// is no choice being overridden here. "Never expires" is still one option away
// for a credential that genuinely needs it.
const defaultTokenExpiry = "90d"

// tokenForm is what the issue-a-token form holds, so a refused submission
// comes back with everything the reader entered still in it.
type tokenForm struct {
	Name   string
	Scopes []core.Scope
	Expiry string
}

// newTokenForm is the empty form, carrying the proposed expiry.
func newTokenForm() tokenForm { return tokenForm{Expiry: defaultTokenExpiry} }

// tokenFormOf reads the submission back out of the request.
//
// A submission that names no expiry is read as the proposed one, in this one
// place, so the control a refused form comes back carrying is the same choice
// the handler would have applied. Normalising it only inside expiresAt left
// the re-rendered select with nothing selected, which showed the reader the
// first option while the handler meant the default.
func tokenFormOf(r *http.Request) tokenForm {
	out := tokenForm{Name: field(r, "name"), Expiry: field(r, "expires")}
	if out.Expiry == "" {
		out.Expiry = defaultTokenExpiry
	}
	for _, raw := range r.PostForm["scopes"] {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			out.Scopes = append(out.Scopes, core.Scope(trimmed))
		}
	}
	return out
}

// Chose reports whether one scope is selected, so a re-rendered control comes
// back with the same scopes ticked.
func (f tokenForm) Chose(s core.Scope) bool { return slices.Contains(f.Scopes, s) }

// ChoseExpiry reports whether one expiry option is the selected one.
func (f tokenForm) ChoseExpiry(key string) bool { return f.Expiry == key }

// expiresAt resolves the chosen expiry against now, returning nil for a token
// that never expires.
func (f tokenForm) expiresAt(now time.Time) (*time.Time, error) {
	for _, choice := range tokenExpiryChoices {
		if choice.Key != f.Expiry {
			continue
		}
		if choice.Days == 0 {
			return nil, nil
		}
		at := now.UTC().AddDate(0, 0, choice.Days)
		return &at, nil
	}
	return nil, core.Invalid("%q is not one of the expiry choices", f.Expiry).
		WithDetail(core.DetailField, "expires")
}

// showTokens renders the API tokens this reader may see, and a newly issued
// value once.
func (h *handler) showTokens(w http.ResponseWriter, r *http.Request) error {
	return h.renderTokens(w, r, http.StatusOK, newTokenForm(), nil, issuedToken(w, r))
}

// tokenListing returns the tokens this reader may see, labelled with whose
// each one is, and whether the listing reaches past the reader's own.
//
// A reader holding tenant:admin sees the tenant's tokens, because the
// credential that has to stop working now is usually somebody else's: it is
// the one that leaked. Anybody else sees their own, which is all this screen
// ever showed. The command line has taken another actor's identifier all
// along, so the browser was the one surface useless in an incident.
//
// Reaching the tenant's tokens is ListActors plus a listing per actor rather
// than one query, because the service's contract is per-actor on every
// surface. The directory is the size of a tenant's staff and agents, and this
// screen is read by an administrator during an incident, not in a loop.
func (h *handler) tokenListing(r *http.Request, actor *core.Actor) (tokenList, error) {
	if !actor.HasScope(core.ScopeTenantAdmin) {
		tokens, err := h.svc.ListTokens(r.Context(), actor.ID)
		if err != nil {
			return tokenList{}, err
		}
		return tokenList{Rows: h.labelTokens(r, actor, tokens), Whole: true}, nil
	}
	actors, whole, err := h.allActors(r)
	if err != nil {
		return tokenList{}, err
	}
	ids := make([]string, 0, len(actors)+1)
	for _, a := range actors {
		ids = append(ids, a.ID)
	}
	if !slices.Contains(ids, actor.ID) {
		ids = append(ids, actor.ID)
	}
	var all []core.APIToken
	for _, id := range ids {
		tokens, err := h.svc.ListTokens(r.Context(), id)
		if err != nil {
			return tokenList{}, err
		}
		all = append(all, tokens...)
	}
	return tokenList{Rows: h.labelTokens(r, actor, all), Owners: true, Whole: whole}, nil
}

// tokenList is the token table one reader may see: the rows, whether it
// reaches past the reader's own, and whether it is the whole of what it
// claims.
type tokenList struct {
	Rows   []tokenRow
	Owners bool
	// Whole is false when the actor directory is longer than allActors walked,
	// so some actor's tokens are missing from Rows. The listing says so rather
	// than reading as every token of the tenant.
	Whole bool
}

// actorScanPages bounds how many pages allActors will walk, for the reason
// projectScanPages bounds the project walk: an unbounded walk inside a request
// handler is how a request stops returning. At core.MaxPageLimit a page it
// covers more actors than a tenant has staff and agents, and a caller that
// reaches it is told the directory is short rather than handed a truncated one
// it reads as whole.
const actorScanPages = 40

// allActors is this tenant's actor directory, walked to the end of the
// listing, and whether the walk finished.
//
// One page of core.MaxPageLimit was what the token listing had, with the
// cursor discarded, so a tenant past 500 actors lost the tail of its
// credentials off a screen read during an incident. Raising the limit only
// moves where that starts to hurt, so the cursor is walked instead.
func (h *handler) allActors(r *http.Request) ([]core.Actor, bool, error) {
	page := core.Page{Limit: core.MaxPageLimit}
	var out []core.Actor
	for range actorScanPages {
		found, next, err := h.svc.ListActors(r.Context(), page)
		if err != nil {
			return nil, false, err
		}
		out = append(out, found...)
		if next == "" {
			return out, true, nil
		}
		page.Cursor = next
	}
	return out, false, nil
}

// labelTokens names the owner of every row and orders the listing by owner,
// so one actor's credentials sit together rather than interleaved by age.
func (h *handler) labelTokens(r *http.Request, actor *core.Actor, tokens []core.APIToken) []tokenRow {
	ids := make([]string, 0, len(tokens))
	for _, t := range tokens {
		ids = append(ids, t.ActorID)
	}
	names := h.resolveActors(r, ids...)
	out := make([]tokenRow, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, tokenRow{APIToken: t,
			Owner: names.Label(t.ActorID), Mine: t.ActorID == actor.ID})
	}
	slices.SortStableFunc(out, func(a, b tokenRow) int {
		if n := strings.Compare(a.Owner, b.Owner); n != 0 {
			return n
		}
		return a.CreatedAt.Compare(b.CreatedAt)
	})
	return out
}

// renderTokens draws the screen, whether it is being read or being read again
// after a refusal.
func (h *handler) renderTokens(w http.ResponseWriter, r *http.Request, status int,
	form tokenForm, errs fieldErrors, issued string) error {
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return err
	}
	list, err := h.tokenListing(r, actor)
	if err != nil {
		return err
	}
	return h.renderStatus(w, r, status, "tokens.html", "Tokens", tokensView{
		Tokens: list.Rows, Scopes: core.AllScopes, Expiry: tokenExpiryChoices,
		Secret: issuedTokenSecret(issued), Form: form, Errors: errs,
		Owners: list.Owners, Whole: list.Whole})
}

// refuseToken answers a submission the reader can fix by drawing the screen
// again, with the message beside the control it is about and every value they
// entered still in place.
//
// No script is involved: this is the response to the ordinary form POST, so it
// is what a browser with scripting off gets as well. The status is the
// refusal's own, which htmx swaps in because live.js already makes it do so.
func (h *handler) refuseToken(w http.ResponseWriter, r *http.Request, form tokenForm, err error) error {
	if !correctable(err) {
		return err
	}
	return h.renderTokens(w, r, core.KindOf(err).HTTPStatus(), form, refusal(err), "")
}

// issuedTokenCookie carries a freshly minted token to the screen that shows it
// once, so the value never appears in a URL or in the audit trail.
const issuedTokenCookie = "tix_issued_token"

// issuedToken reads and clears the one-time token cookie.
func issuedToken(w http.ResponseWriter, r *http.Request) string {
	return readOneTimeCookie(w, r, issuedTokenCookie, RouteTokens)
}

// createToken mints an API token and shows its value exactly once.
func (h *handler) createToken(w http.ResponseWriter, r *http.Request) error {
	form := tokenFormOf(r)
	expiresAt, err := form.expiresAt(time.Now())
	if err != nil {
		return h.refuseToken(w, r, form, err)
	}
	issued, err := h.svc.CreateToken(r.Context(), core.CreateTokenInput{
		Name: form.Name, Scopes: form.Scopes, ExpiresAt: expiresAt})
	if err != nil {
		return h.refuseToken(w, r, form, err)
	}
	// #nosec G124 -- one-time value, HttpOnly, cleared by the screen that shows it
	http.SetCookie(w, oneTimeCookie(issuedTokenCookie, RouteTokens, issued.Token, h.secureCookie(r)))
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
