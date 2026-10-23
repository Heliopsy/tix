// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// seedUser creates the global user row a second factor hangs off. user_totp
// references users(id), so no enrolment can exist without one.
func seedUser(t *testing.T, f fixture, email string) core.User {
	t.Helper()
	ctx := context.Background()
	u := core.User{Email: email, DisplayName: email}
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateUser(ctx, &u, "argon2id$fake")
	}); err != nil {
		t.Fatalf("creating user %q: %v", email, err)
	}
	return u
}

func TestUserTOTPRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	user := seedUser(t, f, "ada@example.com")

	enrolment := core.UserTOTP{
		UserID:       user.ID,
		SecretCipher: "Y2lwaGVy",
		SecretNonce:  "bm9uY2U",
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.PutUserTOTP(ctx, &enrolment)
	}); err != nil {
		t.Fatalf("writing an enrolment: %v", err)
	}

	err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetUserTOTP(ctx, user.ID)
		if err != nil {
			return err
		}
		if got.SecretCipher != enrolment.SecretCipher || got.SecretNonce != enrolment.SecretNonce {
			t.Fatalf("GetUserTOTP returned %+v, want the sealed secret back", *got)
		}
		if got.Confirmed() {
			t.Fatal("a fresh enrolment reads back confirmed")
		}
		if got.LastStep != 0 || got.Failures != 0 || got.LockedUntil != nil {
			t.Fatalf("a fresh enrolment carries attempt state: %+v", *got)
		}
		if !got.CreatedAt.Equal(clk.Now()) {
			t.Fatalf("enrolment created at %v, want %v", got.CreatedAt, clk.Now())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading back the enrolment: %v", err)
	}

	confirmed := clk.Now().Add(time.Minute)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.ConfirmUserTOTP(ctx, user.ID, confirmed)
	}); err != nil {
		t.Fatalf("confirming: %v", err)
	}
	err = s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetUserTOTP(ctx, user.ID)
		if err != nil {
			return err
		}
		if !got.Confirmed() || !got.ConfirmedAt.Equal(confirmed) {
			t.Fatalf("confirmed enrolment reads back as %+v", *got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading back after confirmation: %v", err)
	}
}

// TestUserTOTPReEnrolmentReplaces is the requirement that a second enrolment
// stops the first secret being accepted, rather than leaving two rows.
func TestUserTOTPReEnrolmentReplaces(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	user := seedUser(t, f, "ada@example.com")

	first := core.UserTOTP{UserID: user.ID, SecretCipher: "Zmlyc3Q", SecretNonce: "bm9uY2Ux"}
	second := core.UserTOTP{UserID: user.ID, SecretCipher: "c2Vjb25k", SecretNonce: "bm9uY2Uy"}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutUserTOTP(ctx, &first); err != nil {
			return err
		}
		if err := tx.ConfirmUserTOTP(ctx, user.ID, clk.Now()); err != nil {
			return err
		}
		return tx.PutUserTOTP(ctx, &second)
	}); err != nil {
		t.Fatalf("re-enrolling: %v", err)
	}

	err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetUserTOTP(ctx, user.ID)
		if err != nil {
			return err
		}
		if got.SecretCipher != second.SecretCipher {
			t.Fatalf("re-enrolment left the previous secret in place: %q", got.SecretCipher)
		}
		if got.Confirmed() {
			t.Fatal("re-enrolment kept the previous confirmation, so an unproved secret counts")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading back after re-enrolment: %v", err)
	}
}

func TestUserTOTPAttemptStatePersists(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	user := seedUser(t, f, "ada@example.com")

	enrolment := core.UserTOTP{UserID: user.ID, SecretCipher: "Y2lwaGVy", SecretNonce: "bm9uY2U"}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.PutUserTOTP(ctx, &enrolment)
	}); err != nil {
		t.Fatalf("writing an enrolment: %v", err)
	}

	until := clk.Now().Add(15 * time.Minute)
	cases := []struct {
		name  string
		write core.UserTOTP
	}{
		{"failures mount", core.UserTOTP{LastStep: 0, Failures: 5, LockedUntil: &until}},
		{"success clears them", core.UserTOTP{LastStep: 123456, Failures: 0, LockedUntil: nil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
				return tx.SaveUserTOTPAttempt(ctx, user.ID, tc.write)
			}); err != nil {
				t.Fatalf("saving attempt state: %v", err)
			}
			err := s.View(ctx, f.scope, func(tx store.Tx) error {
				got, err := tx.GetUserTOTP(ctx, user.ID)
				if err != nil {
					return err
				}
				if got.LastStep != tc.write.LastStep || got.Failures != tc.write.Failures {
					t.Fatalf("attempt state read back as step %d failures %d, want %d and %d",
						got.LastStep, got.Failures, tc.write.LastStep, tc.write.Failures)
				}
				switch {
				case tc.write.LockedUntil == nil && got.LockedUntil != nil:
					t.Fatalf("the lock survived a success: %v", got.LockedUntil)
				case tc.write.LockedUntil != nil && (got.LockedUntil == nil || !got.LockedUntil.Equal(*tc.write.LockedUntil)):
					t.Fatalf("the lock read back as %v, want %v", got.LockedUntil, *tc.write.LockedUntil)
				}
				// Nothing else about the enrolment may move on this path.
				if got.SecretCipher != enrolment.SecretCipher {
					t.Fatalf("recording an attempt changed the secret to %q", got.SecretCipher)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("reading back attempt state: %v", err)
			}
		})
	}
}

