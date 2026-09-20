package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

func adminCtx(actor *core.Actor) context.Context {
	return core.WithActor(context.Background(), actor)
}

func putEndpoint(t *testing.T, l *Local, ctx context.Context, in core.WebhookInput) *core.WebhookEndpoint {
	t.Helper()
	e, err := l.PutWebhook(ctx, in)
	if err != nil {
		t.Fatalf("PutWebhook: %v", err)
	}
	return e
}

// dumpHistory returns every audit entry and event of a tenant as raw JSON, so
// a test can assert that a value appears nowhere in either.
func dumpHistory(t *testing.T, l *Local, scope core.TenantScope) string {
	t.Helper()
	ctx := context.Background()
	var sb strings.Builder
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 500}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			b, err := json.Marshal(e)
			if err != nil {
				return err
			}
			sb.Write(b)
			sb.Write(e.Before)
			sb.Write(e.After)
		}
		events, err := tx.ReadEvents(ctx, 0, 500)
		if err != nil {
			return err
		}
		for _, e := range events {
			b, err := json.Marshal(e)
			if err != nil {
				return err
			}
			sb.Write(b)
			p, err := json.Marshal(e.Payload)
			if err != nil {
				return err
			}
			sb.Write(p)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading history: %v", err)
	}
	return sb.String()
}

func TestPutWebhookCreatesListsAndDeletes(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := adminCtx(actor)

	created := putEndpoint(t, l, ctx, core.WebhookInput{
		URL:        "https://hooks.example.com/tix",
		EventTypes: []string{"task.*"},
		Active:     true,
	})
	if created.ID == "" {
		t.Fatal("created endpoint has no identifier")
	}
	if created.TenantID != scope.TenantID {
		t.Errorf("endpoint tenant = %q, want %q", created.TenantID, scope.TenantID)
	}

	list, err := l.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("ListWebhooks = %+v, want the created endpoint", list)
	}
	if list[0].URL != "https://hooks.example.com/tix" {
		t.Errorf("endpoint url = %q", list[0].URL)
	}

	updated := putEndpoint(t, l, ctx, core.WebhookInput{
		ID:         created.ID,
		URL:        "https://hooks.example.com/other",
		EventTypes: []string{"task.created"},
		Active:     false,
	})
	if updated.ID != created.ID {
		t.Errorf("update created a second endpoint %q", updated.ID)
	}
	list, err = l.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(list) != 1 || list[0].URL != "https://hooks.example.com/other" || list[0].Active {
		t.Fatalf("after update = %+v", list)
	}

	if err := l.DeleteWebhook(ctx, created.ID); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}
	list, err = l.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("endpoint survived deletion: %+v", list)
	}
	if err := l.DeleteWebhook(ctx, created.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting twice = %v, want not found", err)
	}
}

func TestPutWebhookRecordsAuditAndEvent(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := adminCtx(actor)

	beforeEvents, beforeAudits := countRows(t, l, scope)
	created := putEndpoint(t, l, ctx, core.WebhookInput{URL: "https://hooks.example.com/tix", Active: true})
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Fatalf("put wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}

	if err := l.DeleteWebhook(ctx, created.ID); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}
	delEvents, delAudits := countRows(t, l, scope)
	if delEvents != afterEvents+1 || delAudits != afterAudits+1 {
		t.Fatalf("delete wrote %d events and %d audit entries, want one of each",
			delEvents-afterEvents, delAudits-afterAudits)
	}
}

// The generated secret is handed back once, at creation, so the receiving end
// can be configured. It must never be readable again.
func TestGeneratedSecretIsReturnedOnceAndNeverAgain(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := adminCtx(actor)

	created := putEndpoint(t, l, ctx, core.WebhookInput{URL: "https://hooks.example.com/tix", Active: true})
	secret := created.Secret
	if secret == "" {
		t.Fatal("no secret was generated for an endpoint registered without one")
	}

	list, err := l.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	for _, e := range list {
		if e.Secret != "" {
			t.Errorf("listing disclosed the signing secret of %q", e.ID)
		}
	}

	again := putEndpoint(t, l, ctx, core.WebhookInput{ID: created.ID, URL: created.URL, Active: true})
	if again.Secret != "" {
		t.Error("updating an endpoint echoed its signing secret back")
	}

	if history := dumpHistory(t, l, scope); strings.Contains(history, secret) {
		t.Error("the signing secret appears in an audit entry or an event payload")
	}

	var stored string
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		e, err := tx.GetWebhook(context.Background(), created.ID)
		if err != nil {
			return err
		}
		stored = e.Secret
		return nil
	}); err != nil {
		t.Fatalf("reading endpoint: %v", err)
	}
	if stored != secret {
		t.Error("the generated secret was not persisted, so no delivery could be signed")
	}
}

