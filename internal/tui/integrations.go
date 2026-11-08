// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// integrationsState is everything the integrations view holds between frames.
// It is one field on Model so the view stays out of the way of the others.
type integrationsState struct {
	tab  IntegrationsTab
	sel  int
	off  int
	data *integrationsMsg
	// editing is the endpoint or source an open form updates, empty for a new
	// one, and running the source a run form will run.
	editing string
	running string
	// What the last write on a tab produced, drawn there until the view is
	// left: a generated secret, a run or a check, a prune.
	secret    string
	secretFor string
	run       *core.SyncResult
	runErr    string
	check     *core.SyncSource
	prune     *core.PruneResult
}

// integrationsMsg carries every read of the tabs this reader may see. Each read
// carries its own refusal.
type integrationsMsg struct {
	webhooks       []core.WebhookEndpoint
	deliveries     []core.WebhookDelivery
	deliveriesMore bool
	sources        []core.SyncSource
	retention      *core.RetentionPolicy

	webhookErr   string
	deliveryErr  string
	sourceErr    string
	retentionErr string
}

// integrationResultMsg reports a write made from the integrations view.
type integrationResultMsg struct {
	verb   string
	status string
	err    error

	secret    string
	secretFor string
	run       *core.SyncResult
	ran       bool
	check     *core.SyncSource
	prune     *core.PruneResult
}

// integrationPageSize is how many deliveries the tab reads. The rest are one
// command away, which the tab names.
const integrationPageSize = 50

// The actions the integrations view performs, one per tab and key.
type integrationAction int

const (
	integNone integrationAction = iota
	integNewWebhook
	integEditWebhook
	integDeleteWebhook
	integRedeliver
	integPingWebhook
	integNewSource
	integEditSource
	integDeleteSource
	integRunSource
	integEditRetention
	integPrune
)

// integrationBinding is one of the view's keys on one tab, the operation it
// calls and what the footer calls it there.
type integrationBinding struct {
	viewAction
	tab IntegrationsTab
	act integrationAction
	// row is the kind of row the action acts on, or -1 for none.
	row IntegrationRowKind
}

// noRow marks an action that needs no row under the cursor.
const noRow IntegrationRowKind = -1

// integrationBindings are the actions the view offers on every tab. The keys
// are the board's own, so every scheme works here without rebinding anything.
func (k KeyMap) integrationBindings() []integrationBinding {
	act := func(b key.Binding, method, desc string, tab IntegrationsTab, a integrationAction,
		row IntegrationRowKind) integrationBinding {
		return integrationBinding{viewAction{gatedAction{binding: b, methods: []string{method}}, desc}, tab, a, row}
	}
	return []integrationBinding{
		act(k.New, "PutWebhook", "register endpoint", TabWebhooks, integNewWebhook, noRow),
		act(k.Edit, "PutWebhook", "edit endpoint", TabWebhooks, integEditWebhook, rowEndpoint),
		act(k.Delete, "DeleteWebhook", "delete endpoint", TabWebhooks, integDeleteWebhook, rowEndpoint),
		act(k.Enter, "PingWebhook", "ping endpoint", TabWebhooks, integPingWebhook, rowEndpoint),
		act(k.Enter, "RedeliverWebhook", "redeliver", TabWebhooks, integRedeliver, rowDelivery),
		act(k.New, "PutSyncSource", "configure source", TabSync, integNewSource, noRow),
		act(k.Edit, "PutSyncSource", "set up source", TabSync, integEditSource, rowSource),
		act(k.Delete, "DeleteSyncSource", "delete source", TabSync, integDeleteSource, rowSource),
		act(k.Enter, "RunSync", "run source", TabSync, integRunSource, rowSource),
		act(k.Edit, "PutRetention", "edit windows", TabRetention, integEditRetention, noRow),
		act(k.Enter, "Prune", "prune", TabRetention, integPrune, noRow),
	}
}

