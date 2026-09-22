package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

type capture struct {
	method string
	path   string
	query  string
	header http.Header
	body   string
}

func newClient(t *testing.T, handler http.HandlerFunc) (*Client, *capture) {
	t.Helper()
	got := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got.method = r.Method
		got.path = r.URL.EscapedPath()
		got.query = r.URL.RawQuery
		got.header = r.Header.Clone()
		got.body = string(raw)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, "secret-token")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, got
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(wire.HeaderContentType, wire.ContentJSON)
	_, _ = io.WriteString(w, `{"items":[],"next_cursor":"","swept":3}`)
}

func ref(s string) core.TaskRef { return core.TaskRef{ID: s} }

func TestMethodsIssueExpectedRequest(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		call   func(*Client) error
		method string
		path   string
	}{
		{"whoami", func(c *Client) error { _, err := c.WhoAmI(ctx); return err }, http.MethodGet, wire.RouteWhoAmI},
		{"create tenant", func(c *Client) error {
			_, err := c.CreateTenant(ctx, core.CreateTenantInput{Key: "acme"})
			return err
		}, http.MethodPost, wire.RouteTenants},
		{"get tenant", func(c *Client) error { _, err := c.GetTenant(ctx, "acme"); return err }, http.MethodGet, "/api/v1/tenants/acme"},
		{"list tenants", func(c *Client) error { _, _, err := c.ListTenants(ctx, core.Page{}); return err }, http.MethodGet, wire.RouteTenants},
		{"update tenant", func(c *Client) error {
			_, err := c.UpdateTenant(ctx, "acme", core.UpdateTenantInput{})
			return err
		}, http.MethodPatch, "/api/v1/tenants/acme"},
		{"delete tenant", func(c *Client) error { return c.DeleteTenant(ctx, "acme") }, http.MethodDelete, "/api/v1/tenants/acme"},
		{"add domain", func(c *Client) error {
			_, err := c.AddDomain(ctx, core.AddDomainInput{Hostname: "a.example"})
			return err
		}, http.MethodPost, wire.RouteDomains},
		{"list domains", func(c *Client) error { _, err := c.ListDomains(ctx); return err }, http.MethodGet, wire.RouteDomains},
		{"remove domain", func(c *Client) error { return c.RemoveDomain(ctx, "a.example") }, http.MethodDelete, "/api/v1/domains/a.example"},
		{"resolve domain", func(c *Client) error { _, err := c.ResolveDomain(ctx, "a.example"); return err }, http.MethodGet, "/api/v1/domains/a.example"},
		{"add member", func(c *Client) error { _, err := c.AddMember(ctx, "u1", core.Role("admin")); return err }, http.MethodPost, wire.RouteMembers},
		{"list members", func(c *Client) error { _, err := c.ListMembers(ctx); return err }, http.MethodGet, wire.RouteMembers},
		{"remove member", func(c *Client) error { return c.RemoveMember(ctx, "u1") }, http.MethodDelete, "/api/v1/members/u1"},

		{"create project", func(c *Client) error {
			_, err := c.CreateProject(ctx, core.CreateProjectInput{Key: "infra"})
			return err
		}, http.MethodPost, wire.RouteProjects},
		{"get project", func(c *Client) error { _, err := c.GetProject(ctx, "infra"); return err }, http.MethodGet, "/api/v1/projects/infra"},
		{"list projects", func(c *Client) error {
			_, _, err := c.ListProjects(ctx, core.ProjectFilter{IncludeArchived: true})
			return err
		}, http.MethodGet, wire.RouteProjects},
		{"update project", func(c *Client) error {
			_, err := c.UpdateProject(ctx, "infra", core.UpdateProjectInput{})
			return err
		}, http.MethodPatch, "/api/v1/projects/infra"},
		{"archive project", func(c *Client) error { return c.ArchiveProject(ctx, "infra") }, http.MethodDelete, "/api/v1/projects/infra"},
		{"delete project", func(c *Client) error { return c.DeleteProject(ctx, "infra") }, http.MethodDelete, "/api/v1/projects/infra"},
		{"put field def", func(c *Client) error {
			_, err := c.PutFieldDef(ctx, "infra", core.FieldDefInput{Key: "severity"})
			return err
		}, http.MethodPut, "/api/v1/projects/infra/fields"},
		{"list field defs", func(c *Client) error { _, err := c.ListFieldDefs(ctx, "infra"); return err }, http.MethodGet, "/api/v1/projects/infra/fields"},
		{"delete field def", func(c *Client) error { return c.DeleteFieldDef(ctx, "infra", "severity") }, http.MethodDelete, "/api/v1/projects/infra/fields/severity"},

		{"put workflow", func(c *Client) error {
			_, err := c.PutWorkflow(ctx, core.WorkflowInput{Key: "default"})
			return err
		}, http.MethodPut, "/api/v1/workflows/default"},
		{"get workflow", func(c *Client) error { _, err := c.GetWorkflow(ctx, "default"); return err }, http.MethodGet, "/api/v1/workflows/default"},
		{"list workflows", func(c *Client) error { _, err := c.ListWorkflows(ctx); return err }, http.MethodGet, wire.RouteWorkflows},
		{"delete workflow", func(c *Client) error { return c.DeleteWorkflow(ctx, "default") }, http.MethodDelete, "/api/v1/workflows/default"},

		{"create task", func(c *Client) error {
			_, err := c.CreateTask(ctx, core.CreateTaskInput{Title: "t"})
			return err
		}, http.MethodPost, wire.RouteTasks},
		{"get task", func(c *Client) error { _, err := c.GetTask(ctx, ref("infra-1")); return err }, http.MethodGet, "/api/v1/tasks/infra-1"},
		{"list tasks", func(c *Client) error { _, err := c.ListTasks(ctx, core.TaskFilter{}); return err }, http.MethodGet, wire.RouteTasks},
		{"update task", func(c *Client) error {
			_, err := c.UpdateTask(ctx, ref("infra-1"), core.UpdateTaskInput{})
			return err
		}, http.MethodPatch, "/api/v1/tasks/infra-1"},
		{"transition task", func(c *Client) error {
			_, err := c.TransitionTask(ctx, ref("infra-1"), core.TransitionInput{To: "done"})
			return err
		}, http.MethodPost, "/api/v1/tasks/infra-1/transition"},
		{"delete task", func(c *Client) error { return c.DeleteTask(ctx, ref("infra-1"), core.DeleteTaskInput{Hard: true}) }, http.MethodDelete, "/api/v1/tasks/infra-1"},
		{"restore task", func(c *Client) error { _, err := c.RestoreTask(ctx, ref("infra-1")); return err }, http.MethodPost, "/api/v1/tasks/infra-1/restore"},
		{"task tree", func(c *Client) error { _, err := c.TaskTree(ctx, ref("infra-1"), 2); return err }, http.MethodGet, "/api/v1/tasks/infra-1/tree"},
		{"add dependency", func(c *Client) error { return c.AddDependency(ctx, ref("infra-1"), ref("infra-2")) }, http.MethodPost, "/api/v1/tasks/infra-1/deps"},
		{"remove dependency", func(c *Client) error { return c.RemoveDependency(ctx, ref("infra-1"), ref("infra-2")) }, http.MethodDelete, "/api/v1/tasks/infra-1/deps/infra-2"},
		{"list dependencies", func(c *Client) error { _, err := c.ListDependencies(ctx, ref("infra-1")); return err }, http.MethodGet, "/api/v1/tasks/infra-1/deps"},
		{"add tag", func(c *Client) error { return c.AddTag(ctx, ref("infra-1"), "bug") }, http.MethodPost, "/api/v1/tasks/infra-1/tags"},
		{"remove tag", func(c *Client) error { return c.RemoveTag(ctx, ref("infra-1"), "bug") }, http.MethodDelete, "/api/v1/tasks/infra-1/tags/bug"},
		{"list tags", func(c *Client) error { _, err := c.ListTags(ctx); return err }, http.MethodGet, wire.RouteLabels},
		{"add comment", func(c *Client) error { _, err := c.AddComment(ctx, ref("infra-1"), "hi"); return err }, http.MethodPost, "/api/v1/tasks/infra-1/comments"},
		{"list comments", func(c *Client) error { _, err := c.ListComments(ctx, ref("infra-1")); return err }, http.MethodGet, "/api/v1/tasks/infra-1/comments"},
		{"edit comment", func(c *Client) error { _, err := c.EditComment(ctx, "c1", "hi"); return err }, http.MethodPatch, "/api/v1/comments/c1"},
		{"delete comment", func(c *Client) error { return c.DeleteComment(ctx, "c1") }, http.MethodDelete, "/api/v1/comments/c1"},
		{"put artifact", func(c *Client) error {
			_, err := c.PutArtifact(ctx, ref("infra-1"), core.ArtifactInput{Kind: core.ArtifactResult})
			return err
		}, http.MethodPut, "/api/v1/tasks/infra-1/artifacts"},
		{"list artifacts", func(c *Client) error { _, err := c.ListArtifacts(ctx, ref("infra-1")); return err }, http.MethodGet, "/api/v1/tasks/infra-1/artifacts"},

		{"claim task", func(c *Client) error {
			_, err := c.ClaimTask(ctx, ref("infra-1"), core.ClaimInput{})
			return err
		}, http.MethodPost, "/api/v1/tasks/infra-1/claim"},
		{"claim next", func(c *Client) error { _, err := c.ClaimNext(ctx, core.ClaimNextInput{}); return err }, http.MethodPost, wire.RouteClaimNext},
		{"renew lease", func(c *Client) error {
			_, err := c.RenewLease(ctx, ref("infra-1"), "tok", core.Duration(time.Minute))
			return err
		}, http.MethodPost, "/api/v1/tasks/infra-1/claim/renew"},
		{"release lease", func(c *Client) error {
			return c.ReleaseLease(ctx, ref("infra-1"), "tok", core.ReleaseInput{})
		}, http.MethodPost, "/api/v1/tasks/infra-1/claim/release"},
		{"sweep leases", func(c *Client) error { _, err := c.SweepLeases(ctx, 10); return err }, http.MethodPost, wire.RouteClaimSweep},

		{"list audit", func(c *Client) error { _, _, err := c.ListAudit(ctx, core.AuditFilter{}); return err }, http.MethodGet, wire.RouteAudit},
		{"prune", func(c *Client) error { _, err := c.Prune(ctx, core.PruneInput{}); return err }, http.MethodPost, wire.RoutePrune},
		{"get retention", func(c *Client) error { _, err := c.GetRetention(ctx); return err }, http.MethodGet, wire.RouteRetention},
		{"put retention", func(c *Client) error {
			_, err := c.PutRetention(ctx, core.RetentionPolicy{})
			return err
		}, http.MethodPut, wire.RouteRetention},

		{"create user", func(c *Client) error {
			_, err := c.CreateUser(ctx, core.CreateUserInput{Email: "a@b.c"})
			return err
		}, http.MethodPost, wire.RouteUsers},
		{"get user", func(c *Client) error { _, err := c.GetUser(ctx, "u1"); return err }, http.MethodGet, "/api/v1/users/u1"},
		{"list users", func(c *Client) error { _, _, err := c.ListUsers(ctx, core.Page{}); return err }, http.MethodGet, wire.RouteUsers},
		{"update user", func(c *Client) error { _, err := c.UpdateUser(ctx, "u1", core.UpdateUserInput{}); return err }, http.MethodPatch, "/api/v1/users/u1"},
		{"delete user", func(c *Client) error { return c.DeleteUser(ctx, "u1") }, http.MethodDelete, "/api/v1/users/u1"},
		{"login", func(c *Client) error { _, err := c.Login(ctx, "a@b.c", "pw"); return err }, http.MethodPost, wire.RouteLogin},
		{"logout", func(c *Client) error { return c.Logout(ctx) }, http.MethodPost, wire.RouteLogout},
		{"create token", func(c *Client) error {
			_, err := c.CreateToken(ctx, core.CreateTokenInput{Name: "agent"})
			return err
		}, http.MethodPost, wire.RouteTokens},
		{"list tokens", func(c *Client) error { _, err := c.ListTokens(ctx, "u1"); return err }, http.MethodGet, wire.RouteTokens},
		{"revoke token", func(c *Client) error { return c.RevokeToken(ctx, "t1") }, http.MethodDelete, "/api/v1/tokens/t1"},

		{"put webhook", func(c *Client) error {
			_, err := c.PutWebhook(ctx, core.WebhookInput{URL: "https://x"})
			return err
		}, http.MethodPut, wire.RouteWebhooks},
		{"list webhooks", func(c *Client) error { _, err := c.ListWebhooks(ctx); return err }, http.MethodGet, wire.RouteWebhooks},
		{"delete webhook", func(c *Client) error { return c.DeleteWebhook(ctx, "w1") }, http.MethodDelete, "/api/v1/webhooks/w1"},
		{"list deliveries", func(c *Client) error {
			_, _, err := c.ListDeliveries(ctx, core.DeliveryFilter{EndpointID: "w1", Statuses: []core.DeliveryStatus{core.DeliveryFailed}})
			return err
		}, http.MethodGet, wire.RouteDeliveries},
		{"redeliver", func(c *Client) error { return c.RedeliverWebhook(ctx, "d1") }, http.MethodPost, "/api/v1/webhooks/deliveries/d1/redeliver"},

		{"put sync source", func(c *Client) error {
			_, err := c.PutSyncSource(ctx, core.SyncSourceInput{System: core.SystemJira, Name: "jira"})
			return err
		}, http.MethodPut, wire.RouteSyncSources},
		{"list sync sources", func(c *Client) error { _, err := c.ListSyncSources(ctx); return err }, http.MethodGet, wire.RouteSyncSources},
		{"delete sync source", func(c *Client) error { return c.DeleteSyncSource(ctx, "s1") }, http.MethodDelete, "/api/v1/sync/sources/s1"},
		{"run sync", func(c *Client) error { _, err := c.RunSync(ctx, core.RunSyncInput{SourceID: "s1"}); return err }, http.MethodPost, wire.RouteSyncRun},

		{"enrol ssh key", func(c *Client) error {
			_, err := c.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: "ssh-ed25519 AAAA"})
			return err
		}, http.MethodPost, wire.RouteSSHKeys},
		{"list ssh keys", func(c *Client) error { _, err := c.ListSSHKeys(ctx, ""); return err }, http.MethodGet, wire.RouteSSHKeys},
		{"revoke ssh key", func(c *Client) error { return c.RevokeSSHKey(ctx, "k1") }, http.MethodDelete, "/api/v1/ssh-keys/k1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, got := newClient(t, okHandler)
			if err := tc.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if got.method != tc.method {
				t.Errorf("method = %q, want %q", got.method, tc.method)
			}
			if got.path != tc.path {
				t.Errorf("path = %q, want %q", got.path, tc.path)
			}
			if auth := got.header.Get(wire.HeaderAuth); auth != "Bearer secret-token" {
				t.Errorf("authorization = %q", auth)
			}
		})
	}
}

