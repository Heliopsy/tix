// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// TenantConn is a connection pinned to one tenant, as a dialer opens it.
// Close releases whatever the dialer opened; the interface calls it once the
// connection it replaced is no longer being read from.
type TenantConn struct {
	Service core.Service
	Context context.Context
	Close   func() error
}

// TenantDialer opens a connection pinned to a tenant key. It is supplied by
// the command layer, because which database or server a key resolves against
// is configuration, which `internal/tui` does not read.
type TenantDialer func(ctx context.Context, key string) (TenantConn, error)

// TenantReachable reports whether a credential for a tenant is at hand: a
// context holding a token for it, or a database opened directly. It is
// supplied by the command layer, which reads the configuration.
type TenantReachable func(key string) bool

// TenantSwitchPlan validates a typed tenant key against the one in force. It
// is the whole decision the tenant view makes about a typed key before
// anything is dialled, so a typo can only be caught by trying.
func TenantSwitchPlan(current, typed string) (string, error) {
	key := strings.TrimSpace(typed)
	switch {
	case key == "":
		return "", core.Invalid("a tenant key is required")
	case strings.EqualFold(key, strings.TrimSpace(current)):
		return "", core.Invalid("this session is already on tenant %q", key)
	default:
		return key, nil
	}
}

// TenantSwitchNotice is what the status bar says once a switch has landed. A
// lease is held by the tenant it was taken in, so leases this session was
// holding are not released by walking away from them: they are dropped from
// the session and expire on their own, and saying so is the difference
// between a surprise and a decision.
func TenantSwitchNotice(key string, leases int) string {
	notice := "switched to tenant " + key
	if leases == 0 {
		return notice
	}
	return fmt.Sprintf("%s; %s dropped from this session and will expire unreleased",
		notice, plural(leases, "lease", "leases"))
}

// plural renders a count with the word that agrees with it.
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// TenantViewLines is what the tenant view says about switching. The tenants
// to pick from are the reader's memberships, listed above it when there is
// more than one; a typed key still works, and is checked by opening a
// connection pinned to it and asking who the actor is there, as
// `tix tenant use` does.
func TenantViewLines(current, handle string, canSwitch bool) []string {
	lines := []string{"tenant", ""}
	lines = append(lines, "  current: "+orUnknown(current))
	if handle != "" {
		lines = append(lines, "  actor:   "+handle)
	}
	lines = append(lines, "")
	if !canSwitch {
		return append(lines,
			"  This session cannot switch tenants: it was opened without a way to",
			"  dial another one.",
			"",
			"  Switch outside the interface with:  tix tenant use KEY")
	}
	return append(lines,
		"  Pick one of your tenants under memberships with enter, or press enter",
		"  elsewhere and type a tenant key.",
		"",
		"  A switch needs a credential for the tenant: a context holding its",
		"  token, which tix login --tenant KEY stores, or a database opened here.",
		"  A typed key is checked by opening it and asking who you are there, the",
		"  way tix tenant use does.",
		"",
		"  The switch lasts for this session only; tix tenant use writes it down.")
}

// orUnknown names a tenant the session was never told the key of, which is
// what a run against a target with no tenant selected has.
func orUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "not named by this session's configuration"
	}
	return value
}

// openTenant shows the tenant screen and reads what it states. The screen is
// read rather than remembered: it states what the tenant holds after the last
// action it performed, not what the session was configured with.
func (m Model) openTenant() (Model, tea.Cmd) {
	next := m.enterView(viewTenant)
	if next.view != viewTenant {
		return next, nil
	}
	next.adminSel, next.adminOff = 0, 0
	return next, next.loadTenantInfo()
}

// onTenantInfo adopts a read of the tenant, bounding the cursor to the rows
// that came back so a selection held over a removal cannot mark a row that is
// no longer there.
func (m Model) onTenantInfo(msg tenantInfoMsg) (Model, tea.Cmd) {
	m.admin = &msg
	m.adminSel = clamp(m.adminSel, 0, len(TenantRows(m.adminState()))-1)
	return m, nil
}

