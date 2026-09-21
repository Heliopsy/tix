package service

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	extsync "github.com/heliopsy/tix/internal/sync"
	"github.com/heliopsy/tix/internal/sync/generic"
	"github.com/heliopsy/tix/internal/sync/jira"
	"github.com/heliopsy/tix/internal/sync/openproject"
)

// Event types for external sync.
const (
	eventSyncSourcePut     core.EventType = "sync.source.put"
	eventSyncSourceDeleted core.EventType = "sync.source.deleted"
	eventSyncRun           core.EventType = core.EventImportCompleted
)

// Audit actions for external sync.
const (
	auditSyncSourcePut    = "sync.source.put"
	auditSyncSourceDelete = "sync.source.delete"
	auditSyncRun          = "sync.run"
	auditSyncTaskCreate   = "sync.task.create"
	auditSyncTaskUpdate   = "sync.task.update"
)

// syncSourceSubject names import sources in the audit log.
const syncSourceSubject = "sync_source"

// syncEntityTask is the entity type external references are recorded under.
const syncEntityTask = "task"

// systemHandle is the handle of the actor imports are attributed to.
const systemHandle = "system"

// maxSyncPages bounds one run, so a source that keeps offering a next page
// cannot spin forever.
const maxSyncPages = 10000

// syncEnv reads external source configuration. Credentials live only here.
var syncEnv = os.Getenv

// importerFor builds the adapter a source's system names. An unknown system is
// refused with the list of adapters this build carries.
func importerFor(system string, cfg extsync.SourceConfig, m *extsync.Mapping) (extsync.Importer, error) {
	switch system {
	case core.SystemGeneric:
		return generic.New(generic.Options{
			Path:         cfg.File,
			UpdatedField: m.Identity.UpdatedAt,
			PageSize:     cfg.PageSize,
		})
	case core.SystemJira:
		return jira.New(jira.Options{Config: cfg, UpdatedField: m.Identity.UpdatedAt})
	case core.SystemOpenProject:
		return openproject.New(openproject.Options{Config: cfg, UpdatedField: m.Identity.UpdatedAt})
	default:
		return nil, core.Invalid("system %q has no import adapter; this build carries %q, %q and %q",
			system, core.SystemGeneric, core.SystemJira, core.SystemOpenProject)
	}
}

// syncImporters lets a test drive RunSync against a recorded source without
// reaching the network. A nil factory uses the configured adapter.
var syncImporters func(system string, cfg extsync.SourceConfig, m *extsync.Mapping) (extsync.Importer, error)

func buildImporter(system string, cfg extsync.SourceConfig, m *extsync.Mapping) (extsync.Importer, error) {
	if syncImporters != nil {
		return syncImporters(system, cfg, m)
	}
	return importerFor(system, cfg, m)
}

