// SPDX-License-Identifier: AGPL-3.0-or-later

package connect

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestInTenantActsAsTheLocalOwnerOfThatTenant(t *testing.T) {
	home := t.TempDir()
	ctx := context.Background()
	conn, err := Dial(ctx, load(t, home, nil), Overrides{Home: home})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Service.CreateTenant(conn.Context(ctx), core.CreateTenantInput{Key: "acme", Name: "Acme"}); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	acme, err := conn.InTenant(ctx, "ACME")
	if err != nil {
		t.Fatalf("InTenant: %v", err)
	}
	if acme.Info.TenantID == conn.Info.TenantID || acme.Actor.TenantID != acme.Info.TenantID {
		t.Fatalf("acme connection in tenant %q with actor in %q, default is %q",
			acme.Info.TenantID, acme.Actor.TenantID, conn.Info.TenantID)
	}
	if acme.Actor.Handle != LocalHandle || acme.Actor.ID == conn.Actor.ID {
		t.Fatalf("acme actor %+v, want the local actor of acme", acme.Actor)
	}
	if acme.Info.Target.Tenant != "acme" || !acme.direct {
		t.Fatalf("acme target tenant %q direct %v", acme.Info.Target.Tenant, acme.direct)
	}
	if _, err := acme.Service.CreateProject(acme.Context(ctx), core.CreateProjectInput{Key: "ops", Name: "Ops"}); err != nil {
		t.Fatalf("CreateProject in acme: %v", err)
	}
	if _, err := acme.Service.CreateTask(acme.Context(ctx), core.CreateTaskInput{Title: "acme work"}); err != nil {
		t.Fatalf("CreateTask in acme: %v", err)
	}
	assertOnlyTask(t, acme, "acme work")

	again, err := conn.InTenant(ctx, "acme")
	if err != nil || again.Actor.ID != acme.Actor.ID {
		t.Fatalf("second InTenant: actor %v, err %v; want the same local actor", again, err)
	}
	if err := acme.Close(); err != nil {
		t.Fatalf("closing a borrowed connection: %v", err)
	}
	if _, _, err := conn.Service.ListTenants(conn.Context(ctx), core.Page{}); err != nil {
		t.Fatalf("the borrowed connection closed the store: %v", err)
	}
}

func TestInTenantRefusals(t *testing.T) {
	home := t.TempDir()
	ctx := context.Background()
	conn, err := Dial(ctx, load(t, home, nil), Overrides{Home: home})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.InTenant(ctx, "nope"); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("missing tenant: %v", err)
	}
	hosted := Hosted(conn.Service, conn.Actor, core.Via{Kind: core.ViaSSH}, "default")
	if _, err := hosted.InTenant(ctx, "default"); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("hosted connection: %v", err)
	}
}
