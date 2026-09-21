package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/bundle"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// bundleFixture is a tenant with one of everything a bundle can carry, plus the
// work items a bundle must never carry.
type bundleFixture struct {
	l       *Local
	ctx     context.Context
	scope   core.TenantScope
	actor   *core.Actor
	project *core.Project
}

const (
	secretTitle   = "zzsecrettitlezz"
	secretComment = "zzsecretcommentzz"
)

func newBundleFixture(t *testing.T) *bundleFixture {
	t.Helper()
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{
		Key: "dev", Name: "Dev", Definition: devDefinition(),
	}); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "acme", Name: "Acme", WorkflowKey: "dev"})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if _, err := l.PutFieldDef(ctx, "acme", core.FieldDefInput{
		Key: "points", Label: "Points", Type: core.FieldInt, Position: 1,
	}); err != nil {
		t.Fatalf("field: %v", err)
	}
	task, err := l.CreateTask(ctx, core.CreateTaskInput{
		ProjectRef: "acme", Title: secretTitle, Tags: []string{"backend"},
	})
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	if _, err := l.AddComment(ctx, core.TaskRef{ID: task.ID}, secretComment); err != nil {
		t.Fatalf("comment: %v", err)
	}
	if _, err := l.PutWebhook(ctx, core.WebhookInput{
		URL: "https://hooks.example.com/tix", Secret: "shhhhh-do-not-share", Active: true,
	}); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	return &bundleFixture{l: l, ctx: ctx, scope: scope, actor: actor, project: p}
}

func devDefinition() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States:  []core.State{{Key: "todo", Label: "Todo"}, {Key: "done", Label: "Done", Terminal: true}},
		Transitions: []core.Transition{
			{From: "todo", To: "done"},
			{From: "done", To: "todo"},
		},
	}
}

func (f *bundleFixture) export(t *testing.T, in core.BundleExportInput) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := f.l.ExportBundle(f.ctx, in, &buf); err != nil {
		t.Fatalf("ExportBundle: %v", err)
	}
	return buf.Bytes()
}

func (f *bundleFixture) components(t *testing.T, raw []byte) []bundle.Component {
	t.Helper()
	_, comps, err := bundle.Read(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("reading the exported bundle: %v", err)
	}
	return comps
}

func bundleKindsOf(comps []bundle.Component) []core.ComponentKind {
	out := make([]core.ComponentKind, 0, len(comps))
	for _, c := range comps {
		out = append(out, c.Kind)
	}
	return out
}

// A bundle carries a way of working, so nothing about anyone's work may ride
// along with it.
func TestExportBundleCarriesNoWorkItems(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{Name: "everything"})

	for _, forbidden := range []string{
		secretTitle, secretComment, f.actor.ID, f.actor.Handle,
		"shhhhh-do-not-share", f.scope.TenantID, f.project.ID,
		`"task"`, `"comment"`, `"artifact"`, `"audit_entry"`, `"event"`, `"dependency"`,
	} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Errorf("the bundle carries %q:\n%s", forbidden, raw)
		}
	}
	if len(f.components(t, raw)) == 0 {
		t.Fatal("the bundle carried nothing at all")
	}
}

func TestExportOneKindExcludesTheOthers(t *testing.T) {
	f := newBundleFixture(t)

	cases := []struct {
		kind core.ComponentKind
		key  string
	}{
		{core.ComponentWorkflow, "dev"},
		{core.ComponentFieldDef, "points"},
		{core.ComponentTag, "backend"},
		{core.ComponentProject, "acme"},
		{core.ComponentWebhook, "https://hooks.example.com/tix"},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{tc.kind}})
			comps := f.components(t, raw)
			if len(comps) != 1 {
				t.Fatalf("exported %v, want only a %s", bundleKindsOf(comps), tc.kind)
			}
			if comps[0].Kind != tc.kind || comps[0].Key() != tc.key {
				t.Fatalf("exported %s %q, want %s %q", comps[0].Kind, comps[0].Key(), tc.kind, tc.key)
			}
		})
	}
}

func TestExportRefusesUnknownKind(t *testing.T) {
	f := newBundleFixture(t)
	err := f.l.ExportBundle(f.ctx, core.BundleExportInput{Kinds: []core.ComponentKind{"task"}}, &bytes.Buffer{})
	if !core.IsKind(err, core.KindInvalid) {
		t.Errorf("exporting tasks = %v, want an invalid-input error", err)
	}
}

