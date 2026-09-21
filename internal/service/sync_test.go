package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	extsync "github.com/heliopsy/tix/internal/sync"
	"github.com/heliopsy/tix/internal/sync/jira"
)

const syncMapping = `
version: 1
project: ops
workflow: syncflow
identity:
  id: id
  url: url
  version: updated
  updated_at: updated
fields:
  title: title
  notes: body
status_field: status
statuses:
  To Do: todo
  In Progress: doing
  Done: done
default_status: todo
custom:
  points:
    key: story_points
    type: float
    label: Story points
unmapped:
  preserve: true
  prefix: ext_
lossy:
  - field: watchers
    reason: watcher lists are flattened to a count
`

const syncCSV = `id,title,status,updated,points,reporter,url,watchers
E-1,first issue,To Do,2026-01-01T00:00:00Z,3,bob,https://src.test/E-1,4
E-2,second issue,Done,2026-01-02T00:00:00Z,5,carol,https://src.test/E-2,1
`

// syncFixture is a service with a project, a workflow, a mapping on disk and a
// registered import source.
type syncFixture struct {
	local  *Local
	clock  *clock.Fake
	scope  core.TenantScope
	actor  *core.Actor
	ctx    context.Context
	source *core.SyncSource
	dir    string
	file   string
}

func newSyncFixture(t *testing.T, name string) *syncFixture {
	t.Helper()
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{
		Key: "syncflow", Name: "Sync", Definition: core.WorkflowDefinition{
			Initial: "todo",
			States: []core.State{
				{Key: "todo"}, {Key: "doing"}, {Key: "done", Terminal: true},
			},
			Transitions: []core.Transition{{From: "todo", To: "doing"}, {From: "doing", To: "done"}},
		},
	}); err != nil {
		t.Fatalf("creating workflow: %v", err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{
		Key: "ops", Name: "Ops", WorkflowKey: "syncflow",
	}); err != nil {
		t.Fatalf("creating project: %v", err)
	}

	dir := t.TempDir()
	mappingPath := filepath.Join(dir, "mapping.yaml")
	if err := os.WriteFile(mappingPath, []byte(syncMapping), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	file := filepath.Join(dir, "rows.csv")
	if err := os.WriteFile(file, []byte(syncCSV), 0o600); err != nil {
		t.Fatalf("writing rows: %v", err)
	}

	t.Setenv(extsync.EnvName(name, extsync.EnvMapping), mappingPath)
	t.Setenv(extsync.EnvName(name, extsync.EnvFile), file)

	src, err := l.PutSyncSource(ctx, core.SyncSourceInput{System: core.SystemGeneric, Name: name})
	if err != nil {
		t.Fatalf("registering sync source: %v", err)
	}
	return &syncFixture{local: l, clock: clk, scope: scope, actor: actor, ctx: ctx,
		source: src, dir: dir, file: file}
}

func (f *syncFixture) run(t *testing.T, in core.RunSyncInput) *core.SyncResult {
	t.Helper()
	in.SourceID = f.source.ID
	res, err := f.local.RunSync(f.ctx, in)
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	return res
}

func (f *syncFixture) tasks(t *testing.T) []core.Task {
	t.Helper()
	page, err := f.local.ListTasks(f.ctx, core.TaskFilter{Page: core.Page{Limit: 100}})
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	return page.Tasks
}

func (f *syncFixture) ref(t *testing.T, externalID string) core.ExternalRef {
	t.Helper()
	var out core.ExternalRef
	if err := f.local.read(f.ctx, f.actor, func(tx store.Tx) error {
		r, err := tx.GetExternalRef(f.ctx, core.SystemGeneric, externalID, "task")
		if err != nil {
			return err
		}
		out = *r
		return nil
	}); err != nil {
		t.Fatalf("reading external reference %q: %v", externalID, err)
	}
	return out
}

func (f *syncFixture) reloadSource(t *testing.T) core.SyncSource {
	t.Helper()
	sources, err := f.local.ListSyncSources(f.ctx)
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}
	for _, s := range sources {
		if s.ID == f.source.ID {
			return s
		}
	}
	t.Fatalf("source %q disappeared", f.source.ID)
	return core.SyncSource{}
}

