package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// seeded names what a seeded tenant contains, so a test can assert against it.
type seeded struct {
	project string
	first   core.TaskRef
	second  core.TaskRef
}

// seed fills a tenant with one workflow, one project and two linked tasks.
func seed(t *testing.T, l *Local, ctx context.Context, key string) seeded {
	t.Helper()
	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{
		Key:        "flow",
		Name:       "Flow",
		Definition: BuiltinWorkflow(),
	}); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{
		Key: key, Name: strings.ToUpper(key), WorkflowKey: "flow",
	}); err != nil {
		t.Fatalf("project: %v", err)
	}
	if _, err := l.PutFieldDef(ctx, key, core.FieldDefInput{
		Key: "points", Label: "Points", Type: core.FieldInt, Position: 1,
	}); err != nil {
		t.Fatalf("field: %v", err)
	}

	first, err := l.CreateTask(ctx, core.CreateTaskInput{
		ProjectRef: key, Title: "first", Body: "body one", Tags: []string{"ops"},
		Priority: core.PriorityHigh, CustomFields: map[string]any{"points": 3},
	})
	if err != nil {
		t.Fatalf("first task: %v", err)
	}
	second, err := l.CreateTask(ctx, core.CreateTaskInput{
		ProjectRef: key, Title: "second", ParentRef: first.Ref, DependsOn: []string{first.Ref},
	})
	if err != nil {
		t.Fatalf("second task: %v", err)
	}
	if _, err := l.AddComment(ctx, core.TaskRef{ID: first.ID}, "a comment"); err != nil {
		t.Fatalf("comment: %v", err)
	}
	if _, err := l.PutArtifact(ctx, core.TaskRef{ID: first.ID}, core.ArtifactInput{
		Kind: core.ArtifactResult, Name: "result", Payload: map[string]any{"ok": true},
	}); err != nil {
		t.Fatalf("artifact: %v", err)
	}
	return seeded{
		project: key,
		first:   core.TaskRef{ID: first.ID},
		second:  core.TaskRef{ID: second.ID},
	}
}

// newTenant adds a second tenant to the same store and returns an actor in it.
func newTenant(t *testing.T, l *Local, key string) (context.Context, core.TenantScope, *core.Actor) {
	t.Helper()
	ctx := context.Background()
	tenant := core.Tenant{Key: key, Name: key}
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating tenant %q: %v", key, err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	actor := core.Actor{Kind: core.ActorUser, Handle: "operator-" + key, Scopes: []core.Scope{core.ScopeAll}}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("creating actor: %v", err)
	}
	actor.TenantID = tenant.ID
	return core.WithActor(ctx, &actor), scope, &actor
}

// exportBytes exports a tenant and returns the snapshot.
func exportBytes(t *testing.T, l *Local, ctx context.Context, in core.ExportInput) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := l.ExportTo(ctx, in, &buf); err != nil {
		t.Fatalf("export: %v", err)
	}
	return buf.Bytes()
}

// kindsOf counts the record kinds in a snapshot.
func kindsOf(t *testing.T, snapshot []byte) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, line := range strings.Split(strings.TrimRight(string(snapshot), "\n"), "\n") {
		if line == "" {
			continue
		}
		var rec core.SnapshotRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("snapshot line %q: %v", line, err)
		}
		out[string(rec.Kind)]++
	}
	return out
}

func TestExportWritesEveryKindInOrder(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")

	snapshot := exportBytes(t, l, ctx, core.ExportInput{IncludeComments: true, IncludeArtifacts: true})
	counts := kindsOf(t, snapshot)
	for _, kind := range []string{"header", "workflow", "project", "field_def", "tag", "task", "dependency", "comment", "artifact"} {
		if counts[kind] == 0 {
			t.Errorf("snapshot has no %s record: %v", kind, counts)
		}
	}
	if counts["header"] != 1 {
		t.Errorf("snapshot has %d header records, want 1", counts["header"])
	}
	if counts["task"] != 2 {
		t.Errorf("snapshot has %d task records, want 2", counts["task"])
	}
}

func TestExportIsByteIdenticalWhenNothingChanged(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")

	in := core.ExportInput{IncludeComments: true, IncludeArtifacts: true}
	first := exportBytes(t, l, ctx, in)
	second := exportBytes(t, l, ctx, in)
	if !bytes.Equal(first, second) {
		t.Errorf("repeated export differed:\n%s\n%s", first, second)
	}
}

func TestExportOneChangeProducesASmallDiff(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")

	before := strings.Split(string(exportBytes(t, l, ctx, core.ExportInput{})), "\n")
	title := "renamed"
	if _, err := l.UpdateTask(ctx, s.first, core.UpdateTaskInput{Title: &title}); err != nil {
		t.Fatalf("update: %v", err)
	}
	after := strings.Split(string(exportBytes(t, l, ctx, core.ExportInput{})), "\n")

	if len(before) != len(after) {
		t.Fatalf("line count changed from %d to %d", len(before), len(after))
	}
	changed := 0
	for i := range before {
		if before[i] != after[i] {
			changed++
		}
	}
	if changed != 1 {
		t.Errorf("%d lines changed, want 1", changed)
	}
}

