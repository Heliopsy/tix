// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// syncRoutes are the external import screens: the sources themselves and the
// history of what they have done.
func (h *handler) syncRoutes() []route {
	return []route{
		get(RouteSync, "sync.html", h.showSync, "ListSyncSources"),
		get(RouteSyncRuns, "syncruns.html", h.showSyncRuns, "ListSyncSources", "ListAudit"),
		post(RouteSyncSources, h.putSyncSource, "PutSyncSource"),
		post(RouteSyncDelete, h.deleteSyncSource, "DeleteSyncSource"),
		post(RouteSyncRun, h.runSync, "RunSync"),
	}
}

// envName renders an environment variable name with a break opportunity after
// each underscore, so a narrow table cell wraps it between its parts rather
// than through the middle of one. extsync.EnvName emits only A-Z, 0-9 and
// underscore, which envNameSafe asserts, so nothing here can carry markup.
func envName(parts ...string) template.HTML {
	joined := strings.Join(parts, "")
	if !envNameSafe(joined) {
		return template.HTML(template.HTMLEscapeString(joined)) // #nosec G203 -- escaped
	}
	// #nosec G203 -- every character was just checked against [A-Z0-9_*].
	return template.HTML(strings.ReplaceAll(joined, "_", "_<wbr>"))
}

// envNameSafe reports whether a value is drawn only from the alphabet
// extsync.EnvName produces, plus the trailing wildcard the screen appends.
func envNameSafe(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '*':
		default:
			return false
		}
	}
	return true
}

// syncSourceRow is one configured source plus the environment namespace it
// reads its settings from.
//
// The namespace is computed here rather than described in prose because the
// name-to-variable rule (upper case, every run of other characters collapsed
// to an underscore) is not something a reader should have to apply in their
// head to find out why their token is not being picked up. extsync.EnvName is
// the same function the import itself calls, so the prefix shown is the
// prefix read.
type syncSourceRow struct {
	core.SyncSource
	EnvPrefix string
	Settings  []syncSetting
}

// syncSetting is one environment variable a source reads, with what it is for
// and whether this system needs it.
type syncSetting struct {
	Name     string
	Purpose  string
	Required bool
}

// syncSystem is one adapter this build carries, described in the words
// somebody choosing between them needs.
type syncSystem struct {
	Value string
	Label string
	Note  string
}

// syncSystems describes each adapter importerFor (internal/service/sync.go)
// can build. The notes state only what the adapter code actually requires:
// generic reads a local file (internal/sync/generic), and both HTTP adapters
// refuse to build without a base url (internal/sync/httpx.go), with Jira
// additionally refusing without a project or a JQL query.
var syncSystems = []syncSystem{
	{core.SystemGeneric, "generic",
		"A CSV or JSON file on the machine running tix. Pick this for any tracker with no adapter of its own: export it to a file, describe the columns in a mapping, and no code is written and no binary is rebuilt."},
	{core.SystemJira, "jira",
		"A Jira instance, read through its search endpoint. Needs the site url and either a project key or a JQL query."},
	{core.SystemOpenProject, "openproject",
		"An OpenProject instance, read through its API v3 work packages. Needs the instance url, and a project when you want only one."},
}

