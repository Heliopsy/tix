// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// seedInbox gives actor A n items about one task, caused by somebody else.
func seedInbox(t *testing.T, f *apiFixture, n int) []string {
	t.Helper()
	task := f.createTask("inbox target")
	ctx := context.Background()
	var ids []string
	if err := f.store.Update(ctx, core.TenantScope{TenantID: f.tenantA.ID}, func(tx store.Tx) error {
		other := core.Actor{Kind: core.ActorUser, Handle: "somebody"}
		if err := tx.CreateActor(ctx, &other); err != nil {
			return err
		}
		for range n {
			f.clock.Advance(1e9)
			it := &core.InboxItem{RecipientID: f.actorA.ID, Kind: core.InboxComment, TaskID: task.ID,
				ActorID: other.ID, CreatedAt: f.clock.Now()}
			if err := tx.CreateInboxItem(ctx, it); err != nil {
				return err
			}
			ids = append([]string{it.ID}, ids...)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return ids
}

func inboxCount(t *testing.T, f *apiFixture) int {
	t.Helper()
	resp := f.call(http.MethodGet, wire.RouteInboxCount, nil)
	mustStatus(t, resp, http.StatusOK)
	var n core.InboxCount
	decodeBody(t, resp, &n)
	return n.Unread
}

func TestInboxEndpoints(t *testing.T) {
	f := newFixture(t)
	ids := seedInbox(t, f, 3)

	var got []string
	path := wire.RouteInbox + "?limit=2"
	for range 5 {
		resp := f.call(http.MethodGet, path, nil)
		mustStatus(t, resp, http.StatusOK)
		var page wire.Page[core.InboxItem]
		decodeBody(t, resp, &page)
		for _, it := range page.Items {
			got = append(got, it.ID)
			if it.ActorHandle != "somebody" || it.TaskTitle != "inbox target" {
				t.Fatalf("an item is not named: %+v", it)
			}
		}
		if page.NextCursor == "" {
			break
		}
		path = wire.RouteInbox + "?limit=2&cursor=" + page.NextCursor
	}
	if !slices.Equal(got, ids) {
		t.Fatalf("paged = %v, want newest first %v", got, ids)
	}

	read := strings.Replace(wire.RouteInboxRead, "{id}", ids[0], 1)
	resp := f.call(http.MethodPut, read, nil)
	mustStatus(t, resp, http.StatusOK)
	var item core.InboxItem
	decodeBody(t, resp, &item)
	if !item.Read() || inboxCount(t, f) != 2 {
		t.Fatalf("PUT read answered %+v, unread %d", item, inboxCount(t, f))
	}
	resp = f.call(http.MethodGet, wire.RouteInbox+"?unread=true", nil)
	var unread wire.Page[core.InboxItem]
	decodeBody(t, resp, &unread)
	if len(unread.Items) != 2 {
		t.Fatalf("unread only = %d items, want 2", len(unread.Items))
	}
	resp = f.call(http.MethodDelete, read, nil)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	resp = f.call(http.MethodPost, wire.RouteInboxReadAll, nil)
	mustStatus(t, resp, http.StatusOK)
	var all core.InboxReadAll
	decodeBody(t, resp, &all)
	if all.Marked != 3 || inboxCount(t, f) != 0 {
		t.Fatalf("read-all marked %d, unread %d", all.Marked, inboxCount(t, f))
	}

	resp = f.call(http.MethodPut, wire.RouteInboxPrefs, map[string]any{"muted": []string{"status", "mention"}})
	mustStatus(t, resp, http.StatusOK)
	var prefs core.InboxPrefs
	decodeBody(t, resp, &prefs)
	if !slices.Equal(prefs.Muted, []core.InboxKind{core.InboxMention, core.InboxStatus}) {
		t.Fatalf("PUT prefs answered %v", prefs.Muted)
	}
	resp = f.call(http.MethodGet, wire.RouteInboxPrefs, nil)
	decodeBody(t, resp, &prefs)
	if len(prefs.Muted) != 2 {
		t.Fatalf("GET prefs = %v", prefs.Muted)
	}

	resp = f.do(http.MethodGet, wire.RouteInboxCount, f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusOK)
	var other core.InboxCount
	decodeBody(t, resp, &other)
	resp = f.do(http.MethodPut, read, f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
}

func TestInboxEndpointRefusals(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"unknown kind", http.MethodPut, wire.RouteInboxPrefs, map[string]any{"muted": []string{"everything"}}, http.StatusBadRequest},
		{"not json", http.MethodPut, wire.RouteInboxPrefs, "muted", http.StatusBadRequest},
		{"unknown item", http.MethodPut, strings.Replace(wire.RouteInboxRead, "{id}", "01J000000000000000000A", 1), nil, http.StatusNotFound},
		{"bad cursor", http.MethodGet, wire.RouteInbox + "?cursor=nope", nil, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(tc.method, tc.path, tc.body)
			mustStatus(t, resp, tc.want)
			_ = resp.Body.Close()
		})
	}
}

func TestInboxSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	ids := seedInbox(t, f, 2)
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()
	items, next, err := remote.ListInbox(ctx, core.InboxFilter{Page: core.Page{Limit: 1}})
	if err != nil || len(items) != 1 || items[0].ID != ids[0] || next == "" {
		t.Fatalf("list = %+v, next %q, %v", items, next, err)
	}
	if _, err := remote.ReadInboxItem(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.UnreadInboxItem(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	if out, err := remote.ReadAllInbox(ctx); err != nil || out.Marked != 2 {
		t.Fatalf("read all = %+v, %v", out, err)
	}
	if n, err := remote.CountInbox(ctx); err != nil || n.Unread != 0 {
		t.Fatalf("count = %+v, %v", n, err)
	}
	if _, err := remote.PutInboxPrefs(ctx, core.InboxPrefsInput{Muted: []string{"comment"}}); err != nil {
		t.Fatal(err)
	}
	if p, err := remote.GetInboxPrefs(ctx); err != nil || len(p.Muted) != 1 {
		t.Fatalf("prefs = %+v, %v", p, err)
	}
}
