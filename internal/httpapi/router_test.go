package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
)

func TestRouteHappyPaths(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("ship it")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"whoami", http.MethodGet, httpapi.RouteWhoAmI, nil, http.StatusOK},
		{"list tasks", http.MethodGet, httpapi.RouteTasks, nil, http.StatusOK},
		{"get task", http.MethodGet, "/api/v1/tasks/" + task.Ref, nil, http.StatusOK},
		{"update task", http.MethodPatch, "/api/v1/tasks/" + task.Ref,
			core.UpdateTaskInput{Title: strPtr("renamed")}, http.StatusOK},
		{"transition task", http.MethodPost, "/api/v1/tasks/" + task.Ref + "/transition",
			core.TransitionInput{To: "doing"}, http.StatusOK},
		{"task tree", http.MethodGet, "/api/v1/tasks/" + task.Ref + "/tree", nil, http.StatusOK},
		{"task tags", http.MethodGet, "/api/v1/tasks/" + task.Ref + "/tags", nil, http.StatusOK},
		{"add tag", http.MethodPost, "/api/v1/tasks/" + task.Ref + "/tags",
			httpapi.TagRequest{Name: "urgent"}, http.StatusNoContent},
		{"remove tag", http.MethodDelete, "/api/v1/tasks/" + task.Ref + "/tags/urgent",
			nil, http.StatusNoContent},
		{"list tags", http.MethodGet, httpapi.RouteLabels, nil, http.StatusOK},
		{"list deps", http.MethodGet, "/api/v1/tasks/" + task.Ref + "/deps", nil, http.StatusOK},
		{"add comment", http.MethodPost, "/api/v1/tasks/" + task.Ref + "/comments",
			httpapi.CommentRequest{Body: "looks good"}, http.StatusCreated},
		{"list comments", http.MethodGet, "/api/v1/tasks/" + task.Ref + "/comments", nil, http.StatusOK},
		{"put artifact", http.MethodPut, "/api/v1/tasks/" + task.Ref + "/artifacts",
			core.ArtifactInput{Kind: core.ArtifactResult, Name: "out"}, http.StatusOK},
		{"list artifacts", http.MethodGet, "/api/v1/tasks/" + task.Ref + "/artifacts", nil, http.StatusOK},
		{"task audit", http.MethodGet, "/api/v1/tasks/" + task.Ref + "/audit", nil, http.StatusOK},
		{"list projects", http.MethodGet, httpapi.RouteProjects, nil, http.StatusOK},
		{"create project", http.MethodPost, httpapi.RouteProjects,
			core.CreateProjectInput{Key: "ops", Name: "Ops"}, http.StatusCreated},
		{"get project", http.MethodGet, "/api/v1/projects/infra", nil, http.StatusOK},
		{"update project", http.MethodPatch, "/api/v1/projects/infra",
			core.UpdateProjectInput{Name: strPtr("Infrastructure")}, http.StatusOK},
		{"put field", http.MethodPut, "/api/v1/projects/infra/fields",
			core.FieldDefInput{Key: "team", Label: "Team", Type: core.FieldString}, http.StatusOK},
		{"list fields", http.MethodGet, "/api/v1/projects/infra/fields", nil, http.StatusOK},
		{"delete field", http.MethodDelete, "/api/v1/projects/infra/fields/team", nil, http.StatusNoContent},
		{"list workflows", http.MethodGet, httpapi.RouteWorkflows, nil, http.StatusOK},
		{"get workflow", http.MethodGet, "/api/v1/workflows/default", nil, http.StatusOK},
		{"list tenants", http.MethodGet, httpapi.RouteTenants, nil, http.StatusOK},
		{"get tenant", http.MethodGet, "/api/v1/tenants/acme", nil, http.StatusOK},
		{"list members", http.MethodGet, httpapi.RouteMembers, nil, http.StatusOK},
		{"list domains", http.MethodGet, httpapi.RouteDomains, nil, http.StatusOK},
		{"list audit", http.MethodGet, httpapi.RouteAudit, nil, http.StatusOK},
		{"get retention", http.MethodGet, httpapi.RouteRetention, nil, http.StatusOK},
		{"prune", http.MethodPost, httpapi.RoutePrune, core.PruneInput{DryRun: true}, http.StatusOK},
		{"sweep", http.MethodPost, httpapi.RouteClaimSweep, httpapi.SweepRequest{Limit: 10}, http.StatusOK},
		{"create user", http.MethodPost, httpapi.RouteUsers,
			core.CreateUserInput{Email: "a@example.com", Password: "correct-horse-battery"},
			http.StatusCreated},
		{"list users", http.MethodGet, httpapi.RouteUsers, nil, http.StatusOK},
		{"create token", http.MethodPost, httpapi.RouteTokens,
			core.CreateTokenInput{Name: "ci", Scopes: []core.Scope{core.ScopeTaskRead}},
			http.StatusCreated},
		{"list tokens", http.MethodGet, httpapi.RouteTokens, nil, http.StatusOK},
		{"put webhook", http.MethodPut, httpapi.RouteWebhooks,
			core.WebhookInput{URL: "https://example.com/hook", Active: true}, http.StatusOK},
		{"list webhooks", http.MethodGet, httpapi.RouteWebhooks, nil, http.StatusOK},
		{"list deliveries", http.MethodGet, httpapi.RouteDeliveries, nil, http.StatusOK},
		{"put sync source", http.MethodPut, httpapi.RouteSyncSources,
			core.SyncSourceInput{System: core.SystemGeneric, Name: "feed"}, http.StatusOK},
		{"list sync sources", http.MethodGet, httpapi.RouteSyncSources, nil, http.StatusOK},
		{"run sync", http.MethodPost, httpapi.RouteSyncRun,
			core.RunSyncInput{SourceID: "src-1", DryRun: true}, http.StatusOK},
		{"export", http.MethodPost, httpapi.RouteExport, core.ExportInput{}, http.StatusOK},
		{"logout", http.MethodPost, httpapi.RouteLogout, nil, http.StatusNoContent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(tc.method, tc.path, tc.body)
			mustStatus(t, resp, tc.want)
			_ = resp.Body.Close()
		})
	}
}

