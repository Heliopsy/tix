package httpapi_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func TestTaskMutationRoutes(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("first")
	other := f.createTask("second")

	dep := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/deps",
		httpapi.DependencyRequest{DependsOn: other.Ref})
	mustStatus(t, dep, http.StatusNoContent)
	_ = dep.Body.Close()

	badDep := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/deps",
		httpapi.DependencyRequest{DependsOn: "!!"})
	mustStatus(t, badDep, http.StatusBadRequest)
	_ = badDep.Body.Close()

	undep := f.call(http.MethodDelete, "/api/v1/tasks/"+task.Ref+"/deps/"+other.Ref, nil)
	mustStatus(t, undep, http.StatusNoContent)
	_ = undep.Body.Close()

	badUndep := f.call(http.MethodDelete, "/api/v1/tasks/"+task.Ref+"/deps/!!", nil)
	mustStatus(t, badUndep, http.StatusBadRequest)
	_ = badUndep.Body.Close()

	comment := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/comments",
		httpapi.CommentRequest{Body: "first pass"})
	mustStatus(t, comment, http.StatusCreated)
	var created core.Comment
	decodeBody(t, comment, &created)

	edited := f.call(http.MethodPatch, "/api/v1/comments/"+created.ID,
		httpapi.CommentRequest{Body: "second pass"})
	mustStatus(t, edited, http.StatusOK)
	var updated core.Comment
	decodeBody(t, edited, &updated)
	if updated.Body != "second pass" {
		t.Errorf("comment body = %q, want the edit", updated.Body)
	}

	removed := f.call(http.MethodDelete, "/api/v1/comments/"+created.ID, nil)
	mustStatus(t, removed, http.StatusNoContent)
	_ = removed.Body.Close()

	deleted := f.call(http.MethodDelete, "/api/v1/tasks/"+task.Ref, nil)
	mustStatus(t, deleted, http.StatusNoContent)
	_ = deleted.Body.Close()

	restored := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/restore", nil)
	mustStatus(t, restored, http.StatusOK)
	_ = restored.Body.Close()

	hard := f.call(http.MethodDelete, "/api/v1/tasks/"+task.Ref+"?hard=true", nil)
	mustStatus(t, hard, http.StatusNoContent)
	_ = hard.Body.Close()

	gone := f.call(http.MethodGet, "/api/v1/tasks/"+task.Ref, nil)
	mustStatus(t, gone, http.StatusNotFound)
	_ = gone.Body.Close()
}

func TestTaskListFilters(t *testing.T) {
	f := newFixture(t)
	f.createTask("filtered")

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"by project", "?project=infra", http.StatusOK},
		{"by status", "?status=todo", http.StatusOK},
		{"by priority", "?priority=3", http.StatusOK},
		{"by claim state", "?claimed=false&blocked=no", http.StatusOK},
		{"by search", "?q=filtered&root_only=true&include_deleted=true", http.StatusOK},
		{"by due window", "?due_after=2020-01-01T00:00:00Z&due_before=2030-01-01T00:00:00Z",
			http.StatusOK},
		{"sorted", "?sort=updated_at&direction=desc&limit=5", http.StatusOK},
		{"bad priority", "?priority=high", http.StatusBadRequest},
		{"bad limit", "?limit=many", http.StatusBadRequest},
		{"negative limit", "?limit=-1", http.StatusBadRequest},
		{"bad direction", "?direction=sideways", http.StatusBadRequest},
		{"bad sort field", "?sort=colour", http.StatusBadRequest},
		{"bad due window", "?due_before=yesterday", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(http.MethodGet, wire.RouteTasks+tc.query, nil)
			mustStatus(t, resp, tc.want)
			_ = resp.Body.Close()
		})
	}
}