// integrationHelp lists every tab's actions this reader may perform, each
// named with its tab, for the help overlay.
func (k KeyMap) integrationHelp(may func(string) bool) []HelpEntry {
	out := []HelpEntry{{Keys: k.Left.Help().Key + "/" + k.Right.Help().Key, Desc: "previous, next tab"}}
	for _, b := range k.integrationBindings() {
		if b.permitted(may) {
			out = append(out, HelpEntry{Keys: b.binding.Help().Key, Desc: b.tab.String() + ": " + b.desc})
		}
	}
	return out
}

// IntegrationsContext is what the footer needs to know about the open tab.
type IntegrationsContext struct {
	Tab IntegrationsTab
	// Row is the kind of row under the cursor; HasRow reports there is one.
	Row    IntegrationRowKind
	HasRow bool
	// HasRetention reports the windows were read, which editing them needs.
	HasRetention bool
}

// integrationShort offers the open tab's keys that can act on what is under
// the cursor, so the footer never promises a key that would only refuse.
func (k KeyMap) integrationShort(ctx ActionContext) []HelpEntry {
	out := []HelpEntry{{Keys: k.Left.Help().Key + "/" + k.Right.Help().Key, Desc: "tab"}}
	for _, b := range k.integrationBindings() {
		if b.tab != ctx.Integ.Tab || !b.permitted(ctx.May) {
			continue
		}
		if b.row != noRow && (!ctx.Integ.HasRow || ctx.Integ.Row != b.row) {
			continue
		}
		if b.act == integEditRetention && !ctx.Integ.HasRetention {
			continue
		}
		out = append(out, b.entry())
	}
	return out
}

// integrationsContext describes the open tab for the footer.
func (m Model) integrationsContext() IntegrationsContext {
	ctx := IntegrationsContext{Tab: m.integ.tab}
	if row, ok := m.selectedIntegrationRow(); ok {
		ctx.Row, ctx.HasRow = row.Kind, true
	}
	ctx.HasRetention = m.integ.data != nil && m.integ.data.retention != nil
	return ctx
}

// integrationTabs are the tabs this reader is offered.
func (m Model) integrationTabs() []IntegrationsTab { return IntegrationsTabs(m.permits()) }

// openIntegrations shows the view on the first tab this reader may see and
// reads every tab. A result drawn by an earlier visit is not carried in: a
// signing secret is shown once.
func (m Model) openIntegrations() (Model, tea.Cmd) {
	if m.view == viewIntegrations {
		return m, m.loadIntegrations()
	}
	next := m.enterView(viewIntegrations)
	if next.view != viewIntegrations {
		return next, nil
	}
	tabs := next.integrationTabs()
	next.integ = integrationsState{}
	if len(tabs) > 0 {
		next.integ.tab = tabs[0]
	}
	return next, next.loadIntegrations()
}

// integrationsState is what the view renders from.
func (m Model) integrationsState() IntegrationsState {
	s := IntegrationsState{
		Tabs: m.integrationTabs(), Tab: m.integ.tab, Selected: m.integ.sel, Times: m.timeStyle,
		Secret: m.integ.secret, SecretFor: m.integ.secretFor,
		Run: m.integ.run, RunErr: m.integ.runErr, Check: m.integ.check, Prune: m.integ.prune,
	}
	d := m.integ.data
	if d == nil {
		return s
	}
	s.Loaded = true
	s.Webhooks, s.Deliveries, s.DeliveriesMore = d.webhooks, d.deliveries, d.deliveriesMore
	s.Sources, s.Retention = d.sources, d.retention
	s.WebhookErr, s.DeliveryErr, s.SourceErr, s.RetentionErr = d.webhookErr, d.deliveryErr, d.sourceErr, d.retentionErr
	return s
}

// integrationRows are the rows the cursor runs over on the open tab.
func (m Model) integrationRows() []IntegrationRow { return IntegrationRows(m.integrationsState()) }

