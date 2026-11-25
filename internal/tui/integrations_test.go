// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// fakeIntegrations scripts the reads and records the writes the integrations
// view makes, over the ordinary fake for everything else.
type fakeIntegrations struct {
	*fakeService

	webhooks   []core.WebhookEndpoint
	deliveries []core.WebhookDelivery
	sources    []core.SyncSource
	retention  *core.RetentionPolicy
	generated  string
	runErr     error

	reads         []string
	webhookPuts   []core.WebhookInput
	webhooksGone  []string
	redelivered   []string
	pinged        []string
	sourcePuts    []core.SyncSourceInput
	sourcesGone   []string
	runs          []core.RunSyncInput
	retentionPuts []core.RetentionPolicy
	prunes        []core.PruneInput
}

func (f *fakeIntegrations) ListWebhooks(context.Context) ([]core.WebhookEndpoint, error) {
	f.reads = append(f.reads, "ListWebhooks")
	return f.webhooks, nil
}

func (f *fakeIntegrations) ListDeliveries(context.Context, core.DeliveryFilter) ([]core.WebhookDelivery, string, error) {
	f.reads = append(f.reads, "ListDeliveries")
	return f.deliveries, "", nil
}

func (f *fakeIntegrations) ListSyncSources(context.Context) ([]core.SyncSource, error) {
	f.reads = append(f.reads, "ListSyncSources")
	return f.sources, nil
}

func (f *fakeIntegrations) GetRetention(context.Context) (*core.RetentionPolicy, error) {
	f.reads = append(f.reads, "GetRetention")
	return f.retention, nil
}

func (f *fakeIntegrations) PutWebhook(_ context.Context, in core.WebhookInput) (*core.IssuedWebhook, error) {
	f.webhookPuts = append(f.webhookPuts, in)
	e := &core.IssuedWebhook{WebhookEndpoint: core.WebhookEndpoint{
		ID: "w-new", URL: in.URL, Active: in.Active, EventTypes: in.EventTypes}}
	if in.Secret == "" && in.ID == "" {
		e.SigningSecret = f.generated
	}
	return e, nil
}

func (f *fakeIntegrations) PingWebhook(_ context.Context, id string) (*core.WebhookDelivery, error) {
	f.pinged = append(f.pinged, id)
	return &core.WebhookDelivery{ID: "d-ping", EndpointID: id, Status: core.DeliveryDelivered}, nil
}

func (f *fakeIntegrations) DeleteWebhook(_ context.Context, id string) error {
	f.webhooksGone = append(f.webhooksGone, id)
	return nil
}

func (f *fakeIntegrations) RedeliverWebhook(_ context.Context, id string) error {
	f.redelivered = append(f.redelivered, id)
	return nil
}

func (f *fakeIntegrations) PutSyncSource(_ context.Context, in core.SyncSourceInput) (*core.SyncSource, error) {
	f.sourcePuts = append(f.sourcePuts, in)
	return &core.SyncSource{ID: "s-new", Name: in.Name, System: in.System,
		Resolved: &core.SyncResolution{Mapping: core.SyncOrigin{From: core.SyncFromStored},
			Credentials: core.SyncOrigin{From: core.SyncFromNone}}}, nil
}

func (f *fakeIntegrations) DeleteSyncSource(_ context.Context, id string) error {
	f.sourcesGone = append(f.sourcesGone, id)
	return nil
}

func (f *fakeIntegrations) RunSync(_ context.Context, in core.RunSyncInput) (*core.SyncResult, error) {
	f.runs = append(f.runs, in)
	if f.runErr != nil {
		return nil, f.runErr
	}
	return &core.SyncResult{Source: "op", System: core.SystemOpenProject,
		ImportResult: core.ImportResult{Created: map[string]int{"task": 3}, DryRun: in.DryRun}}, nil
}

func (f *fakeIntegrations) PutRetention(_ context.Context, p core.RetentionPolicy) (*core.RetentionPolicy, error) {
	f.retentionPuts = append(f.retentionPuts, p)
	return f.retention, nil
}