func TestExportHonoursProjectSelectionAndAttachments(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "web", Name: "Web", WorkflowKey: "flow"}); err != nil {
		t.Fatalf("second project: %v", err)
	}
	if _, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "web", Title: "elsewhere"}); err != nil {
		t.Fatalf("second project task: %v", err)
	}

	tests := []struct {
		name  string
		in    core.ExportInput
		check func(*testing.T, map[string]int, string)
	}{
		{
			name: "everything",
			in:   core.ExportInput{IncludeComments: true, IncludeArtifacts: true},
			check: func(t *testing.T, counts map[string]int, raw string) {
				if counts["project"] != 2 || counts["task"] != 3 {
					t.Errorf("counts = %v", counts)
				}
			},
		},
		{
			name: "one project",
			in:   core.ExportInput{ProjectRefs: []string{"web"}},
			check: func(t *testing.T, counts map[string]int, raw string) {
				if counts["project"] != 1 || counts["task"] != 1 {
					t.Errorf("counts = %v", counts)
				}
				if strings.Contains(raw, "\"first\"") {
					t.Error("a filtered export carried another project's task")
				}
			},
		},
		{
			name: "attachments excluded by default",
			in:   core.ExportInput{},
			check: func(t *testing.T, counts map[string]int, raw string) {
				if counts["comment"] != 0 || counts["artifact"] != 0 {
					t.Errorf("counts = %v", counts)
				}
			},
		},
		{
			name: "comments only",
			in:   core.ExportInput{IncludeComments: true},
			check: func(t *testing.T, counts map[string]int, raw string) {
				if counts["comment"] == 0 || counts["artifact"] != 0 {
					t.Errorf("counts = %v", counts)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := exportBytes(t, l, ctx, tc.in)
			tc.check(t, kindsOf(t, snapshot), string(snapshot))
		})
	}
}

// A filtered export must not name a task it does not carry, so a dependency or
// parent reaching outside the filter is dropped rather than left dangling.
func TestFilteredExportDropsReferencesOutsideTheFilter(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "web", Name: "Web", WorkflowKey: "flow"}); err != nil {
		t.Fatalf("second project: %v", err)
	}
	outside, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "web", Title: "outside"})
	if err != nil {
		t.Fatalf("outside task: %v", err)
	}
	if err := l.AddDependency(ctx, s.second, core.TaskRef{ID: outside.ID}); err != nil {
		t.Fatalf("dependency: %v", err)
	}

	snapshot := string(exportBytes(t, l, ctx, core.ExportInput{ProjectRefs: []string{"infra"}}))
	if strings.Contains(snapshot, outside.ID) {
		t.Errorf("filtered snapshot names the out-of-scope task %q", outside.ID)
	}
	if counts := kindsOf(t, []byte(snapshot)); counts["dependency"] != 1 {
		t.Errorf("dependency count = %d, want the in-scope edge only", counts["dependency"])
	}
}

// A snapshot travels between machines, so nothing secret may ride along.
func TestExportCarriesNoSecrets(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")

	const planted = "planted-secret-value"
	if _, err := l.CreateUser(ctx, core.CreateUserInput{
		Email: "person@example.com", Password: planted + "-Aa1!",
	}); err != nil {
		t.Fatalf("user: %v", err)
	}
	if _, err := l.PutWebhook(ctx, core.WebhookInput{
		URL: "https://example.com/hook", Secret: planted, EventTypes: []string{"*"},
	}); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	issued, err := l.CreateToken(ctx, core.CreateTokenInput{Name: "agent", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	snapshot := string(exportBytes(t, l, ctx, core.ExportInput{IncludeComments: true, IncludeArtifacts: true}))
	for _, needle := range []string{planted, issued.Token, "password", "secret", "token_hash", "$argon2"} {
		if strings.Contains(snapshot, needle) {
			t.Errorf("snapshot contains %q", needle)
		}
	}
}

func TestExportAndImportRequireTheirScope(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	viewer := &core.Actor{ID: "viewer01", TenantID: scope.TenantID, Kind: core.ActorUser, Role: core.RoleViewer}
	ctx := core.WithActor(context.Background(), viewer)

	if err := l.ExportTo(ctx, core.ExportInput{}, io.Discard); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("export as a viewer = %v, want forbidden", err)
	}
	_, err := l.ImportFrom(ctx, strings.NewReader(""), core.ImportInput{Mode: core.ImportMerge})
	if !core.IsKind(err, core.KindForbidden) {
		t.Errorf("import as a viewer = %v, want forbidden", err)
	}
}

// Export walks the data and writes as it goes: the first records reach the
// reader long before the export has finished reading the tenant.
func TestExportWritesBeforeItFinishesReading(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	body := strings.Repeat("x", 512)
	for range 40 {
		if _, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "bulk", Body: body}); err != nil {
			t.Fatalf("bulk task: %v", err)
		}
	}

	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := l.ExportTo(ctx, core.ExportInput{}, pw)
		_ = pw.CloseWithError(err)
		done <- err
	}()

	reader := bufio.NewReader(pr)
	for range 2 {
		if _, err := reader.ReadBytes('\n'); err != nil {
			t.Fatalf("reading an early record: %v", err)
		}
	}
	select {
	case err := <-done:
		t.Fatalf("export finished before the reader had taken two records: %v", err)
	default:
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("draining: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("export: %v", err)
	}
}