// selectedIntegrationRow is the row under the cursor.
func (m Model) selectedIntegrationRow() (IntegrationRow, bool) {
	rows := m.integrationRows()
	if m.integ.sel < 0 || m.integ.sel >= len(rows) {
		return IntegrationRow{}, false
	}
	return rows[m.integ.sel], true
}

// onIntegrations adopts a read, bounding the cursor to the rows that came back.
func (m Model) onIntegrations(msg integrationsMsg) (Model, tea.Cmd) {
	m.integ.data = &msg
	m.integ.sel = clamp(m.integ.sel, 0, len(m.integrationRows())-1)
	return m, nil
}

// onIntegrationResult reports a write and reads the tabs again, so the view
// states what the write left behind rather than what was there before it.
func (m Model) onIntegrationResult(msg integrationResultMsg) (Model, tea.Cmd) {
	if msg.ran {
		m.integ.run, m.integ.runErr, m.integ.check = msg.run, "", nil
		if msg.err != nil {
			m.integ.run, m.integ.runErr = nil, msg.err.Error()
		}
	}
	if msg.err != nil {
		m.err = msg.verb + " failed: " + msg.err.Error()
		return m, m.loadIntegrations()
	}
	m.status, m.err = msg.status, ""
	if msg.secret != "" {
		m.integ.secret, m.integ.secretFor = msg.secret, msg.secretFor
	}
	if msg.check != nil {
		m.integ.check, m.integ.run, m.integ.runErr = msg.check, nil, ""
	}
	if msg.prune != nil {
		m.integ.prune = msg.prune
	}
	return m, m.loadIntegrations()
}

// handleIntegrationsKey moves the cursor and the tab, and runs the open tab's
// actions as far as this reader's authority reaches.
func (m Model) handleIntegrationsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	rows := len(m.integrationRows())
	switch {
	case key.Matches(msg, m.keys.Back):
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.integ.sel = clamp(m.integ.sel-1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.integ.sel = clamp(m.integ.sel+1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Top):
		m.integ.sel, m.integ.off = 0, 0
		return m, nil
	case key.Matches(msg, m.keys.Bottom):
		m.integ.sel = clamp(rows-1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Left):
		return m.stepTab(-1), nil
	case key.Matches(msg, m.keys.Right):
		return m.stepTab(1), nil
	}
	if b, ok := m.integrationBindingFor(msg); ok {
		if !b.permitted(m.permits()) {
			return m, nil
		}
		return m.performIntegration(b)
	}
	return m, nil
}

// integrationBindingFor picks the open tab's binding for a key. One key can
// act on more than one kind of row, so the binding for the row under the
// cursor wins, and the first one otherwise, which refuses with what it needs.
func (m Model) integrationBindingFor(msg tea.KeyPressMsg) (integrationBinding, bool) {
	row, hasRow := m.selectedIntegrationRow()
	var first *integrationBinding
	for _, b := range m.keys.integrationBindings() {
		if b.tab != m.integ.tab || !key.Matches(msg, b.binding) {
			continue
		}
		if b.row == noRow || (hasRow && row.Kind == b.row) {
			return b, true
		}
		if first == nil {
			first = &b
		}
	}
	if first == nil {
		return integrationBinding{}, false
	}
	return *first, true
}

// stepTab moves to the neighbouring tab this reader is offered, stopping at
// the ends.
func (m Model) stepTab(delta int) Model {
	tabs := m.integrationTabs()
	at := 0
	for i, t := range tabs {
		if t == m.integ.tab {
			at = i
		}
	}
	if len(tabs) == 0 {
		return m
	}
	next := tabs[clamp(at+delta, 0, len(tabs)-1)]
	if next != m.integ.tab {
		m.integ.tab, m.integ.sel, m.integ.off = next, 0, 0
		m.err = ""
	}
	return m
}