func (f *fakeIntegrations) Prune(_ context.Context, in core.PruneInput) (*core.PruneResult, error) {
	f.prunes = append(f.prunes, in)
	return &core.PruneResult{Events: 12, DryRun: in.DryRun}, nil
}

// integrationsService holds one endpoint, one delivery, one source and the
// shipped retention policy.
func integrationsService() *fakeIntegrations {
	policy := core.DefaultRetention("t1")
	last := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	return &fakeIntegrations{
		fakeService: newFakeService(),
		webhooks: []core.WebhookEndpoint{
			{ID: "w1", URL: "https://hooks.example/tix", Active: true, EventTypes: []string{"task.*"}},
		},
		deliveries: []core.WebhookDelivery{
			{ID: "d1", EndpointID: "w1", Status: core.DeliveryFailed, Attempts: 3, LastError: "connection refused"},
		},
		sources: []core.SyncSource{{
			ID: "s1", Name: "op", System: core.SystemOpenProject, LastRunAt: &last, LastStatus: "ok",
			Config:      map[string]string{"url": "https://op.example"},
			Credentials: []core.SyncCredentialHint{{Name: "password", Hint: "wxyz"}},
			Resolved: &core.SyncResolution{
				Mapping:     core.SyncOrigin{From: core.SyncFromStored},
				Credentials: core.SyncOrigin{From: core.SyncFromStored},
				Missing:     []string{"TIX_SYNC_OP_PROJECT"},
			},
		}},
		retention: &policy,
		generated: "whsec-generated-once",
	}
}

// openIntegrations presses the key for a reader and runs the read it asked for.
func openIntegrations(t *testing.T, actor *core.Actor, svc core.Service) Model {
	t.Helper()
	m := New(Config{Access: capability.TUIAccess(actor), Actor: actor, Environ: []string{"NO_COLOR=1"},
		Now: func() time.Time { return time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC) }})
	m.width, m.height = 140, 50
	m.svc = svc
	m, cmd := m.reduce(pressKey("J"))
	if m.view != viewIntegrations {
		t.Fatalf("J did not open the integrations view for %v; view = %v", actor.Scopes, m.view)
	}
	return runCmd(t, m, cmd)
}

// integrationsModel opens the view as a tenant administrator.
func integrationsModel(t *testing.T) (Model, *fakeIntegrations) {
	t.Helper()
	svc := integrationsService()
	return openIntegrations(t, fullActor(), svc), svc
}

// tabLine returns the strip naming the tabs, which is the one row a test about
// which tabs are offered has to read.
func tabLine(t *testing.T, frame string) string {
	t.Helper()
	for _, l := range strings.Split(frame, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "tabs:") {
			return l
		}
	}
	t.Fatalf("the frame draws no tab strip:\n%s", frame)
	return ""
}

// submit runs the open form's answer and every command it leads to that ends
// in one message, as a reader pressing enter would.
func submit(t *testing.T, m Model) Model {
	t.Helper()
	m, cmd := m.submitForm()
	return drain(t, m, cmd)
}

// drain runs a command and whatever single commands follow, stopping at a
// batch or at nothing.
func drain(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for range 4 {
		if cmd == nil {
			return m
		}
		msg := cmd()
		if _, batch := msg.(tea.BatchMsg); batch {
			return m
		}
		m, cmd = m.reduce(msg)
	}
	return m
}

