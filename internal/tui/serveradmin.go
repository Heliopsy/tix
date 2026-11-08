// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// serverState is what the server view renders from. The status report and the
// connection listing carry their own refusals, so a reader refused one still
// gets the other.
type serverState struct {
	loaded    bool
	report    *core.StatusReport
	statusErr string
	conns     *core.ConnectionList
	connErr   string
	sel       int
	off       int
}

// serverMsg carries both reads the server view states.
type serverMsg struct {
	report    *core.StatusReport
	statusErr error
	conns     *core.ConnectionList
	connErr   error
}

// serverEndMsg reports a connection ended, or why it was not.
type serverEndMsg struct {
	label string
	err   error
}

// ServerState is the server view's input, exported for the pure renderer.
type ServerState struct {
	Loaded      bool
	Report      *core.StatusReport
	StatusErr   string
	Connections *core.ConnectionList
	ConnErr     string
	Selected    int
	Times       output.TimeStyle
}

// serverBindings are the server view's actions, each against the operation it
// calls, so the footer, the help overlay and the keystroke agree.
func (k KeyMap) serverBindings() []viewAction {
	return []viewAction{
		{gatedAction{binding: k.Delete, methods: []string{"EndConnection"}}, "end the selected connection"},
	}
}

// serverShort is the server view's footer, dropping the end key when there is
// no connection under the cursor.
func (k KeyMap) serverShort(ctx ActionContext) []HelpEntry {
	if !ctx.HasRows {
		return nil
	}
	return permittedEntries(k.serverBindings(), ctx.May)
}

// permittedEntries renders the actions a reader's authority reaches.
func permittedEntries(actions []viewAction, may func(string) bool) []HelpEntry {
	out := make([]HelpEntry, 0, len(actions))
	for _, a := range actions {
		if a.permitted(may) {
			out = append(out, a.entry())
		}
	}
	return out
}

// ServerLines renders the server view: the installation, the servers
// registered against it, the tenant's work and second factor, and the
// tenant's live connections on the server that answered.
func ServerLines(state ServerState) []TenantLine {
	out := statusLines(state)
	out = append(out, TenantLine{})
	return append(out, connectionLines(state)...)
}

// statusLines are the sections of the status report.
func statusLines(state ServerState) []TenantLine {
	if state.Report == nil {
		return append([]TenantLine{{Text: "installation:", Heading: true}},
			tenantAbsent(state.StatusErr, state.Loaded)...)
	}
	r := *state.Report
	in := r.Installation
	out := []TenantLine{{Text: "installation:", Heading: true},
		tenantRow("version", orNone(in.Version)),
		tenantRow("schema", strconv.Itoa(in.SchemaVersion)),
		tenantRow("engine", orNone(in.Engine)),
		tenantRow("store", orNone(in.Store)),
		tenantRow("observed", state.Times.Format(in.ObservedAt)),
		{},
	}
	out = append(out, serverRows(r, state.Times)...)
	w := r.Work
	out = append(out, TenantLine{},
		TenantLine{Text: "work:", Heading: true},
		tenantRow("tenants", strconv.Itoa(w.Tenants)),
		tenantRow("projects", strconv.Itoa(w.Projects)),
		tenantRow("tasks", strconv.Itoa(w.Tasks)),
		tenantRow("claimed", strconv.Itoa(w.Claimed)),
		tenantRow("unswept", strconv.Itoa(w.LeasesExpiredUnswept)+" expired leases"),
		tenantRow("webhooks", strconv.Itoa(w.WebhooksPending)+" pending, "+
			strconv.Itoa(w.WebhooksFailed)+" failed"))
	f := r.SecondFactor
	out = append(out, TenantLine{},
		TenantLine{Text: "second factor:", Heading: true},
		tenantRow("key", yesNo(f.KeyConfigured, "configured", "not configured")),
		tenantRow("enrolments", strconv.Itoa(f.Enrolments)),
		tenantRow("required", requiredOn(f)))
	if f.Unrecoverable {
		out = append(out, TenantLine{Text: "  enrolments exist and no key opens them", Dim: true})
	}
	return out
}