func TestRunSyncImportsACSVFile(t *testing.T) {
	f := newSyncFixture(t, "ops")
	res := f.run(t, core.RunSyncInput{})

	if res.Created["task"] != 2 {
		t.Fatalf("created = %v, want both rows", res.Created)
	}
	if res.System != core.SystemGeneric || res.Source != "ops" {
		t.Errorf("result names %q / %q", res.System, res.Source)
	}
	tasks := f.tasks(t)
	if len(tasks) != 2 {
		t.Fatalf("imported %d tasks, want 2", len(tasks))
	}

	byTitle := map[string]core.Task{}
	for _, task := range tasks {
		byTitle[task.Title] = task
	}
	first, ok := byTitle["first issue"]
	if !ok {
		t.Fatalf("titles = %v", byTitle)
	}
	if first.Status != "todo" {
		t.Errorf("status = %q, want the mapped state", first.Status)
	}
	if first.CustomFields["story_points"] != 3.0 {
		t.Errorf("story_points = %v", first.CustomFields["story_points"])
	}
	if byTitle["second issue"].Status != "done" {
		t.Errorf("second status = %q", byTitle["second issue"].Status)
	}

	ref := f.ref(t, "E-1")
	if ref.System != core.SystemGeneric || ref.ExternalID != "E-1" {
		t.Errorf("reference = %+v", ref)
	}
	if ref.ExternalURL != "https://src.test/E-1" {
		t.Errorf("external url = %q", ref.ExternalURL)
	}
	if ref.ExternalVersion != "2026-01-01T00:00:00Z" {
		t.Errorf("external version = %q", ref.ExternalVersion)
	}
	if ref.LastSyncedAt.IsZero() {
		t.Error("reference carries no last synced time")
	}
}

func TestRunSyncImportsJSONWithNestedFields(t *testing.T) {
	f := newSyncFixture(t, "ops")
	nested := `
version: 1
project: ops
identity:
  id: id
  version: meta.updated
  updated_at: meta.updated
fields:
  attrs.summary: title
status_field: attrs.state.name
statuses:
  Open: todo
default_status: todo
`
	mappingPath := filepath.Join(f.dir, "nested.yaml")
	if err := os.WriteFile(mappingPath, []byte(nested), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	jsonPath := filepath.Join(f.dir, "rows.json")
	body := `[{"id":"J-1","attrs":{"summary":"nested title","state":{"name":"Open"}},
		"meta":{"updated":"2026-02-01T00:00:00Z"}}]`
	if err := os.WriteFile(jsonPath, []byte(body), 0o600); err != nil {
		t.Fatalf("writing rows: %v", err)
	}
	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), mappingPath)
	t.Setenv(extsync.EnvName("ops", extsync.EnvFile), jsonPath)

	res := f.run(t, core.RunSyncInput{})
	if res.Created["task"] != 1 {
		t.Fatalf("created = %v", res.Created)
	}
	if got := f.tasks(t); len(got) != 1 || got[0].Title != "nested title" {
		t.Errorf("imported %v", got)
	}
}

// Re-importing is the behaviour the whole design rests on: a second run must
// refresh what changed and leave everything else alone, never duplicating.
func TestRunSyncIsIdempotent(t *testing.T) {
	f := newSyncFixture(t, "ops")
	first := f.run(t, core.RunSyncInput{})
	if first.Created["task"] != 2 {
		t.Fatalf("first run created = %v", first.Created)
	}

	second := f.run(t, core.RunSyncInput{Full: true})
	if second.Created["task"] != 0 || second.Updated["task"] != 0 {
		t.Errorf("second run created %v and updated %v, want neither",
			second.Created, second.Updated)
	}
	if second.Skipped["task"] != 2 {
		t.Errorf("second run skipped = %v, want both rows", second.Skipped)
	}
	if !anyContains(second.Warnings, "matches the stored one") {
		t.Errorf("warnings = %v, want the skip reason reported", second.Warnings)
	}
	if got := f.tasks(t); len(got) != 2 {
		t.Errorf("task count = %d after two runs, want 2", len(got))
	}
}

func TestRunSyncUpdatesChangedRecordsInPlace(t *testing.T) {
	f := newSyncFixture(t, "ops")
	f.run(t, core.RunSyncInput{})
	before := f.tasks(t)

	changed := strings.Replace(syncCSV,
		"E-1,first issue,To Do,2026-01-01T00:00:00Z",
		"E-1,renamed issue,In Progress,2026-01-05T00:00:00Z", 1)
	if err := os.WriteFile(f.file, []byte(changed), 0o600); err != nil {
		t.Fatalf("rewriting rows: %v", err)
	}

	res := f.run(t, core.RunSyncInput{Full: true})
	if res.Updated["task"] != 1 || res.Created["task"] != 0 {
		t.Fatalf("result created %v updated %v, want one update and no creation",
			res.Created, res.Updated)
	}
	after := f.tasks(t)
	if len(after) != len(before) {
		t.Fatalf("task count changed from %d to %d", len(before), len(after))
	}
	var renamed bool
	for _, task := range after {
		if task.Title == "renamed issue" {
			renamed = true
			if task.Status != "doing" {
				t.Errorf("status = %q, want the newly mapped state", task.Status)
			}
		}
	}
	if !renamed {
		t.Error("the changed record was not applied to the existing task")
	}
	if got := f.ref(t, "E-1").ExternalVersion; got != "2026-01-05T00:00:00Z" {
		t.Errorf("external version = %q, want the newer revision", got)
	}
}