func TestExportSelectorsChooseTheirKinds(t *testing.T) {
	f := newBundleFixture(t)

	raw := f.export(t, core.BundleExportInput{WorkflowKeys: []string{"DEV"}})
	comps := f.components(t, raw)
	if len(comps) != 1 || comps[0].Kind != core.ComponentWorkflow {
		t.Fatalf("selecting a workflow exported %v", bundleKindsOf(comps))
	}

	raw = f.export(t, core.BundleExportInput{ProjectRefs: []string{"acme"}})
	comps = f.components(t, raw)
	if len(comps) != 1 || comps[0].Kind != core.ComponentProject {
		t.Fatalf("selecting a project exported %v", bundleKindsOf(comps))
	}
}

func TestExportProjectTemplateCarriesConfigurationNotTasks(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentProject}})
	comps := f.components(t, raw)
	if len(comps) != 1 {
		t.Fatalf("exported %d components, want one template", len(comps))
	}
	tpl := comps[0].Project
	if tpl.Workflow == nil || tpl.Workflow.Key != "dev" {
		t.Errorf("template carries workflow %+v, want dev", tpl.Workflow)
	}
	if len(tpl.Workflow.Definition.States) != 2 {
		t.Errorf("template workflow carries %d states, want 2", len(tpl.Workflow.Definition.States))
	}
	if len(tpl.Fields) != 1 || tpl.Fields[0].Key != "points" {
		t.Errorf("template carries fields %+v, want points", tpl.Fields)
	}
	if len(tpl.Tags) != 1 || tpl.Tags[0].Name != "backend" {
		t.Errorf("template carries tags %+v, want backend", tpl.Tags)
	}
	if bytes.Contains(raw, []byte(secretTitle)) {
		t.Error("the template carries a task")
	}
}

func TestWebhookTravelsWithoutItsSecret(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentWebhook}})
	if bytes.Contains(raw, []byte("shhhhh-do-not-share")) || bytes.Contains(raw, []byte("secret")) {
		t.Fatalf("the bundle carries a signing secret:\n%s", raw)
	}
	comps := f.components(t, raw)
	if len(comps) != 1 || comps[0].Webhook.URL != "https://hooks.example.com/tix" {
		t.Fatalf("exported %+v", comps)
	}

	other := newBundleTenant(t, f.l)
	res, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionSkip})
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Error("an endpoint imported without a secret must warn that it is inactive")
	}
	endpoints, err := f.l.ListWebhooks(other)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("imported %d endpoints, want 1", len(endpoints))
	}
	if endpoints[0].Active {
		t.Error("an imported endpoint must stay inactive until a secret is supplied")
	}
}

func TestReExportIsByteIdentical(t *testing.T) {
	f := newBundleFixture(t)
	first := f.export(t, core.BundleExportInput{Name: "team"})
	second := f.export(t, core.BundleExportInput{Name: "team"})
	if !bytes.Equal(first, second) {
		t.Errorf("exporting unchanged components twice differed:\n%s\n---\n%s", first, second)
	}
}

func TestExportRequiresReadPermission(t *testing.T) {
	f := newBundleFixture(t)
	narrow := &core.Actor{
		ID: f.actor.ID, TenantID: f.actor.TenantID, Kind: core.ActorUser,
		Scopes: []core.Scope{core.ScopeTaskRead},
	}
	ctx := core.WithActor(context.Background(), narrow)

	err := f.l.ExportBundle(ctx, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentWorkflow}}, &bytes.Buffer{})
	if !core.IsKind(err, core.KindForbidden) {
		t.Errorf("exporting a workflow without workflow read = %v, want forbidden", err)
	}
	if err := f.l.ExportBundle(ctx, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentTag}}, &bytes.Buffer{}); err != nil {
		t.Errorf("exporting tags with task read = %v, want it allowed", err)
	}
	if err := f.l.ExportBundle(context.Background(), core.BundleExportInput{}, &bytes.Buffer{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("exporting with no actor = %v, want unauthenticated", err)
	}
}

