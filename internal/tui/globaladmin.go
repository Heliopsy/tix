// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// globalMode is the part of the global admin view that is open. The web
// spreads the area over three screens; here they are one view, and Back walks
// out of a mode before it leaves the view.
type globalMode int

// The global admin view's modes.
const (
	globalHome globalMode = iota
	globalUsers
	globalLog
)

// The row kinds and mode names the footer is told about, named once so the
// bindings and the context cannot spell one differently.
const (
	globalRowTenant = "tenant"
	globalRowAdmin  = "admin"
	globalRowUser   = "user"
	globalRowEntry  = "entry"

	globalModeHome  = "tenants"
	globalModeUsers = "users"
	globalModeLog   = "log"
)

// globalLimit is how much of one listing the view reads. The rest belongs to
// the command line's --cursor, and the view says so when there is more.
const globalLimit = 100

// globalState is everything the global admin view renders from. Each listing
// carries its own refusal.
type globalState struct {
	mode   globalMode
	loaded bool

	tenants     []core.Tenant
	tenantsMore bool
	tenantsErr  string
	admins      []core.User
	adminsErr   string
	// credentials are the passwords the last tenant creation generated, shown
	// until the view is read again or left.
	credentials []core.InitialCredential

	tenant    string
	users     []core.TenantUser
	usersMore bool
	usersErr  string
	editing   core.TenantUser

	logEmail string
	log      []core.AccountLogEntry
	logMore  bool
	logFrom  globalMode

	homeSel, usersSel, logSel int
	off                       int
}

// globalHomeMsg carries the tenants and the global admins. keep holds on to
// generated passwords across a read an action asked for.
type globalHomeMsg struct {
	tenants    []core.Tenant
	more       bool
	tenantsErr error
	admins     []core.User
	adminsErr  error
	keep       bool
}

// globalUsersMsg carries one tenant's users.
type globalUsersMsg struct {
	tenant string
	users  []core.TenantUser
	more   bool
	err    error
}

// globalLogMsg carries one account's log.
type globalLogMsg struct {
	email   string
	entries []core.AccountLogEntry
	more    bool
	err     error
	from    globalMode
}

// globalActionMsg reports one global admin action.
type globalActionMsg struct {
	done    string
	failed  string
	err     error
	created *core.CreatedTenant
}

// globalAction is one of the view's keys: what it does, the authority it
// needs, the modes it acts in and whether it needs a row under the cursor.
type globalAction struct {
	viewAction
	modes   []string
	needRow []string
}

// globalBindings are the global admin view's actions, one list for the
// footer, the help overlay and the keystroke.
func (k KeyMap) globalBindings() []globalAction {
	home, users := globalModeHome, globalModeUsers
	return []globalAction{
		{viewAction{gatedAction{binding: k.Enter, methods: []string{"ListTenantUsers", "GetAccountLogFor"}},
			"open users or account log"}, []string{home, users},
			[]string{globalRowTenant, globalRowAdmin, globalRowUser}},
		{viewAction{gatedAction{binding: k.New, methods: []string{"CreateTenant", "GrantGlobalAdmin"}},
			"create tenant or grant global admin"}, []string{home}, nil},
		{viewAction{gatedAction{binding: k.Delete, methods: []string{"DeleteTenant", "RevokeGlobalAdmin"}},
			"delete tenant or revoke global admin"}, []string{home}, []string{globalRowTenant, globalRowAdmin}},
		{viewAction{gatedAction{binding: k.New, methods: []string{"CreateTenantUser", "AddTenantMember"}},
			"create user or add an account"}, []string{users}, nil},
		{viewAction{gatedAction{binding: k.Edit, methods: []string{"UpdateTenantUser"}},
			"edit the selected user"}, []string{users}, []string{globalRowUser}},
		{viewAction{gatedAction{binding: k.Delete, methods: []string{"DeleteTenantUser"}},
			"delete the selected user"}, []string{users}, []string{globalRowUser}},
		{viewAction{gatedAction{binding: k.Filter, methods: []string{"GetAccountLogFor"}},
			"account log by email"}, []string{home, users}, nil},
	}
}

// globalHelp is every action of the view the reader's authority reaches.
func (k KeyMap) globalHelp(may func(string) bool) []HelpEntry {
	out := []HelpEntry{}
	for _, a := range k.globalBindings() {
		if a.permitted(may) {
			out = append(out, a.entry())
		}
	}
	return out
}