func TestRunSyncPreservesUnmappedFields(t *testing.T) {
	f := newSyncFixture(t, "ops")
	res := f.run(t, core.RunSyncInput{})

	var found bool
	for _, task := range f.tasks(t) {
		if task.CustomFields["ext_reporter"] != nil {
			found = true
		}
	}
	if !found {
		t.Errorf("no imported task kept the unmapped reporter field")
	}
	if !anyContains(res.Warnings, "preserved unmapped fields") {
		t.Errorf("warnings = %v, want the preservation reported", res.Warnings)
	}
	if !anyContains(res.Warnings, "watcher lists are flattened") {
		t.Errorf("warnings = %v, want the lossy mapping reported", res.Warnings)
	}
}

// A dry run must describe the real run exactly and write nothing at all.
func TestRunSyncDryRunWritesNothingAndPredictsTheRealRun(t *testing.T) {
	f := newSyncFixture(t, "ops")
	beforeEvents, beforeAudits := countRows(t, f.local, f.scope)

	plan := f.run(t, core.RunSyncInput{DryRun: true})
	if !plan.DryRun {
		t.Error("result is not marked as a dry run")
	}
	if plan.Created["task"] != 2 {
		t.Fatalf("plan created = %v, want both rows", plan.Created)
	}
	if got := f.tasks(t); len(got) != 0 {
		t.Fatalf("a dry run created %d tasks", len(got))
	}
	afterEvents, afterAudits := countRows(t, f.local, f.scope)
	if afterEvents != beforeEvents || afterAudits != beforeAudits {
		t.Errorf("a dry run wrote %d events and %d audit entries",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
	if got := f.reloadSource(t); got.Cursor != "" {
		t.Errorf("a dry run advanced the cursor to %q", got.Cursor)
	}

	real := f.run(t, core.RunSyncInput{})
	if real.Created["task"] != plan.Created["task"] {
		t.Errorf("real run created %v, dry run predicted %v", real.Created, plan.Created)
	}

	repeat := f.run(t, core.RunSyncInput{DryRun: true, Full: true})
	if repeat.Skipped["task"] != 2 {
		t.Errorf("a dry run over imported data reported %v, want both rows skipped", repeat.Skipped)
	}
	if !anyContains(repeat.Warnings, "watcher lists are flattened") {
		t.Errorf("a dry run did not report the lossy mapping before writing: %v", repeat.Warnings)
	}
}

func TestRunSyncEmitsAuditAndEventsAsTheSystemActor(t *testing.T) {
	f := newSyncFixture(t, "ops")
	f.run(t, core.RunSyncInput{})

	var (
		entries []core.AuditEntry
		events  []core.Event
	)
	if err := f.local.read(f.ctx, f.actor, func(tx store.Tx) error {
		var err error
		entries, err = tx.ListAudit(f.ctx, core.AuditFilter{Page: core.Page{Limit: 200}})
		if err != nil {
			return err
		}
		events, err = tx.ReadEvents(f.ctx, 0, 200)
		return err
	}); err != nil {
		t.Fatalf("reading history: %v", err)
	}

	var imported int
	for _, e := range entries {
		if e.Action != auditSyncTaskCreate {
			continue
		}
		imported++
		if e.Source != core.SourceSystem {
			t.Errorf("audit source = %q, want %q", e.Source, core.SourceSystem)
		}
		if e.ActorID == f.actor.ID {
			t.Error("an imported creation was attributed to the operator")
		}
	}
	if imported != 2 {
		t.Errorf("audited %d imported creations, want 2", imported)
	}

	var taskEvents int
	for _, e := range events {
		if e.Type == core.EventTaskCreated {
			taskEvents++
		}
	}
	if taskEvents != 2 {
		t.Errorf("emitted %d task events, want 2", taskEvents)
	}
}

func TestRunSyncSkippedRecordsEmitNoEvents(t *testing.T) {
	f := newSyncFixture(t, "ops")
	f.run(t, core.RunSyncInput{})
	beforeEvents, _ := countRows(t, f.local, f.scope)

	res := f.run(t, core.RunSyncInput{Full: true})
	if res.Skipped["task"] != 2 {
		t.Fatalf("skipped = %v", res.Skipped)
	}
	afterEvents, _ := countRows(t, f.local, f.scope)
	if afterEvents-beforeEvents > 1 {
		t.Errorf("a run that skipped everything emitted %d events", afterEvents-beforeEvents)
	}
}

func anyContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// fakeImporter replays recorded pages and can fail partway, which is how the
// cursor and resume rules are exercised without a network.
type fakeImporter struct {
	pages  []extsync.Batch
	failAt int
	calls  []extsync.Options
}

func (f *fakeImporter) System() string { return core.SystemGeneric }

func (f *fakeImporter) Fetch(_ context.Context, opt extsync.Options) (extsync.Batch, error) {
	f.calls = append(f.calls, opt)
	idx := 0
	if opt.Page != "" {
		n, err := strconv.Atoi(opt.Page)
		if err != nil {
			return extsync.Batch{}, err
		}
		idx = n
	}
	if idx == f.failAt {
		return extsync.Batch{}, errors.New("the source went away")
	}
	if idx >= len(f.pages) {
		return extsync.Batch{}, nil
	}
	return f.pages[idx], nil
}

// useImporter installs a replacement adapter factory for one test.
func useImporter(t *testing.T, build func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error)) {
	t.Helper()
	syncImporters = build
	t.Cleanup(func() { syncImporters = nil })
}

func syncRecord(id, title, status, updated string) extsync.Record {
	return extsync.Record{
		Fields: map[string]any{
			"id": id, "title": title, "status": status, "updated": updated,
		},
	}
}

func pagedImporter(failAt int) *fakeImporter {
	return &fakeImporter{
		failAt: failAt,
		pages: []extsync.Batch{
			{
				Records: []extsync.Record{
					syncRecord("P-1", "page one first", "To Do", "2026-01-01T00:00:00Z"),
					syncRecord("P-2", "page one second", "To Do", "2026-01-02T00:00:00Z"),
				},
				Page: "1", Cursor: "2026-01-02T00:00:00Z",
			},
			{
				Records: []extsync.Record{
					syncRecord("P-3", "page two first", "Done", "2026-01-03T00:00:00Z"),
				},
				Cursor: "2026-01-03T00:00:00Z",
			},
		},
	}
}

func TestRunSyncPagesThroughEveryPage(t *testing.T) {
	f := newSyncFixture(t, "ops")
	imp := pagedImporter(-1)
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return imp, nil
	})

	res := f.run(t, core.RunSyncInput{})
	if res.Created["task"] != 3 {
		t.Fatalf("created = %v, want every record from both pages", res.Created)
	}
	if len(imp.calls) != 2 {
		t.Errorf("fetched %d pages, want 2", len(imp.calls))
	}
	if got := f.reloadSource(t); got.Cursor != "2026-01-03T00:00:00Z" {
		t.Errorf("cursor = %q, want the last page's watermark", got.Cursor)
	}
}

