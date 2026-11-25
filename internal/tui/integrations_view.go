// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// IntegrationsTab names one tab of the integrations view.
type IntegrationsTab int

// The tabs, in the order the view offers them.
const (
	TabWebhooks IntegrationsTab = iota
	TabSync
	TabRetention
)

// integrationTabs is every tab, in order.
var integrationTabs = []IntegrationsTab{TabWebhooks, TabSync, TabRetention}

// String names a tab the way its heading does.
func (t IntegrationsTab) String() string {
	switch t {
	case TabSync:
		return "sync"
	case TabRetention:
		return "retention"
	default:
		return "webhooks"
	}
}

// read is the operation a tab cannot be drawn without, which is what decides
// whether a reader is offered it at all.
func (t IntegrationsTab) read() string {
	switch t {
	case TabSync:
		return "ListSyncSources"
	case TabRetention:
		return "GetRetention"
	default:
		return "ListWebhooks"
	}
}

// IntegrationsTabs are the tabs this reader's authority reaches, in order.
func IntegrationsTabs(may func(string) bool) []IntegrationsTab {
	var out []IntegrationsTab
	for _, t := range integrationTabs {
		if may != nil && may(t.read()) {
			out = append(out, t)
		}
	}
	return out
}

// IntegrationsState is everything the integrations view renders from. Each read
// carries its own refusal, so a refused delivery log does not cost the
// endpoints above it.
type IntegrationsState struct {
	Tabs []IntegrationsTab
	Tab  IntegrationsTab

	Webhooks       []core.WebhookEndpoint
	Deliveries     []core.WebhookDelivery
	DeliveriesMore bool
	Sources        []core.SyncSource
	Retention      *core.RetentionPolicy

	WebhookErr   string
	DeliveryErr  string
	SourceErr    string
	RetentionErr string

	// Secret is a signing secret tix generated, shown once, for SecretFor.
	Secret    string
	SecretFor string
	// Run is the last sync run's answer, RunErr why it was refused, and Check
	// the last source checked without saving.
	Run    *core.SyncResult
	RunErr string
	Check  *core.SyncSource
	// Prune is the last prune's report.
	Prune *core.PruneResult

	Selected int
	Loaded   bool
	Times    output.TimeStyle
}

// IntegrationRowKind names what a selectable row of the view stands for.
type IntegrationRowKind int

// The selectable rows.
const (
	rowEndpoint IntegrationRowKind = iota
	rowDelivery
	rowSource
)

// IntegrationRow is one row the cursor can rest on.
type IntegrationRow struct {
	Kind  IntegrationRowKind
	ID    string
	Label string
}

// IntegrationRows are the rows the cursor runs over on the open tab: the
// endpoints and then the deliveries, or the sources. Retention has none.
func IntegrationRows(s IntegrationsState) []IntegrationRow {
	var out []IntegrationRow
	switch s.Tab {
	case TabWebhooks:
		for _, w := range s.Webhooks {
			out = append(out, IntegrationRow{Kind: rowEndpoint, ID: w.ID, Label: w.URL})
		}
		for _, d := range s.Deliveries {
			out = append(out, IntegrationRow{Kind: rowDelivery, ID: d.ID, Label: d.ID})
		}
	case TabSync:
		for _, src := range s.Sources {
			out = append(out, IntegrationRow{Kind: rowSource, ID: src.ID, Label: src.Name})
		}
	}
	return out
}

// integLabelWidth aligns the attribute names under a source or a window.
const integLabelWidth = 20

// IntegrationsView renders the open tab under a strip naming every tab offered.
func IntegrationsView(s IntegrationsState) []TenantLine {
	out := []TenantLine{tabStrip(s), {}}
	switch s.Tab {
	case TabSync:
		out = append(out, syncLines(s)...)
	case TabRetention:
		out = append(out, retentionLines(s)...)
	default:
		out = append(out, webhookLines(s)...)
	}
	return out
}

// tabStrip names the offered tabs, the open one in brackets, so a terminal with
// no colour still says which one is drawn.
func tabStrip(s IntegrationsState) TenantLine {
	parts := make([]string, 0, len(s.Tabs))
	for _, t := range s.Tabs {
		if t == s.Tab {
			parts = append(parts, "["+t.String()+"]")
			continue
		}
		parts = append(parts, t.String())
	}
	return TenantLine{Text: "tabs: " + strings.Join(parts, "  "), Heading: true}
}

