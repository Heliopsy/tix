// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp_test

import (
	"context"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/heliopsy/tix/api"
	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/mcp"
)

// A tool call is recorded as MCP on the token the server was started with,
// because tix mcp dials with the mcp hint. The audit listing the agent reads
// back over the API carries the channel too.
func TestAToolCallIsRecordedAsMCP(t *testing.T) {
	a := newLiveAPI(t)
	token := a.mint(t, core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeAuditRead)
	ctx := context.Background()
	conn, err := connect.Dial(ctx, &config.Resolved{},
		connect.Overrides{Server: a.url, Token: token, ClientHint: core.ClientHintMCP})
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	remote := conn.Service.(*client.Client)

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
	tools, err := mcp.Plan(doc, ov, mcp.Options{HasScope: func(s string) bool { return conn.Actor.HasScope(core.Scope(s)) }})
	if err != nil {
		t.Fatal(err)
	}
	serverSide, clientSide := sdk.NewInMemoryTransports()
	ss, err := mcp.NewServer(tools, remote, "test").Connect(ctx, serverSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test"}, nil).Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if out, isErr := call(t, cs, "task_create", map[string]any{"project_ref": "infra", "title": "from an agent"}); isErr {
		t.Fatalf("task_create failed: %v", out)
	}
	entries, _, err := remote.ListAudit(ctx, core.AuditFilter{Actions: []string{"task.create"}, Vias: []core.ViaKind{core.ViaMCP}})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Via.TokenName != "agent" || entries[0].Via.TokenID == "" {
		t.Fatalf("mcp entries = %+v, want the one create on the agent token", entries)
	}
}