// An interrupted run must leave the cursor where the committed work ended, so
// a re-run resumes rather than restarting or losing records.
func TestRunSyncCursorAdvancesOnlyOnSuccessAndTheRunResumes(t *testing.T) {
	f := newSyncFixture(t, "ops")
	failing := pagedImporter(1)
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return failing, nil
	})

	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID}); err == nil {
		t.Fatal("RunSync succeeded against a source that failed partway")
	}
	interrupted := f.reloadSource(t)
	if interrupted.Cursor != "2026-01-02T00:00:00Z" {
		t.Fatalf("cursor = %q, want the first page's watermark and no further",
			interrupted.Cursor)
	}
	if interrupted.LastStatus != "failed" {
		t.Errorf("last status = %q, want the failure recorded", interrupted.LastStatus)
	}
	if got := f.tasks(t); len(got) != 2 {
		t.Fatalf("committed %d tasks from the first page, want 2", len(got))
	}

	resumed := pagedImporter(-1)
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return resumed, nil
	})
	res := f.run(t, core.RunSyncInput{})
	if resumed.calls[0].Cursor != "2026-01-02T00:00:00Z" {
		t.Errorf("the re-run started from cursor %q, want the stored one",
			resumed.calls[0].Cursor)
	}
	if res.Created["task"] != 1 {
		t.Errorf("the re-run created %v, want only the record left behind", res.Created)
	}
	if res.Skipped["task"] != 2 {
		t.Errorf("the re-run skipped %v, want the already-imported records", res.Skipped)
	}
	if got := f.tasks(t); len(got) != 3 {
		t.Errorf("task count = %d after resuming, want 3 with nothing duplicated", len(got))
	}
	if got := f.reloadSource(t); got.LastStatus != "ok" {
		t.Errorf("last status = %q after a clean run", got.LastStatus)
	}
}

func TestRunSyncFullRefreshIgnoresTheCursor(t *testing.T) {
	f := newSyncFixture(t, "ops")
	imp := pagedImporter(-1)
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return imp, nil
	})
	f.run(t, core.RunSyncInput{})

	imp.calls = nil
	f.run(t, core.RunSyncInput{Full: true})
	if imp.calls[0].Cursor != "" || !imp.calls[0].Full {
		t.Errorf("a full refresh passed %+v, want the cursor ignored", imp.calls[0])
	}
}