// syncSettingsFor names the environment variables a source of this system
// reads, in the order somebody filling them in would meet them. Every entry
// names a suffix extsync.LoadSourceConfig actually reads; a variable outside
// that set would be ignored, so none is invented here.
func syncSettingsFor(system string) []syncSetting {
	mapping := syncSetting{extsync.EnvMapping, "the mapping file to translate records with", true}
	pageSize := syncSetting{extsync.EnvPageSize, "how many records one page fetches", false}
	switch system {
	case core.SystemGeneric:
		return []syncSetting{
			mapping,
			{extsync.EnvFile, "the .csv or .json file to read", true},
			pageSize,
		}
	case core.SystemJira:
		return []syncSetting{
			mapping,
			{extsync.EnvURL, "the Jira site's base url", true},
			{extsync.EnvToken, "an API token, sent as a bearer credential", false},
			{extsync.EnvUser, "the account, when the site wants basic auth", false},
			{extsync.EnvPassword, "that account's password or token", false},
			{extsync.EnvProject, "a project key, when you want one project", false},
			{extsync.EnvQuery, "a JQL query, instead of a project key", false},
			pageSize,
		}
	case core.SystemOpenProject:
		return []syncSetting{
			mapping,
			{extsync.EnvURL, "the OpenProject instance url", true},
			{extsync.EnvToken, "an API token, sent as a bearer credential", false},
			{extsync.EnvUser, "the account, when the instance wants basic auth", false},
			{extsync.EnvPassword, "that account's password or token", false},
			{extsync.EnvProject, "a project identifier, when you want one project", false},
			{extsync.EnvQuery, "an API filter expression", false},
			pageSize,
		}
	default:
		return []syncSetting{mapping}
	}
}

// syncView is what the external sync screen renders.
type syncView struct {
	Sources []syncSourceRow
	Systems []syncSystem
	Result  *core.SyncResult
	// Prefix is the environment namespace pattern, shown so the rule is
	// visible before any source exists to demonstrate it.
	Prefix string
}

// showSync renders the configured external import sources.
func (h *handler) showSync(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildSyncView(r, nil)
	if err != nil {
		return err
	}
	return h.render(w, r, "sync.html", "Sync", data)
}

// buildSyncView loads the sources and decorates each with the environment
// namespace it reads, so both the screen and a run's result page describe a
// source the same way.
func (h *handler) buildSyncView(r *http.Request, result *core.SyncResult) (syncView, error) {
	sources, err := h.svc.ListSyncSources(r.Context())
	if err != nil {
		return syncView{}, err
	}
	rows := make([]syncSourceRow, 0, len(sources))
	for _, s := range sources {
		rows = append(rows, syncSourceRow{
			SyncSource: s,
			EnvPrefix:  extsync.EnvName(s.Name, ""),
			Settings:   syncSettingsFor(s.System),
		})
	}
	return syncView{
		Sources: rows, Systems: syncSystems, Result: result,
		Prefix: extsync.EnvPrefix,
	}, nil
}

// putSyncSource registers or updates an external import source.
//
// Only the name and the system travel. A source record holds nothing else:
// PutSyncSource writes an ID, a system and a name, and everything the adapter
// needs to reach the source is read from the environment when a run starts.
// The screen used to offer a mapping path and a free-text configuration box,
// neither of which was stored anywhere, so a reader who filled them in got a
// saved source and no working import.
func (h *handler) putSyncSource(w http.ResponseWriter, r *http.Request) error {
	in := core.SyncSourceInput{
		ID:     field(r, "id"),
		System: field(r, "system"),
		Name:   field(r, "name"),
	}
	if _, err := h.svc.PutSyncSource(r.Context(), in); err != nil {
		return err
	}
	redirect(w, r, RouteSync, "source saved, now set "+extsync.EnvName(in.Name, "")+"* in the server's environment")
	return nil
}

// deleteSyncSource removes an external import source.
func (h *handler) deleteSyncSource(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteSyncSource(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteSync, "source deleted")
	return nil
}

// runSync runs or refreshes an external import and shows its result.
func (h *handler) runSync(w http.ResponseWriter, r *http.Request) error {
	result, err := h.svc.RunSync(r.Context(), core.RunSyncInput{
		SourceID: field(r, "source_id"),
		DryRun:   checked(r, "dry_run"),
		Full:     checked(r, "full"),
	})
	if err != nil {
		return err
	}
	data, err := h.buildSyncView(r, result)
	if err != nil {
		return err
	}
	return h.render(w, r, "sync.html", "Sync", data)
}