func TestImportRequiresCreatePermission(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentWorkflow}})

	narrow := &core.Actor{
		ID: f.actor.ID, TenantID: f.actor.TenantID, Kind: core.ActorUser,
		Scopes: []core.Scope{core.ScopeTaskRead},
	}
	ctx := core.WithActor(context.Background(), narrow)
	_, err := f.l.ImportBundle(ctx, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionReplace})
	if !core.IsKind(err, core.KindForbidden) {
		t.Errorf("importing a workflow without workflow write = %v, want forbidden", err)
	}
	if _, err := f.l.ImportBundle(context.Background(), bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionSkip}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("importing with no actor = %v, want unauthenticated", err)
	}
}

// newBundleTenant creates a second tenant with an admin actor and returns a
// context authenticated as that actor.
func newBundleTenant(t *testing.T, l *Local) context.Context {
	t.Helper()
	ctx := context.Background()
	tenant := core.Tenant{Key: "other", Name: "Other"}
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating the second tenant: %v", err)
	}
	actor := core.Actor{Kind: core.ActorUser, Handle: "bob", Scopes: []core.Scope{core.ScopeAll}}
	if err := l.store.Update(ctx, core.TenantScope{TenantID: tenant.ID}, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("creating the second tenant's actor: %v", err)
	}
	actor.TenantID = tenant.ID
	return core.WithActor(ctx, &actor)
}

func TestImportIntoAnotherTenantRecreatesComponents(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{Name: "team"})
	other := newBundleTenant(t, f.l)

	if res, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{
		OnCollision: core.CollisionSkip, ProjectRef: "acme",
	}); err == nil {
		t.Fatalf("a loose field definition cannot land before its project exists, got %+v", res)
	}

	trimmed := f.export(t, core.BundleExportInput{
		Name:  "team",
		Kinds: []core.ComponentKind{core.ComponentWorkflow, core.ComponentTag, core.ComponentProject},
	})
	res, err := f.l.ImportBundle(other, bytes.NewReader(trimmed), core.BundleImportInput{OnCollision: core.CollisionSkip})
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	if res.BundleName != "team" || res.BundleVersion != core.BundleVersion {
		t.Errorf("result = %+v, want it to name the bundle", res)
	}

	wf, err := f.l.GetWorkflow(other, "dev")
	if err != nil {
		t.Fatalf("the workflow did not arrive: %v", err)
	}
	source, err := f.l.GetWorkflow(f.ctx, "dev")
	if err != nil {
		t.Fatalf("source workflow: %v", err)
	}
	if wf.ID == source.ID {
		t.Error("the imported workflow reuses the source identifier")
	}
	if wf.TenantID == f.scope.TenantID {
		t.Error("the imported workflow landed in the source tenant")
	}
	project, err := f.l.GetProject(other, "acme")
	if err != nil {
		t.Fatalf("the project template did not arrive: %v", err)
	}
	if project.ID == f.project.ID {
		t.Error("the imported project reuses the source identifier")
	}
	if project.WorkflowID != wf.ID {
		t.Errorf("the imported project uses workflow %q, want the imported %q", project.WorkflowID, wf.ID)
	}
	defs, err := f.l.ListFieldDefs(other, "acme")
	if err != nil {
		t.Fatalf("ListFieldDefs: %v", err)
	}
	if len(defs) != 1 || defs[0].Key != "points" {
		t.Errorf("the template's fields did not arrive: %+v", defs)
	}
	tasks, err := f.l.ListTasks(other, core.TaskFilter{Page: core.Page{Limit: 10}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks.Tasks) != 0 {
		t.Errorf("importing a template brought %d tasks with it", len(tasks.Tasks))
	}
}

// A bundle is data, not a routing instruction.
func TestBundleCannotRedirectItsTarget(t *testing.T) {
	f := newBundleFixture(t)
	other := newBundleTenant(t, f.l)
	doc := `{"record":"header","header":{"name":"redirect","version":1,"tenant":"acme"}}` + "\n" +
		`{"record":"component","component":{"kind":"tag","tag":{"name":"imported"}}}` + "\n"

	if _, err := f.l.ImportBundle(other, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionSkip}); err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	if !hasTag(t, f.l, other, "imported") {
		t.Error("the component did not land in the caller's own tenant")
	}
	if hasTag(t, f.l, f.ctx, "imported") {
		t.Error("the bundle redirected itself into the tenant it named")
	}
}