// globalShort is the footer: the actions the open mode answers, for the row
// under the cursor.
func (k KeyMap) globalShort(ctx ActionContext) []HelpEntry {
	var out []HelpEntry
	for _, a := range k.globalBindings() {
		if a.applies(ctx.Mode, ctx.Row) && a.permitted(ctx.May) {
			out = append(out, a.entry())
		}
	}
	return out
}

// applies reports whether an action acts in a mode on a row kind.
func (a globalAction) applies(mode, row string) bool {
	in := false
	for _, m := range a.modes {
		in = in || m == mode
	}
	if !in {
		return false
	}
	if a.needRow == nil {
		return true
	}
	for _, r := range a.needRow {
		if r == row {
			return true
		}
	}
	return false
}

// GlobalState is the view's input, exported for the pure renderer.
type GlobalState struct {
	Mode        globalMode
	Loaded      bool
	Own         string
	Tenants     []core.Tenant
	TenantsMore bool
	TenantsErr  string
	Admins      []core.User
	AdminsErr   string
	Credentials []core.InitialCredential
	Tenant      string
	Users       []core.TenantUser
	UsersMore   bool
	UsersErr    string
	LogEmail    string
	Log         []core.AccountLogEntry
	LogMore     bool
	Selected    int
	Times       output.TimeStyle
}

// GlobalLines renders the open mode of the global admin view.
func GlobalLines(s GlobalState) []TenantLine {
	switch s.Mode {
	case globalUsers:
		return globalUserLines(s)
	case globalLog:
		return globalLogLines(s)
	default:
		return globalHomeLines(s)
	}
}

// globalHomeLines list every tenant, any generated passwords and the global
// admins, under one cursor running over the tenants and then the admins.
func globalHomeLines(s GlobalState) []TenantLine {
	out := []TenantLine{{Text: "tenants (" + strconv.Itoa(len(s.Tenants)) + "):", Heading: true}}
	if len(s.Tenants) == 0 {
		out = append(out, tenantAbsent(s.TenantsErr, s.Loaded)...)
	}
	for i, t := range s.Tenants {
		selected := i == s.Selected
		row := SelectionMarker(selected) + pad(t.Key, tenantRowWidth) + orNone(t.Name)
		if t.ID == s.Own {
			row += "  (this session)"
		}
		out = append(out, TenantLine{Text: row, Selected: selected})
	}
	if s.TenantsMore {
		out = append(out, TenantLine{Text: "  more with tix tenant ls --cursor", Dim: true})
	}
	if len(s.Credentials) > 0 {
		out = append(out, TenantLine{}, TenantLine{Text: "generated passwords (shown once):", Heading: true})
		for _, c := range s.Credentials {
			out = append(out, TenantLine{Text: "  " + pad(c.Email, tenantRowWidth) + pad("@"+c.Handle, 16) + c.Password})
		}
	}
	out = append(out, TenantLine{},
		TenantLine{Text: "global admins (" + strconv.Itoa(len(s.Admins)) + "):", Heading: true})
	if len(s.Admins) == 0 {
		out = append(out, tenantAbsent(s.AdminsErr, s.Loaded)...)
	}
	for i, u := range s.Admins {
		selected := len(s.Tenants)+i == s.Selected
		out = append(out, TenantLine{Text: SelectionMarker(selected) + pad(u.Email, tenantRowWidth) + u.DisplayName,
			Selected: selected})
	}
	return out
}

// globalUserLines list one tenant's users with their role and state.
func globalUserLines(s GlobalState) []TenantLine {
	out := []TenantLine{{Text: "users of " + s.Tenant + " (" + strconv.Itoa(len(s.Users)) + "):", Heading: true}}
	if len(s.Users) == 0 {
		return append(out, tenantAbsent(s.UsersErr, s.Loaded)...)
	}
	for i, u := range s.Users {
		selected := i == s.Selected
		role := string(u.Role)
		if role == "" {
			role = "no membership"
		}
		row := SelectionMarker(selected) + pad(u.Email, tenantRowWidth) + pad(role, 14) + u.DisplayName
		if u.DisabledAt != nil {
			row += "  disabled"
		}
		out = append(out, TenantLine{Text: row, Selected: selected})
	}
	if s.UsersMore {
		out = append(out, TenantLine{Text: "  more with tix tenant user ls --cursor", Dim: true})
	}
	return out
}