// webhookLines list the endpoints and the newest deliveries.
func webhookLines(s IntegrationsState) []TenantLine {
	out := []TenantLine{{Text: "endpoints (" + strconv.Itoa(len(s.Webhooks)) + "):", Heading: true}}
	if len(s.Webhooks) == 0 {
		out = append(out, tenantAbsent(s.WebhookErr, s.Loaded)...)
	}
	urls := map[string]string{}
	for i, w := range s.Webhooks {
		urls[w.ID] = w.URL
		selected := s.Selected == i
		state := "active"
		if !w.Active {
			state = "off"
		}
		events := "every event type"
		if len(w.EventTypes) > 0 {
			events = strings.Join(w.EventTypes, ", ")
		}
		out = append(out, TenantLine{
			Text:     SelectionMarker(selected) + pad(w.URL, tenantRowWidth) + "  " + pad(state, 8) + events,
			Selected: selected,
		})
	}
	if s.Secret != "" {
		out = append(out, TenantLine{}, TenantLine{Text: "signing secret for " + s.SecretFor +
			", shown once: " + s.Secret, Heading: true})
	}
	out = append(out, TenantLine{},
		TenantLine{Text: "deliveries (" + strconv.Itoa(len(s.Deliveries)) + ", newest first):", Heading: true})
	if len(s.Deliveries) == 0 {
		out = append(out, tenantAbsent(s.DeliveryErr, s.Loaded)...)
	}
	for i, d := range s.Deliveries {
		selected := s.Selected == len(s.Webhooks)+i
		out = append(out, TenantLine{Text: SelectionMarker(selected) + DeliveryLine(d, urls), Selected: selected})
	}
	if s.DeliveriesMore {
		out = append(out, TenantLine{Text: "  older deliveries: tix webhook deliveries --cursor", Dim: true})
	}
	return out
}

// DeliveryLine states one delivery: where it stands, how often it was tried,
// why the last attempt failed and where it went. The endpoint is named by its
// URL wherever the listing above holds it.
func DeliveryLine(d core.WebhookDelivery, urls map[string]string) string {
	reason := d.LastError
	if reason == "" {
		reason = "no error recorded"
	}
	if d.LastStatusCode != 0 {
		reason += " (HTTP " + strconv.Itoa(d.LastStatusCode) + ")"
	}
	target := urls[d.EndpointID]
	if target == "" {
		target = d.EndpointID
	}
	return pad(string(d.Status), 10) + pad(counted(d.Attempts, "attempt", "attempts"), 12) +
		reason + "  to " + target + "  " + d.ID
}

// syncLines list the sources with what a run would use, then the last run or
// check this view produced.
func syncLines(s IntegrationsState) []TenantLine {
	out := []TenantLine{{Text: "sources (" + strconv.Itoa(len(s.Sources)) + "):", Heading: true}}
	if len(s.Sources) == 0 {
		out = append(out, tenantAbsent(s.SourceErr, s.Loaded)...)
	}
	for i, src := range s.Sources {
		selected := s.Selected == i
		out = append(out, TenantLine{
			Text:     SelectionMarker(selected) + pad(src.Name, 16) + pad(src.System, 14) + lastRun(src, s.Times),
			Selected: selected,
		})
		for _, l := range SourceDetail(src) {
			out = append(out, TenantLine{Text: "    " + l, Dim: true})
		}
	}
	out = append(out, TenantLine{Text: "  sync runs on the server; what it lacks there is named under each source", Dim: true})
	if s.Check != nil {
		out = append(out, TenantLine{})
		out = append(out, checkLines(*s.Check)...)
	}
	switch {
	case s.RunErr != "":
		out = append(out, TenantLine{}, TenantLine{Text: "run refused, nothing was written:", Heading: true},
			TenantLine{Text: "  " + s.RunErr})
	case s.Run != nil:
		out = append(out, TenantLine{})
		out = append(out, RunLines(*s.Run)...)
	}
	return out
}

// lastRun says when a source last ran and how it went.
func lastRun(src core.SyncSource, times output.TimeStyle) string {
	if src.LastRunAt == nil {
		return "never run"
	}
	status := src.LastStatus
	if status == "" {
		status = "unknown"
	}
	return "last run " + times.FormatPtr(src.LastRunAt) + ", " + status
}