// performIntegration runs one of the open tab's actions on the row under the
// cursor, refusing one that needs a row of another kind rather than sending a
// call that names nothing.
func (m Model) performIntegration(b integrationBinding) (Model, tea.Cmd) {
	row, ok := m.selectedIntegrationRow()
	if b.row != noRow && (!ok || row.Kind != b.row) {
		m.err = b.desc + ": " + integrationRowNeed(b.row)
		return m, nil
	}
	switch b.act {
	case integNewWebhook:
		m.integ.editing = ""
		return m.openForm(WebhookForm(nil))
	case integEditWebhook:
		if w, found := m.webhookByID(row.ID); found {
			m.integ.editing = w.ID
			return m.openForm(WebhookForm(&w))
		}
	case integDeleteWebhook:
		return m.askConfirm(Confirm{Kind: confirmDeleteWebhook, Target: row.Label, integID: row.ID},
			"there is no endpoint named to delete")
	case integRedeliver:
		return m, m.redeliver(row.ID)
	case integPingWebhook:
		return m, m.pingWebhook(row.ID, row.Label)
	case integNewSource:
		m.integ.editing = ""
		return m.openForm(SyncSourceForm(nil))
	case integEditSource:
		if src, found := m.sourceByID(row.ID); found {
			m.integ.editing = src.ID
			return m.openForm(SyncSourceForm(&src))
		}
	case integDeleteSource:
		return m.askConfirm(Confirm{Kind: confirmDeleteSyncSource, Target: row.Label, integID: row.ID},
			"there is no source named to delete")
	case integRunSource:
		m.integ.running = row.ID
		return m.openForm(SyncRunForm(row.Label))
	case integEditRetention:
		if m.integ.data == nil || m.integ.data.retention == nil {
			m.err = "the retention windows have not been read, so they cannot be edited"
			return m, nil
		}
		return m.openForm(RetentionForm(*m.integ.data.retention))
	case integPrune:
		return m.openForm(PruneForm())
	}
	return m, nil
}

// integrationRowNeed says which row an action wants under the cursor.
func integrationRowNeed(kind IntegrationRowKind) string {
	switch kind {
	case rowDelivery:
		return "select a delivery first"
	case rowSource:
		return "select a source first"
	default:
		return "select an endpoint first"
	}
}

// webhookByID finds an endpoint in the last read.
func (m Model) webhookByID(id string) (core.WebhookEndpoint, bool) {
	if m.integ.data != nil {
		for _, w := range m.integ.data.webhooks {
			if w.ID == id {
				return w, true
			}
		}
	}
	return core.WebhookEndpoint{}, false
}

// sourceByID finds a source in the last read.
func (m Model) sourceByID(id string) (core.SyncSource, bool) {
	if m.integ.data != nil {
		for _, s := range m.integ.data.sources {
			if s.ID == id {
				return s, true
			}
		}
	}
	return core.SyncSource{}, false
}

// submitIntegrationForm performs what one of the view's forms asked for,
// reporting false for a form that is not one of them.
func (m Model) submitIntegrationForm(form Form) (Model, tea.Cmd, bool) {
	switch form.Kind {
	case formWebhook:
		next := m.closeForm()
		return next, next.putWebhook(WebhookFormInput(next.integ.editing, form)), true
	case formSyncSource:
		next := m.closeForm()
		in, err := SyncSourceFormInput(next.integ.editing, form)
		if err != nil {
			next.err = err.Error()
			return next, nil, true
		}
		return next, next.putSyncSource(in), true
	case formSyncRun:
		next := m.closeForm()
		in := core.RunSyncInput{SourceID: next.integ.running, DryRun: form.Yes("dry_run"), Full: form.Yes("full")}
		return next, next.runSync(in), true
	case formRetention:
		next := m.closeForm()
		policy, err := RetentionFormInput(form)
		if err != nil {
			next.err = err.Error()
			return next, nil, true
		}
		return next, next.putRetention(policy), true
	case formPrune:
		next := m.closeForm()
		in, err := PruneFormInput(form)
		if err != nil {
			next.err = err.Error()
			return next, nil, true
		}
		if in.DryRun {
			return next, next.prune(in), true
		}
		next, cmd := next.askConfirm(Confirm{Kind: confirmPrune, Target: "this tenant's history",
			Note: "past the retention windows", prune: in}, "there is nothing named to prune")
		return next, cmd, true
	}
	return m, nil, false
}