func hasTag(t *testing.T, l *Local, ctx context.Context, name string) bool {
	t.Helper()
	tags, err := l.ListTags(ctx)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	for _, tag := range tags {
		if tag.Name == name {
			return true
		}
	}
	return false
}

func TestBundleImportRefusesUnsupportedAndMissingVersions(t *testing.T) {
	f := newBundleFixture(t)
	cases := []struct {
		name string
		doc  string
		want []string
	}{
		{"unsupported", `{"record":"header","header":{"version":99}}`, []string{"99", "1"}},
		{"missing", `{"record":"header","header":{}}`, []string{"0", "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforeEvents, beforeAudits := countRows(t, f.l, f.scope)
			_, err := f.l.ImportBundle(f.ctx, strings.NewReader(tc.doc+"\n"),
				core.BundleImportInput{OnCollision: core.CollisionReplace})
			if err == nil {
				t.Fatal("an unreadable bundle version must be refused")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %v does not name version %s", err, want)
				}
			}
			afterEvents, afterAudits := countRows(t, f.l, f.scope)
			if afterEvents != beforeEvents || afterAudits != beforeAudits {
				t.Error("a refused bundle wrote something")
			}
		})
	}
}

func TestImportRefusesAnInvalidComponentAndWritesNothing(t *testing.T) {
	f := newBundleFixture(t)
	doc := `{"record":"header","header":{"name":"broken","version":1}}` + "\n" +
		`{"record":"component","component":{"kind":"workflow","workflow":{"key":"broken","definition":` +
		`{"initial":"todo","states":[{"key":"todo"},{"key":"done","terminal":true}],` +
		`"transitions":[{"from":"todo","to":"nowhere"}]}}}}` + "\n" +
		`{"record":"component","component":{"kind":"tag","tag":{"name":"survivor"}}}` + "\n"

	beforeEvents, beforeAudits := countRows(t, f.l, f.scope)
	_, err := f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionReplace})
	if err == nil {
		t.Fatal("a workflow with a transition to an undefined state must be refused")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("error %v does not name the offending component", err)
	}
	if hasTag(t, f.l, f.ctx, "survivor") {
		t.Error("a refused bundle wrote one of its components")
	}
	afterEvents, afterAudits := countRows(t, f.l, f.scope)
	if afterEvents != beforeEvents || afterAudits != beforeAudits {
		t.Error("a refused bundle wrote an audit entry or an event")
	}
}

func TestImportIsAtomic(t *testing.T) {
	f := newBundleFixture(t)
	other := newBundleTenant(t, f.l)
	doc := `{"record":"header","header":{"name":"partial","version":1}}` + "\n" +
		`{"record":"component","component":{"kind":"workflow","workflow":{"key":"early","definition":` +
		`{"initial":"todo","states":[{"key":"todo"},{"key":"done","terminal":true}]}}}}` + "\n" +
		`{"record":"component","component":{"kind":"project_template","project":{"key":"late"}}}` + "\n"

	if _, err := f.l.ImportBundle(other, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionReplace}); err == nil {
		t.Fatal("a template with no workflow to attach to must be refused")
	}
	if _, err := f.l.GetWorkflow(other, "early"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("a failed import left its first component behind: %v", err)
	}
}

func TestCollisionWithNoPolicyNamesTheComponent(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentWorkflow}})

	_, err := f.l.ImportBundle(f.ctx, bytes.NewReader(raw), core.BundleImportInput{})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a collision with no policy = %v, want a conflict", err)
	}
	if !strings.Contains(err.Error(), "dev") {
		t.Errorf("error %v does not name the colliding component", err)
	}

	fresh := `{"record":"header","header":{"version":1}}` + "\n" +
		`{"record":"component","component":{"kind":"tag","tag":{"name":"nothing-collides"}}}` + "\n"
	_, err = f.l.ImportBundle(f.ctx, strings.NewReader(fresh), core.BundleImportInput{})
	if !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an import with no policy and no collision = %v, want the missing-policy refusal", err)
	}
}

