package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
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
	// Nothing changed since the export, so both tasks are matched by ULID and
	// reported unchanged rather than created as duplicates.
	if result.Created["task"] != 0 || result.Skipped["task"] != 2 {
		t.Errorf("re-importing into the same tenant did not match the existing tasks by identifier: %v", result)
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

// gatedReader hands out its first chunk, then blocks inside the second Read
// until it is released, which is how a slow upload behaves.
type gatedReader struct {
	first, rest []byte
	stage       int
	entered     chan struct{}
	release     chan struct{}
}

func newGatedReader(first, rest string) *gatedReader {
	return &gatedReader{
		first: []byte(first), rest: []byte(rest),
		entered: make(chan struct{}), release: make(chan struct{}),
	}
}

func (g *gatedReader) Read(p []byte) (int, error) {
	switch g.stage {
	case 0:
		g.stage++
		return copy(p, g.first), nil
	case 1:
		g.stage++
		close(g.entered)
		<-g.release
		return copy(p, g.rest), nil
	default:
		return 0, io.EOF
	}
}

// The upload is drained before any transaction opens. An import holds one
// write transaction for its whole run, so reading the client inside it lets a
// slow upload hold the sqlite write lock against every other writer in the
// process. The reader below stalls after its first chunk: an importer that
// decided anything before reading again would return without ever asking for
// the rest.
func TestImportDrainsTheUploadBeforeDeciding(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	r := newGatedReader(
		`{"kind":"header","header":{"version":99,"tenant_key":"acme"}}`+"\n",
		`{"kind":"project","project":{"key":"infra"}}`+"\n")

	done := make(chan error, 1)
	go func() {
		_, err := l.ImportFrom(ctx, r, core.ImportInput{Mode: core.ImportMerge})
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("import returned %v after one read; the rest of the upload would be read inside the transaction", err)
	case <-r.entered:
	}
	close(r.release)

	err := <-done
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("import = %v, want the version refusal", err)
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error %q does not name the unsupported version", err)
	}
}

// A snapshot larger than the bound is refused rather than buffered.
func TestImportRefusesAnOversizedSnapshot(t *testing.T) {
	_, err := bufferSnapshot(io.LimitReader(zeros{}, maxSnapshotBytes+1))
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("bufferSnapshot = %v, want a refusal", err)
	}
}

// zeros is an endless reader, used to reach the snapshot size bound without
// holding the bytes anywhere.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = '\n'
	}
	return len(p), nil
}

// A snapshot too large for memory still imports, from a temporary file.
func TestBufferSnapshotSpillsToDisk(t *testing.T) {
	body := strings.Repeat("x", maxSnapshotMemory+1024)
	buffered, err := bufferSnapshot(strings.NewReader(body))
	if err != nil {
		t.Fatalf("bufferSnapshot: %v", err)
	}
	defer buffered.Close()
	if buffered.file == nil {
		t.Fatal("a snapshot past the memory bound stayed in memory")
	}
	got, err := io.ReadAll(buffered)
	if err != nil {
		t.Fatalf("reading the buffered snapshot: %v", err)
	}
	if string(got) != body {
		t.Fatalf("buffered %d bytes, want %d", len(got), len(body))
	}
	name := buffered.file.Name()
	if err := buffered.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Errorf("temporary file %q survived the import", name)
	}
}

// The whole upload is consumed, whatever pace it arrives at.
func TestImportConsumesTheWholeUpload(t *testing.T) {
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
	for _, kind := range []string{"comment", "artifact", "project", "workflow", "field_def", "tag"} {
		if result.Updated[kind] == 0 {
			t.Errorf("re-import created %s records instead of updating them: %v", kind, result)
		}
	}
	if result.Skipped["dependency"] != 1 {
		t.Errorf("re-import did not skip the existing dependency: %v", result.Skipped)
	}
	// Nothing about either task changed since the export, so a correctly
	// audited re-import reports them unchanged rather than as updates.
	if result.Created["task"] != 0 || result.Updated["task"] != 0 || result.Skipped["task"] != 2 {
		t.Errorf("re-import did not report the unchanged tasks accurately: %v", result)
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

func TestRoundTripPreservesProjectAppearance(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	if _, err := l.UpdateProject(source, "infra", core.UpdateProjectInput{
		Color: strPtr("violet"), Icon: strPtr("🚀"),
	}); err != nil {
		t.Fatalf("update project: %v", err)
	}
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	target, _, _ := newTenant(t, l, "beta")
	if _, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import: %v", err)
	}

	got, err := l.GetProject(target, "infra")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if got.Color != core.ColorViolet {
		t.Errorf("colour = %q, want violet", got.Color)
	}
	if got.Icon != "🚀" {
		t.Errorf("icon = %q, want 🚀", got.Icon)
	}
}

func strPtr(s string) *string { return &s }

// auditActionCounts tallies task audit entries by action, so a test can check
// what an import actually recorded rather than only how many rows changed.
func auditActionCounts(t *testing.T, l *Local, scope core.TenantScope) map[string]int {
	t.Helper()
	counts := map[string]int{}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 1000}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.SubjectType == "task" {
				counts[e.Action]++
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("reading audit: %v", err)
	}
	return counts
}