// adminState is what the tenant screen renders from.
func (m Model) adminState() TenantAdminState {
	handle := ""
	if m.actor != nil {
		handle = m.actor.Handle
	}
	state := TenantAdminState{
		Current: m.currentTenant(), Handle: handle, CanSwitch: m.dialTenant != nil,
		Selected: m.adminSel, Reachable: m.reachable, OverSSH: m.overSSH,
	}
	if m.admin == nil {
		return state
	}
	state.Loaded = true
	state.Tenant, state.Tenants = m.admin.tenant, m.admin.tenants
	state.Domains, state.Members, state.Handles = m.admin.domains, m.admin.members, m.admin.handles
	state.Memberships = m.admin.memberships
	state.TenantErr, state.TenantsErr = m.admin.tenantErr, m.admin.tenantsErr
	state.DomainErr, state.MemberErr = m.admin.domainErr, m.admin.memberErr
	return state
}

// adminRows are the domains and the members the cursor runs over.
func (m Model) adminRows() []TenantRow { return TenantRows(m.adminState()) }

// selectedAdminRow is the domain or the membership the removal key acts on.
func (m Model) selectedAdminRow() (TenantRow, bool) {
	rows := m.adminRows()
	if m.adminSel < 0 || m.adminSel >= len(rows) {
		return TenantRow{}, false
	}
	return rows[m.adminSel], true
}

// handleTenantKey moves the cursor, opens the input the view gathers a tenant
// key with, and runs the actions that administer the tenant in force.
func (m Model) handleTenantKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	rows := len(m.adminRows())
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.adminSel = clamp(m.adminSel-1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.adminSel = clamp(m.adminSel+1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Top):
		m.adminSel, m.adminOff = 0, 0
		return m, nil
	case key.Matches(msg, m.keys.Bottom):
		m.adminSel = clamp(rows-1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		if row, ok := m.selectedAdminRow(); ok && row.Kind == tenantRowMembership {
			return m.chooseMembership(row.Key)
		}
		if m.overSSH {
			m.err = "over ssh the username picks the tenant; reconnect with " + SSHTenantHint("TENANT")
			return m, nil
		}
		if m.dialTenant == nil {
			m.err = "this session cannot switch tenants; use tix tenant use KEY"
			return m, nil
		}
		return m.startPrompt(promptTenant, ""), textinput.Blink
	}
	return m.handleAdminKey(msg)
}

// currentTenant is the key of the tenant this session works in: the one it
// was told, else the one whoami named.
func (m Model) currentTenant() string {
	if m.tenantKey == "" && m.actor != nil {
		return m.actor.TenantKey
	}
	return m.tenantKey
}

// chooseMembership switches to one of the reader's tenants, or says why it
// cannot: over SSH the username picks the tenant, and on a server a switch
// needs a context holding the tenant's token.
func (m Model) chooseMembership(tenant string) (Model, tea.Cmd) {
	switch {
	case strings.EqualFold(tenant, m.currentTenant()):
		m.err = "this session is already on tenant " + tenant
	case m.overSSH:
		m.err = "over ssh the username picks the tenant; reconnect with " + SSHTenantHint(tenant)
	case m.dialTenant == nil:
		m.err = "this session cannot switch tenants; use tix tenant use " + tenant
	case m.reachable != nil && !m.reachable(tenant):
		m.err = "no credential for tenant " + tenant + "; run tix login --tenant " + tenant + ", then switch"
	default:
		m.status, m.err = "reaching tenant "+tenant+"...", ""
		return m, m.switchTenant(tenant)
	}
	return m, nil
}

// SSHTenantHint is the command that opens a tenant over SSH. The listener
// does not know the name it is reached by, so HOST stays for the reader.
func SSHTenantHint(tenant string) string { return "ssh " + tenant + "@HOST" }

// handleAdminKey runs the tenant screen's actions, refusing a key this reader's
// authority does not reach rather than letting the service refuse it.
func (m Model) handleAdminKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.mayAdminPress(msg) {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Edit):
		return m.openTenantForm()
	case key.Matches(msg, m.keys.New):
		return m.openTenantAddForm()
	case key.Matches(msg, m.keys.Delete):
		return m.confirmAdminRemoval()
	}
	return m, nil
}

// mayAdminPress reports whether this reader holds the authority the pressed key
// acts through, asked of the same list the footer and the help overlay filter
// with so the three cannot disagree.
func (m Model) mayAdminPress(msg tea.KeyPressMsg) bool {
	for _, a := range m.keys.tenantBindings() {
		if key.Matches(msg, a.binding) {
			return a.permitted(m.permits())
		}
	}
	return true
}