func TestSuppliedSecretIsNeverEchoedBack(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := adminCtx(actor)

	const supplied = "s3cr3t-value-from-the-operator"
	created := putEndpoint(t, l, ctx, core.WebhookInput{
		URL: "https://hooks.example.com/tix", Secret: supplied, Active: true,
	})
	if created.Secret != "" {
		t.Error("a caller-supplied secret was echoed back in the returned endpoint")
	}
	if history := dumpHistory(t, l, scope); strings.Contains(history, supplied) {
		t.Error("a caller-supplied secret reached the audit log or the event stream")
	}

	if err := l.DeleteWebhook(ctx, created.ID); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}
	if history := dumpHistory(t, l, scope); strings.Contains(history, supplied) {
		t.Error("deletion wrote the signing secret into history")
	}
}

func TestPutWebhookKeepsTheExistingSecretOnUpdate(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := adminCtx(actor)

	created := putEndpoint(t, l, ctx, core.WebhookInput{URL: "https://hooks.example.com/tix", Active: true})
	secret := created.Secret

	putEndpoint(t, l, ctx, core.WebhookInput{ID: created.ID, URL: "https://hooks.example.com/moved", Active: true})

	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		e, err := tx.GetWebhook(context.Background(), created.ID)
		if err != nil {
			return err
		}
		if e.Secret != secret {
			t.Errorf("update replaced the signing secret; deliveries would fail verification")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading endpoint: %v", err)
	}
}

func TestPutWebhookRejectsBadTargets(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := adminCtx(actor)

	cases := []struct {
		name string
		in   core.WebhookInput
	}{
		{"empty", core.WebhookInput{URL: ""}},
		{"relative", core.WebhookInput{URL: "/hooks"}},
		{"not http", core.WebhookInput{URL: "ftp://example.com/hooks"}},
		{"no host", core.WebhookInput{URL: "https://"}},
		{"plaintext to the internet", core.WebhookInput{URL: "http://hooks.example.com/tix"}},
		{"bad filter", core.WebhookInput{URL: "https://hooks.example.com", EventTypes: []string{"*.created"}}},
		{"empty filter", core.WebhookInput{URL: "https://hooks.example.com", EventTypes: []string{" "}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.PutWebhook(ctx, tc.in); !core.IsKind(err, core.KindInvalid) {
				t.Errorf("PutWebhook(%+v) = %v, want a validation error", tc.in, err)
			}
		})
	}

	list, err := l.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("a rejected registration was stored: %+v", list)
	}
}

func TestPutWebhookAllowsPlaintextToLoopbackOrWithOptOut(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := adminCtx(actor)

	for _, target := range []string{"http://127.0.0.1:8080/hooks", "http://localhost:9000/hooks", "http://[::1]:9000/hooks"} {
		if _, err := l.PutWebhook(ctx, core.WebhookInput{URL: target, Active: true}); err != nil {
			t.Errorf("PutWebhook(%q) = %v, want a loopback target to be accepted", target, err)
		}
	}

	// The opt-out is a constructor option rather than an ambient environment
	// variable, so a deployment cannot weaken this by accident.
	lax, _, _, laxActor := newLocalWith(t, WithInsecureWebhooks(true))
	if _, err := lax.PutWebhook(adminCtx(laxActor),
		core.WebhookInput{URL: "http://hooks.example.com/tix", Active: true}); err != nil {
		t.Errorf("PutWebhook with the explicit opt-out = %v, want it accepted", err)
	}
}