// runIntegrationConfirm makes the call a confirmation from this view held back,
// reporting false for a confirmation that is not one of them.
func (m Model) runIntegrationConfirm(c Confirm) (tea.Cmd, bool) {
	switch c.Kind {
	case confirmDeleteWebhook:
		return m.deleteWebhook(c.integID, c.Target), true
	case confirmDeleteSyncSource:
		return m.deleteSyncSource(c.integID, c.Target), true
	case confirmPrune:
		return m.prune(c.prune), true
	}
	return nil, false
}

// integrationLines renders the view, scrolled so a long delivery log or a
// tall run result is still reachable.
func (m Model) integrationLines(layout Layout) []string {
	raw := IntegrationsView(m.integrationsState())
	rows := VisibleRows(layout.BodyHeight, len(raw))
	at := 0
	for i, l := range raw {
		if l.Selected {
			at = i
		}
	}
	offset := ScrollWindow(m.integ.off, at, rows, len(raw))
	out := make([]string, 0, rows+1)
	for i := offset; i < len(raw) && i < offset+rows; i++ {
		out = append(out, m.fit(m.tenantStyle(raw[i])))
	}
	if hint := ScrollHint(offset, rows, len(raw)); hint != "" {
		out = append(out, m.theme.Dim.Render("  "+hint))
	}
	return out
}

// The forms.

// WebhookNote says what the secret field does, so a reader knows a blank one
// is not a mistake.
const WebhookNote = "a blank secret is generated and shown once, or kept when editing; event types are comma separated, task.* matches a family, blank is every event"

// WebhookForm registers an endpoint, or edits one seeded with its values. The
// secret is never seeded: the service does not return it.
func WebhookForm(e *core.WebhookEndpoint) Form {
	title, url, events, active := "register endpoint", "", "", yesValue
	if e != nil {
		title, url, events = "edit endpoint "+e.URL, e.URL, strings.Join(e.EventTypes, ",")
		if !e.Active {
			active = noValue
		}
	}
	return Form{
		Kind: formWebhook, Title: title, Note: WebhookNote,
		Fields: []FormField{
			{Key: "url", Label: "url", Value: url, Kind: FieldText, Limit: 2048},
			{Key: "events", Label: "event types", Value: events, Kind: FieldText, Limit: 1024},
			{Key: "active", Label: "active", Options: switchOptions(), Value: active},
			{Key: "secret", Label: "secret", Kind: FieldText, Secret: true, Limit: 512},
		},
	}
}

// WebhookFormInput is what an answered webhook form asks PutWebhook for.
func WebhookFormInput(id string, f Form) core.WebhookInput {
	return core.WebhookInput{
		ID: id, URL: strings.TrimSpace(f.Value("url")), Secret: f.Value("secret"),
		EventTypes: splitList(f.Value("events")), Active: f.Yes("active"),
	}
}

// splitList reads a comma separated answer, dropping blanks.
func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// SyncSourceNote says what a blank answer does on the source form.
const SyncSourceNote = "runs execute on the server; blank credentials and mapping keep what is stored; check only saves nothing"

// trackerOnly shows a field for every system except generic, which reads a
// file instead of calling a tracker.
var trackerOnly = FieldCondition{Key: "system", Value: core.SystemGeneric, Not: true}