func TestCollisionPolicies(t *testing.T) {
	t.Run("skip leaves the original alone", func(t *testing.T) {
		f := newBundleFixture(t)
		doc := renamedWorkflowDoc("dev", "Imported")
		res, err := f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionSkip})
		if err != nil {
			t.Fatalf("ImportBundle: %v", err)
		}
		if len(res.Outcomes) != 1 || res.Outcomes[0].Action != core.ActionSkipped {
			t.Fatalf("outcomes = %+v, want one skip", res.Outcomes)
		}
		wf, err := f.l.GetWorkflow(f.ctx, "dev")
		if err != nil || wf.Name != "Dev" {
			t.Errorf("skip changed the existing workflow: %+v, %v", wf, err)
		}
	})

	t.Run("rename produces a free key and reports it", func(t *testing.T) {
		f := newBundleFixture(t)
		doc := renamedWorkflowDoc("dev", "Imported")
		res, err := f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionRename})
		if err != nil {
			t.Fatalf("ImportBundle: %v", err)
		}
		out := res.Outcomes[0]
		if out.Action != core.ActionRenamed || out.NewKey != "dev-2" {
			t.Fatalf("outcome = %+v, want a rename to dev-2", out)
		}
		if _, err := f.l.GetWorkflow(f.ctx, "dev-2"); err != nil {
			t.Errorf("the renamed workflow is missing: %v", err)
		}
		original, err := f.l.GetWorkflow(f.ctx, "dev")
		if err != nil || original.Name != "Dev" {
			t.Errorf("rename disturbed the original: %+v, %v", original, err)
		}

		again, err := f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionRename})
		if err != nil {
			t.Fatalf("second import: %v", err)
		}
		if again.Outcomes[0].NewKey != "dev-3" {
			t.Errorf("a second rename produced %q, want dev-3", again.Outcomes[0].NewKey)
		}
	})

	t.Run("replace updates in place and is audited", func(t *testing.T) {
		f := newBundleFixture(t)
		doc := renamedWorkflowDoc("dev", "Imported")
		before := auditActions(t, f.l, f.scope)

		res, err := f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionReplace})
		if err != nil {
			t.Fatalf("ImportBundle: %v", err)
		}
		if res.Outcomes[0].Action != core.ActionUpdated {
			t.Fatalf("outcome = %+v, want an update", res.Outcomes[0])
		}
		wf, err := f.l.GetWorkflow(f.ctx, "dev")
		if err != nil {
			t.Fatalf("GetWorkflow: %v", err)
		}
		if wf.Name != "Imported" {
			t.Errorf("workflow name = %q, want the replacement %q", wf.Name, "Imported")
		}
		after := auditActions(t, f.l, f.scope)
		if len(after) != len(before)+1 || after[len(after)-1] != auditBundleImport {
			t.Errorf("replace was not audited: %v", after)
		}
	})
}

func renamedWorkflowDoc(key, name string) string {
	return `{"record":"header","header":{"name":"team","version":1}}` + "\n" +
		fmt.Sprintf(`{"record":"component","component":{"kind":"workflow","workflow":{"key":%q,"name":%q,`, key, name) +
		`"definition":{"initial":"todo","states":[{"key":"todo"},{"key":"done","terminal":true}],` +
		`"transitions":[{"from":"todo","to":"done"}]}}}}` + "\n"
}

func auditActions(t *testing.T, l *Local, scope core.TenantScope) []string {
	t.Helper()
	var out []string
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 1000}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			out = append(out, e.Action)
		}
		return nil
	}); err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	return out
}