// openTenantForm offers the attributes an edit can change.
func (m Model) openTenantForm() (Model, tea.Cmd) {
	if m.admin == nil || m.admin.tenant == nil {
		m.err = "this session cannot read the tenant, so it cannot edit it"
		return m, nil
	}
	m.form, m.err = TenantEditForm(*m.admin.tenant), ""
	return m, nil
}

// openTenantAddForm asks which of the two additions the key meant.
func (m Model) openTenantAddForm() (Model, tea.Cmd) {
	form := TenantAddForm(m.mayPerform("AddDomain"), m.mayPerform("AddMember"))
	if !form.Open() {
		m.err = "this session may add neither a domain nor a member"
		return m, nil
	}
	m.form, m.err = form, ""
	return m, nil
}

// confirmAdminRemoval names the selected domain or member and waits for
// agreement, refusing a screen with nothing on it to remove rather than opening
// a question over nothing.
func (m Model) confirmAdminRemoval() (Model, tea.Cmd) {
	row, ok := m.selectedAdminRow()
	if !ok {
		m.err = "this tenant has no domain and no member to remove"
		return m, nil
	}
	if row.Kind == tenantRowMembership {
		m.err = "a membership is not removed here; select a domain or a member"
		return m, nil
	}
	confirm := Confirm{Kind: confirmRemoveDomain, Target: row.Label, hostname: row.Key}
	if row.Kind == tenantRowMember {
		confirm = Confirm{Kind: confirmRemoveMember, Target: row.Label, actorID: row.Key}
	}
	return m.askConfirm(confirm, "there is nothing named to remove")
}

// applyTenantEdit performs what the edit form asked for. The theme is answered
// by the form; the name is free text and hands over to the single-line prompt,
// seeded with the value it would replace.
func (m Model) applyTenantEdit(form Form) (Model, tea.Cmd) {
	if m.admin == nil || m.admin.tenant == nil {
		return m, nil
	}
	switch form.Value("attribute") {
	case attrTheme:
		if form.Value(attrTheme) == customColour {
			seed := ""
			if _, custom := core.CustomTheme(m.admin.tenant.Theme); custom {
				seed = m.admin.tenant.Theme
			}
			return m.startPrompt(promptTenantAccent, seed), textinput.Blink
		}
		theme := ThemeValue(form.Value(attrTheme))
		return m, m.updateTenant(core.UpdateTenantInput{Theme: &theme}, attrTheme)
	case attrName:
		return m.startPrompt(promptTenantName, m.admin.tenant.Name), textinput.Blink
	case attrMFA:
		policy := core.MFAPolicy(form.Value(attrMFA))
		return m, m.updateTenant(core.UpdateTenantInput{MFAPolicy: &policy}, attrMFA)
	case attrMFAGrace:
		var grace core.Duration
		if v := form.Value(attrMFAGrace); v != "0" {
			parsed, err := core.ParseDuration(v)
			if err != nil {
				m.err = err.Error()
				return m, nil
			}
			grace = parsed
		}
		return m, m.updateTenant(core.UpdateTenantInput{MFAGrace: &grace}, attrMFAGrace)
	}
	return m, nil
}

// applyTenantAdd sends the chosen addition to the control that gathers it: a
// hostname to the prompt, a membership to the directory the picker offers from.
func (m Model) applyTenantAdd(form Form) (Model, tea.Cmd) {
	if form.Value("subject") == addMember {
		return m, m.loadActors()
	}
	return m.startPrompt(promptDomain, ""), textinput.Blink
}

// onMemberDirectory opens the member form on a directory, or says the tenant
// has nobody to add rather than offering a picker with nothing in it.
func (m Model) onMemberDirectory(msg actorsMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "listing actors: " + msg.err.Error()
		return m, nil
	}
	m.directory = msg.actors
	form := MemberForm(msg.actors)
	if !form.Open() {
		m.err = "this tenant has nobody to add"
		return m, nil
	}
	m.form, m.err = form, ""
	return m, nil
}