// serverRows list the registered servers and say which one answered.
func serverRows(r core.StatusReport, times output.TimeStyle) []TenantLine {
	head := "servers (" + strconv.Itoa(len(r.Servers)) + ", " + strconv.Itoa(r.AttachedCount()) + " attached):"
	out := []TenantLine{{Text: head, Heading: true}}
	for _, s := range r.Servers {
		state := "stale since " + times.Format(s.LastSeenAt)
		if s.Attached {
			state = "attached, up " + s.Uptime.String()
		}
		row := "  " + pad(s.Address, tenantRowWidth) + s.Version + "  " + surfaceList(s.Surfaces) + "  " + state
		if s.Connections != nil {
			row += ", " + plural(*s.Connections, "connection", "connections")
		}
		if s.ID == r.AnsweringServer {
			row += "  (answering)"
		}
		out = append(out, TenantLine{Text: row, Dim: !s.Attached})
	}
	switch {
	case r.AnsweringServer == "":
		out = append(out, TenantLine{Text: "  read against the database directly; no server answered", Dim: true})
	case !r.AnsweringRegistered():
		out = append(out, TenantLine{Text: "  the answering server " + r.AnsweringServer +
			" is not registered", Dim: true})
	case len(r.Servers) == 0:
		out = append(out, TenantLine{Text: "  none", Dim: true})
	}
	return out
}

// surfaceList names what a server serves.
func surfaceList(s []core.ServerSurface) string {
	names := make([]string, 0, len(s))
	for _, x := range s {
		names = append(names, string(x))
	}
	return strings.Join(names, ",")
}

// requiredOn names the sign-in paths the tenant requires a second factor on.
func requiredOn(f core.TOTPReport) string {
	var on []string
	if f.RequireWeb {
		on = append(on, "web")
	}
	if f.RequireCLI {
		on = append(on, "cli")
	}
	if f.RequireSSH {
		on = append(on, "ssh")
	}
	if len(on) == 0 {
		return "nowhere"
	}
	return strings.Join(on, ", ")
}

// yesNo picks between two words.
func yesNo(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// connectionLines list the tenant's live connections with the counts that go
// with them.
func connectionLines(state ServerState) []TenantLine {
	if state.Connections == nil {
		return append([]TenantLine{{Text: "connections:", Heading: true}},
			tenantAbsent(state.ConnErr, state.Loaded)...)
	}
	l := *state.Connections
	out := []TenantLine{{Text: "connections (" + strconv.Itoa(len(l.Connections)) + "):", Heading: true},
		{Text: "  on " + orNone(l.ServerID) + ": " + strconv.Itoa(l.Counts.Events) + " events, " +
			strconv.Itoa(l.Counts.SSH) + " ssh; the server holds " +
			strconv.Itoa(l.Counts.Process) + " across all tenants", Dim: true}}
	if len(l.Connections) == 0 {
		return append(out, TenantLine{Text: "  none", Dim: true})
	}
	for i, c := range l.Connections {
		selected := i == state.Selected
		row := SelectionMarker(selected) + pad(string(c.Surface), 8) + pad(ConnectionLabel(c), tenantRowWidth) +
			pad(orNone(c.Remote), 18) + "since " + state.Times.Format(c.Since)
		if c.Fingerprint != "" {
			row += "  " + c.Fingerprint
		}
		out = append(out, TenantLine{Text: row, Selected: selected})
	}
	return out
}

// ConnectionLabel names who holds a connection.
func ConnectionLabel(c core.Connection) string {
	if c.ActorHandle != "" {
		return "@" + c.ActorHandle
	}
	return c.ActorID
}

// openServer reads the status report and the connections and shows them.
func (m Model) openServer() (Model, tea.Cmd) {
	next := m.enterView(viewServer)
	if next.view != viewServer {
		return next, nil
	}
	next.server = serverState{}
	return next, next.loadServer()
}

// loadServer reads both halves of the view.
func (m Model) loadServer() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		var msg serverMsg
		msg.report, msg.statusErr = svc.Status(ctx)
		msg.conns, msg.connErr = svc.ListConnections(ctx)
		return msg
	}
}