func TestPreviewWritesNothingAndMatchesTheRealImport(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{
		Name:  "team",
		Kinds: []core.ComponentKind{core.ComponentWorkflow, core.ComponentTag, core.ComponentProject},
	})
	other := newBundleTenant(t, f.l)
	scope := actorScope(t, other)

	beforeEvents, beforeAudits := countRows(t, f.l, scope)
	preview, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{
		OnCollision: core.CollisionReplace, Preview: true,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !preview.Preview {
		t.Error("the result does not report itself as a preview")
	}
	afterEvents, afterAudits := countRows(t, f.l, scope)
	if afterEvents != beforeEvents || afterAudits != beforeAudits {
		t.Fatalf("a preview wrote %d events and %d audit entries",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
	if _, err := f.l.GetWorkflow(other, "dev"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("a preview created a workflow: %v", err)
	}

	real, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{
		OnCollision: core.CollisionReplace,
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(preview.Outcomes) != len(real.Outcomes) {
		t.Fatalf("preview reported %+v, the import did %+v", preview.Outcomes, real.Outcomes)
	}
	for i := range preview.Outcomes {
		p, r := preview.Outcomes[i], real.Outcomes[i]
		if p.Kind != r.Kind || p.Key != r.Key || p.Action != r.Action || p.NewKey != r.NewKey {
			t.Errorf("outcome %d: preview %+v, import %+v", i, p, r)
		}
	}
}

func actorScope(t *testing.T, ctx context.Context) core.TenantScope {
	t.Helper()
	actor, err := core.RequireActor(ctx)
	if err != nil {
		t.Fatalf("RequireActor: %v", err)
	}
	return core.TenantScope{TenantID: actor.TenantID}
}

func TestImportAuditNamesTheBundle(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{
		Name:  "ways-of-working",
		Kinds: []core.ComponentKind{core.ComponentWorkflow, core.ComponentTag},
	})
	other := newBundleTenant(t, f.l)
	scope := actorScope(t, other)

	res, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionReplace})
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	changed := 0
	for _, out := range res.Outcomes {
		if out.Action != core.ActionSkipped {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("the import changed nothing")
	}

	entries := bundleAuditEntries(t, f.l, scope)
	if len(entries) != changed {
		t.Fatalf("wrote %d audit entries for %d changed components", len(entries), changed)
	}
	for _, e := range entries {
		var after map[string]any
		if err := json.Unmarshal(e.After, &after); err != nil {
			t.Fatalf("decoding audit state: %v", err)
		}
		if after["bundle"] != "ways-of-working" {
			t.Errorf("audit entry does not name the bundle: %v", after)
		}
		if fmt.Sprint(after["bundle_version"]) != fmt.Sprint(core.BundleVersion) {
			t.Errorf("audit entry does not name the bundle version: %v", after)
		}
	}
}

func bundleAuditEntries(t *testing.T, l *Local, scope core.TenantScope) []core.AuditEntry {
	t.Helper()
	var out []core.AuditEntry
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 1000}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Action == auditBundleImport {
				out = append(out, e)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	return out
}

func TestImportRefusesLooseFieldWithNoTargetProject(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentFieldDef}})

	_, err := f.l.ImportBundle(f.ctx, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionReplace})
	if !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a loose field with no target project = %v, want an invalid-input error", err)
	}
}

func TestImportFieldIntoNamedProject(t *testing.T) {
	f := newBundleFixture(t)
	doc := `{"record":"header","header":{"name":"fields","version":1}}` + "\n" +
		`{"record":"component","component":{"kind":"field_def","field":{"key":"risk","type":"enum","enum_options":["low","high"]}}}` + "\n"

	res, err := f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{
		OnCollision: core.CollisionRename, ProjectRef: "acme",
	})
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	if res.Outcomes[0].Action != core.ActionCreated {
		t.Fatalf("outcome = %+v, want a creation", res.Outcomes[0])
	}
	defs, err := f.l.ListFieldDefs(f.ctx, "acme")
	if err != nil {
		t.Fatalf("ListFieldDefs: %v", err)
	}
	found := false
	for _, d := range defs {
		if d.Key == "risk" {
			found = true
		}
	}
	if !found {
		t.Errorf("the field did not arrive: %+v", defs)
	}

	again, err := f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{
		OnCollision: core.CollisionRename, ProjectRef: "acme",
	})
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if again.Outcomes[0].NewKey != "risk-2" {
		t.Errorf("a colliding field renamed to %q, want risk-2", again.Outcomes[0].NewKey)
	}
}

func TestImportTagCollisionPolicies(t *testing.T) {
	f := newBundleFixture(t)
	doc := `{"record":"header","header":{"name":"tags","version":1}}` + "\n" +
		`{"record":"component","component":{"kind":"tag","tag":{"name":"backend","color":"#123456"}}}` + "\n"

	res, err := f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionRename})
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	if res.Outcomes[0].NewKey != "backend-2" {
		t.Errorf("a colliding tag renamed to %q, want backend-2", res.Outcomes[0].NewKey)
	}
	if !hasTag(t, f.l, f.ctx, "backend-2") {
		t.Error("the renamed tag is missing")
	}

	res, err = f.l.ImportBundle(f.ctx, strings.NewReader(doc), core.BundleImportInput{OnCollision: core.CollisionSkip})
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	if res.Outcomes[0].Action != core.ActionSkipped || res.Outcomes[0].Reason == "" {
		t.Errorf("outcome = %+v, want a skip with a reason", res.Outcomes[0])
	}
}