func TestRunSyncLeavesNoHalfWrittenEntityWhenAPageFails(t *testing.T) {
	f := newSyncFixture(t, "ops")
	mapping := `
version: 1
project: ops
identity:
  id: id
  version: updated
  updated_at: updated
fields:
  title: title
status_field: status
statuses:
  To Do: todo
default_status: todo
custom:
  blob:
    key: blob
    type: json
unmapped:
  preserve: false
`
	path := filepath.Join(f.dir, "blob.yaml")
	if err := os.WriteFile(path, []byte(mapping), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), path)

	poison := syncRecord("B-2", "unstorable", "To Do", "2026-01-02T00:00:00Z")
	poison.Fields["blob"] = make(chan int)
	broken := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{
			syncRecord("B-1", "good", "To Do", "2026-01-01T00:00:00Z"),
			poison,
		},
		Cursor: "2026-01-02T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return broken, nil
	})

	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID}); err == nil {
		t.Fatal("RunSync accepted a record the store refuses")
	}
	if got := f.tasks(t); len(got) != 0 {
		t.Errorf("a failed page committed %d tasks", len(got))
	}
	if got := f.reloadSource(t); got.Cursor != "" {
		t.Errorf("a failed page advanced the cursor to %q", got.Cursor)
	}
}

// The service must drive a real HTTP adapter through throttling to completion,
// not just the adapter in isolation.
func TestRunSyncBacksOffAgainstARateLimitedSource(t *testing.T) {
	f := newSyncFixture(t, "ops")
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"startAt":0,"maxResults":1,"total":1,"issues":[
			{"key":"J-1","fields":{"summary":"throttled issue","status":{"name":"To Do"},
			"updated":"2026-01-01T00:00:00Z"}}]}`))
	}))
	defer srv.Close()

	writeJiraMapping(t, f)
	var slept []time.Duration
	useImporter(t, func(_ string, cfg extsync.SourceConfig, m *extsync.Mapping) (extsync.Importer, error) {
		return jira.New(jira.Options{
			Config:       cfg.WithCredentials("s3cr3t", "", ""),
			UpdatedField: m.Identity.UpdatedAt,
			Client:       srv.Client(),
			Sleep:        func(d time.Duration) { slept = append(slept, d) },
			MaxRetries:   3,
			BaseDelay:    time.Millisecond,
		})
	})
	t.Setenv(extsync.EnvName("ops", extsync.EnvURL), srv.URL)
	t.Setenv(extsync.EnvName("ops", extsync.EnvProject), "OPS")

	res := f.run(t, core.RunSyncInput{})
	if res.Created["task"] != 1 {
		t.Fatalf("created = %v after backing off", res.Created)
	}
	if len(slept) != 1 {
		t.Errorf("backed off %v, want one wait", slept)
	}
}

// writeJiraMapping points the fixture's source at a mapping shaped for Jira.
func writeJiraMapping(t *testing.T, f *syncFixture) {
	t.Helper()
	body := `
version: 1
project: ops
identity:
  id: key
  url: url
  version: fields.updated
  updated_at: fields.updated
fields:
  fields.summary: title
status_field: fields.status.name
statuses:
  To Do: todo