func TestIntegrationTabsFollowTheReadersScopes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		scope   core.Scope
		tab     string
		hidden  []string
		allowed string
	}{
		{"sync only", core.ScopeSyncAdmin, "[sync]", []string{"webhooks", "retention"}, "ListSyncSources"},
		{"webhooks only", core.ScopeWebhookAdmin, "[webhooks]", []string{"sync", "retention"}, "ListWebhooks"},
		{"retention only", core.ScopeTenantAdmin, "[retention]", []string{"webhooks", "sync"}, "GetRetention"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := integrationsService()
			actor := &core.Actor{ID: "u", TenantID: "t1", Kind: core.ActorUser, Role: core.RoleViewer,
				Scopes: []core.Scope{tc.scope}}
			m := openIntegrations(t, actor, svc)
			strip := tabLine(t, m.Frame())
			if !strings.Contains(strip, tc.tab) {
				t.Errorf("the strip reads %q, want it to open on %s", strip, tc.tab)
			}
			for _, other := range tc.hidden {
				if strings.Contains(strip, other) {
					t.Errorf("the strip names %q for a reader without its scope: %q", other, strip)
				}
			}
			for _, read := range svc.reads {
				if read != tc.allowed && read != "ListDeliveries" {
					t.Errorf("the view read %s for a reader without its scope", read)
				}
				if read == "ListDeliveries" && tc.scope != core.ScopeWebhookAdmin {
					t.Errorf("the view read the delivery log for a reader without webhook:admin")
				}
			}
			m, _ = m.reduce(pressKey("right"))
			m, _ = m.reduce(pressKey("left"))
			if got := tabLine(t, m.Frame()); got != strip {
				t.Errorf("stepping the tabs reached one the reader is not offered: %q", got)
			}
		})
	}
}

func TestAReaderWithNoIntegrationScopeIsNotOfferedTheView(t *testing.T) {
	actor := &core.Actor{ID: "u", TenantID: "t1", Kind: core.ActorUser, Role: core.RoleViewer}
	m := New(Config{Access: capability.TUIAccess(actor), Actor: actor, Environ: []string{"NO_COLOR=1"}})
	m.svc = integrationsService()
	m, cmd := m.reduce(pressKey("J"))
	if m.view == viewIntegrations || cmd != nil {
		t.Fatalf("J opened the view or read something for a reader with none of the scopes")
	}
	for _, e := range m.keys.ViewHelp(viewProjects, m.permits()) {
		if e.Keys == "J" {
			t.Fatal("the help names J for a reader who cannot open it")
		}
	}
	for _, a := range m.paletteEntries() {
		if a.id == doIntegrations {
			t.Fatal("the palette offers the integrations view to a reader who cannot open it")
		}
	}
}

func TestTheWebhooksTabListsEndpointsThenDeliveries(t *testing.T) {
	m, svc := integrationsModel(t)
	frame := m.Frame()
	endpoints := adminSection(t, frame, "endpoints (1):")
	for _, want := range []string{"https://hooks.example/tix", "active", "task.*"} {
		if !strings.Contains(endpoints, want) {
			t.Errorf("the endpoints section does not say %q:\n%s", want, endpoints)
		}
	}
	deliveries := adminSection(t, frame, "deliveries (1, newest first):")
	for _, want := range []string{"failed", "3 attempts", "connection refused", "to https://hooks.example/tix"} {
		if !strings.Contains(deliveries, want) {
			t.Errorf("the deliveries section does not say %q:\n%s", want, deliveries)
		}
	}

	m, cmd := m.reduce(pressKey("enter"))
	m = drain(t, m, cmd)
	if len(svc.redelivered) != 0 {
		t.Fatalf("enter on an endpoint redelivered %v", svc.redelivered)
	}
	if !slices.Equal(svc.pinged, []string{"w1"}) {
		t.Fatalf("enter on the endpoint pinged %v, want w1", svc.pinged)
	}
	if !strings.Contains(m.Frame(), "ping to https://hooks.example/tix: delivered") {
		t.Errorf("the tab does not say how the ping went:\n%s", m.Frame())
	}
	m, _ = m.reduce(pressKey("down"))
	if row, _ := m.selectedIntegrationRow(); row.Kind != rowDelivery || row.ID != "d1" {
		t.Fatalf("moving down from the last endpoint selected %+v, want the first delivery", row)
	}
	m, cmd = m.reduce(pressKey("enter"))
	drain(t, m, cmd)
	if len(svc.pinged) != 1 {
		t.Errorf("enter on a delivery pinged again: %v", svc.pinged)
	}
	if !slices.Equal(svc.redelivered, []string{"d1"}) {
		t.Fatalf("enter on the delivery redelivered %v, want d1", svc.redelivered)
	}
}