func TestUserTOTPMissingIsNotFound(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	cases := []struct {
		name string
		run  func(store.Tx) error
	}{
		{"get", func(tx store.Tx) error { _, err := tx.GetUserTOTP(ctx, "nobody"); return err }},
		{"confirm", func(tx store.Tx) error { return tx.ConfirmUserTOTP(ctx, "nobody", clk.Now()) }},
		{"delete", func(tx store.Tx) error { return tx.DeleteUserTOTP(ctx, "nobody") }},
		{"attempt", func(tx store.Tx) error {
			return tx.SaveUserTOTPAttempt(ctx, "nobody", core.UserTOTP{})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tc.run(tx) })
			if !core.IsKind(err, core.KindNotFound) {
				t.Fatalf("%s against no enrolment returned %v, want a not-found error", tc.name, err)
			}
		})
	}
}

func TestRecoveryCodesRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	user := seedUser(t, f, "ada@example.com")

	issued := []core.RecoveryCode{{CodeHash: "hash-a"}, {CodeHash: "hash-b"}, {CodeHash: "hash-c"}}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.ReplaceRecoveryCodes(ctx, user.ID, issued)
	}); err != nil {
		t.Fatalf("issuing recovery codes: %v", err)
	}

	var stored []core.RecoveryCode
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		stored, err = tx.ListRecoveryCodes(ctx, user.ID)
		return err
	}); err != nil {
		t.Fatalf("listing recovery codes: %v", err)
	}
	if len(stored) != len(issued) {
		t.Fatalf("stored %d codes, want %d", len(stored), len(issued))
	}
	for _, c := range stored {
		if c.ID == "" || c.UserID != user.ID || c.UsedAt != nil {
			t.Fatalf("a fresh code reads back as %+v", c)
		}
	}

	spent := clk.Now().Add(time.Hour)
	var ok bool
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		var err error
		ok, err = tx.SpendRecoveryCode(ctx, stored[1].ID, spent)
		return err
	}); err != nil {
		t.Fatalf("spending a code: %v", err)
	}
	if !ok {
		t.Fatal("spending an unused code reported that it was already spent")
	}

	// A code is spent once. The second attempt has to write nothing and say so,
	// or two sign-ins racing on one code would both be let through.
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		var err error
		ok, err = tx.SpendRecoveryCode(ctx, stored[1].ID, spent.Add(time.Minute))
		return err
	}); err != nil {
		t.Fatalf("spending a code twice: %v", err)
	}
	if ok {
		t.Fatal("a code was spent twice")
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		after, err := tx.ListRecoveryCodes(ctx, user.ID)
		if err != nil {
			return err
		}
		if len(after) != len(issued) {
			t.Fatalf("spending a code deleted it: %d codes left", len(after))
		}
		for _, c := range after {
			if c.ID != stored[1].ID {
				if c.UsedAt != nil {
					t.Fatalf("code %q was marked used by spending another", c.ID)
				}
				continue
			}
			if c.UsedAt == nil || !c.UsedAt.Equal(spent) {
				t.Fatalf("the spent code reads back as %+v, want used at %v", c, spent)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back after spending: %v", err)
	}
}

func TestRecoveryCodesRegenerationReplacesEverything(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	user := seedUser(t, f, "ada@example.com")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.ReplaceRecoveryCodes(ctx, user.ID, []core.RecoveryCode{{CodeHash: "old-a"}, {CodeHash: "old-b"}})
	}); err != nil {
		t.Fatalf("issuing the first set: %v", err)
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.ReplaceRecoveryCodes(ctx, user.ID, []core.RecoveryCode{{CodeHash: "new-a"}})
	}); err != nil {
		t.Fatalf("regenerating: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.ListRecoveryCodes(ctx, user.ID)
		if err != nil {
			return err
		}
		if len(got) != 1 || got[0].CodeHash != "new-a" {
			t.Fatalf("regeneration left %+v, want only the new code", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back after regeneration: %v", err)
	}
}

// TestTOTPIsOneEnrolmentAcrossTenants is the design the tables are shaped by:
// the enrolment hangs off the global user, so a second tenant the same person
// belongs to sees the enrolment they already have.
func TestTOTPIsOneEnrolmentAcrossTenants(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	acme := seed(t, s, clk, "acme")
	globex := seed(t, s, clk, "globex")
	user := seedUser(t, acme, "ada@example.com")

	enrolment := core.UserTOTP{UserID: user.ID, SecretCipher: "Y2lwaGVy", SecretNonce: "bm9uY2U"}
	if err := s.Update(ctx, acme.scope, func(tx store.Tx) error {
		if err := tx.PutUserTOTP(ctx, &enrolment); err != nil {
			return err
		}
		if err := tx.ConfirmUserTOTP(ctx, user.ID, clk.Now()); err != nil {
			return err
		}
		return tx.ReplaceRecoveryCodes(ctx, user.ID, []core.RecoveryCode{{CodeHash: "hash-a"}})
	}); err != nil {
		t.Fatalf("enrolling in the first tenant: %v", err)
	}

	if err := s.View(ctx, globex.scope, func(tx store.Tx) error {
		got, err := tx.GetUserTOTP(ctx, user.ID)
		if err != nil {
			return err
		}
		if !got.Confirmed() || got.SecretCipher != enrolment.SecretCipher {
			t.Fatalf("the second tenant sees %+v, want the one enrolment", *got)
		}
		codes, err := tx.ListRecoveryCodes(ctx, user.ID)
		if err != nil {
			return err
		}
		if len(codes) != 1 {
			t.Fatalf("the second tenant sees %d recovery codes, want 1", len(codes))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the enrolment from the second tenant: %v", err)
	}
}

func TestCountConfirmedTOTP(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	proved := seedUser(t, f, "ada@example.com")
	pending := seedUser(t, f, "grace@example.com")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		for _, u := range []core.User{proved, pending} {
			e := core.UserTOTP{UserID: u.ID, SecretCipher: "Y2lwaGVy", SecretNonce: "bm9uY2U"}
			if err := tx.PutUserTOTP(ctx, &e); err != nil {
				return err
			}
		}
		return tx.ConfirmUserTOTP(ctx, proved.ID, clk.Now())
	}); err != nil {
		t.Fatalf("enrolling two users: %v", err)
	}

	var n int
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		n, err = tx.CountConfirmedTOTP(ctx)
		return err
	}); err != nil {
		t.Fatalf("counting enrolments: %v", err)
	}
	if n != 1 {
		t.Fatalf("CountConfirmedTOTP = %d, want 1: an unproved enrolment is not one", n)
	}
}

func TestTenantRequirementsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	var fresh *core.Tenant
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		fresh, err = tx.GetTenant(ctx)
		return err
	}); err != nil {
		t.Fatalf("reading a fresh tenant: %v", err)
	}
	if fresh.RequireTOTPWeb || fresh.RequireTOTPCLI || fresh.RequireTOTPSSH {
		t.Fatalf("a fresh tenant requires a second factor: %+v", *fresh)
	}

	cases := []struct {
		name    string
		surface core.Surface
		apply   func(*core.Tenant)
	}{
		{"web alone", core.SurfaceWeb, func(t *core.Tenant) { t.RequireTOTPWeb = true }},
		{"cli alone", core.SurfaceCLI, func(t *core.Tenant) { t.RequireTOTPCLI = true }},
		{"ssh alone", core.SurfaceSSH, func(t *core.Tenant) { t.RequireTOTPSSH = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := *fresh
			want.RequireTOTPWeb, want.RequireTOTPCLI, want.RequireTOTPSSH = false, false, false
			tc.apply(&want)
			if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
				return u.UpdateTenant(ctx, &want)
			}); err != nil {
				t.Fatalf("setting the requirement: %v", err)
			}
			var got *core.Tenant
			if err := s.View(ctx, f.scope, func(tx store.Tx) error {
				var err error
				got, err = tx.GetTenant(ctx)
				return err
			}); err != nil {
				t.Fatalf("reading the requirement back: %v", err)
			}
			for _, surface := range core.Surfaces {
				if got.RequiresTOTP(surface) != (surface == tc.surface) {
					t.Fatalf("with %s required the tenant reads back as %+v", tc.surface, *got)
				}
			}
		})
	}
}