func TestImportRefusesUnsupportedAndMissingVersions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"unsupported", `{"kind":"header","header":{"version":99,"tenant_key":"acme"}}` + "\n", "99"},
		{"missing", `{"kind":"header","header":{"tenant_key":"acme"}}` + "\n", "no format version"},
		{"empty document", "", "empty"},
		{"not a snapshot", "hello\n", "not a valid record"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, _, scope, actor := newLocal(t)
			ctx := core.WithActor(context.Background(), actor)
			events, audits := countRows(t, l, scope)

			_, err := l.ImportFrom(ctx, strings.NewReader(tc.input), core.ImportInput{Mode: core.ImportMerge})
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("import = %v, want invalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
			if gotEvents, gotAudits := countRows(t, l, scope); gotEvents != events || gotAudits != audits {
				t.Errorf("a refused import wrote %d events and %d audit entries",
					gotEvents-events, gotAudits-audits)
			}
		})
	}
}

func TestImportRequiresAMode(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	_, err := l.ImportFrom(ctx, strings.NewReader(""), core.ImportInput{})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("import without a mode = %v, want invalid", err)
	}
	if !strings.Contains(err.Error(), "mode") {
		t.Errorf("error %q does not say a mode is required", err)
	}
}

// A snapshot must round trip: the content that came out goes back in, and
// comes out again the same.
func TestRoundTripIntoAFreshTenant(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	in := core.ExportInput{IncludeComments: true, IncludeArtifacts: true}
	first := exportBytes(t, l, source, in)

	target, _, _ := newTenant(t, l, "beta")
	result, err := l.ImportFrom(target, bytes.NewReader(first), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Created["task"] != 2 || result.Created["project"] != 1 {
		t.Errorf("created = %v", result.Created)
	}

	second := exportBytes(t, l, target, in)
	if want, got := kindsOf(t, first), kindsOf(t, second); len(want) != len(got) {
		t.Fatalf("round trip changed the record kinds: %v then %v", want, got)
	}
	for kind, n := range kindsOf(t, first) {
		if kindsOf(t, second)[kind] != n {
			t.Errorf("round trip changed %s count: %d then %d", kind, n, kindsOf(t, second)[kind])
		}
	}
	if !strings.Contains(string(second), `"points"`) {
		t.Error("round trip lost the custom field value")
	}
	if !strings.Contains(string(second), `"ops"`) {
		t.Error("round trip lost the task tag")
	}
}

func TestRoundTripPreservesRelationships(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	target, _, _ := newTenant(t, l, "beta")
	if _, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import: %v", err)
	}

	page, err := l.ListTasks(target, core.TaskFilter{})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(page.Tasks) != 2 {
		t.Fatalf("imported %d tasks, want 2", len(page.Tasks))
	}
	var parent, child core.Task
	for _, task := range page.Tasks {
		if task.Title == "first" {
			parent = task
		} else {
			child = task
		}
	}
	if child.ParentID != parent.ID {
		t.Errorf("child parent = %q, want %q", child.ParentID, parent.ID)
	}
	if len(child.DependsOn) != 1 || child.DependsOn[0] != parent.ID {
		t.Errorf("child dependencies = %v, want %q", child.DependsOn, parent.ID)
	}
}

func TestImportIntoAnotherTenantWritesOnlyThere(t *testing.T) {
	l, _, sourceScope, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	target, _, _ := newTenant(t, l, "beta")
	before := exportBytes(t, l, source, core.ExportInput{})
	if _, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportReplace}); err != nil {
		t.Fatalf("import: %v", err)
	}
	if after := exportBytes(t, l, source, core.ExportInput{}); !bytes.Equal(before, after) {
		t.Error("importing into another tenant changed the source tenant")
	}

	page, err := l.ListTasks(target, core.TaskFilter{})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	for _, task := range page.Tasks {
		if task.TenantID == sourceScope.TenantID {
			t.Errorf("task %q landed in the source tenant", task.ID)
		}
	}
}

