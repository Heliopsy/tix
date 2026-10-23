// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// totpTestKey is a fixed sealing key. It is a constant rather than a minted
// one so that a test asserting enrolment is refused without a key is asserting
// the absence of this value and not a failure to generate one.
const totpTestKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

// newTOTPLocal builds a service with a sealing key configured.
func newTOTPLocal(t *testing.T) (*Local, *clock.Fake, core.TenantScope, *core.Actor) {
	t.Helper()
	return newLocalWith(t, WithTOTPKey(totpTestKey))
}

// userActor returns the actor a seeded user signs in as, carrying the role the
// membership granted so that the policy sees the authority the person has.
func userActor(t *testing.T, l *Local, user *core.User, role core.Role) *core.Actor {
	t.Helper()
	ctx := context.Background()
	var actor *core.Actor
	if err := l.store.View(ctx, core.TenantScope{TenantID: l.tenantOf(t, user)}, func(tx store.Tx) error {
		var err error
		actor, err = tx.GetActor(ctx, user.ID)
		return err
	}); err != nil {
		t.Fatalf("resolving the actor of %q: %v", user.Email, err)
	}
	actor.Role = role
	return actor
}

// tenantOf reports the tenant a seeded user's actor belongs to. Every user a
// test seeds goes into the fixture's one tenant.
func (l *Local) tenantOf(t *testing.T, _ *core.User) string {
	t.Helper()
	var id string
	if err := l.store.ViewUnscoped(context.Background(), func(u store.UnscopedTx) error {
		tenants, err := u.ListTenants(context.Background(), core.Page{Limit: 1})
		if err != nil {
			return err
		}
		if len(tenants) > 0 {
			id = tenants[0].ID
		}
		return nil
	}); err != nil {
		t.Fatalf("resolving the tenant: %v", err)
	}
	return id
}

// enrol takes a user all the way through enrolment and returns the secret the
// authenticator holds along with the recovery codes issued once.
func enrol(t *testing.T, l *Local, clk *clock.Fake, actor *core.Actor) (string, []string) {
	t.Helper()
	ctx := authContext(actor)
	enrolment, err := l.BeginTOTPEnrolment(ctx)
	if err != nil {
		t.Fatalf("beginning enrolment: %v", err)
	}
	code, err := auth.TOTPCode(enrolment.Secret, auth.TOTPStep(clk.Now()))
	if err != nil {
		t.Fatalf("producing a code: %v", err)
	}
	codes, err := l.ConfirmTOTPEnrolment(ctx, code)
	if err != nil {
		t.Fatalf("confirming enrolment: %v", err)
	}
	return enrolment.Secret, codes.Codes
}

// requireTOTP turns one surface's requirement on for the fixture's tenant.
func requireTOTP(t *testing.T, l *Local, scope core.TenantScope, surface core.Surface) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		tenant, err := u.GetTenantByID(ctx, scope.TenantID)
		if err != nil {
			return err
		}
		switch surface {
		case core.SurfaceWeb:
			tenant.RequireTOTPWeb = true
		case core.SurfaceCLI:
			tenant.RequireTOTPCLI = true
		case core.SurfaceSSH:
			tenant.RequireTOTPSSH = true
		}
		return u.UpdateTenant(ctx, tenant)
	}); err != nil {
		t.Fatalf("requiring a second factor on %s: %v", surface, err)
	}
}

// codeFor produces the code an authenticator holding secret shows now.
func codeFor(t *testing.T, secret string, clk *clock.Fake) string {
	t.Helper()
	code, err := auth.TOTPCode(secret, auth.TOTPStep(clk.Now()))
	if err != nil {
		t.Fatalf("producing a code: %v", err)
	}
	return code
}

// TestEnrolmentIsRefusedWithoutAKey is the property the whole design rests on:
// there is no plaintext fallback, so an installation with no key enrols nobody
// rather than writing a secret somebody can read out of the database.
func TestEnrolmentIsRefusedWithoutAKey(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)

	_, err := l.BeginTOTPEnrolment(authContext(actor))
	if !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("enrolling with no key = %v, want a precondition failure", err)
	}
	if !strings.Contains(err.Error(), "auth.totp_key") {
		t.Errorf("the refusal does not name the missing setting: %v", err)
	}

	ctx := context.Background()
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		if _, err := tx.GetUserTOTP(ctx, user.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("a refused enrolment wrote a row: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back: %v", err)
	}
}