// applyMember gives the picked actor the picked role.
func (m Model) applyMember(form Form) (Model, tea.Cmd) {
	label := form.Value("actor")
	id := ActorIDFor(m.directory, label)
	if id == "" {
		m.err = "this tenant no longer has " + label
		return m, nil
	}
	return m, m.addMember(id, label, core.Role(form.Value("role")))
}

// switchTenant opens a connection pinned to key and asks it who the actor is.
// Validating through the connection already open is not possible: it answers
// only for its own tenant, so the target has to answer for itself.
func (m Model) switchTenant(key string) tea.Cmd {
	dial, ctx := m.dialTenant, m.ctx
	if dial == nil {
		return nil
	}
	return func() tea.Msg {
		conn, err := dial(ctx, key)
		if err != nil {
			return tenantMsg{key: key, err: err}
		}
		actor, err := conn.Service.WhoAmI(conn.Context)
		if err != nil {
			closeConn(conn)
			return tenantMsg{key: key, err: err}
		}
		return tenantMsg{key: key, conn: conn, actor: actor}
	}
}

// closeConn releases a connection, ignoring a close that fails: the switch it
// belonged to has already been abandoned, and there is nothing left to report
// the failure to.
func closeConn(c TenantConn) {
	if c.Close != nil {
		_ = c.Close()
	}
}

// onTenant adopts the connection a switch opened, or reports why it did not.
// Everything the previous tenant's rows produced is dropped rather than
// carried across: a project list, a board, a lease and an event tail all
// belong to the tenant they were read from.
func (m Model) onTenant(msg tenantMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = core.NotFound("tenant %q could not be reached: %v", msg.key, msg.err).Error()
		return m, nil
	}
	notice := TenantSwitchNotice(msg.key, len(m.leases))
	closeConn(m.ownConn)
	m.ownConn = msg.conn
	m.svc, m.ctx, m.actor, m.tenantKey = msg.conn.Service, msg.conn.Context, msg.actor, msg.key
	m.gen++
	m.projects, m.projectSel, m.projectOff = nil, 0, 0
	m.project, m.workflow, m.tasks, m.columns = core.Project{}, nil, nil, nil
	m.sel, m.rowOff, m.detail, m.detailOff = Selection{}, 0, nil, 0
	m.leases = map[string]string{}
	m.admin, m.adminSel, m.adminOff = nil, 0, 0
	m.integ = integrationsState{}
	m.people = peopleState{}
	m.activity, m.activitySel, m.activityOff = nil, 0, 0
	m.events, m.connected, m.lastSeq = nil, false, 0
	m.err, m.status = "", notice
	m = m.rootView()
	return m, tea.Batch(m.loadProjects(), m.subscribe(m.lastSeq))
}

// tenantLines renders the tenant screen, scrolled so a tenant with more
// domains and members than the terminal has rows is still reachable and says
// that it has more.
func (m Model) tenantLines(layout Layout) []string {
	raw := TenantAdminView(m.adminState())
	rows := VisibleRows(layout.BodyHeight, len(raw))
	offset := ScrollWindow(m.adminOff, m.adminOff, rows, len(raw))
	out := make([]string, 0, rows+1)
	for i := offset; i < len(raw) && i < offset+rows; i++ {
		out = append(out, m.fit(m.tenantStyle(raw[i])))
	}
	if hint := ScrollHint(offset, rows, len(raw)); hint != "" {
		out = append(out, m.theme.Dim.Render("  "+hint))
	}
	return out
}

// tenantStyle draws one line of the tenant screen by what it is.
func (m Model) tenantStyle(l TenantLine) string {
	switch {
	case l.Selected:
		return m.selection().Render(l.Text)
	case l.Heading:
		return m.theme.Header.Render(l.Text)
	case l.Dim:
		return m.theme.Dim.Render(l.Text)
	default:
		return l.Text
	}
}

// submitTenant acts on a typed tenant key, refusing one the session is
// already on before anything is dialled.
func (m Model) submitTenant(text string) (Model, tea.Cmd) {
	key, err := TenantSwitchPlan(m.tenantKey, text)
	if err != nil {
		next := m.closePrompt()
		next.err = err.Error()
		return next, nil
	}
	next := m.closePrompt()
	next.status, next.err = "reaching tenant "+key+"...", ""
	return next, next.switchTenant(key)
}
