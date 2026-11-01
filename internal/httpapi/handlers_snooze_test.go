// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func snoozePath(ref string) string { return strings.Replace(wire.RouteTaskSnooze, "{ref}", ref, 1) }

func TestSnoozeEndpoints(t *testing.T) {
	f := newFixture(t)
	f.createTask("a")
	b := f.createTask("b")

	until := f.clock.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	resp := f.call(http.MethodPut, snoozePath(b.Ref), map[string]any{"until": until})
	var snoozed core.Task
	decodeBody(t, resp, &snoozed)
	if snoozed.SnoozedUntil == nil || snoozed.SnoozedUntil.UTC().Format(time.RFC3339) != until ||
		snoozed.SnoozedByActorID == "" {
		t.Fatalf("PUT snooze answered until %v by %q, want %s by the caller",
			snoozed.SnoozedUntil, snoozed.SnoozedByActorID, until)
	}

	past := f.clock.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	resp = f.call(http.MethodPut, snoozePath(b.Ref), map[string]any{"until": past})
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()

	lists := []struct {
		query string
		want  []string
	}{
		{"", []string{"a"}},
		{"?snoozed=true", []string{"b"}},
		{"?snoozed=false", []string{"a"}},
		{"?snoozed=any", []string{"a", "b"}},
	}
	for _, l := range lists {
		if titles := f.listTitles(l.query); !slices.Equal(titles, l.want) {
			t.Errorf("list %q = %v, want %v", l.query, titles, l.want)
		}
	}
	resp = f.call(http.MethodGet, wire.RouteTasks+"?snoozed=maybe", nil)
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()

	resp = f.call(http.MethodPost, strings.Replace(wire.RouteTaskClaim, "{ref}", b.Ref, 1), nil)
	_ = resp.Body.Close()
	if resp.StatusCode < 400 {
		t.Fatalf("claiming a snoozed task answered %d", resp.StatusCode)
	}

	resp = f.call(http.MethodDelete, snoozePath(b.Ref), nil)
	var back core.Task
	decodeBody(t, resp, &back)
	if back.SnoozedUntil != nil {
		t.Fatalf("DELETE snooze answered a task still snoozed until %v", back.SnoozedUntil)
	}
	if titles := f.listTitles(""); !slices.Equal(titles, []string{"a", "b"}) {
		t.Errorf("after unsnooze = %v", titles)
	}
}

func TestWakeEndpoint(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("parked")
	until := f.clock.Now().Add(time.Minute).UTC().Format(time.RFC3339)
	resp := f.call(http.MethodPut, snoozePath(task.Ref), map[string]any{"until": until})
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()

	f.clock.Advance(2 * time.Minute)
	resp = f.call(http.MethodPost, wire.RouteSnoozeWake, map[string]any{"limit": 10})
	var got httpapi.WakeResult
	decodeBody(t, resp, &got)
	if got.Woken != 1 {
		t.Fatalf("woken = %d, want 1", got.Woken)
	}
}

func TestSnoozeSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	f.createTask("plain")
	parked := f.createTask("parked")

	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()

	got, err := remote.SnoozeTask(ctx, core.TaskRef{ID: parked.ID}, core.SnoozeInput{Until: f.clock.Now().Add(time.Hour)})
	if err != nil || got.SnoozedUntil == nil {
		t.Fatalf("snooze: %+v, %v", got, err)
	}
	for _, tc := range []struct {
		f    core.TaskFilter
		want int
	}{
		{core.TaskFilter{}, 1},
		{core.TaskFilter{Snoozed: core.Yes}, 1},
		{core.TaskFilter{IncludeSnoozed: true}, 2},
	} {
		page, err := remote.ListTasks(ctx, tc.f)
		if err != nil || len(page.Tasks) != tc.want {
			t.Fatalf("listing %+v: %d tasks, %v; want %d", tc.f, len(page.Tasks), err, tc.want)
		}
	}
	if _, err := remote.UnsnoozeTask(ctx, core.TaskRef{ID: parked.ID}); err != nil {
		t.Fatalf("unsnooze: %v", err)
	}
	f.clock.Advance(2 * time.Hour)
	if woken, err := remote.WakeSnoozed(ctx, 0); err != nil || woken != 0 {
		t.Fatalf("wake after an unsnooze = %d, %v; want 0", woken, err)
	}
}