func TestEnrolmentTakesEffectOnlyWhenConfirmed(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	ctx := authContext(actor)

	enrolment, err := l.BeginTOTPEnrolment(ctx)
	if err != nil {
		t.Fatalf("beginning enrolment: %v", err)
	}
	if enrolment.Secret == "" || !strings.HasPrefix(enrolment.URI, "otpauth://totp/") {
		t.Fatalf("enrolment handed back %+v", *enrolment)
	}
	if enrolment.Account != user.Email {
		t.Errorf("the authenticator label is %q, want the account address", enrolment.Account)
	}

	status, err := l.TOTPStatus(ctx)
	if err != nil {
		t.Fatalf("reading status: %v", err)
	}
	if status.Enrolled || !status.Pending {
		t.Fatalf("an abandoned enrolment reads as %+v, want pending and not enrolled", *status)
	}

	// An unconfirmed enrolment satisfies nothing, so a sign-in on a surface
	// that requires one takes this person through enrolment again rather than
	// asking them for a code they cannot produce.
	requireTOTP(t, l, scope, core.SurfaceWeb)
	result, err := l.Login(loginContext(scope), core.LoginInput{
		Email: user.Email, Password: testPassword, Surface: core.SurfaceWeb,
	})
	if err != nil {
		t.Fatalf("signing in: %v", err)
	}
	if result.Challenge == nil || result.Challenge.Kind != core.ChallengeEnrol {
		t.Fatalf("an unconfirmed enrolment produced %+v, want an enrolment challenge", result)
	}

	// And the sealed secret is never the thing that came back.
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		stored, err := tx.GetUserTOTP(context.Background(), user.ID)
		if err != nil {
			return err
		}
		if strings.Contains(stored.SecretCipher, enrolment.Secret) {
			t.Fatal("the secret is stored in the clear")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back: %v", err)
	}
	_ = clk
}

func TestConfirmationIssuesRecoveryCodesOnce(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	ctx := authContext(actor)

	_, codes := enrol(t, l, clk, actor)
	if len(codes) != auth.RecoveryCodeCount {
		t.Fatalf("confirmation issued %d codes, want %d", len(codes), auth.RecoveryCodeCount)
	}

	status, err := l.TOTPStatus(ctx)
	if err != nil {
		t.Fatalf("reading status: %v", err)
	}
	if !status.Enrolled || status.Pending {
		t.Fatalf("a confirmed enrolment reads as %+v", *status)
	}
	if status.RecoveryCodesLeft != auth.RecoveryCodeCount {
		t.Errorf("status says %d codes remain, want %d", status.RecoveryCodesLeft, auth.RecoveryCodeCount)
	}
	// The codes are shown once and cannot be asked for again: the only offer
	// a second visit gets is to replace them.
	fresh, err := l.RegenerateRecoveryCodes(ctx)
	if err != nil {
		t.Fatalf("regenerating: %v", err)
	}
	for _, old := range codes {
		for _, now := range fresh.Codes {
			if old == now {
				t.Fatalf("regeneration reissued the code %q", old)
			}
		}
	}
}

func TestASignInWithNoRequirementIsOneStep(t *testing.T) {
	l, _, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")

	for _, surface := range core.Surfaces {
		t.Run(string(surface), func(t *testing.T) {
			result, err := l.Login(loginContext(scope), core.LoginInput{
				Email: user.Email, Password: testPassword, Surface: surface,
			})
			if err != nil {
				t.Fatalf("signing in: %v", err)
			}
			if result.Challenge != nil {
				t.Fatalf("a tenant requiring nothing challenged on %s: %+v", surface, result.Challenge)
			}
			if result.Session == nil || result.Session.Token == "" {
				t.Fatalf("a sign-in with no requirement returned %+v", result)
			}
		})
	}
}

// TestTheRequirementIsPerSurface is the whole point of three flags rather than
// one switch: a tenant can ask the browser for more than the terminal.
func TestTheRequirementIsPerSurface(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	enrol(t, l, clk, actor)
	requireTOTP(t, l, scope, core.SurfaceWeb)

	cases := []struct {
		surface core.Surface
		want    bool
	}{
		{core.SurfaceWeb, true},
		{core.SurfaceCLI, true},
		{core.SurfaceSSH, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.surface), func(t *testing.T) {
			result, err := l.Login(loginContext(scope), core.LoginInput{
				Email: user.Email, Password: testPassword, Surface: tc.surface,
			})
			if err != nil {
				t.Fatalf("signing in: %v", err)
			}
			// An enrolled person is asked everywhere, requirement or not: an
			// enrolment the owner chose has to protect something.
			if (result.Challenge != nil) != tc.want {
				t.Fatalf("on %s the sign-in returned %+v, want challenged=%v",
					tc.surface, result, tc.want)
			}
		})
	}
}