// SyncSourceForm configures a source, or sets one up seeded with its stored
// settings. Credentials and the mapping document are never seeded: the
// credentials are not returned, and a blank document keeps the stored one.
func SyncSourceForm(src *core.SyncSource) Form {
	title, name, system := "configure source", "", core.SystemGeneric
	cfg, mappingPath := map[string]string{}, ""
	if src != nil {
		title, name, system, mappingPath = "set up source "+src.Name, src.Name, src.System, src.MappingPath
		for k, v := range src.Config {
			cfg[k] = v
		}
	}
	text := func(key, label string, needs FieldCondition) FormField {
		return FormField{Key: key, Label: label, Value: cfg[key], Kind: FieldText, Limit: 1024, Needs: needs}
	}
	return Form{
		Kind: formSyncSource, Title: title, Note: SyncSourceNote,
		Fields: []FormField{
			{Key: "name", Label: "name", Value: name, Kind: FieldText, Limit: 128},
			{Key: "system", Label: "system", Options: core.SyncSystems, Value: system},
			text(core.SyncSettingURL, "url", trackerOnly),
			text(core.SyncSettingProject, "project", trackerOnly),
			text(core.SyncSettingQuery, "query", trackerOnly),
			text(core.SyncSettingFile, "server file", FieldCondition{Key: "system", Value: core.SystemGeneric}),
			text(core.SyncSettingPageSize, "page size", FieldCondition{}),
			{Key: "mapping_path", Label: "mapping file", Value: mappingPath, Kind: FieldText, Limit: 1024},
			{Key: "mapping", Label: "mapping yaml", Kind: FieldProse, Limit: 1 << 16},
			{Key: "token", Label: "api token", Kind: FieldText, Secret: true, Limit: 1024},
			{Key: "user", Label: "user", Kind: FieldText, Limit: 256},
			{Key: "password", Label: "password", Kind: FieldText, Secret: true, Limit: 1024},
			{Key: "clear_mapping", Label: "clear mapping", Options: switchOptions(), Value: noValue},
			{Key: "clear_credentials", Label: "clear creds", Options: switchOptions(), Value: noValue},
			{Key: "check", Label: "check only", Options: switchOptions(), Value: noValue},
		},
	}
}

// SyncSourceFormInput is what an answered source form asks PutSyncSource for,
// refusing a page size that is not a whole number before any call.
func SyncSourceFormInput(id string, f Form) (core.SyncSourceInput, error) {
	in := core.SyncSourceInput{
		ID: id, Name: strings.TrimSpace(f.Value("name")), System: f.Value("system"),
		MappingPath: strings.TrimSpace(f.Value("mapping_path")), Mapping: f.Value("mapping"),
		Token: f.Value("token"), User: strings.TrimSpace(f.Value("user")), Password: f.Value("password"),
		ClearMapping: f.Yes("clear_mapping"), ClearCredentials: f.Yes("clear_credentials"), Check: f.Yes("check"),
	}
	cfg := map[string]any{}
	for _, k := range []string{core.SyncSettingURL, core.SyncSettingProject, core.SyncSettingQuery, core.SyncSettingFile} {
		if v := strings.TrimSpace(f.Value(k)); v != "" {
			cfg[k] = v
		}
	}
	if raw := strings.TrimSpace(f.Value(core.SyncSettingPageSize)); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return in, core.Invalid("page size %q must be a whole number above zero", raw)
		}
		cfg[core.SyncSettingPageSize] = n
	}
	if len(cfg) > 0 {
		in.Config = cfg
	}
	return in, nil
}

// SyncRunForm asks how to run a source. Dry run opens on, as on the web.
func SyncRunForm(name string) Form {
	return Form{
		Kind: formSyncRun, Title: "run source " + name,
		Note: "a dry run reports what would change and writes nothing; full ignores the stored cursor",
		Fields: []FormField{
			{Key: "dry_run", Label: "dry run", Options: switchOptions(), Value: yesValue},
			{Key: "full", Label: "full", Options: switchOptions(), Value: noValue},
		},
	}
}