func TestWebhookAdminRequiresTheScope(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := adminCtx(actor)
	created := putEndpoint(t, l, ctx, core.WebhookInput{URL: "https://hooks.example.com/tix", Active: true})

	viewer := &core.Actor{ID: "v1", TenantID: actor.TenantID, Kind: core.ActorUser, Role: core.RoleViewer}
	vctx := core.WithActor(context.Background(), viewer)

	if _, err := l.PutWebhook(vctx, core.WebhookInput{URL: "https://hooks.example.com/x", Active: true}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer PutWebhook = %v, want forbidden", err)
	}
	if _, err := l.ListWebhooks(vctx); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer ListWebhooks = %v, want forbidden", err)
	}
	if err := l.DeleteWebhook(vctx, created.ID); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer DeleteWebhook = %v, want forbidden", err)
	}
	if _, _, err := l.ListDeliveries(vctx, core.DeliveryFilter{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer ListDeliveries = %v, want forbidden", err)
	}
	if err := l.RedeliverWebhook(vctx, "whatever"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer RedeliverWebhook = %v, want forbidden", err)
	}

	anon := context.Background()
	if _, err := l.ListWebhooks(anon); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("anonymous ListWebhooks = %v, want unauthenticated", err)
	}
}

// otherTenant registers a second tenant with its own endpoint and one queued
// delivery, so cross-tenant lookups can be checked against real rows.
func otherTenant(t *testing.T, l *Local) (endpointID, deliveryID string) {
	t.Helper()
	ctx := context.Background()
	tenant := core.Tenant{Key: "other", Name: "Other"}
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		e := core.WebhookEndpoint{URL: "https://other.example.com/hooks", Secret: "other-secret", Active: true}
		if err := tx.PutWebhook(ctx, &e); err != nil {
			return err
		}
		endpointID = e.ID
		d := core.WebhookDelivery{EndpointID: e.ID, EventSeq: 1}
		if err := tx.EnqueueDelivery(ctx, &d); err != nil {
			return err
		}
		deliveryID = d.ID
		return nil
	}); err != nil {
		t.Fatalf("seeding other tenant: %v", err)
	}
	return endpointID, deliveryID
}

func TestWebhookCrossTenantReportsNotFound(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := adminCtx(actor)
	endpointID, deliveryID := otherTenant(t, l)

	if _, err := l.PutWebhook(ctx, core.WebhookInput{ID: endpointID, URL: "https://hooks.example.com/tix", Active: true}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("updating another tenant's endpoint = %v, want not found", err)
	}
	if err := l.DeleteWebhook(ctx, endpointID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting another tenant's endpoint = %v, want not found", err)
	}
	if err := l.RedeliverWebhook(ctx, deliveryID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("redelivering another tenant's delivery = %v, want not found", err)
	}
	list, err := l.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("listing returned another tenant's endpoints: %+v", list)
	}
	deliveries, _, err := l.ListDeliveries(ctx, core.DeliveryFilter{})
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 0 {
		t.Errorf("listing returned another tenant's deliveries: %+v", deliveries)
	}
}

// seedDeliveries queues n deliveries for an endpoint, each a tick apart so the
// keyset order is well defined.
func seedWebhookDeliveries(t *testing.T, l *Local, scope core.TenantScope, endpointID string, n int) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, 0, n)
	for i := range n {
		if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
			d := core.WebhookDelivery{EndpointID: endpointID, EventSeq: int64(i + 1)}
			if err := tx.EnqueueDelivery(ctx, &d); err != nil {
				return err
			}
			ids = append(ids, d.ID)
			return nil
		}); err != nil {
			t.Fatalf("enqueueing delivery: %v", err)
		}
	}
	return ids
}

