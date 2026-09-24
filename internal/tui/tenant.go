// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

// TenantSwitchPlan validates a typed tenant key against the one in force. It
// is the whole decision the tenant view makes before anything is dialled:
// there is no listing to pick from, so a typo can only be caught by trying.
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

// TenantViewLines is what the tenant view says. There is deliberately no list
// of tenants to choose from: an actor belongs to exactly one tenant, and both
// ListTenants and GetTenant are scoped to the caller's own, so from inside
// "default" a tenant named "acme" and one that was never created are the same
// answer. The only way to find out is to open a connection pinned to the key
// and ask it who the actor is, which is exactly what `tix tenant use` does,
// so the view asks for the key rather than pretending to offer a menu.
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
		"  Type a tenant key to switch this session to it.",
		"",
		"  There is no list to choose from. An actor belongs to one tenant, and",
		"  listing tenants is scoped to that tenant, so a key that does not exist",
		"  and one you cannot see are the same answer. The key is checked by",
		"  opening it and asking who you are there, the way tix tenant use does.",
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

// openTenant shows the tenant view.
func (m Model) openTenant() Model { return m.enterView(viewTenant) }

// handleTenantKey opens the input the tenant view gathers a key with.
func (m Model) handleTenantKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Enter):
		if m.dialTenant == nil {
			m.err = "this session cannot switch tenants; use tix tenant use KEY"
			return m, nil
		}
		return m.startPrompt(promptTenant, ""), textinput.Blink
	}
	return m, nil
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
	m.activity, m.activitySel, m.activityOff = nil, 0, 0
	m.events, m.connected, m.lastSeq = nil, false, 0
	m.err, m.status = "", notice
	m = m.rootView()
	return m, tea.Batch(m.loadProjects(), m.subscribe(m.lastSeq))
}

// tenantLines renders the tenant view.
func (m Model) tenantLines(layout Layout) []string {
	handle := ""
	if m.actor != nil {
		handle = m.actor.Handle
	}
	raw := TenantViewLines(m.tenantKey, handle, m.dialTenant != nil)
	lines := make([]string, 0, len(raw))
	for i, l := range raw {
		if i == 0 {
			lines = append(lines, m.theme.Header.Render(l))
			continue
		}
		lines = append(lines, m.fit(l))
	}
	return WindowLines(lines, 0, layout.BodyHeight)
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