// RetentionForm edits the three windows, seeded with the ones in force.
func RetentionForm(p core.RetentionPolicy) Form {
	window := func(key, label string, d core.Duration) FormField {
		return FormField{Key: key, Label: label, Value: d.Human(), Kind: FieldText, Limit: 32}
	}
	return Form{
		Kind: formRetention, Title: "retention windows",
		Note: "a duration such as 30d, 12h or 2h30m; a blank window keeps its value",
		Fields: []FormField{
			window("events", "events", p.Events),
			window("audit_entries", "audit entries", p.AuditEntries),
			window("webhook_deliveries", "deliveries", p.WebhookDeliveries),
		},
	}
}

// RetentionFormInput reads the windows through the grammar the CLI and the web
// read. A blank window is sent unset, which the service keeps.
func RetentionFormInput(f Form) (core.RetentionPolicy, error) {
	var p core.RetentionPolicy
	for key, target := range map[string]*core.Duration{
		"events": &p.Events, "audit_entries": &p.AuditEntries, "webhook_deliveries": &p.WebhookDeliveries,
	} {
		raw := strings.TrimSpace(f.Value(key))
		if raw == "" {
			continue
		}
		d, err := core.ParseDuration(raw)
		if err != nil || d < 0 {
			return p, core.Invalid("%s %q must be a duration such as 30d, 12h or 2h30m", key, raw)
		}
		*target = d
	}
	return p, nil
}

// PruneForm asks how to prune. Dry run opens on, as on the web.
func PruneForm() Form {
	return Form{
		Kind: formPrune, Title: "prune",
		Note: "a dry run reports counts and removes nothing; a blank limit removes up to 500 of each kind",
		Fields: []FormField{
			{Key: "dry_run", Label: "dry run", Options: switchOptions(), Value: yesValue},
			{Key: "limit", Label: "limit", Kind: FieldText, Limit: 9},
		},
	}
}

// PruneFormInput is what an answered prune form asks Prune for.
func PruneFormInput(f Form) (core.PruneInput, error) {
	in := core.PruneInput{DryRun: f.Yes("dry_run")}
	if raw := strings.TrimSpace(f.Value("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return in, core.Invalid("limit %q must be a whole number", raw)
		}
		in.Limit = n
	}
	return in, nil
}

// The calls.

// loadIntegrations reads every tab this reader may see, each read carrying its
// own refusal. A tab the reader is not offered is not read at all.
func (m Model) loadIntegrations() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, may := m.svc, m.ctx, m.permits()
	return func() tea.Msg {
		out := integrationsMsg{}
		if may("ListWebhooks") {
			out.webhooks, out.webhookErr = errText(svc.ListWebhooks(ctx))
		}
		if may("ListDeliveries") {
			deliveries, next, err := svc.ListDeliveries(ctx, core.DeliveryFilter{Page: core.Page{Limit: integrationPageSize}})
			out.deliveries, out.deliveriesMore = deliveries, next != ""
			if err != nil {
				out.deliveryErr = err.Error()
			}
		}
		if may("ListSyncSources") {
			out.sources, out.sourceErr = errText(svc.ListSyncSources(ctx))
		}
		if may("GetRetention") {
			out.retention, out.retentionErr = errText(svc.GetRetention(ctx))
		}
		return out
	}
}

// errText pairs a read with the words of its refusal.
func errText[T any](v T, err error) (T, string) {
	if err != nil {
		return v, err.Error()
	}
	return v, ""
}

// integrationCall runs one write and reports it.
func (m Model) integrationCall(verb string, call func(core.Service, context.Context) integrationResultMsg) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		msg := call(svc, ctx)
		msg.verb = verb
		return msg
	}
}

// putWebhook registers or updates an endpoint, carrying a secret tix generated
// so the tab can show it once.
func (m Model) putWebhook(in core.WebhookInput) tea.Cmd {
	verb := "register endpoint"
	if in.ID != "" {
		verb = "edit endpoint"
	}
	return m.integrationCall(verb, func(svc core.Service, ctx context.Context) integrationResultMsg {
		e, err := svc.PutWebhook(ctx, in)
		if err != nil {
			return integrationResultMsg{err: err}
		}
		msg := integrationResultMsg{status: "saved endpoint " + e.URL}
		if in.Secret == "" && e.SigningSecret != "" {
			msg.secret, msg.secretFor = e.SigningSecret, e.URL
		}
		return msg
	})
}

