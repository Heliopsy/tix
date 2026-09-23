// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/bundle"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// auditBundleImport names the audit action each imported component records.
const auditBundleImport = "bundle.import"

// bundleRenameLimit bounds the search for a free key when renaming.
const bundleRenameLimit = 100

// bundleReadActions names the permission exporting each component kind needs.
var bundleReadActions = map[core.ComponentKind]authz.Action{
	core.ComponentWorkflow: authz.ActionWorkflowRead,
	core.ComponentFieldDef: authz.ActionProjectRead,
	core.ComponentTag:      authz.ActionTaskRead,
	core.ComponentProject:  authz.ActionProjectRead,
	core.ComponentWebhook:  authz.ActionWebhookAdmin,
}

// bundleWriteActions names the permission creating each component kind needs.
var bundleWriteActions = map[core.ComponentKind]authz.Action{
	core.ComponentWorkflow: authz.ActionWorkflowWrite,
	core.ComponentFieldDef: authz.ActionFieldWrite,
	core.ComponentTag:      authz.ActionTaskUpdate,
	core.ComponentProject:  authz.ActionProjectWrite,
	core.ComponentWebhook:  authz.ActionWebhookAdmin,
}

// bundleEvents names the event each imported component kind emits.
var bundleEvents = map[core.ComponentKind]core.EventType{
	core.ComponentWorkflow: core.EventWorkflowUpdated,
	core.ComponentFieldDef: core.EventFieldUpdated,
	core.ComponentTag:      core.EventLabelAdded,
	core.ComponentProject:  core.EventProjectCreated,
	core.ComponentWebhook:  eventWebhookPut,
}

// bundleSubjects names each component kind in the audit log.
var bundleSubjects = map[core.ComponentKind]string{
	core.ComponentWorkflow: "workflow",
	core.ComponentFieldDef: "field_def",
	core.ComponentTag:      "tag",
	core.ComponentProject:  "project",
	core.ComponentWebhook:  webhookSubject,
}