// onServer adopts a read, bounding the cursor to the connections that came
// back.
func (m Model) onServer(msg serverMsg) (Model, tea.Cmd) {
	s := serverState{loaded: true, report: msg.report, conns: msg.conns, sel: m.server.sel, off: m.server.off}
	if msg.statusErr != nil {
		s.report, s.statusErr = nil, msg.statusErr.Error()
	}
	if msg.connErr != nil {
		s.conns, s.connErr = nil, msg.connErr.Error()
	}
	m.server = s
	m.server.sel = clamp(m.server.sel, 0, len(m.serverRows())-1)
	return m, nil
}

// serverRows are the connections the cursor runs over.
func (m Model) serverRows() []core.Connection {
	if m.server.conns == nil {
		return nil
	}
	return m.server.conns.Connections
}

// handleServerKey moves the cursor and ends the selected connection.
func (m Model) handleServerKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	rows := len(m.serverRows())
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.server.sel = clamp(m.server.sel-1, 0, rows-1)
	case key.Matches(msg, m.keys.Down):
		m.server.sel = clamp(m.server.sel+1, 0, rows-1)
	case key.Matches(msg, m.keys.Top):
		m.server.sel, m.server.off = 0, 0
	case key.Matches(msg, m.keys.Bottom):
		m.server.sel = clamp(rows-1, 0, rows-1)
	case key.Matches(msg, m.keys.Delete):
		if !m.mayPerform("EndConnection") {
			return m, nil
		}
		return m.confirmEnd()
	}
	return m, nil
}

// confirmEnd names the selected connection and waits for agreement.
func (m Model) confirmEnd() (Model, tea.Cmd) {
	rows := m.serverRows()
	if m.server.sel < 0 || m.server.sel >= len(rows) {
		m.err = "this server holds no connection of this tenant to end"
		return m, nil
	}
	c := rows[m.server.sel]
	target := string(c.Surface) + " " + ConnectionLabel(c)
	if c.Remote != "" {
		target += " from " + c.Remote
	}
	return m.askConfirm(Confirm{Kind: confirmEndConnection, Target: target, subject: c.ID,
		Note: "the holder may reconnect, and if it is this session it closes"}, "there is no connection named to end")
}

// endConnection makes the call the reader agreed to.
func (m Model) endConnection(c Confirm) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, id, label := m.svc, m.ctx, c.subject, c.Target
	return func() tea.Msg {
		return serverEndMsg{label: label, err: svc.EndConnection(ctx, id)}
	}
}

// onServerEnd reports the end and reads the view again.
func (m Model) onServerEnd(msg serverEndMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "end the connection failed: " + msg.err.Error()
		return m, nil
	}
	m.status, m.err = "ended "+msg.label, ""
	return m, m.loadServer()
}

// serverLines renders the view, scrolled so a long listing stays reachable.
func (m Model) serverLines(layout Layout) []string {
	raw := ServerLines(ServerState{
		Loaded: m.server.loaded, Report: m.server.report, StatusErr: m.server.statusErr,
		Connections: m.server.conns, ConnErr: m.server.connErr, Selected: m.server.sel,
		Times: m.timeStyle,
	})
	return m.scrolledLines(raw, &m.server.off, layout)
}

// scrolledLines draws a list of styled lines into the body, keeping the
// selected one on screen and saying when there is more.
func (m Model) scrolledLines(raw []TenantLine, off *int, layout Layout) []string {
	selected := 0
	for i, l := range raw {
		if l.Selected {
			selected = i
		}
	}
	rows := VisibleRows(layout.BodyHeight, len(raw))
	offset := ScrollWindow(*off, selected, rows, len(raw))
	out := make([]string, 0, rows+1)
	for i := offset; i < len(raw) && i < offset+rows; i++ {
		out = append(out, m.fit(m.tenantStyle(raw[i])))
	}
	if hint := ScrollHint(offset, rows, len(raw)); hint != "" {
		out = append(out, m.theme.Dim.Render("  "+hint))
	}
	return out
}