// deleteWebhook removes an endpoint the reader agreed to delete.
func (m Model) deleteWebhook(id, url string) tea.Cmd {
	return m.integrationCall("delete endpoint", func(svc core.Service, ctx context.Context) integrationResultMsg {
		return integrationResultMsg{status: "deleted endpoint " + url, err: svc.DeleteWebhook(ctx, id)}
	})
}

// redeliver queues a delivery for another attempt.
func (m Model) redeliver(id string) tea.Cmd {
	return m.integrationCall("redeliver", func(svc core.Service, ctx context.Context) integrationResultMsg {
		return integrationResultMsg{status: "queued delivery " + id + " again", err: svc.RedeliverWebhook(ctx, id)}
	})
}

// pingWebhook sends one endpoint a test delivery and says how it went; the
// delivery tab lists it once the view reads again.
func (m Model) pingWebhook(id, url string) tea.Cmd {
	return m.integrationCall("ping endpoint", func(svc core.Service, ctx context.Context) integrationResultMsg {
		d, err := svc.PingWebhook(ctx, id)
		if err != nil {
			return integrationResultMsg{err: err}
		}
		return integrationResultMsg{status: "ping to " + url + ": " + string(d.Status)}
	})
}

// putSyncSource saves a source, or checks it and saves nothing.
func (m Model) putSyncSource(in core.SyncSourceInput) tea.Cmd {
	verb := "save source"
	if in.Check {
		verb = "check source"
	}
	return m.integrationCall(verb, func(svc core.Service, ctx context.Context) integrationResultMsg {
		src, err := svc.PutSyncSource(ctx, in)
		if err != nil {
			return integrationResultMsg{err: err}
		}
		if in.Check {
			return integrationResultMsg{status: "checked source " + src.Name + ", nothing saved", check: src}
		}
		return integrationResultMsg{status: "saved source " + src.Name}
	})
}

// deleteSyncSource removes a source the reader agreed to delete.
func (m Model) deleteSyncSource(id, name string) tea.Cmd {
	return m.integrationCall("delete source", func(svc core.Service, ctx context.Context) integrationResultMsg {
		return integrationResultMsg{status: "deleted source " + name, err: svc.DeleteSyncSource(ctx, id)}
	})
}

// runSync runs a source, carrying its result or its refusal to the tab.
func (m Model) runSync(in core.RunSyncInput) tea.Cmd {
	return m.integrationCall("run source", func(svc core.Service, ctx context.Context) integrationResultMsg {
		r, err := svc.RunSync(ctx, in)
		msg := integrationResultMsg{ran: true, run: r, err: err}
		if err == nil {
			msg.status = "ran source " + r.Source
			if r.DryRun {
				msg.status += ", dry run: nothing was written"
			}
		}
		return msg
	})
}

// putRetention changes the windows.
func (m Model) putRetention(p core.RetentionPolicy) tea.Cmd {
	return m.integrationCall("save retention", func(svc core.Service, ctx context.Context) integrationResultMsg {
		_, err := svc.PutRetention(ctx, p)
		return integrationResultMsg{status: "saved retention", err: err}
	})
}

// prune removes what is past the windows, or reports it under a dry run.
func (m Model) prune(in core.PruneInput) tea.Cmd {
	return m.integrationCall("prune", func(svc core.Service, ctx context.Context) integrationResultMsg {
		r, err := svc.Prune(ctx, in)
		if err != nil {
			return integrationResultMsg{err: err}
		}
		status := "pruning removed records"
		if r.DryRun {
			status = "dry run: nothing was removed"
		}
		return integrationResultMsg{status: status, prune: r}
	})
}