func TestRouteErrorStatuses(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("ship it")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
		code   core.Kind
	}{
		{"missing task", http.MethodGet, "/api/v1/tasks/infra-999", nil,
			http.StatusNotFound, core.KindNotFound},
		{"malformed ref", http.MethodGet, "/api/v1/tasks/!!", nil,
			http.StatusBadRequest, core.KindInvalid},
		{"task without title", http.MethodPost, httpapi.RouteTasks,
			core.CreateTaskInput{ProjectRef: "infra"}, http.StatusBadRequest, core.KindInvalid},
		{"illegal transition", http.MethodPost, "/api/v1/tasks/" + task.Ref + "/transition",
			core.TransitionInput{To: "done"}, http.StatusUnprocessableEntity, core.KindPrecondition},
		{"unknown transition target", http.MethodPost, "/api/v1/tasks/" + task.Ref + "/transition",
			core.TransitionInput{}, http.StatusBadRequest, core.KindInvalid},
		{"missing project", http.MethodGet, "/api/v1/projects/nope", nil,
			http.StatusNotFound, core.KindNotFound},
		{"missing workflow", http.MethodGet, "/api/v1/workflows/nope", nil,
			http.StatusNotFound, core.KindNotFound},
		{"missing user", http.MethodGet, "/api/v1/users/nobody", nil,
			http.StatusNotFound, core.KindNotFound},
		{"missing webhook", http.MethodDelete, "/api/v1/webhooks/nope", nil,
			http.StatusNotFound, core.KindNotFound},
		{"missing sync source", http.MethodDelete, "/api/v1/sync/sources/nope", nil,
			http.StatusNotFound, core.KindNotFound},
		{"missing delivery", http.MethodPost, "/api/v1/webhooks/deliveries/nope/redeliver", nil,
			http.StatusNotFound, core.KindNotFound},
		{"import without mode", http.MethodPost, httpapi.RouteImport, core.ExportInput{},
			http.StatusBadRequest, core.KindInvalid},
		{"unknown route", http.MethodGet, "/api/tasks", nil,
			http.StatusNotFound, core.KindNotFound},
		{"method not allowed", http.MethodDelete, httpapi.RouteTasks, nil,
			http.StatusMethodNotAllowed, core.KindInvalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(tc.method, tc.path, tc.body)
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d: %s", resp.StatusCode, tc.want, readBody(t, resp))
			}
			if got := resp.Header.Get(httpapi.HeaderContentType); !strings.Contains(got, httpapi.ContentJSON) {
				t.Errorf("content type = %q, want json", got)
			}
			var env httpapi.ErrorEnvelope
			decodeBody(t, resp, &env)
			if env.Error.Code != tc.code {
				t.Errorf("code = %q, want %q", env.Error.Code, tc.code)
			}
			if env.Error.Message == "" {
				t.Error("error message is empty")
			}
		})
	}
}

func TestClaimLifecycle(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("claim me")

	resp := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/claim", core.ClaimInput{})
	mustStatus(t, resp, http.StatusOK)
	var claim core.Claim
	decodeBody(t, resp, &claim)
	if claim.LeaseToken == "" {
		t.Fatal("claim returned no lease token")
	}

	renew := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/claim/renew",
		httpapi.RenewRequest{Token: claim.LeaseToken})
	mustStatus(t, renew, http.StatusOK)
	_ = renew.Body.Close()

	stale := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/claim/renew",
		httpapi.RenewRequest{Token: "not-the-token"})
	if stale.StatusCode != http.StatusConflict && stale.StatusCode != http.StatusNotFound {
		t.Errorf("stale renew status = %d, want a conflict", stale.StatusCode)
	}
	_ = stale.Body.Close()

	release := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/claim/release",
		httpapi.ReleaseRequest{Token: claim.LeaseToken})
	mustStatus(t, release, http.StatusNoContent)
	_ = release.Body.Close()
}