// A snapshot names a tenant, and that name must not be able to redirect the
// write out of the tenant the caller is authorized for.
func TestSnapshotCannotRedirectTheTargetTenant(t *testing.T) {
	l, _, sourceScope, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	target, targetScope, _ := newTenant(t, l, "beta")

	snapshot := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n" +
		`{"kind":"workflow","workflow":{"id":"wfsnapshot1","tenant_id":"` + sourceScope.TenantID + `","key":"flow","name":"Flow","definition":{"initial":"todo","states":[{"key":"todo"},{"key":"done","terminal":true}],"transitions":[{"from":"todo","to":"done"}]}}}` + "\n" +
		`{"kind":"project","project":{"id":"prjsnapshot1","tenant_id":"` + sourceScope.TenantID + `","key":"smuggled","name":"Smuggled","workflow_id":"wfsnapshot1"}}` + "\n"

	if _, err := l.ImportFrom(target, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import: %v", err)
	}
	if _, err := l.GetProject(source, "smuggled"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the snapshot wrote into the tenant it named: %v", err)
	}
	p, err := l.GetProject(target, "smuggled")
	if err != nil {
		t.Fatalf("project in the authorized tenant: %v", err)
	}
	if p.TenantID != targetScope.TenantID {
		t.Errorf("project tenant = %q, want the authorized tenant %q", p.TenantID, targetScope.TenantID)
	}
}

func TestImportRemapsCollidingIdentifiers(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	result, err := l.ImportFrom(source, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Updated["task"] != 2 {
		t.Errorf("re-importing into the same tenant created rows instead of updating: %v", result)
	}

	rewritten := strings.ReplaceAll(string(snapshot), `"key":"infra"`, `"key":"clone"`)
	rewritten = strings.ReplaceAll(rewritten, `"name":"INFRA"`, `"name":"Clone"`)
	result, err = l.ImportFrom(source, strings.NewReader(rewritten), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import of the clone: %v", err)
	}
	if result.Created["project"] != 1 || result.Created["task"] != 2 {
		t.Errorf("clone import = %v", result)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("colliding identifiers were remapped without being reported")
	}
	for _, w := range result.Warnings {
		if !strings.HasPrefix(w, "remapped ") {
			t.Errorf("warning %q does not report a remapping", w)
		}
	}
	page, err := l.ListTasks(source, core.TaskFilter{ProjectKeys: []string{"clone"}})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(page.Tasks) != 2 {
		t.Fatalf("clone has %d tasks, want 2", len(page.Tasks))
	}
	for _, task := range page.Tasks {
		if task.Title == "second" && task.ParentID == "" {
			t.Error("the remapped clone lost its parent link")
		}
	}
}

func TestMergeLeavesUnrelatedDataAloneAndReplaceRemovesIt(t *testing.T) {
	tests := []struct {
		name      string
		mode      core.ImportMode
		wantOther bool
	}{
		{"merge keeps", core.ImportMerge, true},
		{"replace removes", core.ImportReplace, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, _, _, actor := newLocal(t)
			ctx := core.WithActor(context.Background(), actor)
			seed(t, l, ctx, "infra")
			snapshot := exportBytes(t, l, ctx, core.ExportInput{})

			extra, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "extra"})
			if err != nil {
				t.Fatalf("extra task: %v", err)
			}
			if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "web", Name: "Web", WorkflowKey: "flow"}); err != nil {
				t.Fatalf("other project: %v", err)
			}

			result, err := l.ImportFrom(ctx, bytes.NewReader(snapshot), core.ImportInput{Mode: tc.mode})
			if err != nil {
				t.Fatalf("import: %v", err)
			}
			_, err = l.GetTask(ctx, core.TaskRef{ID: extra.ID})
			switch {
			case tc.wantOther && err != nil:
				t.Errorf("merge removed an unreferenced task: %v", err)
			case !tc.wantOther && !core.IsKind(err, core.KindNotFound):
				t.Errorf("replace left an absent task behind: %v", err)
			}
			if !tc.wantOther && result.Deleted["task"] != 1 {
				t.Errorf("replace reported %v deletions, want one task", result.Deleted)
			}
			if _, err := l.GetProject(ctx, "web"); err != nil {
				t.Errorf("a project outside the snapshot was disturbed: %v", err)
			}
		})
	}
}

func TestReplaceRemovesFieldDefinitionsAbsentFromTheSnapshot(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	snapshot := exportBytes(t, l, ctx, core.ExportInput{})

	if _, err := l.PutFieldDef(ctx, "infra", core.FieldDefInput{
		Key: "later", Label: "Later", Type: core.FieldString, Position: 2,
	}); err != nil {
		t.Fatalf("field: %v", err)
	}
	if _, err := l.ImportFrom(ctx, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportReplace}); err != nil {
		t.Fatalf("import: %v", err)
	}
	defs, err := l.ListFieldDefs(ctx, "infra")
	if err != nil {
		t.Fatalf("listing definitions: %v", err)
	}
	for _, d := range defs {
		if d.Key == "later" {
			t.Error("replace kept a field definition the snapshot did not carry")
		}
	}
}