// ExportBundle writes the selected components to w as it walks them.
func (l *Local) ExportBundle(ctx context.Context, in core.BundleExportInput, w io.Writer) error {
	if err := in.Validate(); err != nil {
		return err
	}
	kinds := bundleExportKinds(in)
	actor, err := l.authorizeComponentKinds(ctx, kinds, bundleReadActions)
	if err != nil {
		return err
	}
	enc := bundle.NewEncoder(w)
	if err := enc.Header(in.Name, l.clock.Now()); err != nil {
		return err
	}
	return l.read(ctx, actor, func(tx store.Tx) error {
		projects, err := bundleProjects(ctx, tx, in.ProjectRefs)
		if err != nil {
			return err
		}
		for _, kind := range kinds {
			comps, err := exportComponents(ctx, tx, in, kind, projects)
			if err != nil {
				return err
			}
			bundle.Sort(comps)
			for _, c := range comps {
				if err := enc.Component(c); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// bundleExportKinds resolves the selection to component kinds in bundle order.
func bundleExportKinds(in core.BundleExportInput) []core.ComponentKind {
	wanted := map[core.ComponentKind]bool{}
	switch {
	case len(in.Kinds) > 0:
		for _, k := range in.Kinds {
			wanted[k] = true
		}
	case len(in.WorkflowKeys) > 0 || len(in.ProjectRefs) > 0 || len(in.WebhookIDs) > 0:
		wanted[core.ComponentWorkflow] = len(in.WorkflowKeys) > 0
		wanted[core.ComponentProject] = len(in.ProjectRefs) > 0
		wanted[core.ComponentWebhook] = len(in.WebhookIDs) > 0
	default:
		return core.ComponentKinds
	}
	var out []core.ComponentKind
	for _, k := range core.ComponentKinds {
		if wanted[k] {
			out = append(out, k)
		}
	}
	return out
}

// exportComponents collects one kind of component from the tenant.
func exportComponents(ctx context.Context, tx store.Tx, in core.BundleExportInput, kind core.ComponentKind, projects []core.Project) ([]bundle.Component, error) {
	switch kind {
	case core.ComponentWorkflow:
		return exportWorkflows(ctx, tx, in.WorkflowKeys)
	case core.ComponentFieldDef:
		return exportFields(ctx, tx, projects)
	case core.ComponentTag:
		return exportTags(ctx, tx)
	case core.ComponentProject:
		return exportTemplates(ctx, tx, projects)
	default:
		return exportWebhooks(ctx, tx, in.WebhookIDs)
	}
}

// bundleProjects resolves the selected projects, or every project.
func bundleProjects(ctx context.Context, tx store.Tx, refs []string) ([]core.Project, error) {
	return exportProjects(ctx, tx, refs)
}

// exportWorkflows collects the selected workflows, or every workflow.
func exportWorkflows(ctx context.Context, tx store.Tx, keys []string) ([]bundle.Component, error) {
	var flows []core.Workflow
	if len(keys) == 0 {
		all, err := tx.ListWorkflows(ctx)
		if err != nil {
			return nil, err
		}
		flows = all
	}
	for _, key := range keys {
		wf, err := tx.GetWorkflow(ctx, strings.ToLower(strings.TrimSpace(key)))
		if err != nil {
			return nil, err
		}
		flows = append(flows, *wf)
	}
	out := make([]bundle.Component, 0, len(flows))
	for _, wf := range flows {
		out = append(out, bundle.Component{Kind: core.ComponentWorkflow, Workflow: bundleWorkflow(wf)})
	}
	return out, nil
}

// bundleWorkflow strips a workflow down to what is shareable about it.
func bundleWorkflow(wf core.Workflow) *bundle.Workflow {
	return &bundle.Workflow{Key: wf.Key, Name: wf.Name, Definition: wf.Definition}
}

// exportFields collects the custom field definitions of the given projects,
// keeping the first definition of each key.
func exportFields(ctx context.Context, tx store.Tx, projects []core.Project) ([]bundle.Component, error) {
	seen := map[string]bool{}
	var out []bundle.Component
	for _, p := range projects {
		defs, err := tx.ListFieldDefs(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		for _, d := range defs {
			if seen[d.Key] {
				continue
			}
			seen[d.Key] = true
			out = append(out, bundle.Component{Kind: core.ComponentFieldDef, Field: bundleField(d)})
		}
	}
	return out, nil
}

// bundleField strips a field definition of its identifiers.
func bundleField(d core.FieldDef) *bundle.Field {
	return &bundle.Field{
		Key: d.Key, Label: d.Label, Type: d.Type, Required: d.Required,
		EnumOptions: d.EnumOptions, Default: d.Default, Indexed: d.Indexed, Position: d.Position,
	}
}

// exportTags collects the tenant's tag vocabulary. A tag scoped to one project
// is left behind, because it describes that project's work rather than a way of
// working.
func exportTags(ctx context.Context, tx store.Tx) ([]bundle.Component, error) {
	tags, err := tenantTags(ctx, tx)
	if err != nil {
		return nil, err
	}
	out := make([]bundle.Component, 0, len(tags))
	for _, t := range tags {
		out = append(out, bundle.Component{Kind: core.ComponentTag, Tag: &bundle.Tag{Name: t.Name, Color: t.Color}})
	}
	return out, nil
}

// tenantTags returns the tags that belong to the tenant rather than a project.
func tenantTags(ctx context.Context, tx store.Tx) ([]core.Tag, error) {
	tags, err := tx.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	var out []core.Tag
	for _, t := range tags {
		if t.ProjectID == "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// exportTemplates turns each project into a template: its workflow, field
// definitions, tags and settings, and none of its tasks.
func exportTemplates(ctx context.Context, tx store.Tx, projects []core.Project) ([]bundle.Component, error) {
	tags, err := tenantTags(ctx, tx)
	if err != nil {
		return nil, err
	}
	out := make([]bundle.Component, 0, len(projects))
	for _, p := range projects {
		tpl := &bundle.Project{
			Key: p.Key, Name: p.Name, Description: p.Description,
			Color: p.Color, Icon: p.Icon,
		}
		wf, err := tx.GetWorkflowByID(ctx, p.WorkflowID)
		if err != nil {
			return nil, err
		}
		tpl.Workflow = bundleWorkflow(*wf)
		defs, err := tx.ListFieldDefs(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		for _, d := range defs {
			tpl.Fields = append(tpl.Fields, *bundleField(d))
		}
		for _, t := range tags {
			tpl.Tags = append(tpl.Tags, bundle.Tag{Name: t.Name, Color: t.Color})
		}
		out = append(out, bundle.Component{Kind: core.ComponentProject, Project: tpl})
	}
	return out, nil
}

// exportWebhooks collects endpoint definitions without their signing secrets.
func exportWebhooks(ctx context.Context, tx store.Tx, ids []string) ([]bundle.Component, error) {
	var endpoints []core.WebhookEndpoint
	if len(ids) == 0 {
		all, err := tx.ListWebhooks(ctx)
		if err != nil {
			return nil, err
		}
		endpoints = all
	}
	for _, id := range ids {
		e, err := tx.GetWebhook(ctx, id)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, *e)
	}
	out := make([]bundle.Component, 0, len(endpoints))
	for _, e := range endpoints {
		out = append(out, bundle.Component{Kind: core.ComponentWebhook, Webhook: &bundle.Webhook{
			URL: e.URL, EventTypes: e.EventTypes,
		}})
	}
	return out, nil
}

// authorizeComponentKinds checks the caller may act on every kind involved.
func (l *Local) authorizeComponentKinds(ctx context.Context, kinds []core.ComponentKind, actions map[core.ComponentKind]authz.Action) (*core.Actor, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	for _, k := range kinds {
		action, ok := actions[k]
		if !ok {
			return nil, core.Invalid("component kind %q is not one this build can share", k)
		}
		if _, err := l.authorize(ctx, action, authz.Resource{}); err != nil {
			return nil, err
		}
	}
	return actor, nil
}

// ImportBundle reads a bundle and applies it atomically. With Preview set it
// reports what it would do and writes nothing at all.
func (l *Local) ImportBundle(ctx context.Context, r io.Reader, in core.BundleImportInput) (*core.BundleResult, error) {
	if _, err := core.RequireActor(ctx); err != nil {
		return nil, err
	}
	header, comps, err := bundle.Read(r)
	if err != nil {
		return nil, err
	}
	actor, err := l.authorizeComponentKinds(ctx, importKinds(comps), bundleWriteActions)
	if err != nil {
		return nil, err
	}
	if err := checkFieldTarget(comps, in); err != nil {
		return nil, err
	}
	if in.OnCollision == "" {
		return nil, l.refuseUndecidedCollision(ctx, actor, comps, in)
	}

	imp := &bundleImport{
		local: l, in: in, comps: comps,
		result: &core.BundleResult{
			BundleName:    header.Name,
			BundleVersion: header.Version,
			Outcomes:      []core.ComponentOutcome{},
			Preview:       in.Preview,
		},
		workflows: map[string]string{},
		tags:      map[string]bool{},
	}
	if in.Preview {
		if err := l.read(ctx, actor, func(tx store.Tx) error { return imp.run(ctx, tx, nil) }); err != nil {
			return nil, err
		}
		return imp.result, nil
	}
	if err := l.write(ctx, actor, func(m *mutation) error { return imp.run(ctx, m.tx, m) }); err != nil {
		return nil, err
	}
	return imp.result, nil
}

// importKinds lists the kinds a bundle touches, including the kinds a project
// template carries inside itself.
func importKinds(comps []bundle.Component) []core.ComponentKind {
	wanted := map[core.ComponentKind]bool{}
	for _, c := range comps {
		wanted[c.Kind] = true
		if c.Kind != core.ComponentProject {
			continue
		}
		wanted[core.ComponentWorkflow] = wanted[core.ComponentWorkflow] || c.Project.Workflow != nil
		wanted[core.ComponentFieldDef] = wanted[core.ComponentFieldDef] || len(c.Project.Fields) > 0
		wanted[core.ComponentTag] = wanted[core.ComponentTag] || len(c.Project.Tags) > 0
	}
	var out []core.ComponentKind
	for _, k := range core.ComponentKinds {
		if wanted[k] {
			out = append(out, k)
		}
	}
	return out
}

// checkFieldTarget refuses a bundle whose loose field definitions have nowhere
// to land, before anything is written.
func checkFieldTarget(comps []bundle.Component, in core.BundleImportInput) error {
	if strings.TrimSpace(in.ProjectRef) != "" {
		return nil
	}
	for _, c := range comps {
		if c.Kind == core.ComponentFieldDef {
			return core.Invalid("field definition %q needs a target project; supply one to import it", c.Key())
		}
	}
	return nil
}

// refuseUndecidedCollision names the component that forces a choice, so the
// caller learns what is at stake rather than only that a policy is missing.
func (l *Local) refuseUndecidedCollision(ctx context.Context, actor *core.Actor, comps []bundle.Component, in core.BundleImportInput) error {
	var collision *bundle.Component
	err := l.read(ctx, actor, func(tx store.Tx) error {
		for i := range comps {
			taken, err := componentExists(ctx, tx, comps[i], in)
			if err != nil {
				return err
			}
			if taken {
				collision = &comps[i]
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if collision != nil {
		return core.Conflict("%s %q already exists; choose %q, %q or %q for it",
			collision.Kind, collision.Key(), core.CollisionSkip, core.CollisionRename, core.CollisionReplace).
			WithDetail("kind", string(collision.Kind)).
			WithDetail("key", collision.Key())
	}
	return in.Validate()
}

// componentExists reports whether a component's key is already taken.
func componentExists(ctx context.Context, tx store.Tx, c bundle.Component, in core.BundleImportInput) (bool, error) {
	switch c.Kind {
	case core.ComponentWorkflow:
		wf, err := findWorkflow(ctx, tx, normalKey(c.Key()))
		return wf != nil, err
	case core.ComponentFieldDef:
		p, err := lookupProject(ctx, tx, in.ProjectRef)
		if err != nil {
			return false, err
		}
		d, err := findFieldDef(ctx, tx, p.ID, c.Key())
		return d != nil, err
	case core.ComponentTag:
		t, err := findTenantTag(ctx, tx, c.Key())
		return t != nil, err
	case core.ComponentProject:
		p, err := findProject(ctx, tx, normalKey(c.Key()))
		return p != nil, err
	default:
		e, err := findWebhookByURL(ctx, tx, c.Key())
		return e != nil, err
	}
}

// findTenantTag returns a tenant tag by name, or nil when there is none.
func findTenantTag(ctx context.Context, tx store.Tx, name string) (*core.Tag, error) {
	tags, err := tenantTags(ctx, tx)
	if err != nil {
		return nil, err
	}
	for i := range tags {
		if tags[i].Name == name {
			return &tags[i], nil
		}
	}
	return nil, nil
}

// findWebhookByURL returns the first endpoint with this delivery target.
func findWebhookByURL(ctx context.Context, tx store.Tx, url string) (*core.WebhookEndpoint, error) {
	endpoints, err := tx.ListWebhooks(ctx)
	if err != nil {
		return nil, err
	}
	for i := range endpoints {
		if endpoints[i].URL == url {
			return &endpoints[i], nil
		}
	}
	return nil, nil
}

// normalKey lowercases and trims a key typed or written by hand.
func normalKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// bundleImport applies one bundle. A nil mutation means a preview, which
// decides everything a real import decides and writes nothing.
type bundleImport struct {
	local  *Local
	in     core.BundleImportInput
	comps  []bundle.Component
	result *core.BundleResult

	project   *core.Project
	workflows map[string]string
	tags      map[string]bool
}

// run applies every component in bundle order.
func (i *bundleImport) run(ctx context.Context, tx store.Tx, m *mutation) error {
	for _, c := range i.comps {
		if err := i.apply(ctx, tx, m, c); err != nil {
			return err
		}
	}
	return nil
}

// apply dispatches one component.
func (i *bundleImport) apply(ctx context.Context, tx store.Tx, m *mutation, c bundle.Component) error {
	switch c.Kind {
	case core.ComponentWorkflow:
		return i.applyWorkflow(ctx, tx, m, *c.Workflow)
	case core.ComponentFieldDef:
		return i.applyField(ctx, tx, m, *c.Field)
	case core.ComponentTag:
		return i.applyTag(ctx, tx, m, *c.Tag)
	case core.ComponentProject:
		return i.applyProject(ctx, tx, m, *c.Project)
	default:
		return i.applyWebhook(ctx, tx, m, *c.Webhook)
	}
}

// decide chooses what to do about a component whose key may be taken.
func (i *bundleImport) decide(taken bool, key string, free func(string) (bool, error)) (string, core.ComponentAction, error) {
	if !taken {
		return key, core.ActionCreated, nil
	}
	switch i.in.OnCollision {
	case core.CollisionSkip:
		return key, core.ActionSkipped, nil
	case core.CollisionRename:
		fresh, err := freeKey(key, free)
		return fresh, core.ActionRenamed, err
	default:
		return key, core.ActionUpdated, nil
	}
}

// freeKey returns the first suffixed key nothing else claims.
func freeKey(base string, taken func(string) (bool, error)) (string, error) {
	for n := 2; n <= bundleRenameLimit; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		used, err := taken(candidate)
		if err != nil {
			return "", err
		}
		if !used {
			return candidate, nil
		}
	}
	return "", core.Conflict("no free key near %q after %d attempts", base, bundleRenameLimit)
}

// outcome records what happened to one component and returns it.
func (i *bundleImport) outcome(kind core.ComponentKind, key, target string, action core.ComponentAction) core.ComponentOutcome {
	out := core.ComponentOutcome{Kind: kind, Key: key, Action: action}
	if action == core.ActionRenamed {
		out.NewKey = target
	}
	if action == core.ActionSkipped {
		out.Reason = fmt.Sprintf("%s %q already exists", kind, key)
	}
	i.result.Outcomes = append(i.result.Outcomes, out)
	return out
}

// warn records a note the caller should read after the import.
func (i *bundleImport) warn(format string, args ...any) {
	i.result.Warnings = append(i.result.Warnings, fmt.Sprintf(format, args...))
}

// record writes the audit entry and event for one imported component. Both name
// the bundle the component came from.
func (i *bundleImport) record(m *mutation, subjectID, projectID string, before, after any, out core.ComponentOutcome) error {
	state := map[string]any{
		"bundle":         i.result.BundleName,
		"bundle_version": i.result.BundleVersion,
		"action":         string(out.Action),
		"key":            out.Key,
		"component":      after,
	}
	if out.NewKey != "" {
		state["new_key"] = out.NewKey
	}
	payload := map[string]any{
		"bundle":         i.result.BundleName,
		"bundle_version": i.result.BundleVersion,
		"kind":           string(out.Kind),
		"key":            out.Key,
		"action":         string(out.Action),
	}
	return m.Record(auditBundleImport, bundleEvents[out.Kind], bundleSubjects[out.Kind],
		subjectID, projectID, before, state, payload)
}

// applyWorkflow creates, renames or replaces one workflow.
func (i *bundleImport) applyWorkflow(ctx context.Context, tx store.Tx, m *mutation, w bundle.Workflow) error {
	key := normalKey(w.Key)
	existing, err := findWorkflow(ctx, tx, key)
	if err != nil {
		return err
	}
	target, action, err := i.decide(existing != nil || i.workflows[key] != "", key, func(candidate string) (bool, error) {
		found, err := findWorkflow(ctx, tx, candidate)
		return found != nil || i.workflows[candidate] != "", err
	})
	if err != nil {
		return err
	}
	out := i.outcome(core.ComponentWorkflow, key, target, action)
	if action == core.ActionSkipped {
		return nil
	}

	wf := &core.Workflow{Key: target, Name: bundleLabel(w.Name, target), Definition: w.Definition}
	if action == core.ActionUpdated {
		wf.ID = existing.ID
		wf.Builtin = existing.Builtin
	}
	if m == nil {
		i.workflows[target] = target
		return nil
	}
	if err := tx.PutWorkflow(ctx, wf); err != nil {
		return err
	}
	i.workflows[target] = wf.ID
	return i.record(m, wf.ID, "", auditBefore(existing, action), wf, out)
}

// applyField creates, renames or replaces one custom field definition on the
// target project.
func (i *bundleImport) applyField(ctx context.Context, tx store.Tx, m *mutation, f bundle.Field) error {
	p, err := i.targetProject(ctx, tx)
	if err != nil {
		return err
	}
	existing, err := findFieldDef(ctx, tx, p.ID, f.Key)
	if err != nil {
		return err
	}
	target, action, err := i.decide(existing != nil, f.Key, func(candidate string) (bool, error) {
		found, err := findFieldDef(ctx, tx, p.ID, candidate)
		return found != nil, err
	})
	if err != nil {
		return err
	}
	out := i.outcome(core.ComponentFieldDef, f.Key, target, action)
	if action == core.ActionSkipped || m == nil {
		return nil
	}

	def := fieldFromBundle(f, target, p.ID)
	if action == core.ActionUpdated {
		def.ID = existing.ID
	}
	if err := tx.PutFieldDef(ctx, def); err != nil {
		return err
	}
	return i.record(m, def.ID, p.ID, auditBefore(existing, action), def, out)
}

// fieldFromBundle builds a field definition for the target project.
func fieldFromBundle(f bundle.Field, key, projectID string) *core.FieldDef {
	return &core.FieldDef{
		ProjectID: projectID, Key: key, Label: bundleLabel(f.Label, key),
		Type: f.Type, Required: f.Required, EnumOptions: f.EnumOptions,
		Default: f.Default, Indexed: f.Indexed, Position: f.Position,
	}
}

// targetProject resolves the project loose field definitions land in.
func (i *bundleImport) targetProject(ctx context.Context, tx store.Tx) (*core.Project, error) {
	if i.project != nil {
		return i.project, nil
	}
	p, err := lookupProject(ctx, tx, i.in.ProjectRef)
	if err != nil {
		return nil, err
	}
	i.project = p
	return p, nil
}

// applyTag creates, renames or replaces one tag of the tenant vocabulary.
func (i *bundleImport) applyTag(ctx context.Context, tx store.Tx, m *mutation, t bundle.Tag) error {
	existing, err := findTenantTag(ctx, tx, t.Name)
	if err != nil {
		return err
	}
	target, action, err := i.decide(existing != nil || i.tags[t.Name], t.Name, func(candidate string) (bool, error) {
		found, err := findTenantTag(ctx, tx, candidate)
		return found != nil || i.tags[candidate], err
	})
	if err != nil {
		return err
	}
	out := i.outcome(core.ComponentTag, t.Name, target, action)
	if action == core.ActionSkipped {
		return nil
	}
	i.tags[target] = true
	if m == nil {
		return nil
	}
	tag := &core.Tag{Name: target, Color: t.Color}
	if err := tx.PutTag(ctx, tag); err != nil {
		return err
	}
	return i.record(m, tag.ID, "", auditBefore(existing, action), tag, out)
}

// applyProject creates, renames or replaces one project template, together with
// the workflow, field definitions and tags that describe it.
func (i *bundleImport) applyProject(ctx context.Context, tx store.Tx, m *mutation, p bundle.Project) error {
	key := normalKey(p.Key)
	existing, err := findProject(ctx, tx, key)
	if err != nil {
		return err
	}
	target, action, err := i.decide(existing != nil, key, func(candidate string) (bool, error) {
		found, err := findProject(ctx, tx, candidate)
		return found != nil, err
	})
	if err != nil {
		return err
	}
	out := i.outcome(core.ComponentProject, key, target, action)
	if action == core.ActionSkipped {
		return nil
	}

	workflowID, err := i.templateWorkflow(ctx, tx, m, p, existing)
	if err != nil {
		return err
	}
	if m == nil {
		i.previewTemplate(p)
		return nil
	}
	project := &core.Project{
		Key: target, Name: bundleLabel(p.Name, target), Description: p.Description,
		WorkflowID: workflowID, Color: p.Color, Icon: p.Icon,
	}
	if action == core.ActionUpdated {
		project.ID = existing.ID
		project.CreatedAt = existing.CreatedAt
		err = tx.UpdateProject(ctx, project)
	} else {
		err = tx.CreateProject(ctx, project)
	}
	if err != nil {
		return err
	}
	if err := i.templateContents(ctx, tx, p, project); err != nil {
		return err
	}
	return i.record(m, project.ID, project.ID, auditBefore(existing, action), project, out)
}

// templateWorkflow resolves the workflow a template names, creating it when the
// target tenant has none by that key.
func (i *bundleImport) templateWorkflow(ctx context.Context, tx store.Tx, m *mutation, p bundle.Project, existing *core.Project) (string, error) {
	if p.Workflow == nil {
		if existing != nil {
			return existing.WorkflowID, nil
		}
		wf, err := findWorkflow(ctx, tx, BuiltinWorkflowKey)
		if err != nil {
			return "", err
		}
		if wf == nil {
			return "", core.Invalid("project template %q names no workflow and this tenant has no %q workflow",
				p.Key, BuiltinWorkflowKey)
		}
		return wf.ID, nil
	}
	key := normalKey(p.Workflow.Key)
	if id, ok := i.workflows[key]; ok {
		return id, nil
	}
	found, err := findWorkflow(ctx, tx, key)
	if err != nil {
		return "", err
	}
	if found != nil {
		i.workflows[key] = found.ID
		return found.ID, nil
	}
	return i.createTemplateWorkflow(ctx, tx, m, *p.Workflow, key)
}

// createTemplateWorkflow adds the workflow a template brought with it.
func (i *bundleImport) createTemplateWorkflow(ctx context.Context, tx store.Tx, m *mutation, w bundle.Workflow, key string) (string, error) {
	out := i.outcome(core.ComponentWorkflow, key, key, core.ActionCreated)
	wf := &core.Workflow{Key: key, Name: bundleLabel(w.Name, key), Definition: w.Definition}
	if m == nil {
		i.workflows[key] = key
		return key, nil
	}
	if err := tx.PutWorkflow(ctx, wf); err != nil {
		return "", err
	}
	i.workflows[key] = wf.ID
	return wf.ID, i.record(m, wf.ID, "", nil, wf, out)
}

// previewTemplate registers the tags a template would add, so a preview of a
// bundle carrying both a template and a tag reports each of them once.
func (i *bundleImport) previewTemplate(p bundle.Project) {
	for _, t := range p.Tags {
		i.tags[t.Name] = true
	}
}

// templateContents writes a template's field definitions and tags.
func (i *bundleImport) templateContents(ctx context.Context, tx store.Tx, p bundle.Project, project *core.Project) error {
	for _, f := range p.Fields {
		existing, err := findFieldDef(ctx, tx, project.ID, f.Key)
		if err != nil {
			return err
		}
		def := fieldFromBundle(f, f.Key, project.ID)
		if existing != nil {
			def.ID = existing.ID
		}
		if err := tx.PutFieldDef(ctx, def); err != nil {
			return err
		}
	}
	for _, t := range p.Tags {
		i.tags[t.Name] = true
		if err := tx.PutTag(ctx, &core.Tag{Name: t.Name, Color: t.Color}); err != nil {
			return err
		}
	}
	return nil
}

// applyWebhook registers an endpoint definition. A bundle never carries a
// signing secret, so a new endpoint arrives inactive.
func (i *bundleImport) applyWebhook(ctx context.Context, tx store.Tx, m *mutation, w bundle.Webhook) error {
	existing, err := findWebhookByURL(ctx, tx, w.URL)
	if err != nil {
		return err
	}
	action := core.ActionCreated
	if existing != nil {
		switch i.in.OnCollision {
		case core.CollisionSkip:
			i.outcome(core.ComponentWebhook, w.URL, w.URL, core.ActionSkipped)
			return nil
		case core.CollisionRename:
			i.warn("a second endpoint for %q was created; endpoints are identified by id, not url", w.URL)
		default:
			action = core.ActionUpdated
		}
	}
	out := i.outcome(core.ComponentWebhook, w.URL, w.URL, action)

	endpoint := &core.WebhookEndpoint{URL: w.URL, EventTypes: w.EventTypes}
	var before any
	if action == core.ActionUpdated {
		endpoint.ID = existing.ID
		endpoint.Secret = existing.Secret
		endpoint.Active = existing.Active
		before = webhookPayload(*existing)
	}
	if endpoint.Secret == "" {
		endpoint.Active = false
		i.warn("webhook %q is inactive until a signing secret is supplied", w.URL)
	}
	if m == nil {
		return nil
	}
	if err := tx.PutWebhook(ctx, endpoint); err != nil {
		return err
	}
	return i.record(m, endpoint.ID, "", before, webhookPayload(*endpoint), out)
}

// auditBefore returns the prior state an audit entry compares against, which
// exists only when the component was already there.
func auditBefore(existing any, action core.ComponentAction) any {
	if action != core.ActionUpdated {
		return nil
	}
	return existing
}

// bundleLabel falls back to the key when a component carries no display name.
func bundleLabel(name, key string) string {
	if strings.TrimSpace(name) == "" {
		return key
	}
	return name
}

// Local implements component sharing. This assertion stands here until
// core.Service embeds BundleService.
var _ core.BundleService = (*Local)(nil)