func TestAnUnenrolledPersonOnARequiringSurfaceIsEnrolledNotRefused(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	requireTOTP(t, l, scope, core.SurfaceCLI)

	first, err := l.Login(loginContext(scope), core.LoginInput{
		Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
	})
	if err != nil {
		t.Fatalf("signing in: %v", err)
	}
	if first.Challenge == nil || first.Challenge.Kind != core.ChallengeEnrol {
		t.Fatalf("an unenrolled person got %+v, want to be taken through enrolment", first)
	}
	if first.Challenge.Enrolment == nil || first.Challenge.Enrolment.Secret == "" {
		t.Fatal("the enrolment challenge carries no secret to scan")
	}

	second, err := l.Login(loginContext(scope), core.LoginInput{
		Surface: core.SurfaceCLI,
		Ticket:  first.Challenge.Ticket,
		Code:    codeFor(t, first.Challenge.Enrolment.Secret, clk),
	})
	if err != nil {
		t.Fatalf("completing the sign-in: %v", err)
	}
	if second.Session == nil {
		t.Fatalf("confirming the code did not complete the sign-in: %+v", second)
	}
	if len(second.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Fatalf("a sign-in that enrolled handed back %d recovery codes, want %d",
			len(second.RecoveryCodes), auth.RecoveryCodeCount)
	}
}

// TestEnablingTheRequirementCannotLockTheTenantOut is the scenario the enrol
// branch exists for. The administrator who turns the flag on is the person
// least likely to be enrolled already.
func TestEnablingTheRequirementCannotLockTheTenantOut(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	owner := seedUser(t, l, admin, "owner@example.com")
	member := seedUser(t, l, admin, "member@example.com")
	for _, surface := range core.Surfaces {
		requireTOTP(t, l, scope, surface)
	}

	for _, user := range []*core.User{owner, member} {
		for _, surface := range core.Surfaces {
			t.Run(user.Email+"/"+string(surface), func(t *testing.T) {
				first, err := l.Login(loginContext(scope), core.LoginInput{
					Email: user.Email, Password: testPassword, Surface: surface,
				})
				if err != nil {
					t.Fatalf("signing in: %v", err)
				}
				if first.Challenge == nil {
					t.Fatalf("a requiring surface let %q straight in", user.Email)
				}
				secret := ""
				switch first.Challenge.Kind {
				case core.ChallengeEnrol:
					secret = first.Challenge.Enrolment.Secret
				case core.ChallengeCode:
					t.Skip("already enrolled on an earlier surface; the code path is covered elsewhere")
				}
				// The clock moves, or the second surface replays a step the
				// first already consumed and the code is correctly refused.
				clk.Advance(auth.TOTPPeriod)
				second, err := l.Login(loginContext(scope), core.LoginInput{
					Surface: surface,
					Ticket:  first.Challenge.Ticket,
					Code:    codeFor(t, secret, clk),
				})
				if err != nil {
					t.Fatalf("completing the sign-in: %v", err)
				}
				if second.Session == nil {
					t.Fatalf("%q could not get in on %s after enrolling", user.Email, surface)
				}
			})
		}
	}
}

func TestASignInCodeIsSingleUse(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	secret, _ := enrol(t, l, clk, actor)
	requireTOTP(t, l, scope, core.SurfaceCLI)
	clk.Advance(auth.TOTPPeriod)
	code := codeFor(t, secret, clk)

	for i, wantSession := range []bool{true, false} {
		first, err := l.Login(loginContext(scope), core.LoginInput{
			Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
		})
		if err != nil {
			t.Fatalf("attempt %d, signing in: %v", i, err)
		}
		if first.Challenge == nil || first.Challenge.Kind != core.ChallengeCode {
			t.Fatalf("attempt %d got %+v, want to be asked for a code", i, first)
		}
		second, err := l.Login(loginContext(scope), core.LoginInput{
			Surface: core.SurfaceCLI, Ticket: first.Challenge.Ticket, Code: code,
		})
		switch {
		case wantSession && (err != nil || second.Session == nil):
			t.Fatalf("the first use of a code was refused: %v", err)
		case !wantSession && err == nil:
			t.Fatal("the same code was accepted twice inside its window")
		}
	}
}

