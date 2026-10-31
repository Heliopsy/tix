// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func projectPath(ref string) string { return strings.Replace(wire.RouteProject, "{ref}", ref, 1) }

func TestPatchingAProjectKeyRenamesItAndOldRefsResolve(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("kept")
	if task.Ref != "infra-1" {
		t.Fatalf("seeded task ref = %q, want infra-1", task.Ref)
	}

	resp := f.call(http.MethodPatch, projectPath("infra"), map[string]any{"key": "ops"})
	var renamed core.Project
	decodeBody(t, resp, &renamed)
	if renamed.Key != "ops" {
		t.Fatalf("PATCH key answered %q, want ops", renamed.Key)
	}

	resp = f.call(http.MethodGet, strings.Replace(wire.RouteTask, "{ref}", "infra-1", 1), nil)
	var got core.Task
	decodeBody(t, resp, &got)
	if got.ID != task.ID || got.Ref != "ops-1" {
		t.Errorf("GET infra-1 = %q, want the same task as ops-1", got.Ref)
	}
	resp = f.call(http.MethodGet, projectPath("infra"), nil)
	var byOld core.Project
	decodeBody(t, resp, &byOld)
	if byOld.Key != "ops" {
		t.Errorf("GET project infra = %q, want ops", byOld.Key)
	}

	for _, tc := range []struct {
		key  string
		want int
	}{
		{"9lives", http.StatusBadRequest},
		{"ops", http.StatusOK},
	} {
		resp := f.call(http.MethodPatch, projectPath("ops"), map[string]any{"key": tc.key})
		mustStatus(t, resp, tc.want)
		_ = resp.Body.Close()
	}
	resp = f.call(http.MethodPost, wire.RouteProjects, map[string]any{"key": "infra", "name": "Again"})
	mustStatus(t, resp, http.StatusConflict)
	_ = resp.Body.Close()
}

func TestTheClientSendsAProjectKey(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	key := "ops"
	p, err := remote.UpdateProject(context.Background(), "infra", core.UpdateProjectInput{Key: &key})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if p.Key != "ops" {
		t.Errorf("client rename = %q, want ops", p.Key)
	}
}
