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

func viewPath(pattern, view, group string) string {
	return strings.NewReplacer("{view}", view, "{group}", group).Replace(pattern)
}

func listViewPrefs(t *testing.T, f *apiFixture, view string) map[string]core.GroupPref {
	t.Helper()
	resp := f.call(http.MethodGet, viewPath(wire.RouteViewGroups, view, ""), nil)
	mustStatus(t, resp, http.StatusOK)
	var page wire.Page[core.GroupPref]
	decodeBody(t, resp, &page)
	out := map[string]core.GroupPref{}
	for _, p := range page.Items {
		out[p.Group] = p
	}
	return out
}

func TestViewEndpoints(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPatch, viewPath(wire.RouteViewGroup, "board", "done"), map[string]any{"hidden": true})
	mustStatus(t, resp, http.StatusOK)
	var pref core.GroupPref
	decodeBody(t, resp, &pref)
	if !pref.Hidden || pref.View != "board" || pref.Group != "done" {
		t.Fatalf("PATCH answered %+v", pref)
	}
	if got := listViewPrefs(t, f, "board"); !got["done"].Hidden || len(got) != 1 {
		t.Fatalf("GET after PATCH = %+v", got)
	}

	resp = f.call(http.MethodPut, viewPath(wire.RouteViewOrder, "project:infra", ""), map[string]any{"groups": []string{"doing", "todo"}})
	mustStatus(t, resp, http.StatusOK)
	var page wire.Page[core.GroupPref]
	decodeBody(t, resp, &page)
	if len(page.Items) != 2 || page.Items[0].Group != "doing" || page.Items[0].Position != 1 || page.Items[0].View != "project:infra" {
		t.Fatalf("PUT order answered %+v", page.Items)
	}

	resp = f.call(http.MethodDelete, viewPath(wire.RouteViewGroups, "project:infra", "")+"?group=todo", nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	if got := listViewPrefs(t, f, "project:infra"); len(got) != 1 || got["doing"].Position != 1 {
		t.Fatalf("after resetting one group = %+v", got)
	}
	resp = f.call(http.MethodDelete, viewPath(wire.RouteViewGroups, "project:infra", ""), nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	if got := listViewPrefs(t, f, "project:infra"); len(got) != 0 {
		t.Fatalf("after resetting the view = %+v", got)
	}

	resp = f.do(http.MethodGet, viewPath(wire.RouteViewGroups, "board", ""), f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusOK)
	var other wire.Page[core.GroupPref]
	decodeBody(t, resp, &other)
	if len(other.Items) != 0 {
		t.Fatalf("another tenant's reader sees %+v", other.Items)
	}
}

func TestViewEndpointRefusals(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"unknown view", http.MethodGet, viewPath(wire.RouteViewGroups, "calendar", ""), nil, http.StatusBadRequest},
		{"unknown project", http.MethodGet, viewPath(wire.RouteViewGroups, "project:nope", ""), nil, http.StatusNotFound},
		{"nothing to change", http.MethodPatch, viewPath(wire.RouteViewGroup, "board", "done"), map[string]any{}, http.StatusBadRequest},
		{"no body", http.MethodPatch, viewPath(wire.RouteViewGroup, "board", "done"), nil, http.StatusBadRequest},
		{"a state no workflow has", http.MethodPatch, viewPath(wire.RouteViewGroup, "board", "shipped"), map[string]any{"hidden": true}, http.StatusBadRequest},
		{"an empty order", http.MethodPut, viewPath(wire.RouteViewOrder, "board", ""), map[string]any{"groups": []string{}}, http.StatusBadRequest},
		{"not json", http.MethodPut, viewPath(wire.RouteViewOrder, "board", ""), "groups", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(tc.method, tc.path, tc.body)
			mustStatus(t, resp, tc.want)
			_ = resp.Body.Close()
		})
	}
	if got := listViewPrefs(t, f, "board"); len(got) != 0 {
		t.Fatalf("a refusal wrote %+v", got)
	}
}

func TestViewSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()
	on := true

	if _, err := remote.SetGroupPref(ctx, "project:infra", "done", core.GroupPrefInput{Starred: &on}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := remote.OrderGroups(ctx, "project:infra", core.GroupOrderInput{Groups: []string{"todo"}}); err != nil {
		t.Fatalf("order: %v", err)
	}
	prefs, err := remote.ListGroupPrefs(ctx, "project:infra")
	if err != nil || len(prefs) != 2 {
		t.Fatalf("list = %+v, %v", prefs, err)
	}
	if err := remote.ResetGroupPrefs(ctx, "project:infra", "done"); err != nil {
		t.Fatalf("reset one: %v", err)
	}
	if prefs, _ := remote.ListGroupPrefs(ctx, "project:infra"); len(prefs) != 1 || prefs[0].Group != "todo" {
		t.Fatalf("after reset one = %+v", prefs)
	}
	if err := remote.ResetGroupPrefs(ctx, "project:infra", ""); err != nil {
		t.Fatalf("reset view: %v", err)
	}
	if prefs, _ := remote.ListGroupPrefs(ctx, "project:infra"); len(prefs) != 0 {
		t.Fatalf("after reset = %+v", prefs)
	}
}