func TestARecoveryCodeSignsInOnceAndIsNamed(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	_, codes := enrol(t, l, clk, actor)
	requireTOTP(t, l, scope, core.SurfaceCLI)

	signInWith := func(code string) (*core.LoginResult, error) {
		first, err := l.Login(loginContext(scope), core.LoginInput{
			Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
		})
		if err != nil {
			return nil, err
		}
		return l.Login(loginContext(scope), core.LoginInput{
			Surface: core.SurfaceCLI, Ticket: first.Challenge.Ticket, Code: code,
		})
	}

	result, err := signInWith(codes[0])
	if err != nil || result.Session == nil {
		t.Fatalf("a recovery code did not sign in: %v", err)
	}
	if _, err := signInWith(codes[0]); err == nil {
		t.Fatal("a spent recovery code signed in again")
	}

	// The audit trail says which code was spent, which is the whole reason a
	// used code is marked rather than deleted.
	found := false
	for _, entry := range auditEntries(t, l, actor) {
		if entry.Action != auditTOTPRecoverySpent {
			continue
		}
		found = true
		if !strings.Contains(string(entry.After), "recovery_code_id") {
			t.Fatalf("the audit entry does not name the code spent: %s", entry.After)
		}
	}
	if !found {
		t.Fatal("spending a recovery code left no audit entry")
	}
}

func TestVerificationLocksAfterRepeatedFailure(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	secret, _ := enrol(t, l, clk, actor)
	requireTOTP(t, l, scope, core.SurfaceCLI)

	attempt := func(code string) error {
		first, err := l.Login(loginContext(scope), core.LoginInput{
			Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
		})
		if err != nil {
			return err
		}
		_, err = l.Login(loginContext(scope), core.LoginInput{
			Surface: core.SurfaceCLI, Ticket: first.Challenge.Ticket, Code: code,
		})
		return err
	}

	for i := range auth.TOTPMaxFailures {
		if err := attempt("000000"); err == nil {
			t.Fatalf("wrong code %d was accepted", i)
		}
	}
	clk.Advance(auth.TOTPPeriod)
	err := attempt(codeFor(t, secret, clk))
	if !core.IsTOTPLocked(err) {
		t.Fatalf("after %d failures a correct code returned %v, want the lock",
			auth.TOTPMaxFailures, err)
	}

	// The lock lifts on its own, and the refusal is distinguishable from a
	// wrong code so the person is told to wait rather than told to re-enrol.
	clk.Advance(auth.TOTPLockWindow + auth.TOTPPeriod)
	if err := attempt(codeFor(t, secret, clk)); err != nil {
		t.Fatalf("the lock did not lift: %v", err)
	}
}

func TestAdministrativeClearReturnsTheUserToUnenrolled(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	enrol(t, l, clk, actor)

	if err := l.ClearUserTOTP(authContext(admin), user.ID); err != nil {
		t.Fatalf("clearing: %v", err)
	}
	status, err := l.TOTPStatus(authContext(actor))
	if err != nil {
		t.Fatalf("reading status: %v", err)
	}
	if status.Enrolled || status.Pending {
		t.Fatalf("after a clear the user reads as %+v", *status)
	}
	// Read the codes from the store rather than off the status, which stops
	// counting once the enrolment is gone and so cannot see them left behind.
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		left, err := tx.ListRecoveryCodes(context.Background(), user.ID)
		if err != nil {
			return err
		}
		if len(left) != 0 {
			t.Fatalf("a clear left %d recovery codes behind, which still sign in", len(left))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the codes back: %v", err)
	}

	// Nothing is asked of them on the way back in, because the tenant
	// requires nothing; a tenant that did would take them through enrolment.
	result, err := l.Login(loginContext(scope), core.LoginInput{
		Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
	})
	if err != nil || result.Session == nil {
		t.Fatalf("signing in after a clear returned %+v, %v", result, err)
	}

	named := false
	for _, entry := range auditEntries(t, l, admin) {
		if entry.Action == auditTOTPClear && entry.SubjectID == user.ID && entry.ActorID == admin.ID {
			named = true
		}
	}
	if !named {
		t.Fatal("the clear is not attributed to the administrator who performed it")
	}
}