// Defect 1: export used to stream only live tasks, so importing an earlier
// snapshot into a database where the task had since been deleted resurrected
// it, with import unable to tell the deletion had happened. Deleted tasks
// must travel as tombstones, and applying an older tombstone against a newer
// local edit must not destroy that edit either: both directions of the
// UpdatedAt comparison are exercised below.

func TestExportIncludesSoftDeletedTasksAsTombstones(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	// second is a leaf: deleting it needs no cascade, and first's comment and
	// artifact stay live so the test can tell live attachments from a
	// tombstone's, which must not travel.
	if err := l.DeleteTask(ctx, s.second, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	snapshot := string(exportBytes(t, l, ctx, core.ExportInput{IncludeComments: true, IncludeArtifacts: true}))
	if counts := kindsOf(t, []byte(snapshot)); counts["task"] != 2 {
		t.Errorf("export dropped the deleted task: counts = %v", counts)
	}
	if !strings.Contains(snapshot, `"deleted_at"`) {
		t.Error("the exported tombstone does not carry a deletion time")
	}
	// first's comment and artifact are still live and must still travel.
	if counts := kindsOf(t, []byte(snapshot)); counts["comment"] == 0 || counts["artifact"] == 0 {
		t.Errorf("a live task's attachments were dropped alongside the tombstone: counts = %v", counts)
	}
}

func TestImportDoesNotResurrectATaskDeletedMoreRecentlyThanTheSnapshot(t *testing.T) {
	l, clk, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	stale := exportBytes(t, l, ctx, core.ExportInput{}) // the task is still live here

	clk.Advance(time.Hour)
	if err := l.DeleteTask(ctx, s.second, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	result, err := l.ImportFrom(ctx, bytes.NewReader(stale), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Created["task"] != 0 || result.Updated["task"] != 0 {
		t.Errorf("importing a stale live snapshot resurrected a deleted task: %v", result)
	}
	if result.Skipped["task"] == 0 {
		t.Error("the resurrection attempt was not reported as skipped")
	}
	if len(result.Warnings) == 0 {
		t.Error("the resurrection attempt was not explained in the warnings")
	}

	scope := core.TenantScope{TenantID: actor.TenantID}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		task, err := tx.GetTask(context.Background(), s.second)
		if err != nil {
			return err
		}
		if !task.Deleted() {
			t.Error("the task was resurrected")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading task: %v", err)
	}
}

func TestImportAppliesATombstoneUnlessTheLocalEditIsNewer(t *testing.T) {
	l, clk, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")

	clk.Advance(time.Hour)
	if err := l.DeleteTask(ctx, s.second, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	tombstone := exportBytes(t, l, ctx, core.ExportInput{})

	clk.Advance(time.Hour)
	if _, err := l.RestoreTask(ctx, s.second); err != nil {
		t.Fatalf("restore: %v", err)
	}
	edited := "revived and edited after the deletion"
	if _, err := l.UpdateTask(ctx, s.second, core.UpdateTaskInput{Title: &edited}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// The tombstone predates this edit, so applying it must not remove the
	// task: the edit is newer evidence than the deletion it would apply.
	result, err := l.ImportFrom(ctx, bytes.NewReader(tombstone), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Deleted["task"] != 0 {
		t.Errorf("an older tombstone deleted a task that was edited more recently: %v", result)
	}
	got, err := l.GetTask(ctx, s.second)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != edited {
		t.Errorf("title = %q, the newer edit was lost", got.Title)
	}
}

func TestRoundTripReproducesATombstoneInAnEmptyDatabase(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	s := seed(t, l, source, "infra")
	if err := l.DeleteTask(source, s.second, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	target, targetScope, _ := newTenant(t, l, "beta")
	result, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Deleted["task"] != 1 {
		t.Errorf("importing a tombstone into an empty tenant did not report a deletion: %v", result)
	}
	if result.Created["task"] != 1 {
		t.Errorf("the live task was not created: %v", result)
	}

	if err := l.store.View(context.Background(), targetScope, func(tx store.Tx) error {
		tasks, err := tx.ListTasks(context.Background(), core.TaskFilter{
			ProjectKeys: []string{"infra"}, IncludeDeleted: true,
		})
		if err != nil {
			return err
		}
		if len(tasks) != 2 {
			t.Fatalf("target has %d tasks, want 2 including the tombstone", len(tasks))
		}
		for _, task := range tasks {
			if task.Title == "second" && !task.Deleted() {
				t.Error("the tombstone was imported as a live task")
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("reading target: %v", err)
	}
}

// Defect 2: import used to match existing tasks by (project key, sequence
// number), which is only a per-project counter. Two databases can each mint
// "infra-1" for entirely unrelated tasks, and importing one into the other
// used to merge them silently. Identity is the task's ULID, and a match
// against a different project's row is treated as no match at all.

func TestImportMatchesTaskIdentityByULIDNotByProjectAndSequence(t *testing.T) {
	l, _, _, actorA := newLocal(t)
	a := core.WithActor(context.Background(), actorA)
	seed(t, l, a, "infra")
	snapshotA := exportBytes(t, l, a, core.ExportInput{})

	b, _, _ := newTenant(t, l, "beta")
	if _, err := l.PutWorkflow(b, core.WorkflowInput{
		Key: "flow", Name: "Flow", Definition: BuiltinWorkflow(),
	}); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if _, err := l.CreateProject(b, core.CreateProjectInput{Key: "infra", Name: "INFRA", WorkflowKey: "flow"}); err != nil {
		t.Fatalf("project: %v", err)
	}
	ownTask, err := l.CreateTask(b, core.CreateTaskInput{ProjectRef: "infra", Title: "tenant beta's own first task"})
	if err != nil {
		t.Fatalf("own task: %v", err)
	}
	if ownTask.Seq != 1 {
		t.Fatalf("test setup: tenant beta's own task is not seq 1")
	}

	result, err := l.ImportFrom(b, bytes.NewReader(snapshotA), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Created["task"] != 2 {
		t.Errorf("importing an unrelated snapshot did not create its own tasks: %v", result)
	}
	if len(result.Warnings) == 0 {
		t.Error("the sequence-number collision with tenant beta's own task was not reported")
	}

	got, err := l.GetTask(b, core.TaskRef{ID: ownTask.ID})
	if err != nil {
		t.Fatalf("own task: %v", err)
	}
	if got.Title != "tenant beta's own first task" {
		t.Errorf("the colliding project-and-sequence reference merged an unrelated task: title = %q", got.Title)
	}

	page, err := l.ListTasks(b, core.TaskFilter{ProjectKeys: []string{"infra"}})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(page.Tasks) != 3 {
		t.Fatalf("tenant beta has %d tasks, want 3 (its own plus the two imported)", len(page.Tasks))
	}
}

func TestImportCreatesATaskThatCarriesNoIdentifier(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	project, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	snapshot := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n" +
		`{"kind":"project","project":{"id":"` + project.ID + `","key":"infra","name":"INFRA","workflow_id":"` + project.WorkflowID + `"}}` + "\n" +
		`{"kind":"task","task":{"project_id":"` + project.ID + `","title":"no identifier","status":"todo"}}` + "\n"

	// Run it twice: with nothing to match on, a record with no identifier is
	// always a create, never merged with anything the first run produced.
	for range 2 {
		result, err := l.ImportFrom(ctx, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
		if err != nil {
			t.Fatalf("import: %v", err)
		}
		if result.Created["task"] != 1 {
			t.Errorf("task with no identifier: created = %v, want 1", result.Created)
		}
	}
	page, err := l.ListTasks(ctx, core.TaskFilter{ProjectKeys: []string{"infra"}})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	count := 0
	for _, task := range page.Tasks {
		if task.Title == "no identifier" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("two imports of an identifier-less task produced %d rows, want 2", count)
	}
}

// Defect 3: import never compared UpdatedAt, so an older snapshot imported
// over a newer database silently reverted it. Newer local data must win, and
// the caller must be able to see what was skipped.

func TestImportSkipsAnOlderTaskUpdateAndReportsIt(t *testing.T) {
	l, clk, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	stale := exportBytes(t, l, ctx, core.ExportInput{})

	clk.Advance(time.Hour)
	newer := "edited after the snapshot was taken"
	if _, err := l.UpdateTask(ctx, s.first, core.UpdateTaskInput{Title: &newer}); err != nil {
		t.Fatalf("update: %v", err)
	}

	result, err := l.ImportFrom(ctx, bytes.NewReader(stale), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Updated["task"] != 0 {
		t.Errorf("a stale snapshot overwrote newer local data: %v", result)
	}
	if result.Skipped["task"] == 0 {
		t.Errorf("a stale task update was not reported as skipped: %v", result)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("a stale skip was not explained in the warnings")
	}

	got, err := l.GetTask(ctx, s.first)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != newer {
		t.Errorf("title = %q, an older snapshot overwrote newer local data", got.Title)
	}
}

func TestImportDryRunReportsStaleSkipsWithoutWriting(t *testing.T) {
	l, clk, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	stale := exportBytes(t, l, ctx, core.ExportInput{})

	clk.Advance(time.Hour)
	newer := "edited after the snapshot was taken"
	if _, err := l.UpdateTask(ctx, s.first, core.UpdateTaskInput{Title: &newer}); err != nil {
		t.Fatalf("update: %v", err)
	}

	dry, err := l.ImportFrom(ctx, bytes.NewReader(stale), core.ImportInput{Mode: core.ImportMerge, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Skipped["task"] == 0 {
		t.Errorf("a dry run did not predict the stale skip: %v", dry)
	}
	got, err := l.GetTask(ctx, s.first)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != newer {
		t.Error("a dry run modified the task")
	}
}

// Defect 4: every touched task used to record a task.create audit entry and
// event, even when the import updated an existing task or changed nothing at
// all, so the audit log misrepresented what an import did.

// Every one of these imports a snapshot exported from a tenant back into that
// very same tenant, matching identifiers directly. Importing into a different
// tenant always assigns fresh identifiers on the very first import, because a
// snapshot never anonymises them and the source tenant's own rows are still
// there to collide with; that remapping is identifier remapping (already
// covered above), not a case a repeated identity match could ever hit, since
// nothing durably remembers a mapping across separate import runs.

func TestImportTaskAuditReflectsCreateUpdateDeleteAndNoChange(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	baseline := exportBytes(t, l, ctx, core.ExportInput{})

	afterCreate := auditActionCounts(t, l, scope)
	if afterCreate["task.create"] != 2 {
		t.Fatalf("seeding audited %d task creations, want 2", afterCreate["task.create"])
	}

	// Reimporting the very same, unchanged snapshot must add no task audit
	// entries at all: nothing about either task actually changed.
	if _, err := l.ImportFrom(ctx, bytes.NewReader(baseline), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("no-op reimport: %v", err)
	}
	afterNoop := auditActionCounts(t, l, scope)
	if total := afterNoop["task.create"] + afterNoop["task.update"] + afterNoop["task.delete"]; total != afterCreate["task.create"] {
		t.Errorf("a no-op reimport wrote %d task audit entries, want none beyond the original creations", total-afterCreate["task.create"])
	}

	// Hand-build a snapshot, timestamped after the tasks' current updated_at,
	// that edits the first task and deletes the second, and import it. The
	// tasks are addressed by their real identifiers so this exercises import
	// applying an update and a delete, not creating either from scratch.
	project, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	first, err := l.GetTask(ctx, core.TaskRef{ProjectKey: "infra", Seq: 1})
	if err != nil {
		t.Fatalf("first task: %v", err)
	}
	second, err := l.GetTask(ctx, core.TaskRef{ProjectKey: "infra", Seq: 2})
	if err != nil {
		t.Fatalf("second task: %v", err)
	}
	const later = `"2026-01-01T01:00:00Z"`
	snapshot := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n" +
		`{"kind":"project","project":{"id":"` + project.ID + `","key":"infra","name":"INFRA"}}` + "\n" +
		`{"kind":"task","task":{"id":"` + first.ID + `","project_id":"` + project.ID + `","seq":1,` +
		`"title":"edited by import","status":"` + first.Status + `","updated_at":` + later + `}}` + "\n" +
		`{"kind":"task","task":{"id":"` + second.ID + `","project_id":"` + project.ID + `","seq":2,` +
		`"title":"` + second.Title + `","status":"` + second.Status + `","updated_at":` + later + `,"deleted_at":` + later + `}}` + "\n"

	if _, err := l.ImportFrom(ctx, strings.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("second import: %v", err)
	}
	afterChange := auditActionCounts(t, l, scope)
	if afterChange["task.update"] == 0 {
		t.Errorf("the edited task was not audited as an update: %v", afterChange)
	}
	if afterChange["task.delete"] == 0 {
		t.Errorf("the deleted task was not audited as a delete: %v", afterChange)
	}
	if afterChange["task.create"] != afterCreate["task.create"] {
		t.Errorf("updating and deleting existing tasks was audited as %d creations", afterChange["task.create"])
	}
}

// Round-trip fidelity: reimporting the same snapshot twice must be a no-op
// the second time, and an export/import cycle into an empty database must
// reproduce the original.

func TestReimportingTheSameSnapshotTwiceIsANoOp(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	snapshot := exportBytes(t, l, ctx, core.ExportInput{IncludeComments: true, IncludeArtifacts: true})
	first := exportBytes(t, l, ctx, core.ExportInput{IncludeComments: true, IncludeArtifacts: true})

	result, err := l.ImportFrom(ctx, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	if result.Created["task"] != 0 {
		t.Errorf("reimporting the same snapshot created rows instead of matching the existing ones: %v", result)
	}
	second := exportBytes(t, l, ctx, core.ExportInput{IncludeComments: true, IncludeArtifacts: true})

	if want, got := kindsOf(t, first), kindsOf(t, second); len(want) != len(got) {
		t.Fatalf("a no-op reimport changed the record kinds: %v then %v", want, got)
	}
	for kind, n := range kindsOf(t, first) {
		if kindsOf(t, second)[kind] != n {
			t.Errorf("a no-op reimport changed %s count: %d then %d", kind, n, kindsOf(t, second)[kind])
		}
	}
}

// TestRoundTripIntoAFreshTenant (above) already covers export, import into an
// empty tenant, and export again reproducing the content.

// cyclicSnapshot rewrites an exported snapshot so its two tasks are each
// other's parent, which is the shape a hostile or corrupted snapshot carries.
func cyclicSnapshot(t *testing.T, snapshot []byte) []byte {
	t.Helper()
	lines := strings.Split(strings.TrimRight(string(snapshot), "\n"), "\n")
	var records []core.SnapshotRecord
	var tasks []int
	for _, line := range lines {
		var rec core.SnapshotRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("snapshot line %q: %v", line, err)
		}
		if rec.Kind == core.RecordTask {
			tasks = append(tasks, len(records))
		}
		records = append(records, rec)
	}
	if len(tasks) != 2 {
		t.Fatalf("snapshot carries %d tasks, want 2", len(tasks))
	}
	first := records[tasks[0]].Task
	second := records[tasks[1]].Task
	first.ParentID = second.ID
	second.ParentID = first.ID

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	for i := range records {
		if err := enc.Encode(records[i]); err != nil {
			t.Fatalf("re-encoding line %d: %v", i, err)
		}
	}
	return out.Bytes()
}

// A snapshot naming A the parent of B and B the parent of A used to apply
// verbatim, after which resolving a parent walked the cycle without end inside
// an open write transaction.
func TestImportRejectsAParentCycle(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")

	target, scope, _ := newTenant(t, l, "beta")
	snapshot := cyclicSnapshot(t, exportBytes(t, l, source, core.ExportInput{}))

	_, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("import = %v, want a refusal naming the cycle", err)
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error %q does not name the cycle", err)
	}

	var count int
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		tasks, err := tx.ListTasks(context.Background(), core.TaskFilter{
			IncludeDeleted: true, Page: core.Page{Limit: 100},
		})
		count = len(tasks)
		return err
	}); err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	if count != 0 {
		t.Errorf("the refused import left %d tasks behind", count)
	}
}

// A dry run decides everything a real run decides, so it reports the cycle too.
func TestImportDryRunReportsAParentCycle(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")

	target, _, _ := newTenant(t, l, "beta")
	snapshot := cyclicSnapshot(t, exportBytes(t, l, source, core.ExportInput{}))

	_, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{
		Mode: core.ImportMerge, DryRun: true,
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("dry run = %v, want a refusal naming the cycle", err)
	}
}

// Every mutation writes its rows, its audit entry and its event in one
// transaction. Imported tags used to write the row alone.
func TestImportedTagWritesAuditAndEvent(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	target, scope, _ := newTenant(t, l, "beta")
	if _, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import: %v", err)
	}

	ctx := context.Background()
	var tagAudits int
	var tagEvents int
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 1000}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Action == auditTagPut {
				tagAudits++
			}
		}
		events, err := tx.ReadEvents(ctx, 0, 1000)
		if err != nil {
			return err
		}
		for _, e := range events {
			if e.Type == core.EventLabelAdded && e.SubjectType == "tag" {
				tagEvents++
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the audit log: %v", err)
	}
	if tagAudits == 0 {
		t.Error("an imported tag wrote no audit entry")
	}
	if tagEvents == 0 {
		t.Error("an imported tag emitted no event")
	}
}