// SourceDetail states what a run of a source would use on the server and what
// it still lacks. A stored credential is named with its hint and never more.
func SourceDetail(src core.SyncSource) []string {
	var out []string
	row := func(label, value string) { out = append(out, pad(label, integLabelWidth)+value) }
	if r := src.Resolved; r != nil {
		row("mapping", originText(r.Mapping, true))
		row("credentials", originText(r.Credentials, false))
	}
	if len(src.Credentials) > 0 {
		names := make([]string, 0, len(src.Credentials))
		for _, c := range src.Credentials {
			name := c.Name
			if c.Hint != "" {
				name += " …" + c.Hint
			}
			names = append(names, name)
		}
		row("stored credentials", strings.Join(names, ", "))
	}
	if len(src.Config) > 0 {
		keys := make([]string, 0, len(src.Config))
		for k := range src.Config {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		settings := make([]string, 0, len(keys))
		for _, k := range keys {
			settings = append(settings, k+" "+src.Config[k])
		}
		row("settings", strings.Join(settings, ", "))
	}
	if src.MappingPath != "" {
		row("mapping file", src.MappingPath)
	}
	if r := src.Resolved; r != nil && len(r.Missing) > 0 {
		row("missing, cannot run", strings.Join(r.Missing, "; "))
	}
	return out
}

// originText says where a run reads its mapping or its credentials from, in
// the browser's words.
func originText(o core.SyncOrigin, mapping bool) string {
	switch o.From {
	case core.SyncFromStored:
		return "stored on the source"
	case core.SyncFromPath:
		return "file on the server " + o.Detail
	case core.SyncFromEnv:
		return "from the environment " + o.Detail
	case core.SyncFromNone:
		if !mapping {
			return "none needed"
		}
	}
	return "missing"
}

// checkLines state a source checked on the server with nothing saved.
func checkLines(src core.SyncSource) []TenantLine {
	out := []TenantLine{{Text: "checked " + src.Name + " on the server, nothing saved:", Heading: true}}
	if r := src.Resolved; r != nil && r.Summary != nil {
		out = append(out,
			TenantLine{Text: "  " + pad("imports into", integLabelWidth) + r.Summary.Project},
			TenantLine{Text: "  " + pad("identified by", integLabelWidth) + r.Summary.Identity},
			TenantLine{Text: "  " + pad("fields", integLabelWidth) + orNone(strings.Join(r.Summary.Fields, ", "))},
			TenantLine{Text: "  " + pad("statuses", integLabelWidth) + orNone(strings.Join(r.Summary.Statuses, ", "))})
	}
	for _, l := range SourceDetail(src) {
		out = append(out, TenantLine{Text: "  " + l})
	}
	if r := src.Resolved; r != nil && r.Ready() {
		out = append(out, TenantLine{Text: "  nothing missing, a run can start"})
	}
	return out
}

// RunLines state what a sync run did, or would do under a dry run.
func RunLines(r core.SyncResult) []TenantLine {
	head := "run result:"
	if r.DryRun {
		head = "run result, dry run, nothing was written:"
	}
	out := []TenantLine{
		{Text: head, Heading: true},
		{Text: "  " + pad("source", integLabelWidth) + r.Source + " (" + r.System + ")"},
		{Text: "  " + pad("created", integLabelWidth) + Counts(r.Created)},
		{Text: "  " + pad("updated", integLabelWidth) + Counts(r.Updated)},
		{Text: "  " + pad("skipped", integLabelWidth) + Counts(r.Skipped)},
	}
	if len(r.UnmatchedUsers) > 0 {
		out = append(out, TenantLine{Text: "  " + pad("no tix account", integLabelWidth) +
			strings.Join(r.UnmatchedUsers, ", ")})
	}
	out = append(out, TenantLine{Text: "  " + pad("warnings", integLabelWidth) + orNone(strings.Join(r.Warnings, "; "))})
	if r.Cursor != "" {
		out = append(out, TenantLine{Text: "  " + pad("cursor now", integLabelWidth) + r.Cursor})
	}
	return out
}

// Counts renders a per-kind tally the way the browser does, in a stable order,
// or "none" for an empty one.
func Counts(by map[string]int) string {
	keys := make([]string, 0, len(by))
	for k, n := range by {
		if n > 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "none"
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+" "+strconv.Itoa(by[k]))
	}
	return strings.Join(parts, ", ")
}

// retentionLines state the three windows and the last prune's report.
func retentionLines(s IntegrationsState) []TenantLine {
	out := []TenantLine{{Text: "retention:", Heading: true}}
	if s.Retention == nil {
		out = append(out, tenantAbsent(s.RetentionErr, s.Loaded)...)
	} else {
		p := *s.Retention
		out = append(out,
			TenantLine{Text: "  " + pad("events", integLabelWidth) + p.Events.Human()},
			TenantLine{Text: "  " + pad("audit entries", integLabelWidth) + p.AuditEntries.Human()},
			TenantLine{Text: "  " + pad("webhook deliveries", integLabelWidth) + p.WebhookDeliveries.Human()})
	}
	out = append(out,
		TenantLine{Text: "  prune deletes events, audit entries and deliveries older than these,", Dim: true},
		TenantLine{Text: "  and tix serve runs it on its own; tasks, projects and comments stay", Dim: true})
	if s.Prune != nil {
		out = append(out, TenantLine{})
		out = append(out, PruneLines(*s.Prune)...)
	}
	return out
}

// PruneLines state what a prune removed, or would remove under a dry run.
func PruneLines(r core.PruneResult) []TenantLine {
	head := "pruned:"
	if r.DryRun {
		head = "prune, dry run, nothing was removed:"
	}
	n := func(v int64) string { return strconv.FormatInt(v, 10) }
	return []TenantLine{
		{Text: head, Heading: true},
		{Text: "  " + pad("events", integLabelWidth) + n(r.Events)},
		{Text: "  " + pad("audit entries", integLabelWidth) + n(r.AuditEntries)},
		{Text: "  " + pad("webhook deliveries", integLabelWidth) + n(r.WebhookDeliveries)},
		{Text: "  " + pad("idempotency keys", integLabelWidth) + n(r.IdempotencyKeys)},
		{Text: "  " + pad("kept for subscribers", integLabelWidth) + n(r.RetainedForSubscribers)},
	}
}