// auditSyncRunAction is the audit action one completed import writes. It
// mirrors the unexported auditSyncRun in internal/service/sync.go, which is
// the only writer of it; TestSyncRunsReadsTheActionTheServiceWrites keeps the
// two from drifting apart.
const auditSyncRunAction = "sync.run"

// syncSubjectType is the audit subject kind a sync source is recorded under,
// mirroring syncSourceSubject in internal/service/sync.go.
const syncSubjectType = "sync_source"

// syncRunsPageSize is how many completed runs one screen shows.
const syncRunsPageSize = 50

// syncRun is one completed import, as the audit trail recorded it.
//
// Everything here is read back out of the entry's own after image, which is
// the whole core.SyncResult the run returned. That is why the row can show
// counts and warnings at all: no separate run table exists, and the audit
// entry is the only place a finished run's numbers are kept.
type syncRun struct {
	At       time.Time
	Source   string
	System   string
	Created  int
	Updated  int
	Skipped  int
	Warnings []string
	ActorID  string
}

// syncRunsView is what the run history screen renders.
type syncRunsView struct {
	Runs       []syncRun
	Sources    []syncSourceRow
	Names      actorNames
	NextCursor string
}

// showSyncRuns renders the history of completed imports, newest first, beside
// the current state of every configured source.
//
// Two tables rather than one, because they answer different questions and
// neither answers the other's. The history is what ran: one row per import
// that finished, with what it did. The source table is where each source
// stands now, and it is the only thing that reports a run that failed --
// see the note the template carries, and syncRun's own comment, for why.
func (h *handler) showSyncRuns(w http.ResponseWriter, r *http.Request) error {
	cursor := r.URL.Query().Get("cursor")
	entries, next, err := h.svc.ListAudit(r.Context(), core.AuditFilter{
		SubjectType: syncSubjectType,
		Actions:     []string{auditSyncRunAction},
		Page: core.Page{
			Cursor: cursor, Sort: "seq",
			Direction: core.Descending, Limit: syncRunsPageSize,
		},
	})
	if err != nil {
		return err
	}
	sources, err := h.buildSyncView(r, nil)
	if err != nil {
		return err
	}

	data := syncRunsView{
		Runs: make([]syncRun, 0, len(entries)), Sources: sources.Sources,
		NextCursor: next,
	}
	actors := make([]string, 0, len(entries))
	for _, entry := range entries {
		data.Runs = append(data.Runs, syncRunOf(entry))
		actors = append(actors, entry.ActorID)
	}
	data.Names = h.resolveActors(r, actors...)
	return h.render(w, r, "syncruns.html", "Sync runs", data)
}

// syncRunOf reads one run out of its audit entry. An entry whose after image
// cannot be read still produces a row: the time, the actor and the fact that
// a run happened are on the entry itself, and losing the whole row because
// its snapshot did not parse would hide the very thing this screen exists to
// show.
func syncRunOf(entry core.AuditEntry) syncRun {
	out := syncRun{At: entry.OccurredAt, ActorID: entry.ActorID}
	var result core.SyncResult
	if len(entry.After) == 0 || json.Unmarshal(entry.After, &result) != nil {
		return out
	}
	// Cursor is deliberately not read: RunSync assigns SyncResult.Cursor
	// after the completion entry has already been written, so the snapshot
	// never carries one. Where a source's cursor stands now comes from the
	// source record instead.
	out.Source, out.System = result.Source, result.System
	out.Created = result.Created[syncEntityTask]
	out.Updated = result.Updated[syncEntityTask]
	out.Skipped = result.Skipped[syncEntityTask]
	out.Warnings = result.Warnings
	return out
}

// syncEntityTask is the key an import tallies its counts under, mirroring the
// constant of the same name in internal/service/sync.go. Tasks are the only
// entity v1 imports, so it is the only key a result carries.
const syncEntityTask = "task"