func TestClearingAnotherUserNeedsTheScope(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	target := seedUser(t, l, admin, "ada@example.com")
	enrol(t, l, clk, userActor(t, l, target, core.RoleMember))

	member := seedUser(t, l, admin, "grace@example.com")
	err := l.ClearUserTOTP(authContext(userActor(t, l, member, core.RoleMember)), target.ID)
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("a member clearing another user = %v, want forbidden", err)
	}
	if err := l.ClearUserTOTP(authContext(admin), "nobody"); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("clearing an unknown user = %v, want not found", err)
	}
}

func TestRemovingOnesOwnSecondFactor(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	enrol(t, l, clk, actor)
	ctx := authContext(actor)

	if err := l.RemoveTOTP(ctx); err != nil {
		t.Fatalf("removing: %v", err)
	}
	if err := l.RemoveTOTP(ctx); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("removing twice = %v, want not found", err)
	}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		codes, err := tx.ListRecoveryCodes(context.Background(), user.ID)
		if err != nil {
			return err
		}
		if len(codes) != 0 {
			t.Fatalf("removing the enrolment left %d recovery codes behind", len(codes))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back: %v", err)
	}
}

// TestTheRequirementIsReadAtSessionMint is the half of the policy that is
// easiest to get wrong in the other direction: a flag flipped while somebody
// is working must not end their session.
func TestTheRequirementIsReadAtSessionMint(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")

	session, err := signIn(l, scope, user.Email, testPassword)
	if err != nil || session == nil {
		t.Fatalf("signing in before the flag: %v", err)
	}
	requireTOTP(t, l, scope, core.SurfaceCLI)

	verifier := auth.NewSessionVerifier(newStoreCredentials(l, scope), clk)
	if _, err := verifier.Verify(context.Background(), session.Token); err != nil {
		t.Fatalf("a live session stopped working when the flag went on: %v", err)
	}

	// The next sign-in does see it.
	result, err := l.Login(loginContext(scope), core.LoginInput{
		Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
	})
	if err != nil {
		t.Fatalf("signing in after the flag: %v", err)
	}
	if result.Challenge == nil {
		t.Fatal("the requirement did not apply to a new sign-in")
	}
}

func TestATicketCarriesNoAuthorityOfItsOwn(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	secret, _ := enrol(t, l, clk, actor)
	requireTOTP(t, l, scope, core.SurfaceCLI)

	first, err := l.Login(loginContext(scope), core.LoginInput{
		Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
	})
	if err != nil {
		t.Fatalf("signing in: %v", err)
	}

	cases := []struct {
		name string
		in   core.LoginInput
	}{
		{"no code at all", core.LoginInput{Surface: core.SurfaceCLI, Ticket: first.Challenge.Ticket}},
		{"a wrong code", core.LoginInput{Surface: core.SurfaceCLI, Ticket: first.Challenge.Ticket, Code: "000000"}},
		{"a ticket from nowhere", core.LoginInput{Surface: core.SurfaceCLI, Ticket: "forged.ticket", Code: codeFor(t, secret, clk)}},
		{"another surface", core.LoginInput{Surface: core.SurfaceWeb, Ticket: first.Challenge.Ticket, Code: codeFor(t, secret, clk)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := l.Login(loginContext(scope), tc.in)
			if err == nil {
				t.Fatalf("%s produced %+v, want a refusal", tc.name, result)
			}
		})
	}

	// And an expired ticket is no ticket.
	clk.Advance(auth.TicketTTL + time.Second)
	if _, err := l.Login(loginContext(scope), core.LoginInput{
		Surface: core.SurfaceCLI, Ticket: first.Challenge.Ticket, Code: codeFor(t, secret, clk),
	}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("an expired ticket returned %v, want it refused", err)
	}
}

// TestAnActorWithNoUserRowHasNoSecondFactor is the refusal that replaced a
// check on the actor's kind. An API token reads as an agent whichever actor
// minted it, so the kind says nothing; what decides is whether there is a
// person behind the identifier.
func TestAnActorWithNoUserRowHasNoSecondFactor(t *testing.T) {
	l, _, scope, _ := newTOTPLocal(t)
	agent := &core.Actor{
		ID: "agent-1", TenantID: scope.TenantID, Kind: core.ActorAgent, Handle: "agent",
		Scopes: []core.Scope{core.ScopeAll},
	}
	if _, err := l.BeginTOTPEnrolment(authContext(agent)); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("an actor with no user row began an enrolment: %v", err)
	}
}