func TestSweepLeasesReadsCount(t *testing.T) {
	c, _ := newClient(t, okHandler)
	n, err := c.SweepLeases(context.Background(), 10)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n != 3 {
		t.Fatalf("swept = %d, want 3", n)
	}
}

func TestArchiveProjectMarksTheQuery(t *testing.T) {
	c, got := newClient(t, okHandler)
	if err := c.ArchiveProject(context.Background(), "infra"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if got.query != "archive=true" {
		t.Fatalf("query = %q", got.query)
	}
}

func TestDeleteTaskHardFlag(t *testing.T) {
	c, got := newClient(t, okHandler)
	if err := c.DeleteTask(context.Background(), ref("infra-1"), core.DeleteTaskInput{Hard: true}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got.query != "hard=true" {
		t.Fatalf("query = %q", got.query)
	}
}

func TestErrorEnvelopeRestoresKind(t *testing.T) {
	kinds := []core.Kind{
		core.KindInvalid, core.KindNotFound, core.KindConflict, core.KindUnauthenticated,
		core.KindForbidden, core.KindLeaseExpired, core.KindNoTaskAvailable,
		core.KindPrecondition, core.KindInternal,
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			domain := (&core.Error{Kind: kind, Message: "tenant key already exists"}).WithDetail("key", "acme")
			c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				httpapi.WriteError(w, domain)
			})

			_, err := c.GetTenant(context.Background(), "acme")
			if err == nil {
				t.Fatal("want error")
			}
			if !core.IsKind(err, kind) {
				t.Fatalf("kind = %q, want %q", core.KindOf(err), kind)
			}
			if kind == core.KindInternal {
				return
			}
			var got *core.Error
			if !errors.As(err, &got) {
				t.Fatal("want *core.Error")
			}
			if got.Message != domain.Message {
				t.Errorf("message = %q, want %q", got.Message, domain.Message)
			}
			if got.Details["key"] != "acme" {
				t.Errorf("details = %v", got.Details)
			}
		})
	}
}

