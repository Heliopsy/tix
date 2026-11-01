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

func filterPath(ref string) string { return strings.Replace(wire.RouteFilter, "{filter}", ref, 1) }

func TestFilterEndpoints(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, wire.RouteFilters, map[string]any{"name": "ops", "query": "tag:ops", "view": "board", "sort": "priority"})
	mustStatus(t, resp, http.StatusCreated)
	var created core.SavedFilter
	decodeBody(t, resp, &created)
	if created.ID == "" || created.Name != "ops" || created.View != "board" || !created.Mine {
		t.Fatalf("POST answered %+v", created)
	}
	resp = f.call(http.MethodPost, wire.RouteFilters, map[string]any{"name": "team", "shared": true})
	mustStatus(t, resp, http.StatusCreated)
	_ = resp.Body.Close()

	resp = f.call(http.MethodGet, filterPath("OPS"), nil)
	mustStatus(t, resp, http.StatusOK)
	var got core.SavedFilter
	decodeBody(t, resp, &got)
	if got.ID != created.ID {
		t.Fatalf("GET by name answered %+v", got)
	}

	resp = f.call(http.MethodPatch, filterPath(created.ID), map[string]any{"name": "ops board", "shared": true})
	mustStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &got)
	if got.Name != "ops board" || !got.Shared {
		t.Fatalf("PATCH answered %+v", got)
	}

	resp = f.call(http.MethodPut, wire.RouteFilterOrder, map[string]any{"filters": []string{"team"}})
	mustStatus(t, resp, http.StatusOK)
	var page wire.Page[core.SavedFilter]
	decodeBody(t, resp, &page)
	if len(page.Items) != 2 || page.Items[0].Name != "team" || page.Items[1].Position != 2 {
		t.Fatalf("PUT order answered %+v", page.Items)
	}

	resp = f.call(http.MethodGet, wire.RouteFilters, nil)
	mustStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &page)
	if len(page.Items) != 2 || page.Items[0].Name != "team" {
		t.Fatalf("GET list answered %+v", page.Items)
	}

	resp = f.call(http.MethodDelete, filterPath("team"), nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()

	resp = f.do(http.MethodGet, wire.RouteFilters, f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusOK)
	var other wire.Page[core.SavedFilter]
	decodeBody(t, resp, &other)
	if len(other.Items) != 0 {
		t.Fatalf("another tenant's reader sees %+v", other.Items)
	}
}

func TestFilterEndpointRefusals(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodPost, wire.RouteFilters, map[string]any{"name": "ops"})
	mustStatus(t, resp, http.StatusCreated)
	_ = resp.Body.Close()
	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"no name", http.MethodPost, wire.RouteFilters, map[string]any{"query": "tag:x"}, http.StatusBadRequest},
		{"a taken name", http.MethodPost, wire.RouteFilters, map[string]any{"name": "OPS"}, http.StatusConflict},
		{"an unreadable expression", http.MethodPost, wire.RouteFilters, map[string]any{"name": "x", "query": "nokey:1"}, http.StatusBadRequest},
		{"an unknown view", http.MethodPost, wire.RouteFilters, map[string]any{"name": "x", "view": "calendar"}, http.StatusBadRequest},
		{"no such filter", http.MethodGet, filterPath("nope"), nil, http.StatusNotFound},
		{"nothing to change", http.MethodPatch, filterPath("ops"), map[string]any{}, http.StatusBadRequest},
		{"an empty order", http.MethodPut, wire.RouteFilterOrder, map[string]any{"filters": []string{}}, http.StatusBadRequest},
		{"delete nothing", http.MethodDelete, filterPath("nope"), nil, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(tc.method, tc.path, tc.body)
			mustStatus(t, resp, tc.want)
			_ = resp.Body.Close()
		})
	}
}

func TestFiltersSurviveTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()

	a, err := remote.CreateSavedFilter(ctx, core.SavedFilterInput{Name: "a", Query: "assignee:me"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := remote.CreateSavedFilter(ctx, core.SavedFilterInput{Name: "b"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	name := "a2"
	if got, err := remote.UpdateSavedFilter(ctx, a.ID, core.SavedFilterUpdate{Name: &name}); err != nil || got.Name != "a2" {
		t.Fatalf("update = %+v, %v", got, err)
	}
	if got, err := remote.GetSavedFilter(ctx, "a2"); err != nil || got.Query != "assignee:me" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	ordered, err := remote.OrderSavedFilters(ctx, core.SavedFilterOrderInput{Filters: []string{"b"}})
	if err != nil || len(ordered) != 2 || ordered[0].Name != "b" {
		t.Fatalf("order = %+v, %v", ordered, err)
	}
	if err := remote.DeleteSavedFilter(ctx, "b"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, err := remote.ListSavedFilters(ctx); err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
}

func TestAssigneeMeOverTheAPI(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodGet, wire.RouteTasks+"?assignee=me", nil)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
}