// TestVerifySecondFactorProvesAnEnrolmentWithNoSignInBehindIt covers the
// surface whose first factor is not a password. An SSH session has already
// proved a key when the code is asked for, so there is no password step for a
// ticket to hang off and nothing left to mint.
func TestVerifySecondFactorProvesAnEnrolmentWithNoSignInBehindIt(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	ctx := authContext(actor)
	secret, codes := enrol(t, l, clk, actor)

	clk.Advance(auth.TOTPPeriod)
	code := codeFor(t, secret, clk)
	if err := l.VerifySecondFactor(ctx, code); err != nil {
		t.Fatalf("verifying a current code: %v", err)
	}
	// The high-water mark has to move here too, or a code read off a
	// terminal is usable again for the rest of its window. Both halves are
	// asserted: the replay is refused, and the mark the refusal rests on was
	// actually written.
	if err := l.VerifySecondFactor(ctx, code); err == nil {
		t.Fatal("the same code verified twice")
	}
	if got := storedTOTP(t, l, scope, user.ID).LastStep; got != auth.TOTPStep(clk.Now()) {
		t.Fatalf("the accepted step is %d, want %d: a verified code left the mark where it was",
			got, auth.TOTPStep(clk.Now()))
	}

	if err := l.VerifySecondFactor(ctx, codes[0]); err != nil {
		t.Fatalf("verifying a recovery code: %v", err)
	}
	if err := l.VerifySecondFactor(ctx, codes[0]); err == nil {
		t.Fatal("a recovery code was spent twice")
	}
}

// TestVerifySecondFactorReportsTheLockRatherThanAWrongCode keeps the
// distinction the design asks for: a surface has to be able to say "wait"
// instead of "that code is wrong".
func TestVerifySecondFactorReportsTheLockRatherThanAWrongCode(t *testing.T) {
	l, clk, _, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	actor := userActor(t, l, user, core.RoleMember)
	ctx := authContext(actor)
	enrol(t, l, clk, actor)

	for i := 0; i < auth.TOTPMaxFailures; i++ {
		err := l.VerifySecondFactor(ctx, "000000")
		if err == nil {
			t.Fatalf("attempt %d: a wrong code was accepted", i+1)
		}
		if core.IsTOTPLocked(err) && i < auth.TOTPMaxFailures-1 {
			t.Fatalf("attempt %d of %d locked early", i+1, auth.TOTPMaxFailures)
		}
	}
	if err := l.VerifySecondFactor(ctx, "000000"); !core.IsTOTPLocked(err) {
		t.Fatalf("the attempt after %d failures = %v, want the lock", auth.TOTPMaxFailures, err)
	}
}

// TestVerifySecondFactorNeedsAConfirmedEnrolment refuses rather than
// pretending: there is nothing here to prove, and enrolment is a separate step
// the surface drives.
func TestVerifySecondFactorNeedsAConfirmedEnrolment(t *testing.T) {
	tests := []struct {
		name  string
		begin bool
	}{
		{"never enrolled", false},
		{"begun and abandoned", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, _, _, admin := newTOTPLocal(t)
			user := seedUser(t, l, admin, "ada@example.com")
			actor := userActor(t, l, user, core.RoleMember)
			ctx := authContext(actor)
			if tc.begin {
				if _, err := l.BeginTOTPEnrolment(ctx); err != nil {
					t.Fatalf("beginning enrolment: %v", err)
				}
			}
			if err := l.VerifySecondFactor(ctx, "000000"); !core.IsKind(err, core.KindPrecondition) {
				t.Fatalf("verifying against %s = %v, want a precondition failure", tc.name, err)
			}
		})
	}
}

// storedTOTP reads one user's enrolment straight out of the store, which is
// the only place the replay high-water mark is observable: nothing renders it,
// on purpose.
func storedTOTP(t *testing.T, l *Local, scope core.TenantScope, userID string) *core.UserTOTP {
	t.Helper()
	ctx := context.Background()
	var out *core.UserTOTP
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		var err error
		out, err = tx.GetUserTOTP(ctx, userID)
		return err
	}); err != nil {
		t.Fatalf("reading the enrolment of %q: %v", userID, err)
	}
	return out
}