func TestClaimNextReportsAnEmptyQueueDistinctly(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, httpapi.RouteClaimNext, core.ClaimNextInput{})
	mustStatus(t, resp, http.StatusNotFound)
	var env httpapi.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindNoTaskAvailable {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindNoTaskAvailable)
	}
}

func TestTaskRefAcceptsIdentifierAndHumanRef(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("addressable")

	byRef := f.call(http.MethodGet, "/api/v1/tasks/"+task.Ref, nil)
	mustStatus(t, byRef, http.StatusOK)
	var fromRef core.Task
	decodeBody(t, byRef, &fromRef)

	byID := f.call(http.MethodGet, "/api/v1/tasks/"+task.ID, nil)
	mustStatus(t, byID, http.StatusOK)
	var fromID core.Task
	decodeBody(t, byID, &fromID)

	if fromRef.ID != fromID.ID {
		t.Fatalf("ref resolved %q but identifier resolved %q", fromRef.ID, fromID.ID)
	}
	if fromRef.Ref != "infra-1" {
		t.Errorf("ref = %q, want infra-1", fromRef.Ref)
	}
}

func TestKeysetPaginationHasNoDuplicatesOrGaps(t *testing.T) {
	f := newFixture(t)
	const total = 7
	for i := range total {
		f.createTask(string(rune('a'+i)) + " task")
	}

	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		path := httpapi.RouteTasks + "?limit=3"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		resp := f.call(http.MethodGet, path, nil)
		mustStatus(t, resp, http.StatusOK)
		var page httpapi.Page[core.Task]
		decodeBody(t, resp, &page)
		pages++

		for _, task := range page.Items {
			if seen[task.ID] {
				t.Fatalf("task %q appeared on more than one page", task.ID)
			}
			seen[task.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > total {
			t.Fatal("pagination did not terminate")
		}
	}

	if len(seen) != total {
		t.Fatalf("saw %d tasks across %d pages, want %d", len(seen), pages, total)
	}
	if pages < 2 {
		t.Fatalf("pages = %d, want the listing to span several pages", pages)
	}
}

func TestInvalidCursorIsRejected(t *testing.T) {
	f := newFixture(t)
	f.createTask("only one")

	resp := f.call(http.MethodGet, httpapi.RouteTasks+"?cursor=not-a-cursor", nil)
	mustStatus(t, resp, http.StatusBadRequest)
	var env httpapi.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindInvalid {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindInvalid)
	}
}

func TestListNegotiatesNDJSON(t *testing.T) {
	f := newFixture(t)
	f.createTask("first")
	f.createTask("second")

	req := f.newRequest(http.MethodGet, httpapi.RouteTasks, nil)
	req.Header.Set(httpapi.HeaderAccept, httpapi.ContentNDJSON)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	mustStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get(httpapi.HeaderContentType); got != httpapi.ContentNDJSON {
		t.Fatalf("content type = %q, want %q", got, httpapi.ContentNDJSON)
	}

	body := readBody(t, resp)
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d ndjson lines, want 2: %q", len(lines), body)
	}
	for _, line := range lines {
		var task core.Task
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			t.Fatalf("line %q is not a task: %v", line, err)
		}
		if task.ID == "" {
			t.Errorf("line %q carried no task", line)
		}
	}
}

func TestEmptyListRendersAnEmptyArray(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodGet, httpapi.RouteTasks, nil)
	mustStatus(t, resp, http.StatusOK)
	body := readBody(t, resp)
	if !strings.Contains(body, `"items":[]`) {
		t.Errorf("body = %s, want an empty items array", body)
	}
}

func TestHealthWorksWithTheDatabaseClosed(t *testing.T) {
	f := newFixture(t)
	if err := f.store.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}

	resp := f.do(http.MethodGet, httpapi.RouteHealth, "unknown.example", "", nil)
	mustStatus(t, resp, http.StatusOK)
	var body httpapi.HealthBody
	decodeBody(t, resp, &body)
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
}

func TestReadyReportsFailingChecks(t *testing.T) {
	f := newFixture(t)

	ready := f.do(http.MethodGet, httpapi.RouteReady, f.hostA, "", nil)
	mustStatus(t, ready, http.StatusOK)
	var body httpapi.ReadyBody
	decodeBody(t, ready, &body)
	if !body.Ready {
		t.Fatalf("server is not ready: %+v", body.Checks)
	}

	if err := f.store.Close(); err != nil {
		t.Fatalf("closing store: %v", err)
	}
	down := f.do(http.MethodGet, httpapi.RouteReady, f.hostA, "", nil)
	if down.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", down.StatusCode, readBody(t, down))
	}
	var after httpapi.ReadyBody
	decodeBody(t, down, &after)
	if after.Ready {
		t.Error("readiness reports ready with the database closed")
	}
	if !hasFailingCheck(after.Checks, "database") {
		t.Errorf("checks = %+v, want the database check to fail", after.Checks)
	}
}

func hasFailingCheck(checks []httpapi.Check, name string) bool {
	for _, c := range checks {
		if c.Name == name && !c.OK {
			return true
		}
	}
	return false
}

func strPtr(s string) *string { return &s }