func TestDryRunWritesNothingAndPredictsTheRealRun(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	snapshot := exportBytes(t, l, source, core.ExportInput{IncludeComments: true, IncludeArtifacts: true})

	target, targetScope, _ := newTenant(t, l, "beta")
	events, audits := countRows(t, l, targetScope)

	dry, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !dry.DryRun {
		t.Error("the result does not report itself as a dry run")
	}
	if gotEvents, gotAudits := countRows(t, l, targetScope); gotEvents != events || gotAudits != audits {
		t.Fatalf("a dry run wrote %d events and %d audit entries", gotEvents-events, gotAudits-audits)
	}
	page, err := l.ListTasks(target, core.TaskFilter{})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Fatalf("a dry run created %d tasks", len(page.Tasks))
	}

	real, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("real import: %v", err)
	}
	for kind, n := range real.Created {
		if dry.Created[kind] != n {
			t.Errorf("dry run predicted %d created %s, real import created %d", dry.Created[kind], kind, n)
		}
	}
	for kind, n := range real.Updated {
		if dry.Updated[kind] != n {
			t.Errorf("dry run predicted %d updated %s, real import updated %d", dry.Updated[kind], kind, n)
		}
	}
}

func TestDryRunReportsValidationErrors(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	snapshot := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n" +
		`{"kind":"project","project":{"id":"prjsnapshot1","key":"orphan","name":"Orphan","workflow_id":"missing"}}` + "\n"

	for _, dryRun := range []bool{true, false} {
		_, err := l.ImportFrom(ctx, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge, DryRun: dryRun})
		if !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("import with dry run %v = %v, want invalid", dryRun, err)
		}
		if !strings.Contains(err.Error(), "workflow") {
			t.Errorf("error %q does not name the broken reference", err)
		}
	}
}

func TestImportReportsEveryValidationError(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	project, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	flow, err := l.GetWorkflow(ctx, "flow")
	if err != nil {
		t.Fatalf("workflow: %v", err)
	}
	events, audits := countRows(t, l, scope)

	snapshot := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n" +
		`{"kind":"workflow","workflow":{"id":"` + flow.ID + `","key":"flow","name":"Flow","definition":{"initial":"todo","states":[{"key":"todo"},{"key":"done","terminal":true}],"transitions":[{"from":"todo","to":"done"}]}}}` + "\n" +
		`{"kind":"project","project":{"id":"` + project.ID + `","key":"infra","name":"INFRA","workflow_id":"` + flow.ID + `"}}` + "\n" +
		`{"kind":"task","task":{"id":"tsksnapshot01","project_id":"` + project.ID + `","seq":41,"title":"bad state","status":"nowhere"}}` + "\n" +
		`{"kind":"task","task":{"id":"tsksnapshot02","project_id":"` + project.ID + `","seq":42,"title":"","status":"todo"}}` + "\n"

	_, err = l.ImportFrom(ctx, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("import = %v, want invalid", err)
	}
	if !strings.Contains(err.Error(), "nowhere") || !strings.Contains(err.Error(), "no title") {
		t.Errorf("error %q does not report both problems", err)
	}
	if gotEvents, gotAudits := countRows(t, l, scope); gotEvents != events || gotAudits != audits {
		t.Errorf("a failed import left %d events and %d audit entries behind",
			gotEvents-events, gotAudits-audits)
	}
}

func TestImportRefusesBrokenStreams(t *testing.T) {
	head := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n"
	workflow := `{"kind":"workflow","workflow":{"id":"wfsnapshot1","key":"flow","name":"Flow","definition":{"initial":"todo","states":[{"key":"todo"},{"key":"done","terminal":true}],"transitions":[{"from":"todo","to":"done"}]}}}` + "\n"
	project := `{"kind":"project","project":{"id":"prjsnapshot1","key":"infra","name":"INFRA","workflow_id":"wfsnapshot1"}}` + "\n"

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"unknown kind", head + workflow + `{"kind":"gremlin","task":{"id":"t1"}}` + "\n", "unknown record kind"},
		{"truncated", head + workflow + project + `{"kind":"task","task":{"id":"tsksnapshot01","proj`, "line 4"},
		{"malformed line", head + workflow + "{oops\n", "line 3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, _, scope, actor := newLocal(t)
			ctx := core.WithActor(context.Background(), actor)
			events, audits := countRows(t, l, scope)

			_, err := l.ImportFrom(ctx, strings.NewReader(tc.input), core.ImportInput{Mode: core.ImportMerge})
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("import = %v, want invalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
			if _, err := l.GetProject(ctx, "infra"); !core.IsKind(err, core.KindNotFound) {
				t.Errorf("a broken stream committed its earlier records: %v", err)
			}
			if gotEvents, gotAudits := countRows(t, l, scope); gotEvents != events || gotAudits != audits {
				t.Errorf("a broken stream left %d events and %d audit entries behind",
					gotEvents-events, gotAudits-audits)
			}
		})
	}
}