func TestImportWebhookCollisionPolicies(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentWebhook}})

	res, err := f.l.ImportBundle(f.ctx, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionSkip})
	if err != nil {
		t.Fatalf("skip: %v", err)
	}
	if res.Outcomes[0].Action != core.ActionSkipped {
		t.Fatalf("outcome = %+v, want a skip", res.Outcomes[0])
	}

	res, err = f.l.ImportBundle(f.ctx, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionReplace})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if res.Outcomes[0].Action != core.ActionUpdated {
		t.Fatalf("outcome = %+v, want an update", res.Outcomes[0])
	}
	endpoints, err := f.l.ListWebhooks(f.ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("replace created %d endpoints, want 1", len(endpoints))
	}
	if !endpoints[0].Active {
		t.Error("replacing an endpoint that already has a secret must not deactivate it")
	}

	res, err = f.l.ImportBundle(f.ctx, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionRename})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if res.Outcomes[0].Action != core.ActionCreated || len(res.Warnings) == 0 {
		t.Errorf("outcome = %+v, warnings = %v; want a second endpoint and an explanation", res.Outcomes[0], res.Warnings)
	}
	endpoints, err = f.l.ListWebhooks(f.ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(endpoints) != 2 {
		t.Errorf("rename produced %d endpoints, want 2", len(endpoints))
	}
}

func TestImportProjectTemplateIntoTenantWithoutItsWorkflow(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{
		Name: "template", Kinds: []core.ComponentKind{core.ComponentProject},
	})
	other := newBundleTenant(t, f.l)

	res, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionReplace})
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	kinds := map[core.ComponentKind]core.ComponentAction{}
	for _, out := range res.Outcomes {
		kinds[out.Kind] = out.Action
	}
	if kinds[core.ComponentWorkflow] != core.ActionCreated {
		t.Errorf("the template's workflow was not reported as created: %+v", res.Outcomes)
	}
	if kinds[core.ComponentProject] != core.ActionCreated {
		t.Errorf("the template was not created: %+v", res.Outcomes)
	}
	if !hasTag(t, f.l, other, "backend") {
		t.Error("the template's tags did not arrive")
	}

	replaced, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionReplace})
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	for _, out := range replaced.Outcomes {
		if out.Kind == core.ComponentProject && out.Action != core.ActionUpdated {
			t.Errorf("re-importing a template = %+v, want an update", out)
		}
	}

	renamed, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionRename})
	if err != nil {
		t.Fatalf("rename import: %v", err)
	}
	for _, out := range renamed.Outcomes {
		if out.Kind == core.ComponentProject && out.NewKey != "acme-2" {
			t.Errorf("renaming a template produced %q, want acme-2", out.NewKey)
		}
	}
	skipped, err := f.l.ImportBundle(other, bytes.NewReader(raw), core.BundleImportInput{OnCollision: core.CollisionSkip})
	if err != nil {
		t.Fatalf("skip import: %v", err)
	}
	if len(skipped.Outcomes) != 1 || skipped.Outcomes[0].Action != core.ActionSkipped {
		t.Errorf("skipping a template = %+v, want one skip and no nested work", skipped.Outcomes)
	}
}

func TestImportEmptyBundleDoesNothing(t *testing.T) {
	f := newBundleFixture(t)
	beforeEvents, beforeAudits := countRows(t, f.l, f.scope)

	res, err := f.l.ImportBundle(f.ctx, strings.NewReader(`{"record":"header","header":{"version":1}}`+"\n"),
		core.BundleImportInput{OnCollision: core.CollisionSkip})
	if err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	if len(res.Outcomes) != 0 {
		t.Errorf("outcomes = %+v, want none", res.Outcomes)
	}
	afterEvents, afterAudits := countRows(t, f.l, f.scope)
	if afterEvents != beforeEvents || afterAudits != beforeAudits {
		t.Error("an empty bundle wrote something")
	}
}

func TestFreeKeyGivesUp(t *testing.T) {
	_, err := freeKey("busy", func(string) (bool, error) { return true, nil })
	if !core.IsKind(err, core.KindConflict) {
		t.Errorf("freeKey with every key taken = %v, want a conflict", err)
	}
	if _, err := freeKey("busy", func(string) (bool, error) {
		return false, core.Internal("store is down")
	}); !core.IsKind(err, core.KindInternal) {
		t.Errorf("freeKey with a failing store = %v, want the failure surfaced", err)
	}
}