func TestRegisteringWithNoSecretShowsTheGeneratedOneOnce(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("n"))
	if m.form.Kind != formWebhook {
		t.Fatalf("n opened form %v, want the endpoint form", m.form.Kind)
	}
	m.form = m.form.SetValue("url", "https://new.example/hook").SetValue("events", "task.*, project.created")
	m = submit(t, m)
	if len(svc.webhookPuts) != 1 {
		t.Fatalf("PutWebhook ran %d times", len(svc.webhookPuts))
	}
	got := svc.webhookPuts[0]
	if got.Secret != "" || !got.Active || !slices.Equal(got.EventTypes, []string{"task.*", "project.created"}) {
		t.Fatalf("PutWebhook was asked for %+v", got)
	}
	if !strings.Contains(m.Frame(), "shown once: whsec-generated-once") {
		t.Fatalf("the generated secret is not on the tab:\n%s", m.Frame())
	}
	m, _ = m.reduce(pressKey("esc"))
	m, cmd := m.reduce(pressKey("J"))
	m = runCmd(t, m, cmd)
	if strings.Contains(m.Frame(), "whsec-generated-once") {
		t.Fatal("the generated secret was shown again after leaving the view")
	}
}

func TestEditingAnEndpointKeepsItsSecret(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("e"))
	if m.form.Kind != formWebhook || m.form.value("url") != "https://hooks.example/tix" {
		t.Fatalf("e opened %+v, want the endpoint form seeded with its URL", m.form)
	}
	if m.form.value("secret") != "" {
		t.Fatal("the edit form was seeded with a secret")
	}
	submit(t, m)
	if len(svc.webhookPuts) != 1 || svc.webhookPuts[0].ID != "w1" || svc.webhookPuts[0].Secret != "" {
		t.Fatalf("PutWebhook was asked for %+v, want w1 with no secret", svc.webhookPuts)
	}
}

func TestDeletingAnEndpointNamesItsURL(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("X"))
	line := confirmLine(t, m.Frame(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, "delete endpoint https://hooks.example/tix?") {
		t.Errorf("the question reads %q", line)
	}
	if len(svc.webhooksGone) != 0 {
		t.Fatal("the endpoint was deleted before the reader agreed")
	}
	m, cmd := m.reduce(pressKey("y"))
	drain(t, m, cmd)
	if !slices.Equal(svc.webhooksGone, []string{"w1"}) {
		t.Fatalf("DeleteWebhook ran for %v", svc.webhooksGone)
	}
}

func TestASyncCredentialIsMaskedAndNeverSeeded(t *testing.T) {
	m, _ := integrationsModel(t)
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("e"))
	if m.form.Kind != formSyncSource {
		t.Fatalf("e on the sync tab opened %v", m.form.Kind)
	}
	for _, key := range []string{"token", "password", "mapping"} {
		if m.form.value(key) != "" {
			t.Errorf("the set up form was seeded with the stored %s", key)
		}
	}
	if m.form.value(core.SyncSettingURL) != "https://op.example" {
		t.Errorf("the set up form did not carry the stored url")
	}
	for i, f := range m.form.Visible() {
		if f.Key == "token" {
			m.form.Sel = i
		}
	}
	m = m.focusField()
	for _, r := range "sekrit-token" {
		m, _ = m.reduce(pressKey(string(r)))
	}
	frame := m.Frame()
	if strings.Contains(frame, "sekrit") {
		t.Fatalf("the form drew the typed token:\n%s", frame)
	}
	if !strings.Contains(frame, strings.Repeat("•", len("sekrit-token"))) {
		t.Fatalf("the form drew no mask for the typed token:\n%s", frame)
	}
	if m.form.value("token") != "sekrit-token" {
		t.Fatalf("the token was not gathered: %q", m.form.value("token"))
	}
}

func TestTheSourceFormOffersOnlyTheChosenSystemsSettings(t *testing.T) {
	t.Parallel()
	visible := func(f Form) []string {
		var keys []string
		for _, field := range f.Visible() {
			keys = append(keys, field.Key)
		}
		return keys
	}
	generic := SyncSourceForm(nil)
	keys := visible(generic)
	if !slices.Contains(keys, core.SyncSettingFile) || slices.Contains(keys, core.SyncSettingURL) {
		t.Errorf("generic offers %v, want a file and no url", keys)
	}
	jira := generic.SetValue("system", core.SystemJira)
	keys = visible(jira)
	if slices.Contains(keys, core.SyncSettingFile) || !slices.Contains(keys, core.SyncSettingURL) ||
		!slices.Contains(keys, core.SyncSettingQuery) {
		t.Errorf("jira offers %v, want a url and a query and no file", keys)
	}
}