// globalLogLines list one account's log, oldest first, as the service orders it.
func globalLogLines(s GlobalState) []TenantLine {
	out := []TenantLine{{Text: "account log of " + s.LogEmail + " (" + strconv.Itoa(len(s.Log)) + "):", Heading: true}}
	if len(s.Log) == 0 {
		return append(out, TenantLine{Text: "  none", Dim: true})
	}
	for i, e := range s.Log {
		selected := i == s.Selected
		out = append(out, TenantLine{Text: SelectionMarker(selected) + AccountLogRow(e, s.Times), Selected: selected})
	}
	if s.LogMore {
		out = append(out, TenantLine{Text: "  more with tix global-admin account-log --cursor", Dim: true})
	}
	return out
}

// AccountLogRow renders one change to an account in words: when, what, in
// which tenant, by whom and through what.
func AccountLogRow(e core.AccountLogEntry, times output.TimeStyle) string {
	parts := []string{times.Format(e.OccurredAt), string(e.Action)}
	if e.TenantKey != "" {
		parts = append(parts, "in "+e.TenantKey)
	}
	by := e.ByEmail
	if by == "" {
		by = "the system"
	}
	if e.ByGlobalAdmin {
		by += " as global admin"
	}
	parts = append(parts, "by "+by)
	if e.Via != "" {
		parts = append(parts, "via "+string(e.Via))
	}
	if len(e.Detail) > 0 {
		keys := make([]string, 0, len(e.Detail))
		for k := range e.Detail {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		details := make([]string, 0, len(keys))
		for _, k := range keys {
			details = append(details, k+"="+fmt.Sprint(e.Detail[k]))
		}
		parts = append(parts, strings.Join(details, " "))
	}
	return strings.Join(parts, "  ")
}

// The subjects the two add forms choose between.
const (
	addTenant      = "tenant"
	addGlobalAdmin = "global admin"
	addNewUser     = "new user"
	addAccount     = "existing account"
)

// emailLimit bounds a typed address, comfortably past what the service takes.
const emailLimit = 254

// GlobalAddForm creates a tenant with its first admins, or grants global admin
// by email, offering only what this reader may do.
func GlobalAddForm(mayTenant, mayGrant bool) Form {
	var subjects []string
	if mayTenant {
		subjects = append(subjects, addTenant)
	}
	if mayGrant {
		subjects = append(subjects, addGlobalAdmin)
	}
	if len(subjects) == 0 {
		return Form{}
	}
	tenant := FieldCondition{Key: "subject", Value: addTenant}
	return Form{
		Kind: formGlobalAdd, Title: "create tenant or grant global admin",
		Note: "first admins: addresses separated by commas; a new address gets a generated password",
		Fields: []FormField{
			{Key: "subject", Label: "subject", Options: subjects, Value: subjects[0]},
			{Key: "key", Label: "key", Kind: FieldText, Limit: 64, Needs: tenant},
			{Key: "name", Label: "name", Kind: FieldText, Limit: 200, Needs: tenant},
			{Key: "admins", Label: "first admins", Kind: FieldText, Limit: 2000, Needs: tenant},
			{Key: "email", Label: "email", Kind: FieldText, Limit: emailLimit,
				Needs: FieldCondition{Key: "subject", Value: addGlobalAdmin}},
		},
	}
}

// GlobalUserAddForm creates a user in a tenant or adds an existing account to
// it, offering only what this reader may do.
func GlobalUserAddForm(tenant string, mayCreate, mayAdd bool) Form {
	var subjects []string
	if mayCreate {
		subjects = append(subjects, addNewUser)
	}
	if mayAdd {
		subjects = append(subjects, addAccount)
	}
	if len(subjects) == 0 {
		return Form{}
	}
	fresh := FieldCondition{Key: "subject", Value: addNewUser}
	return Form{
		Kind: formGlobalUserAdd, Title: "add to " + tenant,
		Note: "a new user with no password signs in some other way until one is set",
		Fields: []FormField{
			{Key: "subject", Label: "subject", Options: subjects, Value: subjects[0]},
			{Key: "email", Label: "email", Kind: FieldText, Limit: emailLimit},
			{Key: "handle", Label: "handle", Kind: FieldText, Limit: 64},
			{Key: "display", Label: "display name", Kind: FieldText, Limit: 200, Needs: fresh},
			{Key: "password", Label: "password", Kind: FieldText, Secret: true, Limit: 1024, Needs: fresh},
			{Key: "role", Label: "role", Options: RoleOptions(), Value: string(core.RoleViewer)},
		},
	}
}

// GlobalUserEditForm changes a user's display name, role and disabled state,
// starting from what they hold now.
func GlobalUserEditForm(u core.TenantUser) Form {
	role := string(u.Role)
	if role == "" {
		role = string(core.RoleViewer)
	}
	disabled := noValue
	if u.DisabledAt != nil {
		disabled = yesValue
	}
	return Form{
		Kind: formGlobalUserEdit, Title: "edit " + u.Email,
		Fields: []FormField{
			{Key: "display", Label: "display name", Kind: FieldText, Limit: 200, Value: u.DisplayName},
			{Key: "role", Label: "role", Options: RoleOptions(), Value: role},
			{Key: "disabled", Label: "disabled", Options: switchOptions(), Value: disabled},
		},
	}
}

// GlobalLogForm asks for the address whose account log to read.
func GlobalLogForm() Form {
	return Form{
		Kind: formGlobalLog, Title: "account log",
		Fields: []FormField{{Key: "email", Label: "email", Kind: FieldText, Limit: emailLimit}},
	}
}

// AdminInputs reads the first admins from one line: addresses separated by
// commas, semicolons or spaces, the way the web form takes them.
func AdminInputs(raw string) []core.TenantAdminInput {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})
	out := make([]core.TenantAdminInput, 0, len(fields))
	for _, f := range fields {
		out = append(out, core.TenantAdminInput{Email: f})
	}
	return out
}