func TestStatusWithoutEnvelope(t *testing.T) {
	tests := []struct {
		status int
		want   core.Kind
	}{
		{http.StatusBadRequest, core.KindInvalid},
		{http.StatusUnauthorized, core.KindUnauthenticated},
		{http.StatusForbidden, core.KindForbidden},
		{http.StatusNotFound, core.KindNotFound},
		{http.StatusConflict, core.KindConflict},
		{http.StatusUnprocessableEntity, core.KindPrecondition},
		{http.StatusBadGateway, core.KindInternal},
		{http.StatusTeapot, core.KindInternal},
	}
	for _, tc := range tests {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, "<html>proxy says no</html>")
			})
			_, err := c.GetTenant(context.Background(), "acme")
			if !core.IsKind(err, tc.want) {
				t.Fatalf("kind = %q, want %q", core.KindOf(err), tc.want)
			}
			if !strings.Contains(err.Error(), "proxy says no") {
				t.Fatalf("message lost: %v", err)
			}
		})
	}
}

func TestEmptyBodyStatusUsesStatusText(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.GetTenant(context.Background(), "acme")
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("message = %v", err)
	}
}

func TestTransportFailureIsInternal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(okHandler))
	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.Close()

	_, err = c.WhoAmI(context.Background())
	if !core.IsKind(err, core.KindInternal) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
	if !strings.Contains(err.Error(), "contacting server") {
		t.Fatalf("message = %v", err)
	}
}

