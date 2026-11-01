// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func TestBulkEndpoint(t *testing.T) {
	f := newFixture(t)
	one, two := f.createTask("one"), f.createTask("two")

	resp := f.do(http.MethodPost, wire.RouteTaskBulk, f.hostA, f.tokenA, map[string]any{
		"action": "priority", "refs": []string{one.Ref, two.Ref, "infra-99"}, "priority": 1,
	})
	mustStatus(t, resp, http.StatusOK)
	var got core.BulkResult
	decodeBody(t, resp, &got)
	if got.Applied != 2 || got.Refused != 1 || len(got.Outcomes) != 3 || got.Outcomes[2].Code != string(core.KindNotFound) {
		t.Fatalf("result = %+v", got)
	}

	for _, tc := range []struct {
		name string
		body map[string]any
		want int
	}{
		{"unknown action", map[string]any{"action": "explode", "refs": []string{one.Ref}}, http.StatusBadRequest},
		{"no refs", map[string]any{"action": "star"}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.do(http.MethodPost, wire.RouteTaskBulk, f.hostA, f.tokenA, tc.body)
			mustStatus(t, resp, tc.want)
			_ = resp.Body.Close()
		})
	}
}

func TestBulkSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	task := f.createTask("one")
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	got, err := remote.BulkTasks(context.Background(), core.BulkTaskInput{
		Action: core.BulkTagAdd, Tag: "ops", Refs: []string{task.Ref},
	})
	if err != nil || got.Applied != 1 {
		t.Fatalf("bulk = %+v, %v", got, err)
	}
}

func TestTaskListingAsCSV(t *testing.T) {
	f := newFixture(t)
	f.createTask(`=HYPERLINK("x")`)
	f.createTask("plain")

	for _, tc := range []struct {
		name, query, accept string
	}{
		{"format parameter", "?format=csv&sort=seq", ""},
		{"accept header", "?sort=seq", "text/csv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, f.server.URL+wire.RouteTasks+tc.query, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = f.hostA
			req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
			if tc.accept != "" {
				req.Header.Set(wire.HeaderAccept, tc.accept)
			}
			resp, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			mustStatus(t, resp, http.StatusOK)
			if ct := resp.Header.Get(wire.HeaderContentType); !strings.HasPrefix(ct, wire.ContentCSV) {
				t.Fatalf("content type = %q", ct)
			}
			raw, _ := io.ReadAll(resp.Body)
			records, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
			if err != nil {
				t.Fatalf("not csv: %v\n%s", err, raw)
			}
			if strings.Join(records[0], ",") != "ref,project,title,status,priority,assignee,tags,due,created,updated" {
				t.Fatalf("header = %v", records[0])
			}
			if len(records) != 3 || records[1][2] != `'=HYPERLINK("x")` || records[2][2] != "plain" {
				t.Fatalf("rows = %v", records)
			}
			if !strings.Contains(string(raw), `"'=HYPERLINK(""x"")"`) {
				t.Errorf("the guarded title is not quoted by RFC 4180:\n%s", raw)
			}
		})
	}

	req, _ := http.NewRequest(http.MethodGet, f.server.URL+wire.RouteTasks+"?format=csv&limit=1", nil)
	req.Host = f.hostA
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.Header.Get(wire.HeaderNextCursor) == "" {
		t.Error("a CSV page with more after it names no next cursor")
	}
}