func TestTaskTreeAndAuditFilters(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("parent")

	tree := f.call(http.MethodGet, "/api/v1/tasks/"+task.Ref+"/tree?depth=2", nil)
	mustStatus(t, tree, http.StatusOK)
	_ = tree.Body.Close()

	badDepth := f.call(http.MethodGet, "/api/v1/tasks/"+task.Ref+"/tree?depth=deep", nil)
	mustStatus(t, badDepth, http.StatusBadRequest)
	_ = badDepth.Body.Close()

	audit := f.call(http.MethodGet,
		wire.RouteAudit+"?action=task.create&actor_id="+f.actorA.ID+
			"&source=api&since=2020-01-01T00:00:00Z&until=2030-01-01T00:00:00Z", nil)
	mustStatus(t, audit, http.StatusOK)
	_ = audit.Body.Close()

	badAudit := f.call(http.MethodGet, wire.RouteAudit+"?since=never", nil)
	mustStatus(t, badAudit, http.StatusBadRequest)
	_ = badAudit.Body.Close()

	badTaskAudit := f.call(http.MethodGet, "/api/v1/tasks/"+task.Ref+"/audit?until=never", nil)
	mustStatus(t, badTaskAudit, http.StatusBadRequest)
	_ = badTaskAudit.Body.Close()
}

func TestProjectAndWorkflowMutationRoutes(t *testing.T) {
	f := newFixture(t)

	definition := core.WorkflowInput{
		Key:  "temp",
		Name: "Temporary",
		Definition: core.WorkflowDefinition{
			Initial: "open",
			States: []core.State{
				{Key: "open", Label: "Open"},
				{Key: "closed", Label: "Closed", Terminal: true},
			},
			Transitions: []core.Transition{{From: "open", To: "closed"}},
		},
	}

	put := f.call(http.MethodPut, wire.RouteWorkflows, definition)
	mustStatus(t, put, http.StatusOK)
	_ = put.Body.Close()

	byKey := f.call(http.MethodPut, "/api/v1/workflows/temp", definition)
	mustStatus(t, byKey, http.StatusOK)
	_ = byKey.Body.Close()

	created := f.call(http.MethodPost, wire.RouteProjects,
		core.CreateProjectInput{Key: "ops", Name: "Ops", WorkflowKey: "temp"})
	mustStatus(t, created, http.StatusCreated)
	_ = created.Body.Close()

	archived := f.call(http.MethodDelete, "/api/v1/projects/ops?archive=true", nil)
	mustStatus(t, archived, http.StatusNoContent)
	_ = archived.Body.Close()

	dropped := f.call(http.MethodDelete, "/api/v1/projects/ops", nil)
	mustStatus(t, dropped, http.StatusNoContent)
	_ = dropped.Body.Close()

	deleted := f.call(http.MethodDelete, "/api/v1/workflows/temp", nil)
	mustStatus(t, deleted, http.StatusNoContent)
	_ = deleted.Body.Close()
}

func TestTenantMutationRoutes(t *testing.T) {
	f := newFixture(t)

	created := f.call(http.MethodPost, wire.RouteTenants,
		core.CreateTenantInput{Key: "third", Name: "Third"})
	mustStatus(t, created, http.StatusCreated)
	_ = created.Body.Close()

	member := f.call(http.MethodPost, wire.RouteMembers,
		httpapi.AddMemberRequest{ActorID: f.actorA.ID, Role: core.RoleMember})
	mustStatus(t, member, http.StatusCreated)
	_ = member.Body.Close()

	unmember := f.call(http.MethodDelete, "/api/v1/members/"+f.actorA.ID, nil)
	mustStatus(t, unmember, http.StatusNoContent)
	_ = unmember.Body.Close()

	domain := f.call(http.MethodPost, wire.RouteDomains,
		core.AddDomainInput{Hostname: "extra.test", CertMode: core.CertNone})
	mustStatus(t, domain, http.StatusCreated)
	_ = domain.Body.Close()

	undomain := f.call(http.MethodDelete, "/api/v1/domains/extra.test", nil)
	mustStatus(t, undomain, http.StatusNoContent)
	_ = undomain.Body.Close()

	renamed := f.call(http.MethodPatch, "/api/v1/tenants/acme",
		core.UpdateTenantInput{Name: strPtr("Acme Corp")})
	mustStatus(t, renamed, http.StatusOK)
	_ = renamed.Body.Close()

	removed := f.call(http.MethodDelete, "/api/v1/tenants/acme", nil)
	if removed.StatusCode != http.StatusNoContent && removed.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want the tenant deleted or refused", removed.StatusCode)
	}
	_ = removed.Body.Close()
}

