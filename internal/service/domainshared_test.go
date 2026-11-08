// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// sharedSetup gives a global admin, a tenant admin of the same tenant, and a
// second tenant "other" that maps other.example.com.
func sharedSetup(t *testing.T) (*Local, context.Context, context.Context, *core.Tenant) {
	t.Helper()
	l, _, _, actor := newLocal(t)
	root, _ := globalAdmin(t, l, actor, "root@acme.test")
	admin := tenantAdmin(t, l, actor, "admin@acme.test")
	created, err := l.CreateTenant(root, core.CreateTenantInput{Key: "other", Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	inner, err := l.inTenant(root, "other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.AddDomain(inner, core.AddDomainInput{Hostname: "other.example.com"}); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	return l, root, admin, &created.Tenant
}

// domainTrail counts the domain.update audit entries and domain.updated
// events recorded in a tenant.
func domainTrail(t *testing.T, l *Local, tenantID string) (audits, events int) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.View(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		entries, err := tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 1000}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Action == auditDomainUpdate {
				audits++
			}
		}
		found, err := tx.ReadEvents(ctx, 0, 1000)
		for _, e := range found {
			if e.Type == eventDomainUpdated {
				events++
			}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return audits, events
}

func TestAGlobalAdminMarksADomainOfAnotherTenantShared(t *testing.T) {
	l, root, _, other := sharedSetup(t)
	on, off := true, false

	got, err := l.UpdateDomain(root, "Other.Example.com:443", core.UpdateDomainInput{Shared: &on})
	if err != nil {
		t.Fatalf("UpdateDomain shared: %v", err)
	}
	if !got.Shared || got.TenantID != other.ID {
		t.Fatalf("UpdateDomain = %+v, want shared and still the other tenant's", got)
	}
	_, shared, err := resolveHost(t, l, "other.example.com")
	if err != nil || !shared {
		t.Fatalf("store reads shared = %v, %v, want true", shared, err)
	}
	if a, e := domainTrail(t, l, other.ID); a != 1 || e != 1 {
		t.Fatalf("after marking shared: %d audit entries and %d events, want one of each", a, e)
	}

	if _, err := l.UpdateDomain(root, "other.example.com", core.UpdateDomainInput{Shared: &on}); err != nil {
		t.Fatalf("UpdateDomain to the value it holds: %v", err)
	}
	if a, e := domainTrail(t, l, other.ID); a != 1 || e != 1 {
		t.Fatalf("setting the held value recorded %d audit entries and %d events, want still one", a, e)
	}

	if got, err = l.UpdateDomain(root, "other.example.com", core.UpdateDomainInput{Shared: &off}); err != nil || got.Shared {
		t.Fatalf("UpdateDomain unshare = %+v, %v", got, err)
	}
	if a, e := domainTrail(t, l, other.ID); a != 2 || e != 2 {
		t.Fatalf("after unsharing: %d audit entries and %d events, want two of each", a, e)
	}

	listed, err := l.ListTenantDomains(root, "other")
	if err != nil || len(listed) != 1 || listed[0].Shared {
		t.Fatalf("ListTenantDomains = %+v, %v, want the one unshared domain", listed, err)
	}
	if _, err := l.UpdateDomain(root, "nosuch.example.com", core.UpdateDomainInput{Shared: &on}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("UpdateDomain on an unmapped host = %v, want not found", err)
	}
}

func TestOnlyAGlobalAdminMarksADomainShared(t *testing.T) {
	l, root, admin, other := sharedSetup(t)
	on := true
	adminActor, _ := core.ActorFrom(admin)
	inner, err := l.inTenant(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.AddDomain(inner, core.AddDomainInput{Hostname: "acme.example.com"}); err != nil {
		t.Fatal(err)
	}
	token := core.WithActor(context.Background(), &core.Actor{ID: adminActor.ID, TenantID: adminActor.TenantID,
		Kind: core.ActorAgent, TokenID: "t-admin", Scopes: []core.Scope{core.ScopeAll}})
	callers := map[string]context.Context{"tenant admin session": admin, "tenant admin * token": token}
	for name, ctx := range callers {
		for _, host := range []string{"acme.example.com", "other.example.com", "nosuch.example.com"} {
			if _, err := l.UpdateDomain(ctx, host, core.UpdateDomainInput{Shared: &on}); !core.IsKind(err, core.KindForbidden) {
				t.Errorf("%s marking %s shared = %v, want forbidden", name, host, err)
			}
		}
		if _, err := l.ListTenantDomains(ctx, "other"); !core.IsKind(err, core.KindForbidden) {
			t.Errorf("%s listing another tenant's domains = %v, want forbidden", name, err)
		}
	}
	for _, host := range []string{"acme.example.com", "other.example.com"} {
		if _, shared, err := resolveHost(t, l, host); err != nil || shared {
			t.Errorf("%s shared = %v, %v after refused attempts, want false", host, shared, err)
		}
	}
	if a, e := domainTrail(t, l, other.ID); a != 0 || e != 0 {
		t.Errorf("refused attempts recorded %d audit entries and %d events, want none", a, e)
	}
}

// resolveHost reads a host's tenant and shared flag straight from the store.
func resolveHost(t *testing.T, l *Local, host string) (*core.Tenant, bool, error) {
	t.Helper()
	var (
		tenant *core.Tenant
		shared bool
	)
	err := l.store.ViewUnscoped(context.Background(), func(u store.UnscopedTx) error {
		var err error
		tenant, shared, err = u.ResolveHost(context.Background(), host)
		return err
	})
	return tenant, shared, err
}
