// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// pagedTenants serves walkTenants a fixed sequence of pages and records the
// page each call asked for, which is the only way to see a walk stop early:
// the tenants on the pages it never asked for are indistinguishable from
// tenants that were not there.
type pagedTenants struct {
	store.Store

	pages [][]core.Tenant
	asked []core.Page
}

func (p *pagedTenants) Unscoped(_ context.Context, fn func(store.UnscopedTx) error) error {
	return fn(&pagedTenantsTx{owner: p})
}

type pagedTenantsTx struct {
	store.UnscopedTx

	owner *pagedTenants
}

func (t *pagedTenantsTx) ListTenants(_ context.Context, page core.Page) ([]core.Tenant, error) {
	t.owner.asked = append(t.owner.asked, page)
	if len(t.owner.asked) > len(t.owner.pages) {
		return nil, nil
	}
	return t.owner.pages[len(t.owner.asked)-1], nil
}

// tenantPage builds n tenants whose created_at strictly increases, so the
// cursor the walk encodes from the last one is a real keyset position.
func tenantPage(prefix string, n int) []core.Tenant {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	page := make([]core.Tenant, 0, n)
	for i := 0; i < n; i++ {
		page = append(page, core.Tenant{
			ID:        fmt.Sprintf("%s-%03d", prefix, i),
			Key:       fmt.Sprintf("%s-%03d", prefix, i),
			CreatedAt: at.Add(time.Duration(i) * time.Second),
		})
	}
	return page
}

// TestWalkTenantsFollowsThePageThatCameBackFull is the page boundary the walk
// exists for. A page returning exactly the limit is the one case that looks
// like the end and is not, and a listener that stops there reaps and counts
// only the tenants of its first page.
func TestWalkTenantsFollowsThePageThatCameBackFull(t *testing.T) {
	full := tenantPage("full", core.MaxPageLimit)
	tail := tenantPage("tail", 2)
	st := &pagedTenants{pages: [][]core.Tenant{full, tail}}

	var seen []string
	if err := walkTenants(context.Background(), st, func(tn core.Tenant) {
		seen = append(seen, tn.ID)
	}); err != nil {
		t.Fatalf("walkTenants: %v", err)
	}

	if want := len(full) + len(tail); len(seen) != want {
		t.Fatalf("the walk visited %d tenants, want %d: a page that came back exactly full "+
			"ended the walk, so everything after the first page is invisible", len(seen), want)
	}
	if seen[len(seen)-1] != tail[len(tail)-1].ID {
		t.Fatalf("the last tenant visited is %q, want %q", seen[len(seen)-1], tail[len(tail)-1].ID)
	}
	if len(st.asked) != 2 {
		t.Fatalf("the walk made %d page requests, want 2: the full page and the short one that ends it",
			len(st.asked))
	}
	// Keyset, never OFFSET: the second request carries the cursor the first
	// page's last row encodes, and the first carries none.
	if st.asked[0].Cursor != "" {
		t.Errorf("the first page asked for cursor %q, want none", st.asked[0].Cursor)
	}
	if st.asked[1].Cursor == "" {
		t.Fatal("the second page asked for no cursor, so the walk is not keyset-paginated")
	}
	cursor, err := core.DecodeCursor(st.asked[1].Cursor)
	if err != nil {
		t.Fatalf("decoding the cursor the walk sent: %v", err)
	}
	if cursor.ID != full[len(full)-1].ID {
		t.Fatalf("the cursor resumes at %q, want the last row of the page before it, %q",
			cursor.ID, full[len(full)-1].ID)
	}
	for _, page := range st.asked {
		if page.Limit != core.MaxPageLimit {
			t.Errorf("a page asked for a limit of %d, want %d", page.Limit, core.MaxPageLimit)
		}
	}
}