func TestUserAndTokenMutationRoutes(t *testing.T) {
	f := newFixture(t)

	created := f.call(http.MethodPost, wire.RouteUsers,
		core.CreateUserInput{Email: "dana@example.com", Password: "correct-horse-battery"})
	mustStatus(t, created, http.StatusCreated)
	var user core.User
	decodeBody(t, created, &user)

	updated := f.call(http.MethodPatch, "/api/v1/users/"+user.ID,
		core.UpdateUserInput{DisplayName: strPtr("Dana")})
	mustStatus(t, updated, http.StatusOK)
	_ = updated.Body.Close()

	token := f.call(http.MethodPost, wire.RouteTokens,
		core.CreateTokenInput{Name: "ci", Scopes: []core.Scope{core.ScopeTaskRead}})
	mustStatus(t, token, http.StatusCreated)
	var issued core.IssuedToken
	decodeBody(t, token, &issued)

	revoked := f.call(http.MethodDelete, "/api/v1/tokens/"+issued.ID, nil)
	mustStatus(t, revoked, http.StatusNoContent)
	_ = revoked.Body.Close()

	deleted := f.call(http.MethodDelete, "/api/v1/users/"+user.ID, nil)
	mustStatus(t, deleted, http.StatusNoContent)
	_ = deleted.Body.Close()
}

func TestRetentionAndWebhookMutationRoutes(t *testing.T) {
	f := newFixture(t)

	policy := core.DefaultRetention(f.tenantA.ID)
	put := f.call(http.MethodPut, wire.RouteRetention, policy)
	mustStatus(t, put, http.StatusOK)
	_ = put.Body.Close()

	hook := f.call(http.MethodPut, wire.RouteWebhooks,
		core.WebhookInput{ID: "hook-2", URL: "https://example.com/hook", Active: true})
	mustStatus(t, hook, http.StatusOK)
	_ = hook.Body.Close()

	dropped := f.call(http.MethodDelete, "/api/v1/webhooks/hook-2", nil)
	mustStatus(t, dropped, http.StatusNoContent)
	_ = dropped.Body.Close()

	source := f.call(http.MethodPut, wire.RouteSyncSources,
		core.SyncSourceInput{ID: "src-2", System: core.SystemGeneric, Name: "feed"})
	mustStatus(t, source, http.StatusOK)
	_ = source.Body.Close()

	unsource := f.call(http.MethodDelete, "/api/v1/sync/sources/src-2", nil)
	mustStatus(t, unsource, http.StatusNoContent)
	_ = unsource.Body.Close()
}