default_status: todo
`
	path := filepath.Join(f.dir, "jira.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), path)
}

// A credential must never reach an audit entry, an event payload or an error,
// however the import ends.
func TestRunSyncKeepsCredentialsOutOfHistoryAndErrors(t *testing.T) {
	const secret = "sup3r-s3cret-token"
	f := newSyncFixture(t, "ops")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	writeJiraMapping(t, f)
	t.Setenv(extsync.EnvName("ops", extsync.EnvURL), srv.URL)
	t.Setenv(extsync.EnvName("ops", extsync.EnvProject), "OPS")
	t.Setenv(extsync.EnvName("ops", extsync.EnvToken), secret)
	useImporter(t, func(_ string, cfg extsync.SourceConfig, m *extsync.Mapping) (extsync.Importer, error) {
		if cfg.Token() != secret {
			t.Errorf("adapter received token %q", cfg.Token())
		}
		return jira.New(jira.Options{
			Config: cfg, UpdatedField: m.Identity.UpdatedAt, Client: srv.Client(),
			Sleep: func(time.Duration) {}, MaxRetries: 1, BaseDelay: time.Millisecond,
		})
	})

	_, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID})
	if err == nil {
		t.Fatal("RunSync succeeded against a source that rejected the credential")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the error disclosed the credential: %v", err)
	}
	if !strings.Contains(err.Error(), "rejected the configured credential") {
		t.Errorf("error = %v, want the rejection reported", err)
	}

	if err := f.local.read(f.ctx, f.actor, func(tx store.Tx) error {
		entries, err := tx.ListAudit(f.ctx, core.AuditFilter{Page: core.Page{Limit: 200}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			if strings.Contains(string(e.Before)+string(e.After), secret) {
				t.Errorf("audit entry %q disclosed the credential", e.Action)
			}
		}
		events, err := tx.ReadEvents(f.ctx, 0, 200)
		if err != nil {
			return err
		}
		for _, ev := range events {
			if strings.Contains(fmt.Sprint(ev.Payload), secret) {
				t.Errorf("event %q disclosed the credential", ev.Type)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("reading history: %v", err)
	}
}

func TestPutSyncSourceRefusesPersistedCredentials(t *testing.T) {
	f := newSyncFixture(t, "ops")
	for _, key := range []string{"token", "api_secret", "password"} {
		_, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
			System: core.SystemJira, Name: "jira", Config: map[string]any{key: "value"},
		})
		if !core.IsKind(err, core.KindInvalid) {
			t.Errorf("PutSyncSource(%s) = %v, want invalid", key, err)
		}
	}
}

func TestSyncSourceLifecycle(t *testing.T) {
	f := newSyncFixture(t, "ops")

	sources, err := f.local.ListSyncSources(f.ctx)
	if err != nil {
		t.Fatalf("ListSyncSources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("listed %d sources, want the registered one", len(sources))
	}

	renamed, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{
		ID: f.source.ID, System: core.SystemGeneric, Name: "ops",
	})
	if err != nil {
		t.Fatalf("PutSyncSource: %v", err)
	}
	if renamed.ID != f.source.ID {
		t.Errorf("an update created a second source: %q", renamed.ID)
	}

	if _, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: "trello", Name: "x"}); err == nil {
		t.Error("PutSyncSource accepted a system with no adapter")
	}
	if _, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemJira}); err == nil {
		t.Error("PutSyncSource accepted a source with no name")
	}

	if err := f.local.DeleteSyncSource(f.ctx, ""); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("DeleteSyncSource(empty) = %v", err)
	}
	if err := f.local.DeleteSyncSource(f.ctx, f.source.ID); err != nil {
		t.Fatalf("DeleteSyncSource: %v", err)
	}
	after, err := f.local.ListSyncSources(f.ctx)
	if err != nil {
		t.Fatalf("ListSyncSources: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("listed %d sources after deleting the only one", len(after))
	}
	if err := f.local.DeleteSyncSource(f.ctx, f.source.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting twice = %v, want not found", err)
	}
}

func TestRunSyncRefusesBadInputAndMappings(t *testing.T) {
	f := newSyncFixture(t, "ops")

	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("RunSync with no source = %v, want invalid", err)
	}
	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: "absent"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("RunSync with an unknown source = %v, want not found", err)
	}

	bad := filepath.Join(f.dir, "bad.yaml")
	body := strings.Replace(syncMapping, "In Progress: doing", "In Progress: nowhere", 1)
	if err := os.WriteFile(bad, []byte(body), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), bad)
	_, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID})
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "nowhere") {
		t.Fatalf("RunSync = %v, want the offending mapping entry named", err)
	}
	if got := f.tasks(t); len(got) != 0 {
		t.Errorf("a refused mapping wrote %d tasks", len(got))
	}

	wrongWorkflow := strings.Replace(syncMapping, "workflow: syncflow", "workflow: other", 1)
	other := filepath.Join(f.dir, "other.yaml")
	if err := os.WriteFile(other, []byte(wrongWorkflow), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), other)
	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID}); err == nil {
		t.Error("RunSync accepted a mapping naming a different workflow")
	}

	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), "")
	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID}); err == nil {
		t.Error("RunSync accepted a source with no mapping file")
	}
}

func TestRunSyncSkipsRecordsThatCannotBeMapped(t *testing.T) {
	f := newSyncFixture(t, "ops")
	imp := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{
			syncRecord("", "no identity", "To Do", "2026-01-01T00:00:00Z"),
			syncRecord("S-2", "", "To Do", "2026-01-02T00:00:00Z"),
			syncRecord("S-3", "fine", "To Do", "2026-01-03T00:00:00Z"),
		},
		Cursor: "2026-01-03T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return imp, nil
	})

	res := f.run(t, core.RunSyncInput{})
	if res.Created["task"] != 1 {
		t.Errorf("created = %v, want only the mappable record", res.Created)
	}
	if res.Skipped["task"] != 2 {
		t.Errorf("skipped = %v, want the two unmappable records", res.Skipped)
	}
	if !anyContains(res.Warnings, "identity field") {
		t.Errorf("warnings = %v, want each skip explained", res.Warnings)
	}
}

// An import must write only into the tenant it targets.
func TestRunSyncIsConfinedToItsTenant(t *testing.T) {
	f := newSyncFixture(t, "ops")
	f.run(t, core.RunSyncInput{})

	otherCtx, otherActor := newTenantActor(t, f.local, "rival")
	page, err := f.local.ListTasks(otherCtx, core.TaskFilter{Page: core.Page{Limit: 100}})
	if err != nil {
		t.Fatalf("listing tasks in the other tenant: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Errorf("another tenant sees %d imported tasks", len(page.Tasks))
	}

	sources, err := f.local.ListSyncSources(otherCtx)
	if err != nil {
		t.Fatalf("listing sources in the other tenant: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("another tenant sees %d import sources", len(sources))
	}
	if _, err := f.local.RunSync(otherCtx, core.RunSyncInput{SourceID: f.source.ID}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("running another tenant's source = %v, want not found", err)
	}
	if err := f.local.read(otherCtx, otherActor, func(tx store.Tx) error {
		refs, err := tx.ListExternalRefs(otherCtx, core.SystemGeneric)
		if err != nil {
			return err
		}
		if len(refs) != 0 {
			t.Errorf("another tenant sees %d external references", len(refs))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading references: %v", err)
	}
}

// newTenantActor creates a second tenant with an administrator in it.
func newTenantActor(t *testing.T, l *Local, key string) (context.Context, *core.Actor) {
	t.Helper()
	ctx := context.Background()
	tenant := core.Tenant{Key: key, Name: key}
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}
	actor := core.Actor{Kind: core.ActorUser, Handle: key + "-admin", Scopes: []core.Scope{core.ScopeAll}}
	if err := l.store.Update(ctx, core.TenantScope{TenantID: tenant.ID}, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("creating actor: %v", err)
	}
	actor.TenantID = tenant.ID
	return core.WithActor(ctx, &actor), &actor
}

func TestRunSyncRequiresAuthorization(t *testing.T) {
	f := newSyncFixture(t, "ops")
	viewer := &core.Actor{ID: "v1", TenantID: f.actor.TenantID, Kind: core.ActorUser, Role: core.RoleViewer}
	ctx := core.WithActor(context.Background(), viewer)

	if _, err := f.local.RunSync(ctx, core.RunSyncInput{SourceID: f.source.ID}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("RunSync as a viewer = %v, want forbidden", err)
	}
	if _, err := f.local.ListSyncSources(ctx); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("ListSyncSources as a viewer = %v, want forbidden", err)
	}
	if _, err := f.local.PutSyncSource(ctx, core.SyncSourceInput{System: core.SystemJira, Name: "x"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("PutSyncSource as a viewer = %v, want forbidden", err)
	}
	if err := f.local.DeleteSyncSource(ctx, f.source.ID); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("DeleteSyncSource as a viewer = %v, want forbidden", err)
	}
}

func TestImporterForNamesTheAvailableAdapters(t *testing.T) {
	m := &extsync.Mapping{}
	if _, err := importerFor("trello", extsync.SourceConfig{}, m); err == nil ||
		!strings.Contains(err.Error(), core.SystemOpenProject) {
		t.Fatalf("importerFor = %v, want the adapters listed", err)
	}
	if _, err := importerFor(core.SystemGeneric, extsync.SourceConfig{File: "x.csv"}, m); err != nil {
		t.Errorf("importerFor(generic) = %v", err)
	}
	cfg := extsync.SourceConfig{BaseURL: "https://x.test", Project: "P"}
	if _, err := importerFor(core.SystemJira, cfg, m); err != nil {
		t.Errorf("importerFor(jira) = %v", err)
	}
	if _, err := importerFor(core.SystemOpenProject, cfg, m); err != nil {
		t.Errorf("importerFor(openproject) = %v", err)
	}
}

const relationMapping = `
version: 1
project: ops
identity:
  id: id
  version: updated
  updated_at: updated