func TestCheckingASourceSavesNothingAndDrawsTheCheck(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("n"))
	m.form = m.form.SetValue("name", "csv").SetValue("check", yesValue).SetValue("page_size", "50")
	m = submit(t, m)
	if len(svc.sourcePuts) != 1 || !svc.sourcePuts[0].Check {
		t.Fatalf("PutSyncSource was asked for %+v, want a check", svc.sourcePuts)
	}
	if got := svc.sourcePuts[0].Config[core.SyncSettingPageSize]; got != 50 {
		t.Errorf("page size was sent as %v", got)
	}
	if !strings.Contains(m.Frame(), "checked csv on the server, nothing saved:") {
		t.Fatalf("the check is not drawn on the tab:\n%s", m.Frame())
	}
}

func TestASyncRunIsADryRunByDefault(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("right"))
	sources := adminSection(t, m.Frame(), "sources (1):")
	for _, want := range []string{"op", "openproject", "password …wxyz", "TIX_SYNC_OP_PROJECT", "url https://op.example"} {
		if !strings.Contains(sources, want) {
			t.Errorf("the sources section does not say %q:\n%s", want, sources)
		}
	}
	m, _ = m.reduce(pressKey("enter"))
	if m.form.Kind != formSyncRun {
		t.Fatalf("enter on a source opened %v", m.form.Kind)
	}
	m = submit(t, m)
	if len(svc.runs) != 1 || !svc.runs[0].DryRun || svc.runs[0].SourceID != "s1" {
		t.Fatalf("RunSync was asked for %+v, want a dry run of s1", svc.runs)
	}
	if !strings.Contains(m.Frame(), "run result, dry run, nothing was written:") {
		t.Fatalf("the run is not drawn on the tab:\n%s", m.Frame())
	}
}

func TestARefusedRunIsDrawnOnTheTab(t *testing.T) {
	m, svc := integrationsModel(t)
	svc.runErr = core.Invalid("the server lacks TIX_SYNC_OP_PROJECT")
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("enter"))
	m = submit(t, m)
	section := adminSection(t, m.Frame(), "run refused, nothing was written:")
	if !strings.Contains(section, "TIX_SYNC_OP_PROJECT") {
		t.Fatalf("the refusal does not say why:\n%s", section)
	}
}

func TestDeletingASourceNamesIt(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("X"))
	line := confirmLine(t, m.Frame(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, "delete source op?") {
		t.Errorf("the question reads %q", line)
	}
	m, _ = m.reduce(pressKey("esc"))
	if len(svc.sourcesGone) != 0 || m.confirm.Open() {
		t.Fatal("declining deleted the source or left the question open")
	}
}

func TestABlankRetentionWindowIsKept(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("right"))
	retention := adminSection(t, m.Frame(), "retention:")
	if !strings.Contains(retention, "30d") || !strings.Contains(retention, "365d") {
		t.Fatalf("the windows are not drawn:\n%s", retention)
	}
	m, _ = m.reduce(pressKey("e"))
	if m.form.Kind != formRetention {
		t.Fatalf("e on the retention tab opened %v", m.form.Kind)
	}
	m.form = m.form.SetValue("events", "7d").SetValue("audit_entries", "")
	submit(t, m)
	if len(svc.retentionPuts) != 1 {
		t.Fatalf("PutRetention ran %d times", len(svc.retentionPuts))
	}
	got := svc.retentionPuts[0]
	if got.Events.D() != 7*24*time.Hour || got.AuditEntries != 0 {
		t.Fatalf("PutRetention was asked for %+v, want events 7d and audit unset", got)
	}
	if _, err := RetentionFormInput(RetentionForm(core.RetentionPolicy{}).SetValue("events", "soon")); err == nil {
		t.Fatal("a window that does not read was accepted")
	}
}