func TestImportStreamsTheRequestBody(t *testing.T) {
	f := newFixture(t)

	snapshot := []byte(`{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n")
	req := f.newRequest(http.MethodPost, wire.RouteImport+"?mode=merge&dry_run=true",
		bytes.NewReader(snapshot))
	req.Header.Set(wire.HeaderContentType, wire.ContentNDJSON)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	mustStatus(t, resp, http.StatusOK)
	var result core.ImportResult
	decodeBody(t, resp, &result)
	if !result.DryRun {
		t.Error("dry run was not carried through")
	}
}

func TestExportStreamsNDJSON(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, wire.RouteExport, core.ExportInput{})
	mustStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get(wire.HeaderContentType); got != wire.ContentNDJSON {
		t.Errorf("content type = %q, want %q", got, wire.ContentNDJSON)
	}
	if body := readBody(t, resp); !bytes.Contains([]byte(body), []byte(`"kind":"header"`)) {
		t.Errorf("body = %s, want a snapshot header", body)
	}
}

func TestDeleteRoutesReportMissingResources(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("present")

	cases := []struct {
		name string
		path string
		want int
	}{
		{"user", "/api/v1/users/nobody", http.StatusNotFound},
		{"token", "/api/v1/tokens/nothing", http.StatusNotFound},
		{"workflow", "/api/v1/workflows/nope", http.StatusNotFound},
		{"task", "/api/v1/tasks/infra-999", http.StatusNotFound},
		{"comment", "/api/v1/comments/01JJJJJJJJJJJJJJJJJJJJJJJJ", http.StatusNotFound},
		{"member", "/api/v1/members/nobody", http.StatusNotFound},
		{"domain", "/api/v1/domains/nowhere.test", http.StatusNotFound},
		{"tenant", "/api/v1/tenants/nope", http.StatusNotFound},
		{"field", "/api/v1/projects/nope/fields/team", http.StatusNotFound},
		{"tag", "/api/v1/tasks/" + task.Ref + "/tags/absent", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(http.MethodDelete, tc.path, nil)
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d: %s", resp.StatusCode, tc.want, readBody(t, resp))
			}
			_ = resp.Body.Close()
		})
	}
}

func TestOptionalBodiesRejectMalformedJSON(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("claimable")

	paths := []string{
		wire.RouteClaimSweep,
		wire.RoutePrune,
		wire.RouteClaimNext,
		"/api/v1/tasks/" + task.Ref + "/claim",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := f.newRequest(http.MethodPost, path, bytes.NewReader([]byte("{oops")))
			req.Header.Set(wire.HeaderContentType, wire.ContentJSON)
			resp, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatalf("sending request: %v", err)
			}
			mustStatus(t, resp, http.StatusBadRequest)
			_ = resp.Body.Close()
		})
	}
}

func TestMissingPathResourcesOnWriteRoutes(t *testing.T) {
	f := newFixture(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"comment edit", http.MethodPatch, "/api/v1/comments/01JJJJJJJJJJJJJJJJJJJJJJJJ",
			httpapi.CommentRequest{Body: "x"}, http.StatusNotFound},
		{"field on missing project", http.MethodPut, "/api/v1/projects/nope/fields",
			core.FieldDefInput{Key: "k", Label: "K", Type: core.FieldString}, http.StatusNotFound},
		{"artifact on missing task", http.MethodPut, "/api/v1/tasks/infra-999/artifacts",
			core.ArtifactInput{Kind: core.ArtifactResult}, http.StatusNotFound},
		{"comment on missing task", http.MethodPost, "/api/v1/tasks/infra-999/comments",
			httpapi.CommentRequest{Body: "x"}, http.StatusNotFound},
		{"tag on missing task", http.MethodPost, "/api/v1/tasks/infra-999/tags",
			httpapi.TagRequest{Name: "x"}, http.StatusNotFound},
		{"restore missing task", http.MethodPost, "/api/v1/tasks/infra-999/restore",
			nil, http.StatusNotFound},
		{"tree of missing task", http.MethodGet, "/api/v1/tasks/infra-999/tree",
			nil, http.StatusNotFound},
		{"deps of missing task", http.MethodGet, "/api/v1/tasks/infra-999/deps",
			nil, http.StatusNotFound},
		{"comments of missing task", http.MethodGet, "/api/v1/tasks/infra-999/comments",
			nil, http.StatusNotFound},
		{"artifacts of missing task", http.MethodGet, "/api/v1/tasks/infra-999/artifacts",
			nil, http.StatusNotFound},
		{"tags of missing task", http.MethodGet, "/api/v1/tasks/infra-999/tags",
			nil, http.StatusNotFound},
		{"audit of missing task", http.MethodGet, "/api/v1/tasks/infra-999/audit",
			nil, http.StatusNotFound},
		{"dep on missing task", http.MethodPost, "/api/v1/tasks/infra-999/deps",
			httpapi.DependencyRequest{DependsOn: "infra-1"}, http.StatusNotFound},
		{"update missing task", http.MethodPatch, "/api/v1/tasks/infra-999",
			core.UpdateTaskInput{Title: strPtr("x")}, http.StatusNotFound},
		{"claim missing task", http.MethodPost, "/api/v1/tasks/infra-999/claim",
			core.ClaimInput{}, http.StatusNotFound},
		{"renew missing task", http.MethodPost, "/api/v1/tasks/infra-999/claim/renew",
			httpapi.RenewRequest{Token: "x"}, http.StatusNotFound},
		{"release missing task", http.MethodPost, "/api/v1/tasks/infra-999/claim/release",
			httpapi.ReleaseRequest{Token: "x"}, http.StatusNotFound},
		{"fields of missing project", http.MethodGet, "/api/v1/projects/nope/fields",
			nil, http.StatusNotFound},
		{"update missing project", http.MethodPatch, "/api/v1/projects/nope",
			core.UpdateProjectInput{Name: strPtr("x")}, http.StatusNotFound},
		{"delete missing project", http.MethodDelete, "/api/v1/projects/nope",
			nil, http.StatusNotFound},
		{"archive missing project", http.MethodDelete, "/api/v1/projects/nope?archive=true",
			nil, http.StatusNotFound},
		{"update missing tenant", http.MethodPatch, "/api/v1/tenants/nope",
			core.UpdateTenantInput{Name: strPtr("x")}, http.StatusNotFound},
		{"update missing user", http.MethodPatch, "/api/v1/users/nobody",
			core.UpdateUserInput{DisplayName: strPtr("x")}, http.StatusNotFound},
		{"run missing sync source", http.MethodPost, wire.RouteSyncRun,
			core.RunSyncInput{SourceID: "nope"}, http.StatusNotFound},
		{"malformed ref on claim", http.MethodPost, "/api/v1/tasks/!!/claim",
			core.ClaimInput{}, http.StatusBadRequest},
		{"malformed ref on renew", http.MethodPost, "/api/v1/tasks/!!/claim/renew",
			httpapi.RenewRequest{Token: "x"}, http.StatusBadRequest},
		{"malformed ref on release", http.MethodPost, "/api/v1/tasks/!!/claim/release",
			httpapi.ReleaseRequest{Token: "x"}, http.StatusBadRequest},
		{"malformed ref on transition", http.MethodPost, "/api/v1/tasks/!!/transition",
			core.TransitionInput{To: "doing"}, http.StatusBadRequest},
		{"malformed ref on update", http.MethodPatch, "/api/v1/tasks/!!",
			core.UpdateTaskInput{}, http.StatusBadRequest},
		{"malformed ref on delete", http.MethodDelete, "/api/v1/tasks/!!", nil,
			http.StatusBadRequest},
		{"malformed ref on restore", http.MethodPost, "/api/v1/tasks/!!/restore", nil,
			http.StatusBadRequest},
		{"malformed ref on tree", http.MethodGet, "/api/v1/tasks/!!/tree", nil,
			http.StatusBadRequest},
		{"malformed ref on deps", http.MethodGet, "/api/v1/tasks/!!/deps", nil,
			http.StatusBadRequest},
		{"malformed ref on tags", http.MethodGet, "/api/v1/tasks/!!/tags", nil,
			http.StatusBadRequest},
		{"malformed ref on comments", http.MethodGet, "/api/v1/tasks/!!/comments", nil,
			http.StatusBadRequest},
		{"malformed ref on artifacts", http.MethodGet, "/api/v1/tasks/!!/artifacts", nil,
			http.StatusBadRequest},
		{"malformed ref on audit", http.MethodGet, "/api/v1/tasks/!!/audit", nil,
			http.StatusBadRequest},
		{"malformed ref on tag delete", http.MethodDelete, "/api/v1/tasks/!!/tags/x", nil,
			http.StatusBadRequest},
		{"malformed ref on dep delete", http.MethodDelete, "/api/v1/tasks/!!/deps/infra-1", nil,
			http.StatusBadRequest},
		{"malformed ref on dep add", http.MethodPost, "/api/v1/tasks/!!/deps",
			httpapi.DependencyRequest{DependsOn: "infra-1"}, http.StatusBadRequest},
		{"malformed ref on artifact put", http.MethodPut, "/api/v1/tasks/!!/artifacts",
			core.ArtifactInput{Kind: core.ArtifactResult}, http.StatusBadRequest},
		{"malformed ref on comment add", http.MethodPost, "/api/v1/tasks/!!/comments",
			httpapi.CommentRequest{Body: "x"}, http.StatusBadRequest},
		{"malformed ref on tag add", http.MethodPost, "/api/v1/tasks/!!/tags",
			httpapi.TagRequest{Name: "x"}, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(tc.method, tc.path, tc.body)
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d: %s", resp.StatusCode, tc.want, readBody(t, resp))
			}
			_ = resp.Body.Close()
		})
	}
}

// The remote client calls GET /domains/{hostname} for ResolveDomain; without
// this route it 404s and remote tenant resolution silently breaks.
func TestResolveDomainRoute(t *testing.T) {
	f := newFixture(t)

	created := f.call(http.MethodPost, wire.RouteDomains,
		core.AddDomainInput{Hostname: "resolve.example.com"})
	defer func() { _ = created.Body.Close() }()
	mustStatus(t, created, http.StatusCreated)

	got := f.call(http.MethodGet, "/api/v1/domains/resolve.example.com", nil)
	defer func() { _ = got.Body.Close() }()
	mustStatus(t, got, http.StatusOK)

	var tenant core.Tenant
	decodeBody(t, got, &tenant)
	if tenant.ID == "" {
		t.Error("resolved tenant carries no id")
	}
}