// isGlobalForm reports whether a form belongs to the global admin view.
func isGlobalForm(k formKind) bool {
	switch k {
	case formGlobalAdd, formGlobalUserAdd, formGlobalUserEdit, formGlobalLog:
		return true
	default:
		return false
	}
}

// openGlobal shows the tenants and the global admins.
func (m Model) openGlobal() (Model, tea.Cmd) {
	next := m.enterView(viewGlobal)
	if next.view != viewGlobal {
		return next, nil
	}
	next.global = globalState{}
	return next, next.loadGlobalHome(false)
}

// reloadGlobal reads the open mode again, which is what a refresh asks for.
// Generated passwords do not survive it.
func (m Model) reloadGlobal() tea.Cmd { return m.reloadGlobalMode(false) }

// reloadGlobalMode reads whichever mode is open.
func (m Model) reloadGlobalMode(keep bool) tea.Cmd {
	switch m.global.mode {
	case globalUsers:
		return m.loadGlobalUsers(m.global.tenant)
	case globalLog:
		return m.loadGlobalLog(m.global.logEmail, m.global.logFrom)
	default:
		return m.loadGlobalHome(keep)
	}
}

// loadGlobalHome reads the tenants and the global admins.
func (m Model) loadGlobalHome(keep bool) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		msg := globalHomeMsg{keep: keep}
		var next string
		msg.tenants, next, msg.tenantsErr = svc.ListTenants(ctx, core.Page{Limit: globalLimit})
		msg.more = next != ""
		msg.admins, msg.adminsErr = svc.ListGlobalAdmins(ctx)
		return msg
	}
}

// loadGlobalUsers reads one tenant's users.
func (m Model) loadGlobalUsers(tenant string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		users, next, err := svc.ListTenantUsers(ctx, tenant, core.UserFilter{Page: core.Page{Limit: globalLimit}})
		return globalUsersMsg{tenant: tenant, users: users, more: next != "", err: err}
	}
}

// loadGlobalLog reads one account's log.
func (m Model) loadGlobalLog(email string, from globalMode) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		entries, next, err := svc.GetAccountLogFor(ctx, email, core.Page{Limit: globalLimit})
		return globalLogMsg{email: email, entries: entries, more: next != "", err: err, from: from}
	}
}

// onGlobalHome adopts the tenants and the admins.
func (m Model) onGlobalHome(msg globalHomeMsg) (Model, tea.Cmd) {
	g := &m.global
	g.loaded = true
	g.tenants, g.tenantsMore, g.tenantsErr = msg.tenants, msg.more, refusalText(msg.tenantsErr)
	g.admins, g.adminsErr = msg.admins, refusalText(msg.adminsErr)
	if msg.tenantsErr != nil {
		g.tenants = nil
	}
	if msg.adminsErr != nil {
		g.admins = nil
	}
	if !msg.keep {
		g.credentials = nil
	}
	g.homeSel = clamp(g.homeSel, 0, len(g.tenants)+len(g.admins)-1)
	return m, nil
}