func TestPathParametersAreEncoded(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
		want string
	}{
		{"slash in ref", func(c *Client) error {
			_, err := c.GetTask(context.Background(), ref("weird/ref"))
			return err
		}, "/api/v1/tasks/weird%2Fref"},
		{"space in ref", func(c *Client) error {
			_, err := c.GetTask(context.Background(), ref("weird ref"))
			return err
		}, "/api/v1/tasks/weird%20ref"},
		{"tag with slash", func(c *Client) error {
			return c.RemoveTag(context.Background(), ref("infra-1"), "area/net")
		}, "/api/v1/tasks/infra-1/tags/area%2Fnet"},
		{"hostname with space", func(c *Client) error {
			return c.RemoveDomain(context.Background(), "bad host")
		}, "/api/v1/domains/bad%20host"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, got := newClient(t, okHandler)
			if err := tc.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if got.path != tc.want {
				t.Fatalf("path = %q, want %q", got.path, tc.want)
			}
		})
	}
}

func TestCursorRoundTrips(t *testing.T) {
	cursor := core.Cursor{SortValue: "2026-01-01", ID: "x", Sort: core.SortCreatedAt, Direction: core.Ascending}.Encode()
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(wire.Page[core.Task]{
			Items:      []core.Task{{ID: "t1"}},
			NextCursor: "next-cursor",
		})
	})

	page, err := c.ListTasks(context.Background(), core.TaskFilter{
		Statuses: []string{"open"},
		Page:     core.Page{Limit: 25, Cursor: cursor, Sort: core.SortCreatedAt, Direction: core.Ascending},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.NextCursor != "next-cursor" {
		t.Errorf("next cursor = %q", page.NextCursor)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != "t1" {
		t.Errorf("tasks = %v", page.Tasks)
	}
	if !strings.Contains(got.query, "cursor="+cursor) {
		t.Errorf("query = %q, want cursor", got.query)
	}
	if !strings.Contains(got.query, "limit=25") || !strings.Contains(got.query, "status=open") {
		t.Errorf("query = %q", got.query)
	}
}

func TestTaskFilterQueryCarriesEveryField(t *testing.T) {
	due := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	c, got := newClient(t, okHandler)
	_, err := c.ListTasks(context.Background(), core.TaskFilter{
		ProjectIDs:     []string{"p1"},
		ProjectKeys:    []string{"infra"},
		Tags:           []string{"bug"},
		AssigneeIDs:    []string{"a1"},
		CreatorIDs:     []string{"c1"},
		ClaimedBy:      []string{"agent"},
		Priorities:     []core.Priority{core.PriorityHigh},
		DueBefore:      &due,
		DueAfter:       &due,
		ParentID:       "p",
		Claimed:        core.Yes,
		Blocked:        core.No,
		Query:          "search me",
		CustomFields:   map[string]any{"severity": "high"},
		IncludeDeleted: true,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{
		"project_id=p1", "project=infra", "tag=bug", "assignee=a1", "creator=c1",
		"claimed_by=agent", "priority=2", "due_before=", "due_after=", "parent_id=p",
		"claimed=true", "blocked=false", "q=search", "custom_fields=", "include_deleted=true",
	} {
		if !strings.Contains(got.query, want) {
			t.Errorf("query %q missing %q", got.query, want)
		}
	}
}

func TestTaskFilterQueryKeepsCustomFieldTypes(t *testing.T) {
	c, got := newClient(t, okHandler)
	_, err := c.ListTasks(context.Background(), core.TaskFilter{
		CustomFields: map[string]any{"points": 3},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := "custom_fields=" + url.QueryEscape(`{"points":3}`)
	if !strings.Contains(got.query, want) {
		t.Fatalf("query = %q, want %q", got.query, want)
	}
}

func TestParentIsNullIsSeparateFromParentID(t *testing.T) {
	c, got := newClient(t, okHandler)
	if _, err := c.ListTasks(context.Background(), core.TaskFilter{ParentIsNull: true}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(got.query, "root_only=true") {
		t.Fatalf("query = %q", got.query)
	}
}

func TestAuditFilterQuery(t *testing.T) {
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c, got := newClient(t, okHandler)
	_, _, err := c.ListAudit(context.Background(), core.AuditFilter{
		SubjectType: "task",
		SubjectID:   "t1",
		ActorIDs:    []string{"a1"},
		Actions:     []string{"task.created"},
		Sources:     []core.Source{core.Source("cli")},
		Since:       &since,
		Until:       &since,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for _, want := range []string{"subject_type=task", "subject_id=t1", "actor_id=a1", "action=task.created", "source=cli", "since=", "until="} {
		if !strings.Contains(got.query, want) {
			t.Errorf("query %q missing %q", got.query, want)
		}
	}
}

func TestBodyIsMarshalledUnvalidated(t *testing.T) {
	c, got := newClient(t, okHandler)
	if _, err := c.CreateTask(context.Background(), core.CreateTaskInput{Title: ""}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.Contains(got.body, `"title":""`) {
		t.Fatalf("body = %q", got.body)
	}
	if got.header.Get(wire.HeaderContentType) != wire.ContentJSON {
		t.Fatalf("content type = %q", got.header.Get(wire.HeaderContentType))
	}
}

func TestSessionCookieAuthentication(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(wire.SessionCookieName)
		if err != nil || cookie.Value != "sess-1" {
			t.Errorf("cookie = %v, err = %v", cookie, err)
		}
		if r.Header.Get(wire.HeaderAuth) != "" {
			t.Errorf("unexpected bearer header")
		}
		if r.Header.Get("User-Agent") != "tix-test" {
			t.Errorf("user agent = %q", r.Header.Get("User-Agent"))
		}
		okHandler(w, r)
	}))
	defer srv.Close()

	c, err := New(srv.URL, "", WithSessionCookie("sess-1"), WithUserAgent("tix-test"),
		WithHTTPClient(&http.Client{}), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.WhoAmI(context.Background()); err != nil {
		t.Fatalf("whoami: %v", err)
	}
}

func TestNewRejectsUnusableURL(t *testing.T) {
	tests := []string{"", "   ", "ftp://example.com", "://bad"}
	for _, in := range tests {
		if _, err := New(in, "t"); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("New(%q) kind = %q", in, core.KindOf(err))
		}
	}
}

func TestBaseURLPathIsPreserved(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		okHandler(w, r)
	}))
	defer srv.Close()

	c, err := New(srv.URL+"/tix/", "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.WhoAmI(context.Background()); err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if path != "/tix"+wire.RouteWhoAmI {
		t.Fatalf("path = %q", path)
	}
}

func TestNoContentResponseDecodes(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.DeleteUser(context.Background(), "u1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestMalformedSuccessBodyIsInternal(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{not json")
	})
	_, err := c.WhoAmI(context.Background())
	if !core.IsKind(err, core.KindInternal) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
}

func TestTimeoutIsApplied(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	c, err := New(srv.URL, "t", WithTimeout(20*time.Millisecond))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.WhoAmI(context.Background()); !core.IsKind(err, core.KindInternal) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
}

func TestClientSatisfiesService(t *testing.T) {
	c, _ := newClient(t, okHandler)
	var svc core.Service = c
	if err := svc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestLeaseBodiesUseTheServerFieldNames(t *testing.T) {
	c, got := newClient(t, okHandler)
	if _, err := c.RenewLease(context.Background(), ref("infra-1"), "tok", core.Duration(time.Minute)); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !strings.Contains(got.body, `"token":"tok"`) || !strings.Contains(got.body, `"ttl":"1m0s"`) {
		t.Fatalf("renew body = %q", got.body)
	}

	err := c.ReleaseLease(context.Background(), ref("infra-1"), "tok", core.ReleaseInput{Status: "done"})
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if !strings.Contains(got.body, `"token":"tok"`) || !strings.Contains(got.body, `"status":"done"`) {
		t.Fatalf("release body = %q", got.body)
	}
}

// A session is ended with the cookie that names it, while the bearer token the
// client was built with still authenticates the request. The client the copy
// came from keeps presenting no cookie at all.
func TestWithSessionSendsTheSessionCookieOnlyOnTheCopy(t *testing.T) {
	var cookies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := ""
		if cookie, err := r.Cookie(wire.SessionCookieName); err == nil {
			value = cookie.Value
		}
		cookies = append(cookies, value)
		if got := r.Header.Get(wire.HeaderAuth); got != "Bearer pat-1" {
			t.Errorf("authorization = %q", got)
		}
		okHandler(w, r)
	}))
	defer srv.Close()

	c, err := New(srv.URL, "pat-1", WithHTTPClient(&http.Client{}), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.WithSession("sess-2").Logout(context.Background()); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if err := c.Logout(context.Background()); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if want := []string{"sess-2", ""}; !slices.Equal(cookies, want) {
		t.Fatalf("cookies = %q, want %q", cookies, want)
	}
}

// The client marshals and unmarshals; it holds no opinion about what an ssh
// public key is. Text the service would refuse goes out exactly as given, and
// the enrolled record comes back exactly as the server rendered it.
func TestSSHKeyBodyIsMarshalledUnvalidated(t *testing.T) {
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(wire.HeaderContentType, wire.ContentJSON)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"k1","tenant_id":"t1","actor_id":"u1",`+
			`"fingerprint":"SHA256:abc","public_key":"ssh-ed25519 AAAA","label":"laptop",`+
			`"created_at":"2024-01-01T00:00:00Z","revoked_at":"2024-02-01T00:00:00Z"}`)
	})

	key, err := c.EnrolSSHKey(context.Background(), core.EnrolSSHKeyInput{
		PublicKey: "-----BEGIN OPENSSH PRIVATE KEY-----", Label: "laptop"})
	if err != nil {
		t.Fatalf("enrol: %v", err)
	}
	if !strings.Contains(got.body, "BEGIN OPENSSH PRIVATE KEY") {
		t.Errorf("body = %q, want the submission sent through untouched", got.body)
	}
	if key.Fingerprint != "SHA256:abc" || key.PublicKey != "ssh-ed25519 AAAA" {
		t.Errorf("key = %+v, want the server's own fields", key)
	}
	if key.Active() {
		t.Error("a key the server reported revoked came back active")
	}
}

// A revoked key is part of the listing, so the client must not filter one out.
func TestListSSHKeysCarriesRevokedKeysThrough(t *testing.T) {
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(wire.HeaderContentType, wire.ContentJSON)
		_, _ = io.WriteString(w, `{"items":[`+
			`{"id":"live","fingerprint":"SHA256:one"},`+
			`{"id":"dead","fingerprint":"SHA256:two","revoked_at":"2024-02-01T00:00:00Z"}]}`)
	})

	keys, err := c.ListSSHKeys(context.Background(), "u1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got.query != "actor_id=u1" {
		t.Errorf("query = %q, want the actor carried as a parameter", got.query)
	}
	if len(keys) != 2 {
		t.Fatalf("listed %d keys, want both", len(keys))
	}
	if !keys[0].Active() || keys[1].Active() {
		t.Errorf("revocation did not survive the round trip: %+v", keys)
	}
}

// An actor left empty asks the server who the caller is, so no parameter goes.
func TestListSSHKeysWithoutAnActorSendsNoParameter(t *testing.T) {
	c, got := newClient(t, okHandler)
	if _, err := c.ListSSHKeys(context.Background(), ""); err != nil {
		t.Fatalf("list: %v", err)
	}
	if got.query != "" {
		t.Fatalf("query = %q, want none", got.query)
	}
}
