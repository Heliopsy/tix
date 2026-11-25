// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
	"github.com/heliopsy/tix/internal/sync/openproject"
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
	segments := strings.SplitAfter(joined, "_")
	for i, segment := range segments {
		segments[i] = chunked(segment, envNameChunk)
	}
	// #nosec G203 -- every character was just checked against [A-Z0-9_*].
	return template.HTML(strings.Join(segments, "<wbr>"))
}

// envNameChunk is the longest run an environment variable name keeps without a
// break opportunity. A source name with one very long word otherwise makes a
// segment wider than a phone, and the cell holding it pushes the page sideways.
const envNameChunk = 12

// chunked puts a break opportunity every n characters of a segment longer than
// n, and leaves a shorter one whole.
func chunked(part string, n int) string {
	if len(part) <= n+1 {
		return part
	}
	var b strings.Builder
	for len(part) > n+1 {
		b.WriteString(part[:n])
		b.WriteString("<wbr>")
		part = part[n:]
	}
	b.WriteString(part)
	return b.String()
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
	Stored    []syncStored
}

// syncStored is one setting stored on a source, which wins over its variable.
type syncStored struct {
	Key   string
	Value string
}

// storedSettings lists what a source stores, the mapping first and then the
// config keys in the order its system documents them.
func storedSettings(s core.SyncSource) []syncStored {
	var out []syncStored
	if s.MappingPath != "" {
		out = append(out, syncStored{"mapping_path", s.MappingPath})
	}
	for _, key := range core.SyncSettingKeys(s.System) {
		if value := s.Config[key]; value != "" {
			out = append(out, syncStored{key, value})
		}
	}
	return out
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
		"A CSV or JSON file on the machine running tix. Pick this for any tracker with no adapter of its own: export it to a file and describe the columns in a mapping, with no code to write and no binary to rebuild."},
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
	// Refusal is why a run was refused before it wrote anything, such as a
	// mapping file that is missing or does not parse.
	Refusal string
	// Prefix is the environment namespace pattern, shown so the rule is
	// visible before any source exists to demonstrate it.
	Prefix string
	// Preview is the source as the dialog's Check found it, nothing saved.
	Preview *core.SyncSource
	// Editing is the source the dialog was opened to set up, if any.
	Editing string
}

// syncPreviewKey carries a checked source to the screen drawn for it.
type syncPreviewKey struct{}

// showSync renders the configured external import sources.
func (h *handler) showSync(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildSyncView(r, nil)
	if err != nil {
		return err
	}
	if preview, ok := r.Context().Value(syncPreviewKey{}).(*core.SyncSource); ok {
		data.Preview = preview
	}
	r = h.prefillSource(r, &data)
	return h.render(w, r, "sync.html", "Sync", data)
}

// prefillSource opens the source dialog holding an existing source's values
// when its Set up link asked for it, so saving it again changes only what the
// reader edits rather than clearing the settings they did not retype.
func (h *handler) prefillSource(r *http.Request, data *syncView) *http.Request {
	d := dialogOf(r)
	id := r.URL.Query().Get("source")
	if !d.Opens(syncDialog) || d.Refused() {
		return r
	}
	if r.URL.Query().Get("template") == core.SystemOpenProject {
		values := url.Values{"system": {core.SystemOpenProject}, "mapping": {string(openproject.DefaultMapping())}}
		return r.WithContext(context.WithValue(r.Context(), dialogKey{}, dialogState{Open: syncDialog, Values: values}))
	}
	if id == "" {
		return r
	}
	for _, s := range data.Sources {
		if s.ID != id {
			continue
		}
		values := url.Values{"name": {s.Name}, "system": {s.System}, "mapping_path": {s.MappingPath},
			"mapping": {s.Mapping}}
		for key, value := range s.Config {
			values.Set(key, value)
		}
		data.Editing = s.Name
		return r.WithContext(context.WithValue(r.Context(), dialogKey{}, dialogState{Open: syncDialog, Values: values}))
	}
	return r
}

// syncDialog is the id of the dialog that configures a source.
const syncDialog = "source-new"

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
			Stored:     storedSettings(s),
		})
	}
	return syncView{
		Sources: rows, Systems: syncSystems, Result: result,
		Prefix: extsync.EnvPrefix,
	}, nil
}

