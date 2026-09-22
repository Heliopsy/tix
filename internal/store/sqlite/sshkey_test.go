package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func enrolKey(t *testing.T, f fixture, fingerprint, label string) core.SSHKey {
	t.Helper()
	ctx := context.Background()
	k := core.SSHKey{
		ActorID:     f.actor.ID,
		Fingerprint: fingerprint,
		PublicKey:   "ssh-ed25519 AAAA" + fingerprint,
		Label:       label,
	}
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateSSHKey(ctx, &k)
	}); err != nil {
		t.Fatalf("enrolling ssh key %q: %v", fingerprint, err)
	}
	return k
}

func TestSSHKeyRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	first := enrolKey(t, f, "SHA256:one", "laptop")
	clk.Advance(time.Minute)
	second := enrolKey(t, f, "SHA256:two", "")

	err := s.View(ctx, f.scope, func(tx store.Tx) error {
		for _, want := range []core.SSHKey{first, second} {
			got, err := tx.GetSSHKey(ctx, want.ID)
			if err != nil {
				return err
			}
			if got.TenantID != f.tenant.ID || got.ActorID != f.actor.ID {
				t.Fatalf("GetSSHKey %q returned tenant %q actor %q", want.ID, got.TenantID, got.ActorID)
			}
			if got.Fingerprint != want.Fingerprint || got.PublicKey != want.PublicKey || got.Label != want.Label {
				t.Fatalf("GetSSHKey %q returned %+v, want %+v", want.ID, *got, want)
			}
			if !got.CreatedAt.Equal(want.CreatedAt) {
				t.Fatalf("GetSSHKey %q created at %v, want %v", want.ID, got.CreatedAt, want.CreatedAt)
			}
			if !got.Active() {
				t.Fatalf("a fresh key %q is not active", want.ID)
			}
		}

		keys, err := tx.ListSSHKeys(ctx, f.actor.ID)
		if err != nil {
			return err
		}
		if len(keys) != 2 {
			t.Fatalf("ListSSHKeys returned %d keys, want 2", len(keys))
		}
		if keys[0].ID != second.ID || keys[1].ID != first.ID {
			t.Fatalf("ListSSHKeys is not newest first: %q then %q", keys[0].Fingerprint, keys[1].Fingerprint)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading back ssh keys: %v", err)
	}
}

func TestSSHKeyMissingIsNotFound(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	cases := []struct {
		name string
		run  func(store.Tx) error
	}{
		{"get", func(tx store.Tx) error { _, err := tx.GetSSHKey(ctx, "nope"); return err }},
		{"revoke", func(tx store.Tx) error { return tx.RevokeSSHKey(ctx, "nope", clk.Now()) }},
		{"touch", func(tx store.Tx) error { return tx.TouchSSHKey(ctx, "nope", clk.Now()) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tc.run(tx) })
			if !core.IsKind(err, core.KindNotFound) {
				t.Fatalf("%s of an unknown key returned %v, want a not-found error", tc.name, err)
			}
		})
	}
}

func TestSSHKeyDuplicateFingerprintConflicts(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	enrolKey(t, f, "SHA256:one", "laptop")

	dup := core.SSHKey{ActorID: f.actor.ID, Fingerprint: "SHA256:one", PublicKey: "ssh-ed25519 AAAAdup"}
	err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateSSHKey(ctx, &dup)
	})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("enrolling the same fingerprint twice returned %v, want a conflict", err)
	}
}

func TestSSHKeyIsEnrolledPerTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	acme := seed(t, s, clk, "acme")
	globex := seed(t, s, clk, "globex")

	const shared = "SHA256:shared"
	mine := enrolKey(t, acme, shared, "acme laptop")
	theirs := enrolKey(t, globex, shared, "globex laptop")
	if mine.ID == theirs.ID {
		t.Fatalf("the same fingerprint in two tenants reused identifier %q", mine.ID)
	}

	for _, own := range []struct {
		fixture fixture
		want    core.SSHKey
	}{{acme, mine}, {globex, theirs}} {
		err := s.View(ctx, own.fixture.scope, func(tx store.Tx) error {
			keys, err := tx.ListSSHKeys(ctx, own.fixture.actor.ID)
			if err != nil {
				return err
			}
			if len(keys) != 1 || keys[0].ID != own.want.ID || keys[0].TenantID != own.fixture.tenant.ID {
				t.Fatalf("ListSSHKeys leaked in tenant %q: %+v", own.fixture.tenant.Key, keys)
			}
			if _, err := tx.GetSSHKey(ctx, own.want.ID); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			t.Fatalf("listing ssh keys of %q: %v", own.fixture.tenant.Key, err)
		}
	}

	err := s.View(ctx, acme.scope, func(tx store.Tx) error {
		_, err := tx.GetSSHKey(ctx, theirs.ID)
		return err
	})
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("GetSSHKey reached another tenant's key: %v", err)
	}

	var found []core.SSHKey
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		var err error
		found, err = u.FindSSHKeysByFingerprint(ctx, shared)
		return err
	}); err != nil {
		t.Fatalf("finding ssh keys by fingerprint: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("FindSSHKeysByFingerprint returned %d enrolments, want 2: %+v", len(found), found)
	}
	tenants := map[string]bool{}
	for _, k := range found {
		if k.TenantID == "" {
			t.Fatalf("FindSSHKeysByFingerprint returned a row without a tenant: %+v", k)
		}
		tenants[k.TenantID] = true
	}
	if !tenants[acme.tenant.ID] || !tenants[globex.tenant.ID] {
		t.Fatalf("FindSSHKeysByFingerprint missed a tenant: %+v", found)
	}
}

func TestSSHKeyRevocationAndTouch(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	live := enrolKey(t, f, "SHA256:live", "kept")
	doomed := enrolKey(t, f, "SHA256:doomed", "lost")

	used := clk.Now().Add(time.Hour)
	revoked := clk.Now().Add(2 * time.Hour)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.TouchSSHKey(ctx, live.ID, used); err != nil {
			return err
		}
		return tx.RevokeSSHKey(ctx, doomed.ID, revoked)
	}); err != nil {
		t.Fatalf("touching and revoking: %v", err)
	}

	err := s.View(ctx, f.scope, func(tx store.Tx) error {
		keys, err := tx.ListSSHKeys(ctx, f.actor.ID)
		if err != nil {
			return err
		}
		if len(keys) != 2 {
			t.Fatalf("ListSSHKeys hid a revoked key: %+v", keys)
		}
		got, err := tx.GetSSHKey(ctx, doomed.ID)
		if err != nil {
			return err
		}
		if got.Active() || got.RevokedAt == nil || !got.RevokedAt.Equal(revoked) {
			t.Fatalf("revoked key reads back as %+v", *got)
		}
		touched, err := tx.GetSSHKey(ctx, live.ID)
		if err != nil {
			return err
		}
		if touched.LastUsedAt == nil || !touched.LastUsedAt.Equal(used) {
			t.Fatalf("touched key reads back as %+v", *touched)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading back after revocation: %v", err)
	}

	var found []core.SSHKey
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		var err error
		found, err = u.FindSSHKeysByFingerprint(ctx, "SHA256:doomed")
		return err
	}); err != nil {
		t.Fatalf("finding a revoked fingerprint: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("FindSSHKeysByFingerprint returned a revoked key: %+v", found)
	}
}
