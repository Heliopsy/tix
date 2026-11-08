// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// membershipModel opens the tenant screen for a reader who is a member of the
// default tenant and of other, with a dialer recording what it was asked for.
func membershipModel(t *testing.T, configure func(*Model)) (Model, *[]string) {
	t.Helper()
	m := boardModel(t)
	svc := adminService()
	svc.whoAmI = &core.Actor{ID: "u-full", TenantID: "t1", TenantKey: "default", Memberships: []core.AccountMembership{
		{TenantID: "t1", TenantKey: "default", TenantName: "Acme Works", Role: core.RoleAdmin, Handle: "ada"},
		{TenantID: "t2", TenantKey: "other", TenantName: "Other", Role: core.RoleMember, Handle: "ada"},
	}}
	keys := &[]string{}
	closed := 0
	m.svc, m.tenantKey = svc, "default"
	m.actor = &core.Actor{ID: "u-full", TenantID: "t1", Kind: core.ActorUser, Role: core.RoleAdmin, Handle: "ada"}
	m.dialTenant = dialerFor(newFakeService(), &closed, keys)
	if configure != nil {
		configure(&m)
	}
	m, cmd := m.reduce(pressKey("T"))
	m = runCmd(t, m, cmd)
	return m, keys
}

// selectMembership moves the cursor onto the membership row naming tenant.
func selectMembership(t *testing.T, m Model, tenant string) Model {
	t.Helper()
	for i, r := range m.adminRows() {
		if r.Kind == tenantRowMembership && r.Key == tenant {
			m.adminSel = i
			return m
		}
	}
	t.Fatalf("no membership row for %q in %+v", tenant, m.adminRows())
	return m
}

func TestTheTenantScreenListsTheReadersTenantsWithTheirRoles(t *testing.T) {
	m, _ := membershipModel(t, func(m *Model) { m.reachable = func(string) bool { return false } })
	frame := m.Frame()
	if got := adminRow(t, frame, "memberships (2):", "default"); !strings.Contains(got, "admin") || !strings.Contains(got, "current") {
		t.Errorf("the current tenant's row = %q, want its role and marked current", got)
	}
	if got := adminRow(t, frame, "memberships (2):", "other"); !strings.Contains(got, "member") ||
		!strings.Contains(got, "tix login --tenant other") {
		t.Errorf("a tenant with no credential = %q, want its role and the login that reaches it", got)
	}
}

func TestOneMembershipOffersNoList(t *testing.T) {
	m, _ := membershipModel(t, func(m *Model) {
		svc := m.svc.(*fakeService)
		svc.whoAmI.Memberships = svc.whoAmI.Memberships[:1]
	})
	if strings.Contains(m.Frame(), "memberships (") {
		t.Errorf("a reader in one tenant was offered a list:\n%s", m.Frame())
	}
	for _, r := range m.adminRows() {
		if r.Kind == tenantRowMembership {
			t.Fatalf("a reader in one tenant has a membership row: %+v", r)
		}
	}
}

func TestEnterOnAReachableMembershipSwitches(t *testing.T) {
	m, keys := membershipModel(t, func(m *Model) { m.reachable = func(k string) bool { return k == "other" } })
	if got := adminRow(t, m.Frame(), "memberships (2):", "other"); !strings.Contains(got, "enter switches") {
		t.Errorf("a reachable tenant's row = %q, want it to say enter switches", got)
	}
	m = selectMembership(t, m, "other")
	m, cmd := m.reduce(pressKey("enter"))
	if m.prompt != promptNone {
		t.Fatal("enter on a membership opened the key prompt instead of switching")
	}
	m = runCmd(t, m, cmd)
	if len(*keys) != 1 || (*keys)[0] != "other" || m.tenantKey != "other" {
		t.Fatalf("dialled %v and landed on %q, want a switch to other", *keys, m.tenantKey)
	}
}

func TestEnterOnAMembershipWithNoCredentialNamesTheLogin(t *testing.T) {
	m, keys := membershipModel(t, func(m *Model) { m.reachable = func(string) bool { return false } })
	m = selectMembership(t, m, "other")
	m, cmd := m.reduce(pressKey("enter"))
	if cmd != nil || len(*keys) != 0 {
		t.Fatalf("a tenant with no credential was dialled: %v", *keys)
	}
	if !strings.Contains(m.err, "tix login --tenant other") || m.tenantKey != "default" {
		t.Fatalf("err = %q, tenant %q; want the login named and the session left where it was", m.err, m.tenantKey)
	}
}

// Over SSH the username picked the tenant, so the screen shows the reader's
// tenants and the ssh command for each rather than a switch.
func TestOverSSHTheTenantScreenNamesTheSSHCommand(t *testing.T) {
	m, keys := membershipModel(t, func(m *Model) { m.overSSH, m.dialTenant = true, nil })
	frame := m.Frame()
	if got := adminRow(t, frame, "memberships (2):", "other"); !strings.Contains(got, "ssh other@") {
		t.Errorf("over ssh the other tenant's row = %q, want the ssh command", got)
	}
	if !strings.Contains(adminSection(t, frame, "switch:"), "username picks the tenant") {
		t.Errorf("the switch section does not say the username picks the tenant:\n%s", frame)
	}
	m = selectMembership(t, m, "other")
	m, cmd := m.reduce(pressKey("enter"))
	if cmd != nil || len(*keys) != 0 || !strings.Contains(m.err, "ssh other@") {
		t.Fatalf("enter over ssh = err %q, dialled %v; want the ssh command and no dial", m.err, *keys)
	}
}

func TestAMembershipRowIsNotRemoved(t *testing.T) {
	m, _ := membershipModel(t, nil)
	m = selectMembership(t, m, "other")
	m, _ = m.reduce(pressKey("X"))
	if !strings.Contains(m.err, "not removed here") {
		t.Fatalf("X on a membership = err %q, want a refusal", m.err)
	}
}