// onGlobalUsers opens a tenant's users, saying in place why they are not
// shown when the read was refused.
func (m Model) onGlobalUsers(msg globalUsersMsg) (Model, tea.Cmd) {
	g := &m.global
	if g.mode != globalUsers || g.tenant != msg.tenant {
		g.usersSel, g.off = 0, 0
	}
	g.mode, g.tenant = globalUsers, msg.tenant
	g.users, g.usersMore, g.usersErr = msg.users, msg.more, refusalText(msg.err)
	if msg.err != nil {
		g.users = nil
	}
	g.usersSel = clamp(g.usersSel, 0, len(g.users)-1)
	m.err = ""
	return m, nil
}

// onGlobalLog opens an account's log, or says why it could not.
func (m Model) onGlobalLog(msg globalLogMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "reading the account log of " + msg.email + ": " + msg.err.Error()
		return m, nil
	}
	g := &m.global
	if g.mode != globalLog || g.logEmail != msg.email {
		g.logSel, g.off = 0, 0
		g.logFrom = msg.from
	}
	g.mode, g.logEmail = globalLog, msg.email
	g.log, g.logMore = msg.entries, msg.more
	g.logSel = clamp(g.logSel, 0, len(g.log)-1)
	m.err = ""
	return m, nil
}

// onGlobalAction reports an action and reads the open mode again, keeping any
// passwords the action generated.
func (m Model) onGlobalAction(msg globalActionMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.failed + " failed: " + msg.err.Error()
		return m, nil
	}
	m.status, m.err = msg.done, ""
	if msg.created != nil && len(msg.created.InitialCredentials) > 0 {
		m.global.credentials = msg.created.InitialCredentials
	}
	return m, m.reloadGlobalMode(true)
}

// refusalText renders an error for a section, empty when there is none.
func refusalText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// globalRows names the kind of each row the cursor runs over in the open mode.
func (m Model) globalRows() []string {
	g := m.global
	var out []string
	switch g.mode {
	case globalUsers:
		for range g.users {
			out = append(out, globalRowUser)
		}
	case globalLog:
		for range g.log {
			out = append(out, globalRowEntry)
		}
	default:
		for range g.tenants {
			out = append(out, globalRowTenant)
		}
		for range g.admins {
			out = append(out, globalRowAdmin)
		}
	}
	return out
}

// globalSel is the open mode's cursor.
func (m *Model) globalSel() *int {
	switch m.global.mode {
	case globalUsers:
		return &m.global.usersSel
	case globalLog:
		return &m.global.logSel
	default:
		return &m.global.homeSel
	}
}

// globalContext names the open mode and the kind of row under the cursor, for
// the footer.
func (m Model) globalContext() (string, string) {
	mode := map[globalMode]string{globalHome: globalModeHome, globalUsers: globalModeUsers,
		globalLog: globalModeLog}[m.global.mode]
	rows := m.globalRows()
	sel := *m.globalSel()
	if sel < 0 || sel >= len(rows) {
		return mode, ""
	}
	return mode, rows[sel]
}

// handleGlobalKey moves the cursor, walks out of a mode and runs the view's
// actions, refusing a key the open mode or the reader's authority does not
// reach rather than letting the service refuse it.
func (m Model) handleGlobalKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	rows := len(m.globalRows())
	sel := m.globalSel()
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.globalBack()
	case key.Matches(msg, m.keys.Up):
		*sel = clamp(*sel-1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		*sel = clamp(*sel+1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Top):
		*sel, m.global.off = 0, 0
		return m, nil
	case key.Matches(msg, m.keys.Bottom):
		*sel = clamp(rows-1, 0, rows-1)
		return m, nil
	}
	mode, row := m.globalContext()
	for _, a := range m.keys.globalBindings() {
		if !key.Matches(msg, a.binding) || !a.applies(mode, row) {
			continue
		}
		if !a.permitted(m.permits()) {
			return m, nil
		}
		return m.globalPerform(a.desc, row)
	}
	return m, nil
}

// globalBack leaves a log for where it was opened from, a tenant's users for
// the tenants, and the tenants for the view below.
func (m Model) globalBack() (Model, tea.Cmd) {
	switch m.global.mode {
	case globalLog:
		m.global.mode, m.global.off = m.global.logFrom, 0
		return m, nil
	case globalUsers:
		m.global.mode, m.global.off = globalHome, 0
		return m, nil
	}
	m.global.credentials = nil
	return m.leave(nil)
}