// putSyncSource registers or updates an external import source, or, for the
// dialog's Check button, validates it on the server and draws what a run
// would use without saving anything.
//
// The form offers every setting any system stores, and sends on only the ones
// filled in. The service refuses a setting the chosen system does not take,
// so a url typed into a generic source is an error rather than ignored. An
// uploaded mapping file wins over the text pasted beside it.
func (h *handler) putSyncSource(w http.ResponseWriter, r *http.Request) error {
	in := core.SyncSourceInput{
		ID:               field(r, "id"),
		System:           field(r, "system"),
		Name:             field(r, "name"),
		MappingPath:      field(r, "mapping_path"),
		Mapping:          prose(r, "mapping"),
		Token:            field(r, "token"),
		User:             field(r, "user"),
		Password:         field(r, "password"),
		ClearMapping:     checked(r, "clear_mapping"),
		ClearCredentials: checked(r, "clear_credentials"),
		Check:            field(r, "intent") == "check",
	}
	uploaded, err := uploadedMapping(r)
	if err != nil {
		return h.refuseIn(w, r, syncDialog, RouteSync, core.OnField(err, "mapping"), h.showSync)
	}
	if uploaded != "" {
		in.Mapping = uploaded
		r.PostForm.Set("mapping", uploaded)
	}
	for _, key := range syncFormSettings {
		if value := field(r, key); value != "" {
			if in.Config == nil {
				in.Config = map[string]any{}
			}
			in.Config[key] = value
		}
	}
	for _, secret := range []string{"token", "password"} {
		r.PostForm.Del(secret)
	}
	source, err := h.svc.PutSyncSource(r.Context(), in)
	if err != nil {
		return h.refuseIn(w, r, syncDialog, RouteSync, err, h.showSync)
	}
	if in.Check {
		state := dialogState{Open: syncDialog, Values: r.PostForm}
		if state.Values == nil {
			state.Values = url.Values{}
		}
		ctx := context.WithValue(context.WithValue(r.Context(), dialogKey{}, state), syncPreviewKey{}, source)
		clone := r.Clone(ctx)
		clone.Method = http.MethodGet
		clone.URL = &url.URL{Path: RouteSync}
		w.Header().Set("HX-Replace-Url", RouteSync+"?"+url.Values{DialogParam: {syncDialog}}.Encode())
		return h.showSync(w, clone)
	}
	message := "source saved; runs execute on the server"
	if source.Resolved != nil && !source.Resolved.Ready() {
		message = "source saved, but it cannot run yet: the server is missing " + strings.Join(source.Resolved.Missing, "; ")
	}
	redirect(w, r, RouteSync, message)
	return nil
}

// maxUploadedMapping bounds how much of an uploaded mapping file is read; the
// service refuses anything over its own, smaller limit with a message.
const maxUploadedMapping = 1 << 20

// uploadedMapping is the mapping file chosen in the dialog, or empty.
func uploadedMapping(r *http.Request) (string, error) {
	if r.MultipartForm == nil {
		return "", nil
	}
	files := r.MultipartForm.File["mapping_file"]
	if len(files) == 0 || files[0].Size == 0 {
		return "", nil
	}
	file, err := files[0].Open()
	if err != nil {
		return "", core.Invalid("the uploaded mapping file could not be read")
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, maxUploadedMapping))
	if err != nil {
		return "", core.Invalid("the uploaded mapping file could not be read")
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	return strings.TrimSpace(text), nil
}

// syncFormSettings are the config keys the source form offers, the union of
// what every system takes.
var syncFormSettings = []string{
	core.SyncSettingURL, core.SyncSettingProject, core.SyncSettingQuery,
	core.SyncSettingFile, core.SyncSettingPageSize,
}

// deleteSyncSource removes an external import source.
func (h *handler) deleteSyncSource(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteSyncSource(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteSync, "source deleted")
	return nil
}

// runSync runs or refreshes an external import and shows its result, or why
// it was refused, at the top of the sources screen.
//
// The answer is drawn at /sync rather than at the address it was posted to,
// which has no screen: reloading it or coming back to it must not run the
// import again or land on a missing page.
func (h *handler) runSync(w http.ResponseWriter, r *http.Request) error {
	result, err := h.svc.RunSync(r.Context(), core.RunSyncInput{
		SourceID: field(r, "source_id"),
		DryRun:   checked(r, "dry_run"),
		Full:     checked(r, "full"),
	})
	status := http.StatusOK
	if err != nil {
		if !correctable(err) {
			return err
		}
		status = core.KindOf(err).HTTPStatus()
	}
	data, buildErr := h.buildSyncView(r, result)
	if buildErr != nil {
		return buildErr
	}
	if err != nil {
		data.Refusal = safeMessage(err, core.KindOf(err))
	}
	w.Header().Set("HX-Replace-Url", RouteSync)
	return h.renderStatus(w, r, status, "sync.html", "Sync", data)
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
	// Token is the name the API token that started the run had then, empty
	// when no token did.
	Token string
}

// syncRunsView is what the run history screen renders.
type syncRunsView struct {
	Runs    []syncRun
	Sources []syncSourceRow
	Names   actorNames
	Pager   pager
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
	read := func(cursor string, limit int) ([]core.AuditEntry, string, error) {
		return h.svc.ListAudit(r.Context(), core.AuditFilter{
			SubjectType: syncSubjectType,
			Actions:     []string{auditSyncRunAction},
			Page: core.Page{
				Cursor: cursor, Sort: "seq",
				Direction: core.Descending, Limit: limit,
			},
		})
	}
	pos, err := locate(r, syncRunsPageSize, walkOf(read))
	if err != nil {
		return err
	}
	entries, next, err := read(pos.Cursor, syncRunsPageSize)
	if err != nil {
		return err
	}
	sources, err := h.buildSyncView(r, nil)
	if err != nil {
		return err
	}

	data := syncRunsView{
		Runs: make([]syncRun, 0, len(entries)), Sources: sources.Sources,
		Pager: newPager(r, RouteSyncRuns, next, len(entries), "runs").counted(pos, syncRunsPageSize),
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
	out := syncRun{At: entry.OccurredAt, ActorID: entry.ActorID, Token: entry.Via.TokenName}
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
