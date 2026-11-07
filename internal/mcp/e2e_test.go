// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/heliopsy/tix/api"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/mcp"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
)

// tokens resolves the tokens a test minted.
type tokens map[string]*core.APIToken

func (m tokens) TokenByHash(_ context.Context, hash string) (*core.APIToken, error) {
	return m[hash], nil
}

// liveAPI is a real tix HTTP server on a scratch SQLite store.
type liveAPI struct {
	url    string
	clk    *clock.Fake
	lookup tokens
	tenant string
	actor  string
}

func newLiveAPI(t *testing.T) *liveAPI {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(sqlitetest.Path(t), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	tenant := core.Tenant{Key: "acme", Name: "Acme"}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, &tenant) }); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	actor := core.Actor{Kind: core.ActorAgent, Handle: "agent"}
	wf := core.Workflow{Key: "default", Name: "Default", Definition: service.BuiltinWorkflow()}
	project := core.Project{Key: "infra", Name: "infra"}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.CreateActor(ctx, &actor); err != nil {
			return err
		}
		if err := tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		project.WorkflowID = wf.ID
		return tx.CreateProject(ctx, &project)
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	api := &liveAPI{clk: clk, lookup: tokens{}, tenant: tenant.ID, actor: actor.ID}
	router, err := httpapi.New(httpapi.Config{
		Service:         service.New(st, service.WithClock(clk), service.WithHooks(service.HookOff)),
		Authenticator:   auth.NewChain(auth.NewBearerAuthenticator(auth.NewTokenVerifier(api.lookup, clk))),
		DefaultTenantID: tenant.ID,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("building router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	api.url = srv.URL
	return api
}

func (a *liveAPI) mint(t *testing.T, scopes ...core.Scope) string {
	t.Helper()
	minted, err := auth.MintAPIToken(a.clk, a.tenant, core.CreateTokenInput{Name: "agent", ActorID: a.actor, Scopes: scopes})
	if err != nil {
		t.Fatalf("minting token: %v", err)
	}
	stored := minted.Issued.APIToken
	a.lookup[minted.Hash] = &stored
	return minted.Issued.Token
}

// session connects an MCP client, over an in-memory transport, to a tix mcp
// server planned exactly as the command plans it: scopes from whoami.
func (a *liveAPI) session(t *testing.T, token string, allowDestructive bool) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	remote, err := client.New(a.url, token)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	me, err := remote.WhoAmI(ctx)
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	raw, err := api.JSON()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := mcp.LoadDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	ov, err := mcp.BuiltinOverlay()
	if err != nil {
		t.Fatal(err)
	}
	tools, err := mcp.Plan(doc, ov, mcp.Options{
		AllowDestructive: allowDestructive,
		HasScope:         func(s string) bool { return me.HasScope(core.Scope(s)) },
	})
	if err != nil {
		t.Fatal(err)
	}
	serverSide, clientSide := sdk.NewInMemoryTransports()
	serverSession, err := mcp.NewServer(tools, remote, "test").Connect(ctx, serverSide, nil)
	if err != nil {
		t.Fatalf("connecting server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test"}, nil).Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatalf("connecting client: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *sdk.ClientSession) []string {
	t.Helper()
	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("listing tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	return names
}

func call(t *testing.T, cs *sdk.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("calling %s: %v", name, err)
	}
	text := res.Content[0].(*sdk.TextContent).Text
	out := map[string]any{}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s answered %q, not a JSON object: %v", name, text, err)
	}
	return out, res.IsError
}

func TestAnAgentWorksAQueueThroughTheTools(t *testing.T) {
	a := newLiveAPI(t)
	cs := a.session(t, a.mint(t, core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeTaskClaim), false)

	created, isErr := call(t, cs, "task_create", map[string]any{"project_ref": "infra", "title": "migrate the database"})
	if isErr {
		t.Fatalf("task_create failed: %v", created)
	}
	ref, _ := created["ref"].(string)
	if ref == "" || created["title"] != "migrate the database" {
		t.Fatalf("task_create answered %v", created)
	}

	listed, isErr := call(t, cs, "task_list", map[string]any{"project": []string{"infra"}})
	items, _ := listed["items"].([]any)
	if isErr || len(items) != 1 || items[0].(map[string]any)["ref"] != ref {
		t.Fatalf("task_list answered %v, want the one task %s", listed, ref)
	}

	claimed, isErr := call(t, cs, "claim_next", map[string]any{"project_refs": []string{"infra"}})
	task, _ := claimed["task"].(map[string]any)
	if isErr || task["ref"] != ref || claimed["lease_token"] == "" {
		t.Fatalf("claim_next answered %v, want %s with a lease token", claimed, ref)
	}

	again, isErr := call(t, cs, "claim_next", map[string]any{"project_refs": []string{"infra"}})
	errBody, _ := again["error"].(map[string]any)
	if !isErr || errBody["code"] != string(core.KindNoTaskAvailable) {
		t.Fatalf("a second claim_next answered %v, want a no_task_available tool error", again)
	}

	shown, isErr := call(t, cs, "task_show", map[string]any{"ref": ref})
	if isErr || shown["ref"] != ref {
		t.Fatalf("task_show answered %v", shown)
	}
}

func TestToolsFollowTheTokenAndTheDestructiveSwitch(t *testing.T) {
	a := newLiveAPI(t)
	narrow := toolNames(t, a.session(t, a.mint(t, core.ScopeTaskRead, core.ScopeTaskDelete), false))
	if !slices.Contains(narrow, "task_list") {
		t.Fatalf("a task:read token was not offered task_list: %v", narrow)
	}
	for _, absent := range []string{"task_create", "project_create", "task_delete"} {
		if slices.Contains(narrow, absent) {
			t.Errorf("a task:read and task:delete token without --allow-destructive was offered %s", absent)
		}
	}

	wide := toolNames(t, a.session(t, a.mint(t, core.ScopeTaskRead, core.ScopeTaskDelete), true))
	if !slices.Contains(wide, "task_delete") {
		t.Errorf("--allow-destructive did not offer task_delete to a token holding task:delete: %v", wide)
	}
	if slices.Contains(wide, "project_delete") {
		t.Errorf("--allow-destructive offered project_delete to a token without project:write")
	}
}