func TestBundleLabelFallsBackToKey(t *testing.T) {
	if got := bundleLabel("  ", "key"); got != "key" {
		t.Errorf("bundleLabel = %q, want the key", got)
	}
	if got := bundleLabel("Name", "key"); got != "Name" {
		t.Errorf("bundleLabel = %q, want the name", got)
	}
}

func TestCollisionScanCoversEveryKind(t *testing.T) {
	f := newBundleFixture(t)
	cases := []struct {
		kind core.ComponentKind
		key  string
	}{
		{core.ComponentWorkflow, "dev"},
		{core.ComponentFieldDef, "points"},
		{core.ComponentTag, "backend"},
		{core.ComponentProject, "acme"},
		{core.ComponentWebhook, "https://hooks.example.com/tix"},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{tc.kind}})
			_, err := f.l.ImportBundle(f.ctx, bytes.NewReader(raw), core.BundleImportInput{ProjectRef: "acme"})
			if !core.IsKind(err, core.KindConflict) {
				t.Fatalf("importing an existing %s with no policy = %v, want a conflict", tc.kind, err)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error %v does not name %q", err, tc.key)
			}
		})
	}
}

func TestExportSelectedWebhookByID(t *testing.T) {
	f := newBundleFixture(t)
	endpoints, err := f.l.ListWebhooks(f.ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	raw := f.export(t, core.BundleExportInput{WebhookIDs: []string{endpoints[0].ID}})
	comps := f.components(t, raw)
	if len(comps) != 1 || comps[0].Kind != core.ComponentWebhook {
		t.Fatalf("selecting a webhook exported %v", bundleKindsOf(comps))
	}
	if err := f.l.ExportBundle(f.ctx, core.BundleExportInput{WebhookIDs: []string{"nope"}}, &bytes.Buffer{}); err == nil {
		t.Error("selecting an unknown endpoint must be refused")
	}
	if err := f.l.ExportBundle(f.ctx, core.BundleExportInput{WorkflowKeys: []string{"nope"}}, &bytes.Buffer{}); err == nil {
		t.Error("selecting an unknown workflow must be refused")
	}
}

func TestBundleCarriesProjectAppearance(t *testing.T) {
	f := newBundleFixture(t)
	if _, err := f.l.UpdateProject(f.ctx, "acme", core.UpdateProjectInput{
		Color: strPtr("teal"), Icon: strPtr("AC"),
	}); err != nil {
		t.Fatalf("update project: %v", err)
	}

	raw := f.export(t, core.BundleExportInput{Kinds: []core.ComponentKind{core.ComponentProject}})
	comps := f.components(t, raw)
	if len(comps) != 1 || comps[0].Project == nil {
		t.Fatalf("exported %d components, want one template", len(comps))
	}
	if comps[0].Project.Color != core.ColorTeal || comps[0].Project.Icon != "AC" {
		t.Errorf("template appearance = %q/%q, want teal/AC", comps[0].Project.Color, comps[0].Project.Icon)
	}

	other := newBundleTenant(t, f.l)
	if _, err := f.l.ImportBundle(other, bytes.NewReader(raw),
		core.BundleImportInput{OnCollision: core.CollisionReplace}); err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	got, err := f.l.GetProject(other, "acme")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if got.Color != core.ColorTeal {
		t.Errorf("imported colour = %q, want teal", got.Color)
	}
	if got.Icon != "AC" {
		t.Errorf("imported icon = %q, want AC", got.Icon)
	}
}

func TestBundleRefusesAnAppearanceOutsideThePalette(t *testing.T) {
	f := newBundleFixture(t)
	raw := `{"record":"header","header":{"version":1}}` + "\n" +
		`{"record":"component","component":{"kind":"project_template","project":{"key":"acme","color":"chartreuse"}}}` + "\n"
	_, err := f.l.ImportBundle(f.ctx, strings.NewReader(raw),
		core.BundleImportInput{OnCollision: core.CollisionReplace})
	if err == nil || !strings.Contains(err.Error(), "palette") {
		t.Fatalf("import = %v, want a palette complaint", err)
	}
}