func TestListDeliveriesPaginatesByCursor(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := adminCtx(actor)
	endpoint := putEndpoint(t, l, ctx, core.WebhookInput{URL: "https://hooks.example.com/tix", Active: true})
	want := seedWebhookDeliveries(t, l, scope, endpoint.ID, 7)

	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		page, next, err := l.ListDeliveries(ctx, core.DeliveryFilter{
			EndpointID: endpoint.ID,
			Page:       core.Page{Limit: 2, Cursor: cursor},
		})
		if err != nil {
			t.Fatalf("ListDeliveries: %v", err)
		}
		pages++
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
		for _, d := range page {
			if seen[d.ID] {
				t.Fatalf("delivery %q was returned on two pages", d.ID)
			}
			seen[d.ID] = true
		}
		if next == "" {
			break
		}
		if next == cursor {
			t.Fatal("the cursor did not advance")
		}
		cursor = next
	}
	if len(seen) != len(want) {
		t.Fatalf("paged over %d deliveries, want %d", len(seen), len(want))
	}
	for _, id := range want {
		if !seen[id] {
			t.Errorf("delivery %q was never returned", id)
		}
	}
}

func TestListDeliveriesFiltersAndRejectsBadPages(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := adminCtx(actor)
	endpoint := putEndpoint(t, l, ctx, core.WebhookInput{URL: "https://hooks.example.com/tix", Active: true})
	ids := seedWebhookDeliveries(t, l, scope, endpoint.ID, 3)

	if err := l.store.Update(context.Background(), scope, func(tx store.Tx) error {
		return tx.MarkDelivered(context.Background(), ids[0], 200, l.clock.Now())
	}); err != nil {
		t.Fatalf("marking delivered: %v", err)
	}

	done, next, err := l.ListDeliveries(ctx, core.DeliveryFilter{
		Statuses: []core.DeliveryStatus{core.DeliveryDelivered},
	})
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if next != "" {
		t.Errorf("a short page returned a next cursor %q", next)
	}
	if len(done) != 1 || done[0].ID != ids[0] {
		t.Fatalf("delivered listing = %+v", done)
	}

	if _, _, err := l.ListDeliveries(ctx, core.DeliveryFilter{Page: core.Page{Sort: "nonsense"}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an unknown sort field = %v, want a validation error", err)
	}
	if _, _, err := l.ListDeliveries(ctx, core.DeliveryFilter{Page: core.Page{Limit: -1}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a negative limit = %v, want a validation error", err)
	}
	if _, _, err := l.ListDeliveries(ctx, core.DeliveryFilter{Page: core.Page{Cursor: "not-a-cursor"}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a malformed cursor = %v, want a validation error", err)
	}
	mismatched := core.Cursor{SortValue: "x", ID: "y", Sort: core.SortCreatedAt, Direction: core.Ascending}.Encode()
	if _, _, err := l.ListDeliveries(ctx, core.DeliveryFilter{Page: core.Page{Cursor: mismatched}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a cursor from another ordering = %v, want a validation error", err)
	}
}

func TestRedeliverWebhookRequeuesTheDelivery(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := adminCtx(actor)
	endpoint := putEndpoint(t, l, ctx, core.WebhookInput{URL: "https://hooks.example.com/tix", Active: true})
	ids := seedWebhookDeliveries(t, l, scope, endpoint.ID, 1)

	if err := l.store.Update(context.Background(), scope, func(tx store.Tx) error {
		return tx.MarkFailed(context.Background(), ids[0], 500, "endpoint responded 500", l.clock.Now(), true)
	}); err != nil {
		t.Fatalf("failing the delivery: %v", err)
	}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	if err := l.RedeliverWebhook(ctx, ids[0]); err != nil {
		t.Fatalf("RedeliverWebhook: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("redelivery wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}

	pending, _, err := l.ListDeliveries(ctx, core.DeliveryFilter{
		Statuses: []core.DeliveryStatus{core.DeliveryPending},
	})
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != ids[0] {
		t.Fatalf("redelivery did not requeue the delivery: %+v", pending)
	}
	if pending[0].EndpointID != endpoint.ID || pending[0].EventSeq != 1 {
		t.Errorf("redelivery changed the target or the event: %+v", pending[0])
	}

	if err := l.RedeliverWebhook(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("redelivering an unknown delivery = %v, want not found", err)
	}
}

func TestWebhookIdentifiersAreRequired(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := adminCtx(actor)

	if err := l.DeleteWebhook(ctx, "  "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("DeleteWebhook with a blank identifier = %v, want a validation error", err)
	}
	if err := l.RedeliverWebhook(ctx, "  "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("RedeliverWebhook with a blank identifier = %v, want a validation error", err)
	}
}