// globalPerform runs one of the view's actions on the row under the cursor.
func (m Model) globalPerform(desc, row string) (Model, tea.Cmd) {
	switch desc {
	case "open users or account log":
		return m.globalOpen(row)
	case "create tenant or grant global admin":
		return m.openForm(GlobalAddForm(m.mayPerform("CreateTenant"), m.mayPerform("GrantGlobalAdmin")))
	case "delete tenant or revoke global admin":
		return m.confirmGlobalRemoval(row)
	case "create user or add an account":
		return m.openForm(GlobalUserAddForm(m.global.tenant,
			m.mayPerform("CreateTenantUser"), m.mayPerform("AddTenantMember")))
	case "edit the selected user":
		u := m.global.users[m.global.usersSel]
		m.global.editing = u
		return m.openForm(GlobalUserEditForm(u))
	case "delete the selected user":
		u := m.global.users[m.global.usersSel]
		return m.askConfirm(Confirm{Kind: confirmDeleteTenantUser, Target: u.Email + " from " + m.global.tenant,
			subject: u.ID, within: m.global.tenant}, "there is no user named to delete")
	case "account log by email":
		return m.openForm(GlobalLogForm())
	}
	return m, nil
}

// globalOpen opens a tenant's users, or the account log of an admin or user.
func (m Model) globalOpen(row string) (Model, tea.Cmd) {
	g := m.global
	switch row {
	case globalRowTenant:
		if !m.mayPerform("ListTenantUsers") {
			return m, nil
		}
		return m, m.loadGlobalUsers(g.tenants[g.homeSel].Key)
	case globalRowAdmin:
		if !m.mayPerform("GetAccountLogFor") {
			return m, nil
		}
		return m, m.loadGlobalLog(g.admins[g.homeSel-len(g.tenants)].Email, globalHome)
	case globalRowUser:
		if !m.mayPerform("GetAccountLogFor") {
			return m, nil
		}
		return m, m.loadGlobalLog(g.users[g.usersSel].Email, globalUsers)
	}
	return m, nil
}

// confirmGlobalRemoval names the selected tenant or admin and waits for
// agreement. The tenant this session is pinned to is refused: deleting it
// would end the session's own access mid-action.
func (m Model) confirmGlobalRemoval(row string) (Model, tea.Cmd) {
	g := m.global
	switch row {
	case globalRowTenant:
		if !m.mayPerform("DeleteTenant") {
			return m, nil
		}
		t := g.tenants[g.homeSel]
		if m.actor != nil && t.ID == m.actor.TenantID {
			m.err = "this session is on " + t.Key + "; delete it from another tenant or with tix tenant rm"
			return m, nil
		}
		return m.askConfirm(Confirm{Kind: confirmDeleteTenant, Target: t.Key, subject: t.Key,
			Note: "with everything in it"}, "there is no tenant named to delete")
	case globalRowAdmin:
		if !m.mayPerform("RevokeGlobalAdmin") {
			return m, nil
		}
		u := g.admins[g.homeSel-len(g.tenants)]
		return m.askConfirm(Confirm{Kind: confirmRevokeGlobal, Target: u.Email, subject: u.Email},
			"there is no global admin named to revoke")
	}
	return m, nil
}

// runGlobalConfirm makes the call a global admin confirmation stood in front of.
func (m Model) runGlobalConfirm(c Confirm) tea.Cmd {
	switch c.Kind {
	case confirmDeleteTenant:
		return m.globalCall("deleted tenant "+c.subject, "delete the tenant", func(svc core.Service) error {
			return svc.DeleteTenant(m.ctx, c.subject)
		})
	case confirmRevokeGlobal:
		return m.globalCall("revoked global admin from "+c.subject, "revoke global admin",
			func(svc core.Service) error { return svc.RevokeGlobalAdmin(m.ctx, c.subject) })
	case confirmDeleteTenantUser:
		return m.globalCall("deleted user "+c.Target, "delete the user", func(svc core.Service) error {
			return svc.DeleteTenantUser(m.ctx, c.within, c.subject)
		})
	}
	return nil
}

// globalCall runs one action and reports it.
func (m Model) globalCall(done, failed string, call func(core.Service) error) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc := m.svc
	return func() tea.Msg {
		return globalActionMsg{done: done, failed: failed, err: call(svc)}
	}
}

