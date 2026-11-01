// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func templatePath(route, ref string) string { return strings.Replace(route, "{ref}", ref, 1) }

func TestTemplateEndpoints(t *testing.T) {
	f := newFixture(t)
	src := f.createTask("ship it")

	resp := f.do(http.MethodPost, wire.RouteTemplates, f.hostA, f.tokenA,
		map[string]any{"name": "release", "task_ref": src.Ref, "due_in": "3d"})
	mustStatus(t, resp, http.StatusCreated)
	var saved core.TaskTemplate
	decodeBody(t, resp, &saved)
	if saved.Name != "release" || saved.Task.Title != "ship it" || saved.DueIn != core.Duration(72*time.Hour) {
		t.Fatalf("saved = %+v", saved)
	}

	cases := []struct {
		name, method, path, host, token string
		body                            any
		want                            int
	}{
		{"list", http.MethodGet, wire.RouteTemplates, f.hostA, f.tokenA, nil, http.StatusOK},
		{"show in another case", http.MethodGet, templatePath(wire.RouteTemplate, "RELEASE"), f.hostA, f.tokenA, nil, http.StatusOK},
		{"unknown", http.MethodGet, templatePath(wire.RouteTemplate, "nope"), f.hostA, f.tokenA, nil, http.StatusNotFound},
		{"another tenant", http.MethodGet, templatePath(wire.RouteTemplate, "release"), f.hostB, f.tokenB, nil, http.StatusNotFound},
		{"duplicate name", http.MethodPost, wire.RouteTemplates, f.hostA, f.tokenA,
			map[string]any{"name": "Release", "task_ref": src.Ref}, http.StatusConflict},
		{"rename", http.MethodPatch, templatePath(wire.RouteTemplate, "release"), f.hostA, f.tokenA,
			map[string]any{"name": "ship"}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.do(tc.method, tc.path, tc.host, tc.token, tc.body)
			mustStatus(t, resp, tc.want)
			_ = resp.Body.Close()
		})
	}

	resp = f.do(http.MethodPost, templatePath(wire.RouteTemplateUse, "ship"), f.hostA, f.tokenA, map[string]any{"title": "ship {N}"})
	mustStatus(t, resp, http.StatusCreated)
	var task core.Task
	decodeBody(t, resp, &task)
	if task.Title != "ship 1" || task.DueAt == nil || task.ID == src.ID {
		t.Fatalf("used = %+v, want a fresh task titled ship 1 with a due date", task)
	}

	resp = f.do(http.MethodDelete, templatePath(wire.RouteTemplate, "ship"), f.hostA, f.tokenA, nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	resp = f.do(http.MethodPost, templatePath(wire.RouteTemplateUse, "ship"), f.hostA, f.tokenA, nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
}

func TestTemplatesSurviveTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	src := f.createTask("standup")
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()

	if _, err := remote.SaveTemplate(ctx, core.SaveTemplateInput{Name: "daily", TaskRef: src.Ref, TenantWide: true}); err != nil {
		t.Fatalf("save: %v", err)
	}
	items, next, err := remote.ListTemplates(ctx, core.TemplateFilter{ProjectRef: "infra"})
	if err != nil || next != "" || len(items) != 1 || !items[0].TenantWide() {
		t.Fatalf("list = %+v %q %v", items, next, err)
	}
	title := "Daily {N}"
	if _, err := remote.UpdateTemplate(ctx, "daily", core.UpdateTemplateInput{Title: &title}); err != nil {
		t.Fatalf("update: %v", err)
	}
	none := core.Duration(0)
	task, err := remote.UseTemplate(ctx, "daily", core.UseTemplateInput{ProjectRef: "infra", DueIn: &none})
	if err != nil || task.Title != "Daily 1" || task.DueAt != nil {
		t.Fatalf("use = %+v, %v", task, err)
	}
	got, err := remote.GetTemplate(ctx, "daily")
	if err != nil || got.Uses != 1 {
		t.Fatalf("show = %+v, %v", got, err)
	}
	if err := remote.DeleteTemplate(ctx, "daily"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