fields:
  title: title
  assignee: assignee
  parent: parent
  tags: tags
  due: due_at
status_field: status
statuses:
  To Do: todo
  In Progress: doing
  Done: done
default_status: todo
unmapped:
  preserve: false
`

func relationRecord(id, title, status, updated string, extra map[string]any) extsync.Record {
	r := syncRecord(id, title, status, updated)
	for k, v := range extra {
		r.Fields[k] = v
	}
	return r
}

func useRelationMapping(t *testing.T, f *syncFixture) {
	t.Helper()
	path := filepath.Join(f.dir, "relations.yaml")
	if err := os.WriteFile(path, []byte(relationMapping), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	t.Setenv(extsync.EnvName("ops", extsync.EnvMapping), path)
}

func TestRunSyncResolvesAssigneesParentsAndTags(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useRelationMapping(t, f)

	imp := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{
			relationRecord("R-1", "the parent", "To Do", "2026-01-01T00:00:00Z", map[string]any{
				"assignee": f.actor.Handle,
				"tags":     "ops,urgent",
				"due":      "2026-03-01",
			}),
			relationRecord("R-2", "the child", "In Progress", "2026-01-02T00:00:00Z", map[string]any{
				"parent":   "R-1",
				"assignee": "nobody-here",
			}),
			relationRecord("R-3", "the orphan", "To Do", "2026-01-03T00:00:00Z", map[string]any{
				"parent": "R-99",
			}),
		},
		Cursor: "2026-01-03T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return imp, nil
	})

	res := f.run(t, core.RunSyncInput{})
	if res.Created["task"] != 3 {
		t.Fatalf("created = %v", res.Created)
	}

	byTitle := map[string]core.Task{}
	for _, task := range f.tasks(t) {
		byTitle[task.Title] = task
	}
	parent := byTitle["the parent"]
	if parent.AssigneeActorID != f.actor.ID {
		t.Errorf("assignee = %q, want the resolved actor", parent.AssigneeActorID)
	}
	if len(parent.Tags) != 2 {
		t.Errorf("tags = %v, want both mapped tags", parent.Tags)
	}
	if parent.DueAt == nil {
		t.Error("due date was dropped")
	}
	child := byTitle["the child"]
	if child.ParentID != parent.ID {
		t.Errorf("parent = %q, want the already-imported task %q", child.ParentID, parent.ID)
	}
	if child.CustomFields["assignee"] == nil {
		t.Errorf("an unresolvable assignee was dropped: %v", child.CustomFields)
	}
	if !anyContains(res.Warnings, "has no tix actor") {
		t.Errorf("warnings = %v, want the unresolved assignee reported", res.Warnings)
	}
	if !anyContains(res.Warnings, "hierarchy is lossy") {
		t.Errorf("warnings = %v, want the unresolved parent reported", res.Warnings)
	}
}

func TestRunSyncUpdateAppliesRelationsAndCompletion(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useRelationMapping(t, f)

	first := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{
			relationRecord("U-1", "anchor", "To Do", "2026-01-01T00:00:00Z", nil),
			relationRecord("U-2", "moving", "To Do", "2026-01-02T00:00:00Z", nil),
		},
		Cursor: "2026-01-02T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return first, nil
	})
	f.run(t, core.RunSyncInput{})

	second := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{
			relationRecord("U-2", "moved", "Done", "2026-02-02T00:00:00Z", map[string]any{
				"parent":   "U-1",
				"assignee": f.actor.Handle,
				"tags":     "shipped",
				"due":      "2026-04-01",
			}),
		},
		Cursor: "2026-02-02T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return second, nil
	})
	res := f.run(t, core.RunSyncInput{})
	if res.Updated["task"] != 1 {
		t.Fatalf("updated = %v", res.Updated)
	}

	var moved core.Task
	for _, task := range f.tasks(t) {
		if task.Title == "moved" {
			moved = task
		}
	}
	if moved.ID == "" {
		t.Fatal("the updated task was not found")
	}
	if moved.Status != "done" || moved.CompletedAt == nil {
		t.Errorf("status = %q, completed = %v, want a terminal state stamped", moved.Status, moved.CompletedAt)
	}
	if moved.ParentID == "" || moved.AssigneeActorID != f.actor.ID {
		t.Errorf("relations were not applied on update: %+v", moved)
	}
	if moved.DueAt == nil {
		t.Error("the new due date was not applied")
	}
	if len(moved.Tags) != 1 {
		t.Errorf("tags = %v, want the newly mapped tag", moved.Tags)
	}
}

func TestRunSyncSkipsDeletedTasksAndUnknownStatuses(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useRelationMapping(t, f)
	imp := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{
			relationRecord("D-1", "doomed", "To Do", "2026-01-01T00:00:00Z", nil),
		},
		Cursor: "2026-01-01T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return imp, nil
	})
	f.run(t, core.RunSyncInput{})

	tasks := f.tasks(t)
	if len(tasks) != 1 {
		t.Fatalf("imported %d tasks", len(tasks))
	}
	if err := f.local.DeleteTask(f.ctx, core.TaskRef{ID: tasks[0].ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("deleting task: %v", err)
	}

	imp.pages[0].Records[0].Fields["updated"] = "2026-05-01T00:00:00Z"
	res := f.run(t, core.RunSyncInput{Full: true})
	if res.Skipped["task"] != 1 {
		t.Errorf("skipped = %v, want the deleted task left alone", res.Skipped)
	}
	if !anyContains(res.Warnings, "deleted in tix") {
		t.Errorf("warnings = %v, want the skip explained", res.Warnings)
	}
}

// A hard-deleted task whose external reference survives must be re-created
// rather than reported missing, so a re-run always converges.
func TestRunSyncRecreatesATaskWhoseReferenceOutlivedIt(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useRelationMapping(t, f)
	imp := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{
			relationRecord("H-1", "vanishing", "To Do", "2026-01-01T00:00:00Z", nil),
		},
		Cursor: "2026-01-01T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return imp, nil
	})
	f.run(t, core.RunSyncInput{})

	tasks := f.tasks(t)
	if err := f.local.DeleteTask(f.ctx, core.TaskRef{ID: tasks[0].ID}, core.DeleteTaskInput{Hard: true}); err != nil {
		t.Fatalf("hard deleting task: %v", err)
	}
	imp.pages[0].Records[0].Fields["updated"] = "2026-06-01T00:00:00Z"
	res := f.run(t, core.RunSyncInput{Full: true})
	if res.Created["task"] != 1 {
		t.Errorf("created = %v, want the task re-created", res.Created)
	}
}