// applyGlobalForm performs what one of the view's forms gathered.
func (m Model) applyGlobalForm(form Form) (Model, tea.Cmd) {
	switch form.Kind {
	case formGlobalAdd:
		return m, m.applyGlobalAdd(form)
	case formGlobalUserAdd:
		return m, m.applyGlobalUserAdd(form)
	case formGlobalUserEdit:
		return m, m.applyGlobalUserEdit(form)
	case formGlobalLog:
		return m, m.loadGlobalLog(strings.TrimSpace(form.Value("email")), m.global.mode)
	}
	return m, nil
}

// applyGlobalAdd creates the tenant or grants global admin.
func (m Model) applyGlobalAdd(form Form) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	if form.Value("subject") == addGlobalAdmin {
		email := strings.TrimSpace(form.Value("email"))
		return m.globalCall("granted global admin to "+email, "grant global admin", func(svc core.Service) error {
			_, err := svc.GrantGlobalAdmin(ctx, core.GlobalAdminInput{Email: email})
			return err
		})
	}
	in := core.CreateTenantInput{Key: strings.TrimSpace(form.Value("key")),
		Name: strings.TrimSpace(form.Value("name")), Admins: AdminInputs(form.Value("admins"))}
	return func() tea.Msg {
		created, err := svc.CreateTenant(ctx, in)
		return globalActionMsg{done: "created tenant " + in.Key, failed: "create the tenant", err: err, created: created}
	}
}

// applyGlobalUserAdd creates a user in the open tenant or adds an account to it.
func (m Model) applyGlobalUserAdd(form Form) tea.Cmd {
	ctx, tenant := m.ctx, m.global.tenant
	email := strings.TrimSpace(form.Value("email"))
	handle := strings.TrimSpace(form.Value("handle"))
	role := core.Role(form.Value("role"))
	if form.Value("subject") == addAccount {
		in := core.AddTenantMemberInput{Email: email, Handle: handle, Role: role}
		return m.globalCall("added "+email+" to "+tenant, "add the account", func(svc core.Service) error {
			_, err := svc.AddTenantMember(ctx, tenant, in)
			return err
		})
	}
	in := core.CreateUserInput{Email: email, Handle: handle, Role: role,
		DisplayName: strings.TrimSpace(form.Value("display")), Password: form.Value("password")}
	return m.globalCall("created user "+email+" in "+tenant, "create the user", func(svc core.Service) error {
		_, err := svc.CreateTenantUser(ctx, tenant, in)
		return err
	})
}

// applyGlobalUserEdit sends only what the form changed.
func (m Model) applyGlobalUserEdit(form Form) tea.Cmd {
	ctx, tenant, u := m.ctx, m.global.tenant, m.global.editing
	var in core.UpdateUserInput
	changed := false
	if name := strings.TrimSpace(form.Value("display")); name != u.DisplayName {
		in.DisplayName, changed = &name, true
	}
	if role := core.Role(form.Value("role")); role != u.Role {
		in.Role, changed = &role, true
	}
	if disabled := form.Yes("disabled"); disabled != (u.DisabledAt != nil) {
		in.Disabled, changed = &disabled, true
	}
	if !changed {
		return func() tea.Msg { return globalActionMsg{done: "nothing changed for " + u.Email} }
	}
	return m.globalCall("edited "+u.Email, "edit the user", func(svc core.Service) error {
		_, err := svc.UpdateTenantUser(ctx, tenant, u.ID, in)
		return err
	})
}

// globalLines renders the open mode, scrolled so a long listing stays reachable.
func (m Model) globalLines(layout Layout) []string {
	g := m.global
	own := ""
	if m.actor != nil {
		own = m.actor.TenantID
	}
	raw := GlobalLines(GlobalState{
		Mode: g.mode, Loaded: g.loaded, Own: own,
		Tenants: g.tenants, TenantsMore: g.tenantsMore, TenantsErr: g.tenantsErr,
		Admins: g.admins, AdminsErr: g.adminsErr, Credentials: g.credentials,
		Tenant: g.tenant, Users: g.users, UsersMore: g.usersMore, UsersErr: g.usersErr,
		LogEmail: g.logEmail, Log: g.log, LogMore: g.logMore,
		Selected: *m.globalSel(), Times: m.timeStyle,
	})
	return m.scrolledLines(raw, &m.global.off, layout)
}