// PutSyncSource registers an external import source or updates one in place.
//
// Nothing a source needs to authenticate is stored: credentials, endpoints and
// the mapping file are read from the environment at run time, so no credential
// can reach a snapshot, an audit entry or an event.
func (l *Local) PutSyncSource(ctx context.Context, in core.SyncSourceInput) (*core.SyncSource, error) {
	actor, err := l.authorize(ctx, authz.ActionSyncAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if err := refuseCredentialKeys(in.Config); err != nil {
		return nil, err
	}

	var out core.SyncSource
	err = l.write(ctx, actor, func(m *mutation) error {
		var before any
		if in.ID != "" {
			existing, err := m.tx.GetSyncSource(ctx, in.ID)
			if err != nil {
				return err
			}
			before = *existing
		}
		src := core.SyncSource{ID: in.ID, System: in.System, Name: in.Name}
		if err := m.tx.PutSyncSource(ctx, &src); err != nil {
			return err
		}
		out = src
		return m.Record(auditSyncSourcePut, eventSyncSourcePut, syncSourceSubject, src.ID, "",
			before, src, map[string]any{"system": src.System, "name": src.Name})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// refuseCredentialKeys rejects a registration that tries to persist a secret,
// rather than storing it and redacting it later.
func refuseCredentialKeys(config map[string]any) error {
	for _, key := range sortedMapKeys(config) {
		if isSecretKey(key) {
			return core.Invalid("sync source configuration must not carry %q; credentials are read from %s%s_* at run time",
				key, extsync.EnvPrefix, "<SOURCE>")
		}
	}
	return nil
}

func sortedMapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ListSyncSources returns this tenant's import sources.
func (l *Local) ListSyncSources(ctx context.Context) ([]core.SyncSource, error) {
	actor, err := l.authorize(ctx, authz.ActionSyncAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := []core.SyncSource{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		rows, err := tx.ListSyncSources(ctx)
		if err != nil {
			return err
		}
		out = rows
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteSyncSource removes an import source and its cursor.
func (l *Local) DeleteSyncSource(ctx context.Context, sourceID string) error {
	actor, err := l.authorize(ctx, authz.ActionSyncAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	if strings.TrimSpace(sourceID) == "" {
		return core.Invalid("sync source identifier is required")
	}
	return l.write(ctx, actor, func(m *mutation) error {
		existing, err := m.tx.GetSyncSource(ctx, sourceID)
		if err != nil {
			return err
		}
		if err := m.tx.DeleteSyncSource(ctx, sourceID); err != nil {
			return err
		}
		return m.Record(auditSyncSourceDelete, eventSyncSourceDeleted, syncSourceSubject, sourceID, "",
			*existing, nil, map[string]any{"system": existing.System, "name": existing.Name})
	})
}

// syncRun is the state of one import, shared by the dry run and the real one.
type syncRun struct {
	local     *Local
	system    *core.Actor
	source    core.SyncSource
	mapping   *extsync.Mapping
	project   core.Project
	workflow  core.WorkflowDefinition
	dryRun    bool
	full      bool
	cursor    string
	result    *core.SyncResult
	warned    map[string]bool
	processed int
}

// syncDecision is what an import has decided to do with one external record.
type syncDecision struct {
	action   string
	reason   string
	existing *core.Task
}

// Decisions an import can reach for one external record.
const (
	syncCreate = "create"
	syncUpdate = "update"
	syncSkip   = "skip"
)

// RunSync imports from a configured source, creating what is new, updating
// what changed at the source, and skipping what did not. A run is idempotent:
// every entity it touches carries an external reference, so a second run
// refreshes rather than duplicating.
func (l *Local) RunSync(ctx context.Context, in core.RunSyncInput) (*core.SyncResult, error) {
	actor, err := l.authorize(ctx, authz.ActionSyncAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.SourceID) == "" {
		return nil, core.Invalid("sync source identifier is required")
	}

	var source core.SyncSource
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := tx.GetSyncSource(ctx, in.SourceID)
		if err != nil {
			return err
		}
		source = *found
		return nil
	}); err != nil {
		return nil, err
	}

	cfg, err := extsync.LoadSourceConfig(source.Name, syncEnv)
	if err != nil {
		return nil, err
	}
	mapping, err := extsync.LoadMapping(cfg.MappingPath)
	if err != nil {
		return nil, err
	}

	run := &syncRun{
		local:   l,
		source:  source,
		mapping: mapping,
		dryRun:  in.DryRun,
		full:    in.Full,
		cursor:  source.Cursor,
		warned:  map[string]bool{},
		result: &core.SyncResult{
			ImportResult: core.ImportResult{
				Created: map[string]int{}, Updated: map[string]int{},
				Skipped: map[string]int{}, DryRun: in.DryRun,
			},
			System: source.System,
			Source: source.Name,
		},
	}
	if in.Full {
		run.cursor = ""
	}
	if err := run.prepare(ctx, actor); err != nil {
		return nil, err
	}

	importer, err := buildImporter(source.System, cfg, mapping)
	if err != nil {
		return nil, err
	}
	if err := run.pull(ctx, importer); err != nil {
		return nil, err
	}
	run.result.Cursor = run.cursor
	return run.result, nil
}

// prepare resolves the target project and workflow, refuses a mapping the
// workflow cannot satisfy, and makes sure the mapped custom fields exist before
// a single record is read.
func (r *syncRun) prepare(ctx context.Context, actor *core.Actor) error {
	if err := r.local.read(ctx, actor, func(tx store.Tx) error {
		project, err := tx.GetProject(ctx, r.mapping.Project)
		if err != nil {
			return err
		}
		wf, err := tx.GetWorkflowByID(ctx, project.WorkflowID)
		if err != nil {
			return err
		}
		if r.mapping.Workflow != "" && r.mapping.Workflow != wf.Key {
			return core.Invalid("mapping names workflow %q but project %q uses workflow %q",
				r.mapping.Workflow, project.Key, wf.Key)
		}
		r.project, r.workflow = *project, wf.Definition
		return nil
	}); err != nil {
		return err
	}
	if err := r.mapping.ValidateAgainst(r.workflow); err != nil {
		return err
	}
	if r.dryRun {
		return nil
	}
	return r.local.write(systemContext(ctx), actor, func(m *mutation) error {
		system, err := ensureSystemActor(ctx, m.tx)
		if err != nil {
			return err
		}
		r.system = system
		return r.ensureFieldDefs(ctx, m)
	})
}

// systemContext attributes what follows to the system rather than to the
// operator who started the import.
func systemContext(ctx context.Context) context.Context {
	return core.WithSource(ctx, core.SourceSystem)
}

// ensureSystemActor returns the tenant's system actor, creating it on first
// import so imported rows have an author that is not the operator.
func ensureSystemActor(ctx context.Context, tx store.Tx) (*core.Actor, error) {
	existing, err := tx.GetActorByHandle(ctx, systemHandle)
	if err == nil {
		actor := core.SystemActor(existing.TenantID)
		actor.ID = existing.ID
		return actor, nil
	}
	if !core.IsKind(err, core.KindNotFound) {
		return nil, err
	}
	created := core.Actor{Kind: core.ActorSystem, Handle: systemHandle, DisplayName: "System"}
	if err := tx.CreateActor(ctx, &created); err != nil {
		return nil, err
	}
	actor := core.SystemActor(created.TenantID)
	actor.ID = created.ID
	return actor, nil
}

// ensureFieldDefs creates the custom field definitions the mapping names, so
// imported values are native fields rather than opaque payload.
func (r *syncRun) ensureFieldDefs(ctx context.Context, m *mutation) error {
	if len(r.mapping.Custom) == 0 {
		return nil
	}
	defs, err := m.tx.ListFieldDefs(ctx, r.project.ID)
	if err != nil {
		return err
	}
	have := make(map[string]bool, len(defs))
	for _, d := range defs {
		have[d.Key] = true
	}
	for _, ext := range mappingCustomOrder(r.mapping) {
		cf := r.mapping.Custom[ext]
		if have[cf.Key] {
			continue
		}
		label := cf.Label
		if label == "" {
			label = cf.Key
		}
		def := core.FieldDef{
			ProjectID: r.project.ID, Key: cf.Key, Label: label, Type: cf.FieldType(),
		}
		if err := m.tx.PutFieldDef(ctx, &def); err != nil {
			return err
		}
		if err := m.Record("field.put", core.EventFieldUpdated, "field_def", def.ID, r.project.ID,
			nil, def, map[string]any{"key": def.Key, "type": def.Type}); err != nil {
			return err
		}
		have[cf.Key] = true
	}
	return nil
}

func mappingCustomOrder(m *extsync.Mapping) []string {
	out := make([]string, 0, len(m.Custom))
	for k := range m.Custom {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// pull walks every page the source offers, committing each page and its cursor
// together so an interrupted run resumes instead of restarting.
func (r *syncRun) pull(ctx context.Context, importer extsync.Importer) error {
	opt := extsync.Options{Cursor: r.cursor, Full: r.full}
	for page := 0; page < maxSyncPages; page++ {
		batch, err := importer.Fetch(ctx, opt)
		if err != nil {
			return r.fail(ctx, err)
		}
		if err := r.consume(ctx, batch); err != nil {
			return r.fail(ctx, err)
		}
		if batch.Page == "" {
			return r.finish(ctx)
		}
		opt.Page = batch.Page
	}
	return r.fail(ctx, core.Internal("source %q offered more than %d pages", r.source.Name, maxSyncPages))
}

// consume applies one page, in one transaction, together with its cursor.
func (r *syncRun) consume(ctx context.Context, batch extsync.Batch) error {
	if r.dryRun {
		return r.local.read(ctx, r.operator(), func(tx store.Tx) error {
			for _, rec := range batch.Records {
				if err := r.planOne(ctx, tx, rec); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return r.local.write(systemContext(ctx), r.system, func(m *mutation) error {
		for _, rec := range batch.Records {
			if err := r.applyOne(ctx, m, rec); err != nil {
				return err
			}
		}
		return r.advance(ctx, m, batch.Cursor, "running")
	})
}

// operator is the identity a dry run reads with; it writes nothing.
func (r *syncRun) operator() *core.Actor {
	if r.system != nil {
		return r.system
	}
	return core.SystemActor(r.project.TenantID)
}

// advance moves the stored cursor forward, which only ever happens in the same
// transaction as the records the cursor covers.
func (r *syncRun) advance(ctx context.Context, m *mutation, cursor, status string) error {
	if cursor != "" {
		r.cursor = cursor
	}
	now := m.now
	src := core.SyncSource{
		ID: r.source.ID, System: r.source.System, Name: r.source.Name,
		Cursor: r.cursor, LastRunAt: &now, LastStatus: status,
	}
	return m.tx.PutSyncSource(ctx, &src)
}

// finish records the run's outcome and emits the completion event.
func (r *syncRun) finish(ctx context.Context) error {
	if r.dryRun {
		return nil
	}
	return r.local.write(systemContext(ctx), r.system, func(m *mutation) error {
		if err := r.advance(ctx, m, "", "ok"); err != nil {
			return err
		}
		return m.Record(auditSyncRun, eventSyncRun, syncSourceSubject, r.source.ID, r.project.ID,
			nil, r.result, map[string]any{
				"system": r.source.System, "source": r.source.Name,
				"created": r.result.Created[syncEntityTask],
				"updated": r.result.Updated[syncEntityTask],
				"skipped": r.result.Skipped[syncEntityTask],
			})
	})
}

// fail reports how far the run got without disclosing anything the adapter was
// configured with.
func (r *syncRun) fail(ctx context.Context, cause error) error {
	if !r.dryRun && r.system != nil {
		_ = r.local.write(systemContext(ctx), r.system, func(m *mutation) error {
			return r.advance(ctx, m, "", "failed")
		})
	}
	return core.Internal("import from %q stopped after %d records: %s",
		r.source.Name, r.processed, messageOf(cause)).Wrap(cause)
}

// messageOf returns an error's message without its wrapped chain, so a cause
// carrying a url or a header cannot be re-rendered into a new error.
func messageOf(err error) string {
	var domain *core.Error
	if errors.As(err, &domain) {
		return domain.Message
	}
	return "the source could not be read"
}

// planOne reports what a record would do without writing anything.
func (r *syncRun) planOne(ctx context.Context, tx store.Tx, rec extsync.Record) error {
	mapped, err := r.mapping.Apply(rec)
	if err != nil {
		r.skip(mapped.ExternalID, err.Error())
		return nil
	}
	r.processed++
	r.note(mapped.Warnings)

	decision, err := r.decide(ctx, tx, mapped)
	if err != nil {
		return err
	}
	switch decision.action {
	case syncCreate:
		r.result.Created[syncEntityTask]++
	case syncUpdate:
		r.result.Updated[syncEntityTask]++
	default:
		r.skip(mapped.ExternalID, decision.reason)
	}
	return nil
}

// applyOne creates or updates one task and records its external reference.
func (r *syncRun) applyOne(ctx context.Context, m *mutation, rec extsync.Record) error {
	mapped, err := r.mapping.Apply(rec)
	if err != nil {
		r.skip(mapped.ExternalID, err.Error())
		return nil
	}
	r.processed++
	r.note(mapped.Warnings)

	decision, err := r.decide(ctx, m.tx, mapped)
	if err != nil {
		return err
	}
	if decision.action == syncSkip {
		r.skip(mapped.ExternalID, decision.reason)
		return nil
	}

	fields, err := r.resolveFields(ctx, m.tx, &mapped)
	if err != nil {
		return err
	}
	if decision.action == syncUpdate {
		cyclic, err := closesParentCycle(ctx, decision.existing.ID, fields.parent, storeParentOf(m.tx))
		if err != nil {
			return err
		}
		if cyclic {
			r.skip(mapped.ExternalID, "parent "+mapped.ExternalParentID+" would close a parent cycle")
			return nil
		}
	}
	var task *core.Task
	if decision.action == syncCreate {
		task, err = r.create(ctx, m, mapped, fields)
	} else {
		task, err = r.update(ctx, m, *decision.existing, mapped, fields)
	}
	if err != nil {
		return err
	}
	return m.tx.PutExternalRef(ctx, &core.ExternalRef{
		EntityType:      syncEntityTask,
		EntityID:        task.ID,
		System:          r.source.System,
		ExternalID:      mapped.ExternalID,
		ExternalURL:     mapped.ExternalURL,
		ExternalVersion: mapped.ExternalVersion,
		LastSyncedAt:    m.now,
	})
}

// decide classifies a record against the external reference already stored for
// it, which is what makes a re-import an update rather than a duplicate.
func (r *syncRun) decide(ctx context.Context, tx store.Tx, mapped extsync.Mapped) (syncDecision, error) {
	ref, err := tx.GetExternalRef(ctx, r.source.System, mapped.ExternalID, syncEntityTask)
	if core.IsKind(err, core.KindNotFound) {
		return syncDecision{action: syncCreate}, nil
	}
	if err != nil {
		return syncDecision{}, err
	}
	existing, err := tx.GetTask(ctx, core.TaskRef{ID: ref.EntityID})
	if core.IsKind(err, core.KindNotFound) {
		return syncDecision{action: syncCreate}, nil
	}
	if err != nil {
		return syncDecision{}, err
	}
	if existing.Deleted() {
		return syncDecision{action: syncSkip, reason: "the imported task is deleted in tix"}, nil
	}
	if mapped.ExternalVersion != "" && ref.ExternalVersion == mapped.ExternalVersion {
		return syncDecision{action: syncSkip, existing: existing,
			reason: "external version " + mapped.ExternalVersion + " matches the stored one"}, nil
	}
	return syncDecision{action: syncUpdate, existing: existing}, nil
}

// resolveFields turns external actor and parent identifiers into tix ones,
// preserving what cannot be resolved rather than dropping it.
func (r *syncRun) resolveFields(ctx context.Context, tx store.Tx, mapped *extsync.Mapped) (syncFields, error) {
	out := syncFields{custom: map[string]any{}}
	for k, v := range mapped.CustomFields {
		out.custom[k] = v
	}
	if mapped.Assignee != "" {
		actor, err := tx.GetActorByHandle(ctx, mapped.Assignee)
		switch {
		case err == nil:
			out.assignee = actor.ID
		case core.IsKind(err, core.KindNotFound):
			out.custom[r.mapping.Unmapped.Prefix+"assignee"] = mapped.Assignee
			r.warn("assignee " + mapped.Assignee + " has no tix actor; preserved as a custom field")
		default:
			return out, err
		}
	}
	if mapped.ExternalParentID != "" {
		ref, err := tx.GetExternalRef(ctx, r.source.System, mapped.ExternalParentID, syncEntityTask)
		switch {
		case err == nil:
			out.parent = ref.EntityID
		case core.IsKind(err, core.KindNotFound):
			r.warn("parent " + mapped.ExternalParentID + " has not been imported; hierarchy is lossy for this run")
		default:
			return out, err
		}
	}
	return out, nil
}

// syncFields holds the parts of a record that needed a store lookup.
type syncFields struct {
	assignee string
	parent   string
	custom   map[string]any
}

func (r *syncRun) create(ctx context.Context, m *mutation, mapped extsync.Mapped, fields syncFields) (*core.Task, error) {
	task := &core.Task{
		ProjectID:       r.project.ID,
		ParentID:        fields.parent,
		Title:           mapped.Title,
		Body:            mapped.Body,
		Status:          r.statusOf(mapped.Status),
		Priority:        mapped.Priority,
		AssigneeActorID: fields.assignee,
		CreatorActorID:  m.actor.ID,
		DueAt:           mapped.DueAt,
		CustomFields:    fields.custom,
	}
	if r.workflow.IsTerminal(task.Status) {
		completed := m.now
		task.CompletedAt = &completed
	}
	if err := m.tx.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	if err := r.attachTags(ctx, m, task.ID, mapped.Tags); err != nil {
		return nil, err
	}
	r.result.Created[syncEntityTask]++
	if err := m.Record(auditSyncTaskCreate, core.EventTaskCreated, syncEntityTask, task.ID, r.project.ID,
		nil, task, r.payload(mapped, task)); err != nil {
		return nil, err
	}
	return task, nil
}

func (r *syncRun) update(ctx context.Context, m *mutation, existing core.Task, mapped extsync.Mapped, fields syncFields) (*core.Task, error) {
	before := existing
	updated := existing
	updated.Title = mapped.Title
	updated.Body = mapped.Body
	updated.Status = r.statusOf(mapped.Status)
	if mapped.Priority != 0 {
		updated.Priority = mapped.Priority
	}
	if fields.assignee != "" {
		updated.AssigneeActorID = fields.assignee
	}
	if fields.parent != "" {
		updated.ParentID = fields.parent
	}
	if mapped.DueAt != nil {
		updated.DueAt = mapped.DueAt
	}
	merged := map[string]any{}
	for k, v := range existing.CustomFields {
		merged[k] = v
	}
	for k, v := range fields.custom {
		merged[k] = v
	}
	updated.CustomFields = merged
	if r.workflow.IsTerminal(updated.Status) && updated.CompletedAt == nil {
		completed := m.now
		updated.CompletedAt = &completed
	}
	if err := m.tx.UpdateTask(ctx, &updated); err != nil {
		return nil, err
	}
	if err := r.attachTags(ctx, m, updated.ID, mapped.Tags); err != nil {
		return nil, err
	}
	r.result.Updated[syncEntityTask]++
	if err := m.Record(auditSyncTaskUpdate, core.EventTaskUpdated, syncEntityTask, updated.ID, r.project.ID,
		before, updated, r.payload(mapped, &updated)); err != nil {
		return nil, err
	}
	return &updated, nil
}

// payload describes an imported change to subscribers. It carries the link back
// to the source record and nothing the source was reached with.
func (r *syncRun) payload(mapped extsync.Mapped, task *core.Task) map[string]any {
	return map[string]any{
		"ref": task.Ref, "title": task.Title, "status": task.Status,
		"system": r.source.System, "external_id": mapped.ExternalID,
		"external_url": mapped.ExternalURL,
	}
}

func (r *syncRun) attachTags(ctx context.Context, m *mutation, taskID string, tags []string) error {
	for _, name := range tags {
		if strings.TrimSpace(name) == "" {
			continue
		}
		if _, err := attachTaskLabel(ctx, m.tx, taskID, name); err != nil {
			return err
		}
	}
	return nil
}

// statusOf falls back to the workflow's initial state when a mapping left a
// record without one.
func (r *syncRun) statusOf(status string) string {
	if status != "" && r.workflow.HasState(status) {
		return status
	}
	return r.workflow.Initial
}

func (r *syncRun) skip(externalID, reason string) {
	r.result.Skipped[syncEntityTask]++
	if externalID == "" {
		externalID = "(unidentified record)"
	}
	r.warn("skipped " + externalID + ": " + reason)
}

func (r *syncRun) note(warnings []string) {
	for _, w := range warnings {
		r.warn(w)
	}
}

// maxSyncWarnings bounds the report, so one broken mapping cannot turn a large
// import into an unreadable result.
const maxSyncWarnings = 200

func (r *syncRun) warn(message string) {
	if r.warned[message] {
		return
	}
	r.warned[message] = true
	if len(r.result.Warnings) >= maxSyncWarnings {
		return
	}
	r.result.Warnings = append(r.result.Warnings, message)
}