func TestImportAuditsAndEmitsEventsAsTheSystemActor(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	target, targetScope, _ := newTenant(t, l, "beta")
	if _, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import: %v", err)
	}

	ctx := context.Background()
	var (
		entries []core.AuditEntry
		events  []core.Event
	)
	if err := l.store.View(ctx, targetScope, func(tx store.Tx) error {
		var err error
		entries, err = tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 500}})
		if err != nil {
			return err
		}
		events, err = tx.ReadEvents(ctx, 0, 500)
		return err
	}); err != nil {
		t.Fatalf("reading history: %v", err)
	}

	tasks, imports := 0, 0
	for _, e := range entries {
		if e.ActorID != core.SystemActor(targetScope.TenantID).ID {
			t.Errorf("audit entry %q is attributed to %q, not the system actor", e.Action, e.ActorID)
		}
		switch e.Action {
		case "task.create":
			tasks++
		case auditImport:
			imports++
		}
	}
	if tasks != 2 {
		t.Errorf("audited %d task creations, want 2", tasks)
	}
	if imports != 1 {
		t.Errorf("recorded %d import entries, want 1", imports)
	}
	completed := false
	for _, e := range events {
		if e.Type == core.EventImportCompleted {
			completed = true
		}
	}
	if !completed {
		t.Error("the import emitted no completion event")
	}
}

// The importer acts on the header while the rest of the snapshot is still
// unwritten: the writer below sends nothing more until the import has returned,
// so an importer that buffered the whole stream first would never finish.
func TestImportActsOnAnEarlyRecordBeforeALaterOneIsSent(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	pr, pw := io.Pipe()
	release := make(chan struct{})
	var sent sync.WaitGroup
	sent.Add(1)
	go func() {
		defer sent.Done()
		_, _ = pw.Write([]byte(`{"kind":"header","header":{"version":99,"tenant_key":"acme"}}` + "\n"))
		<-release
		_, _ = pw.Write([]byte(`{"kind":"project","project":{"key":"infra"}}` + "\n"))
		_ = pw.Close()
	}()

	_, err := l.ImportFrom(ctx, pr, core.ImportInput{Mode: core.ImportMerge})
	close(release)
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("import = %v, want the version refusal", err)
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error %q does not name the unsupported version", err)
	}
	_ = pr.CloseWithError(err)
	sent.Wait()
}

// The whole stream is consumed incrementally: each record is read and applied
// before the next one exists.
func TestImportConsumesRecordsOneAtATime(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	snapshot := exportBytes(t, l, source, core.ExportInput{})
	lines := strings.SplitAfter(strings.TrimRight(string(snapshot), "\n")+"\n", "\n")

	target, _, _ := newTenant(t, l, "beta")
	pr, pw := io.Pipe()
	consumed := make([]int, 0, len(lines))
	var mu sync.Mutex
	go func() {
		for idx, line := range lines {
			if line == "" {
				continue
			}
			if _, err := pw.Write([]byte(line)); err != nil {
				break
			}
			mu.Lock()
			consumed = append(consumed, idx)
			mu.Unlock()
		}
		_ = pw.Close()
	}()

	result, err := l.ImportFrom(target, pr, core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Created["task"] != 2 {
		t.Errorf("created = %v", result.Created)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(consumed) < 2 {
		t.Fatalf("the importer took %d records from the pipe", len(consumed))
	}
}

// failAfter fails once it has accepted n writes, so a test can interrupt an
// export at each stage of the walk.
type failAfter struct {
	n   int
	err error
}

func (f *failAfter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, f.err
	}
	f.n--
	return len(p), nil
}

func TestExportReportsAWriteFailureAtEveryStage(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")

	boom := errors.New("write failure")
	in := core.ExportInput{IncludeComments: true, IncludeArtifacts: true}
	for stage := range 9 {
		w := &failAfter{n: stage, err: boom}
		if err := l.ExportTo(ctx, in, w); !errors.Is(err, boom) {
			t.Errorf("export interrupted after %d records = %v, want the write failure", stage, err)
		}
	}
}

func TestExportPagesThroughLargeProjects(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	for range transferPage {
		if _, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "bulk"}); err != nil {
			t.Fatalf("bulk task: %v", err)
		}
	}

	counts := kindsOf(t, exportBytes(t, l, ctx, core.ExportInput{}))
	if counts["task"] != transferPage+2 {
		t.Errorf("exported %d tasks, want %d", counts["task"], transferPage+2)
	}
}

