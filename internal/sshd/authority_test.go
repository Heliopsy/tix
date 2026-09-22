package sshd

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestASessionHoldsExactlyItsActorsAuthority asks the policy rather than the
// session. For every action tix knows, the answer for the actor a key resolved
// to must be the answer for that actor's membership and nothing else: not one
// action more, not one fewer.
func TestASessionHoldsExactlyItsActorsAuthority(t *testing.T) {
	e, _, st := newEnrolled(t)
	policy := authz.New()

	cases := []struct {
		role    core.Role
		allowed authz.Action
		denied  authz.Action
	}{
		{core.RoleViewer, authz.ActionTaskRead, authz.ActionTaskCreate},
		{core.RoleMember, authz.ActionTaskCreate, authz.ActionUserAdmin},
		{core.RoleAdmin, authz.ActionTenantAdmin, ""},
	}
	for _, tc := range cases {
		t.Run(string(tc.role), func(t *testing.T) {
			handle := "user-" + string(tc.role)
			fingerprint := "SHA256:" + handle
			enrol(t, st, "acme", handle, tc.role, fingerprint)

			session, err := e.ActorByFingerprint(asUser(NeutralUser), fingerprint)
			if err != nil {
				t.Fatalf("ActorByFingerprint: %v", err)
			}
			res := authz.Resource{TenantID: session.TenantID}

			// The membership, and only the membership. Whatever the policy says
			// about this is what the session must be able to do.
			membership := &core.Actor{
				ID: session.ID, TenantID: session.TenantID, Kind: core.ActorUser, Role: tc.role,
			}
			for _, action := range authz.Actions() {
				if got, want := policy.Allowed(session, action, res), policy.Allowed(membership, action, res); got != want {
					t.Errorf("%s on %s: session allowed = %v, the membership allows %v",
						action, tc.role, got, want)
				}
			}

			// Both actors being equally broken would satisfy the comparison
			// above, so the roles are pinned to real answers as well.
			if !policy.Allowed(session, tc.allowed, res) {
				t.Errorf("a %s session cannot %s", tc.role, tc.allowed)
			}
			if tc.denied != "" && policy.Allowed(session, tc.denied, res) {
				t.Errorf("a %s session can %s", tc.role, tc.denied)
			}
			if session.Role != tc.role {
				t.Errorf("session role = %q, want the membership's %q", session.Role, tc.role)
			}
			if session.ProjectID != "" {
				t.Errorf("session is pinned to project %q, which no key granted", session.ProjectID)
			}
		})
	}
}

// TestAnEnrolledKeyGrantsNoScopeOfItsOwn hands the lookup an actor record that
// claims every scope there is. The session must still hold only what the
// membership grants: authority comes from the role, and a key carries none.
//
// The store is wrapped rather than the row edited because the actor columns
// carry no scopes today. That is exactly why this is worth asserting: the
// clearing in enrolled.go is what keeps the claim true if they ever do.
func TestAnEnrolledKeyGrantsNoScopeOfItsOwn(t *testing.T) {
	e, _, st := newEnrolled(t)
	enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")
	e.store = scopedActorStore{Store: st}

	session, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:alice")
	if err != nil {
		t.Fatalf("ActorByFingerprint: %v", err)
	}
	if len(session.Scopes) != 0 {
		t.Fatalf("the session carries scopes of its own: %v", session.Scopes)
	}

	policy := authz.New()
	res := authz.Resource{TenantID: session.TenantID}
	if policy.Allowed(session, authz.ActionUserAdmin, res) {
		t.Fatal("a member's session administers users, so the key granted authority the membership does not")
	}
	if !policy.Allowed(session, authz.ActionTaskCreate, res) {
		t.Fatal("a member's session cannot create a task, so this proves nothing")
	}
}

// scopedActorStore reads actors that claim every scope in existence.
type scopedActorStore struct{ store.Store }

func (s scopedActorStore) View(ctx context.Context, scope core.TenantScope, fn func(store.Tx) error) error {
	return s.Store.View(ctx, scope, func(tx store.Tx) error { return fn(scopedActorTx{Tx: tx}) })
}

type scopedActorTx struct{ store.Tx }

func (t scopedActorTx) GetActor(ctx context.Context, actorID string) (*core.Actor, error) {
	a, err := t.Tx.GetActor(ctx, actorID)
	if a != nil {
		a.Scopes = []core.Scope{core.ScopeAll}
	}
	return a, err
}

// TestADemotionLandsOnTheNextConnection proves the role is read per connection
// rather than stamped on the key when it was enrolled. A key outlives the
// authority of the person holding it, so a demotion that only took effect when
// the key was replaced would be a demotion in name only.
func TestADemotionLandsOnTheNextConnection(t *testing.T) {
	e, _, st := newEnrolled(t)
	acme := enrol(t, st, "acme", "alice", core.RoleAdmin, "SHA256:alice")
	policy := authz.New()

	before, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:alice")
	if err != nil {
		t.Fatalf("before the demotion: %v", err)
	}
	res := authz.Resource{TenantID: before.TenantID}
	if !policy.Allowed(before, authz.ActionTenantAdmin, res) {
		t.Fatal("an administrator's session cannot administer the tenant, so this proves nothing")
	}

	setRole(t, st, acme, core.RoleViewer)

	after, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:alice")
	if err != nil {
		t.Fatalf("after the demotion: %v", err)
	}
	if after.Role != core.RoleViewer {
		t.Fatalf("session role = %q, want the demoted %q", after.Role, core.RoleViewer)
	}
	if policy.Allowed(after, authz.ActionTenantAdmin, res) {
		t.Fatal("a demoted key still administers the tenant")
	}
	if policy.Allowed(after, authz.ActionTaskCreate, res) {
		t.Fatal("a demoted key still writes tasks")
	}
	if !policy.Allowed(after, authz.ActionTaskRead, res) {
		t.Fatal("the demotion took away more than the role did")
	}
}

// setRole replaces an actor's membership with one carrying a different role.
func setRole(t *testing.T, st store.Store, e enrolment, role core.Role) {
	t.Helper()
	ctx := context.Background()
	scope := core.TenantScope{TenantID: e.tenant.ID}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.RemoveMember(ctx, e.actor.ID); err != nil {
			return err
		}
		return tx.AddMember(ctx, &core.Membership{
			TenantID: e.tenant.ID, ActorID: e.actor.ID, Role: role,
		})
	}); err != nil {
		t.Fatalf("setting the role of %q to %q: %v", e.actor.ID, role, err)
	}
}