func TestARealPruneAsksFirstAndADryRunDoesNot(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("left"))
	m.integ.tab = TabRetention
	m, _ = m.reduce(pressKey("enter"))
	m = submit(t, m)
	if len(svc.prunes) != 1 || !svc.prunes[0].DryRun {
		t.Fatalf("the default prune was %+v, want a dry run", svc.prunes)
	}
	if !strings.Contains(m.Frame(), "prune, dry run, nothing was removed:") {
		t.Fatalf("the dry run's counts are not drawn:\n%s", m.Frame())
	}

	m, _ = m.reduce(pressKey("enter"))
	m.form = m.form.SetValue("dry_run", noValue).SetValue("limit", "100")
	m, cmd := m.submitForm()
	if cmd != nil || len(svc.prunes) != 1 {
		t.Fatal("a prune that writes ran without asking")
	}
	line := confirmLine(t, m.Frame(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, "prune this tenant's history") {
		t.Errorf("the question reads %q", line)
	}
	m, cmd = m.reduce(pressKey("y"))
	drain(t, m, cmd)
	if len(svc.prunes) != 2 || svc.prunes[1].DryRun || svc.prunes[1].Limit != 100 {
		t.Fatalf("the agreed prune was %+v", svc.prunes)
	}
}

// A reader who may read the webhooks and change nothing is offered no write,
// in the footer or at the keystroke, and none of the calls is made.
func TestAReadOnlyReaderIsOfferedNoWrites(t *testing.T) {
	m, svc := integrationsModel(t)
	m.allowed = ActionAccess{"ListWebhooks": true, "ListDeliveries": true}
	footer := m.keys.ShortHelp(viewIntegrations, m.actionContext())
	for _, e := range footer {
		for _, desc := range []string{"register endpoint", "edit endpoint", "delete endpoint", "redeliver", "ping endpoint"} {
			if e.Desc == desc {
				t.Errorf("the footer offers %q to a reader who may not", desc)
			}
		}
	}
	for _, k := range []string{"n", "e", "X"} {
		next, cmd := m.reduce(pressKey(k))
		if next.form.Open() || next.confirm.Open() || cmd != nil {
			t.Errorf("%s opened something for a reader who may not write", k)
		}
	}
	if _, cmd := m.reduce(pressKey("enter")); cmd != nil {
		t.Error("enter on an endpoint made a call for a reader who may not ping")
	}
	m, _ = m.reduce(pressKey("down"))
	if _, cmd := m.reduce(pressKey("enter")); cmd != nil {
		t.Error("enter on a delivery made a call for a reader who may not redeliver")
	}
	if len(svc.webhookPuts)+len(svc.webhooksGone)+len(svc.redelivered)+len(svc.pinged) != 0 {
		t.Fatal("a write reached the service")
	}
}

// The footer offers a row's action only while that kind of row is selected.
func TestTheFooterFollowsTheRowUnderTheCursor(t *testing.T) {
	m, _ := integrationsModel(t)
	has := func(m Model, desc string) bool {
		for _, e := range m.keys.ShortHelp(viewIntegrations, m.actionContext()) {
			if e.Desc == desc {
				return true
			}
		}
		return false
	}
	if !has(m, "edit endpoint") || !has(m, "ping endpoint") || has(m, "redeliver") {
		t.Error("on an endpoint the footer should offer the edit and the ping and not the redelivery")
	}
	m, _ = m.reduce(pressKey("down"))
	if has(m, "edit endpoint") || has(m, "ping endpoint") || !has(m, "redeliver") {
		t.Error("on a delivery the footer should offer the redelivery and not the edit or the ping")
	}
}

func TestAgreeingDeletesTheSourceAndSaysSo(t *testing.T) {
	m, svc := integrationsModel(t)
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("X"))
	m, cmd := m.reduce(pressKey("y"))
	m = drain(t, m, cmd)
	if len(svc.sourcesGone) != 1 {
		t.Fatalf("agreeing deleted %v", svc.sourcesGone)
	}
	if !strings.Contains(m.status, "deleted source op") {
		t.Errorf("the status %q does not name the deleted source", m.status)
	}
}