func TestReimportUpdatesAttachmentsInPlace(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	in := core.ExportInput{IncludeComments: true, IncludeArtifacts: true}
	snapshot := exportBytes(t, l, ctx, in)

	result, err := l.ImportFrom(ctx, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	for _, kind := range []string{"comment", "artifact", "task", "project", "workflow", "field_def", "tag"} {
		if result.Updated[kind] == 0 {
			t.Errorf("re-import created %s records instead of updating them: %v", kind, result)
		}
	}
	if result.Skipped["dependency"] != 1 {
		t.Errorf("re-import did not skip the existing dependency: %v", result.Skipped)
	}

	comments, err := l.ListComments(ctx, s.first)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("re-import left %d comments, want 1", len(comments))
	}
	artifacts, err := l.ListArtifacts(ctx, s.first)
	if err != nil {
		t.Fatalf("artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Errorf("re-import left %d artifacts, want 1", len(artifacts))
	}
	if counts := kindsOf(t, exportBytes(t, l, ctx, in)); counts["task"] != 2 {
		t.Errorf("re-import changed the task count to %d", counts["task"])
	}
}

func TestImportRejectsRecordsItCannotPlace(t *testing.T) {
	head := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n"
	flow := `{"kind":"workflow","workflow":{"id":"wfsnapshot1","key":"flow","name":"Flow","definition":{"initial":"todo","states":[{"key":"todo"},{"key":"done","terminal":true}],"transitions":[{"from":"todo","to":"done"}]}}}` + "\n"
	project := `{"kind":"project","project":{"id":"prjsnapshot1","key":"infra","name":"INFRA","workflow_id":"wfsnapshot1"}}` + "\n"
	task := `{"kind":"task","task":{"id":"tsksnapshot01","project_id":"prjsnapshot1","seq":1,"title":"one","status":"todo"}}` + "\n"

	tests := []struct {
		name  string
		lines string
		want  string
	}{
		{"workflow is not a state machine", head +
			`{"kind":"workflow","workflow":{"id":"wf2","key":"broken","name":"Broken","definition":{"initial":"nowhere","states":[{"key":"todo"}]}}}` + "\n",
			"is not valid"},
		{"project key is unusable", head + flow +
			`{"kind":"project","project":{"id":"p2","key":"-bad-","name":"Bad","workflow_id":"wfsnapshot1"}}` + "\n",
			"project key"},
		{"field definition without a project", head + flow + project +
			`{"kind":"field_def","field_def":{"id":"fd1","project_id":"missing","key":"points","type":"int"}}` + "\n",
			"which the snapshot does not contain"},
		{"field definition without a key", head + flow + project +
			`{"kind":"field_def","field_def":{"id":"fd1","project_id":"prjsnapshot1","key":"","type":"int"}}` + "\n",
			"has no key"},
		{"field definition with an unknown type", head + flow + project +
			`{"kind":"field_def","field_def":{"id":"fd1","project_id":"prjsnapshot1","key":"points","type":"quantum"}}` + "\n",
			"unknown type"},
		{"tag without a name", head + flow + project +
			`{"kind":"tag","tag":{"id":"tag1","name":"  "}}` + "\n",
			"has no name"},
		{"tag of an unknown project", head + flow + project +
			`{"kind":"tag","tag":{"id":"tag1","project_id":"missing","name":"ops"}}` + "\n",
			"which the snapshot does not contain"},
		{"task of an unknown project", head + flow + project +
			`{"kind":"task","task":{"id":"t1","project_id":"missing","seq":1,"title":"one"}}` + "\n",
			"which the snapshot does not contain"},
		{"task with an undefined custom field", head + flow + project +
			`{"kind":"task","task":{"id":"tsksnapshot01","project_id":"prjsnapshot1","seq":1,"title":"one","status":"todo","custom_fields":{"nope":1}}}` + "\n",
			"is not defined"},
		{"task parented outside the snapshot", head + flow + project +
			`{"kind":"task","task":{"id":"tsksnapshot01","project_id":"prjsnapshot1","seq":1,"title":"one","status":"todo","parent_id":"missing"}}` + "\n",
			"has parent"},
		{"task parented to itself", head + flow + project +
			`{"kind":"task","task":{"id":"tsksnapshot01","project_id":"prjsnapshot1","seq":1,"title":"one","status":"todo","parent_id":"tsksnapshot01"}}` + "\n",
			"its own parent"},
		{"dependency on an unknown task", head + flow + project + task +
			`{"kind":"dependency","dependency":{"task_id":"tsksnapshot01","depends_on":"missing"}}` + "\n",
			"depends on task"},
		{"dependency from an unknown task", head + flow + project + task +
			`{"kind":"dependency","dependency":{"task_id":"missing","depends_on":"tsksnapshot01"}}` + "\n",
			"names task"},
		{"dependency on itself", head + flow + project + task +
			`{"kind":"dependency","dependency":{"task_id":"tsksnapshot01","depends_on":"tsksnapshot01"}}` + "\n",
			"depend on itself"},
		{"comment on an unknown task", head + flow + project + task +
			`{"kind":"comment","comment":{"id":"c1","task_id":"missing","body":"hi"}}` + "\n",
			"which the snapshot does not contain"},
		{"empty comment", head + flow + project + task +
			`{"kind":"comment","comment":{"id":"c1","task_id":"tsksnapshot01","body":"   "}}` + "\n",
			"comment body is required"},
		{"artifact on an unknown task", head + flow + project + task +
			`{"kind":"artifact","artifact":{"id":"a1","task_id":"missing","kind":"result"}}` + "\n",
			"which the snapshot does not contain"},
		{"artifact of an unknown kind", head + flow + project + task +
			`{"kind":"artifact","artifact":{"id":"a1","task_id":"tsksnapshot01","kind":"gremlin"}}` + "\n",
			"unknown kind"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, _, scope, actor := newLocal(t)
			ctx := core.WithActor(context.Background(), actor)
			events, audits := countRows(t, l, scope)

			_, err := l.ImportFrom(ctx, strings.NewReader(tc.lines), core.ImportInput{Mode: core.ImportMerge})
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("import = %v, want invalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
			if gotEvents, gotAudits := countRows(t, l, scope); gotEvents != events || gotAudits != audits {
				t.Errorf("a refused snapshot wrote %d events and %d audit entries",
					gotEvents-events, gotAudits-audits)
			}
		})
	}
}

// A snapshot that omits identifiers is still importable: the target assigns
// them, and a dry run predicts the same counts.
func TestImportAcceptsASnapshotWithoutIdentifiers(t *testing.T) {
	snapshot := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n" +
		`{"kind":"workflow","workflow":{"key":"flow","name":"Flow","definition":{"initial":"todo","states":[{"key":"todo"},{"key":"done","terminal":true}],"transitions":[{"from":"todo","to":"done"}]}}}` + "\n" +
		`{"kind":"project","project":{"key":"infra","name":"INFRA","workflow_id":""}}` + "\n"

	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	dry, err := l.ImportFrom(ctx, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	real, err := l.ImportFrom(ctx, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if dry.Created["project"] != real.Created["project"] {
		t.Errorf("dry run predicted %v, real import did %v", dry.Created, real.Created)
	}
	if _, err := l.GetProject(ctx, "infra"); err != nil {
		t.Errorf("project was not created: %v", err)
	}
}

// A snapshot carrying only tasks lands in a project the target already has, and
// the workflow those tasks are checked against comes from the store.
func TestImportChecksStatusesAgainstTheStoredWorkflow(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	project, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	snapshot := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n" +
		`{"kind":"project","project":{"id":"` + project.ID + `","key":"infra","name":"INFRA","workflow_id":"unknown"}}` + "\n" +
		`{"kind":"task","task":{"id":"tsksnapshot01","project_id":"` + project.ID + `","seq":50,"title":"fresh"}}` + "\n"

	result, err := l.ImportFrom(ctx, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Created["task"] != 1 {
		t.Errorf("created = %v", result.Created)
	}
	task, err := l.GetTask(ctx, core.TaskRef{ProjectKey: "infra", Seq: 50})
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	if task.Status != BuiltinWorkflow().Initial {
		t.Errorf("task status = %q, want the workflow's initial state", task.Status)
	}
}

func TestImportRefusesADependencyCycle(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	first, err := l.GetTask(ctx, s.first)
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	second, err := l.GetTask(ctx, s.second)
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	project, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	snapshot := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n" +
		`{"kind":"project","project":{"id":"` + project.ID + `","key":"infra","name":"INFRA","workflow_id":"` + project.WorkflowID + `"}}` + "\n" +
		`{"kind":"task","task":{"id":"` + first.ID + `","project_id":"` + project.ID + `","seq":1,"title":"first","status":"todo"}}` + "\n" +
		`{"kind":"task","task":{"id":"` + second.ID + `","project_id":"` + project.ID + `","seq":2,"title":"second","status":"todo"}}` + "\n" +
		`{"kind":"dependency","dependency":{"task_id":"` + first.ID + `","depends_on":"` + second.ID + `"}}` + "\n"

	_, err = l.ImportFrom(ctx, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("import = %v, want invalid", err)
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error %q does not mention the cycle", err)
	}
}

func TestReplaceDryRunCountsDeletionsWithoutMakingThem(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	snapshot := exportBytes(t, l, ctx, core.ExportInput{})
	extra, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "extra"})
	if err != nil {
		t.Fatalf("extra task: %v", err)
	}
	if _, err := l.PutFieldDef(ctx, "infra", core.FieldDefInput{
		Key: "later", Label: "Later", Type: core.FieldString, Position: 2,
	}); err != nil {
		t.Fatalf("field: %v", err)
	}

	result, err := l.ImportFrom(ctx, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportReplace, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if result.Deleted["task"] != 1 || result.Deleted["field_def"] != 1 {
		t.Errorf("dry run deletions = %v", result.Deleted)
	}
	if _, err := l.GetTask(ctx, core.TaskRef{ID: extra.ID}); err != nil {
		t.Errorf("a replace dry run deleted a task: %v", err)
	}
}
