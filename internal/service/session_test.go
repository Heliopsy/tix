package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// loginContext returns the unauthenticated, tenant-pinned context a login
// arrives on.
func loginContext(scope core.TenantScope) context.Context {
	return core.WithTenant(context.Background(), scope)
}

// seedUser creates a user with a password and returns it.
func seedUser(t *testing.T, l *Local, admin *core.Actor, email string) *core.User {
	t.Helper()
	user, err := l.CreateUser(authContext(admin), core.CreateUserInput{
		Email: email, Password: testPassword, Role: core.RoleMember,
	})
	if err != nil {
		t.Fatalf("seeding user %q: %v", email, err)
	}
	return user
}

func TestLoginIssuesASessionWhoseHashAloneIsStored(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")

	session, err := l.Login(loginContext(scope), "Ada@Example.com", testPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if session.Token == "" {
		t.Fatal("Login returned no token")
	}
	if session.ActorID != user.ID || session.TenantID != scope.TenantID {
		t.Errorf("session = %+v, want the user's actor and tenant", session)
	}
	if want := clk.Now().Add(SessionTTL); !session.ExpiresAt.Equal(want) {
		t.Errorf("expiry = %s, want %s", session.ExpiresAt, want)
	}

	ctx := context.Background()
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		if _, _, err := tx.GetSessionByHash(ctx, session.Token); !core.IsKind(err, core.KindNotFound) {
			t.Error("the raw session token is stored in the database")
		}
		actorID, expires, err := tx.GetSessionByHash(ctx, auth.HashToken(session.Token))
		if err != nil {
			return err
		}
		if actorID != user.ID {
			t.Errorf("stored session actor = %q, want %q", actorID, user.ID)
		}
		if expires.Before(clk.Now()) {
			t.Error("the stored session is already expired")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the session back: %v", err)
	}
}

// A wrong password, an unknown address and a disabled account must be one
// outcome, or login becomes an account enumeration oracle.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	seedUser(t, l, admin, "ada@example.com")

	disabled := seedUser(t, l, admin, "grace@example.com")
	yes := true
	if _, err := l.UpdateUser(authContext(admin), disabled.ID, core.UpdateUserInput{Disabled: &yes}); err != nil {
		t.Fatalf("disabling the account: %v", err)
	}
	noPassword, err := l.CreateUser(authContext(admin), core.CreateUserInput{Email: "mute@example.com"})
	if err != nil {
		t.Fatalf("creating a passwordless user: %v", err)
	}
	other := otherTenantActor(t, l)
	foreign, err := l.CreateUser(authContext(other), core.CreateUserInput{
		Email: "eve@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("creating a user in the other tenant: %v", err)
	}
	_, _ = noPassword, foreign

	cases := []struct {
		name     string
		email    string
		password string
	}{
		{"wrong password", "ada@example.com", "not-the-password"},
		{"unknown email", "nobody@example.com", testPassword},
		{"disabled account", "grace@example.com", testPassword},
		{"no password set", "mute@example.com", testPassword},
		{"user of another tenant", "eve@example.com", testPassword},
		{"empty email", "", testPassword},
		{"empty password", "ada@example.com", ""},
	}
	want := errInvalidCredentials()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session, err := l.Login(loginContext(scope), tc.email, tc.password)
			if session != nil {
				t.Fatal("a failed login returned a session")
			}
			if !core.IsKind(err, core.KindUnauthenticated) {
				t.Fatalf("Login = %v, want unauthenticated", err)
			}
			if err.Error() != want.Error() {
				t.Errorf("Login = %q, want the uniform %q", err, want)
			}
		})
	}
}

// Absence must not be detectable by timing, so a login for an address nobody
// holds still verifies against a hash.
func TestLoginForAnUnknownEmailStillCostsAHashVerification(t *testing.T) {
	// The real hasher, so the verification dominates: under the cheap test
	// parameters the cost being measured is smaller than the database noise
	// around it, which made this assertion a coin flip.
	l, _, scope, admin := newLocalWith(t, WithHasher(auth.NewHasher()))
	seedUser(t, l, admin, "ada@example.com")
	l.dummyHash()

	measure := func(email string) time.Duration {
		best := time.Hour
		for range 3 {
			start := time.Now()
			if _, err := l.Login(loginContext(scope), email, "not-the-password"); err == nil {
				t.Fatal("a wrong password logged in")
			}
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}

	known := measure("ada@example.com")
	unknown := measure("nobody@example.com")
	if unknown*3 < known {
		t.Errorf("an unknown email cost %s against %s for a known one; absence is detectable by timing",
			unknown, known)
	}
}

func TestLoginRequiresATenant(t *testing.T) {
	l, _, _, _ := newLocal(t)

	if _, err := l.Login(context.Background(), "ada@example.com", testPassword); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("Login without a tenant = %v, want invalid", err)
	}
}

// A hash minted under weaker parameters is upgraded the next time its owner
// proves the password, which is the only moment the plaintext is available.
func TestLoginUpgradesAWeakHash(t *testing.T) {
	// The rest of the suite runs a cheap hasher, so this test needs the real
	// policy for there to be anything to upgrade to.
	l, _, scope, admin := newLocalWith(t, WithHasher(auth.NewHasher()))
	user := seedUser(t, l, admin, "ada@example.com")

	weak, err := auth.NewHasherWithParams(auth.TestParams()).Hash(testPassword)
	if err != nil {
		t.Fatalf("minting a weak hash: %v", err)
	}
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		stored, err := tx.GetUser(ctx, user.ID)
		if err != nil {
			return err
		}
		return tx.UpdateUser(ctx, stored, weak)
	}); err != nil {
		t.Fatalf("installing the weak hash: %v", err)
	}

	if _, err := l.Login(loginContext(scope), "ada@example.com", testPassword); err != nil {
		t.Fatalf("Login: %v", err)
	}
	upgraded := storedPasswordHash(t, l, "ada@example.com")
	if upgraded == weak {
		t.Fatal("a weak hash survived a successful login")
	}
	if auth.NeedsRehash(upgraded, auth.DefaultParams()) {
		t.Error("the upgraded hash is still weaker than the policy")
	}
	if err := auth.Verify(upgraded, testPassword); err != nil {
		t.Errorf("the upgraded hash does not verify the password: %v", err)
	}
}

func TestLogoutInvalidatesTheSession(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")

	session, err := l.Login(loginContext(scope), "ada@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	creds := newStoreCredentials(l, scope)
	verifier := auth.NewSessionVerifier(creds, clk)
	ctx := context.Background()
	actor, err := verifier.Verify(ctx, session.Token)
	if err != nil {
		t.Fatalf("verifying a fresh session: %v", err)
	}
	if actor.ID != user.ID {
		t.Errorf("verified actor = %q, want %q", actor.ID, user.ID)
	}

	out := WithSessionToken(authContext(actor), session.Token)
	if err := l.Logout(out); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := verifier.Verify(ctx, session.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("verifying after logout = %v, want unauthenticated", err)
	}
	if err := l.Logout(out); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("a second logout = %v, want unauthenticated", err)
	}
}

func TestLogoutNeedsAnActorAndAToken(t *testing.T) {
	l, _, _, admin := newLocal(t)

	if err := l.Logout(context.Background()); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("Logout with no actor = %v, want unauthenticated", err)
	}
	if err := l.Logout(authContext(admin)); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("Logout with no session token = %v, want unauthenticated", err)
	}
}

// Nothing a credential is made of may survive in history, which operators read
// and ship elsewhere.
func TestNoCredentialReachesHistory(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)

	user := seedUser(t, l, admin, "ada@example.com")
	session, err := l.Login(loginContext(scope), "ada@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "agent", ActorID: user.ID, Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	next := "another-long-enough-secret"
	if _, err := l.UpdateUser(ctx, user.ID, core.UpdateUserInput{Password: &next}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	logout := WithSessionToken(authContext(&core.Actor{
		ID: user.ID, TenantID: scope.TenantID, Kind: core.ActorUser, Role: core.RoleMember,
	}), session.Token)
	if err := l.Logout(logout); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	secrets := map[string]string{
		"the password":      testPassword,
		"the new password":  next,
		"the session token": session.Token,
		"the session hash":  auth.HashToken(session.Token),
		"the token value":   issued.Token,
		"the token hash":    auth.HashToken(issued.Token),
		"the stored hash":   storedPasswordHash(t, l, "ada@example.com"),
	}

	history := historyText(t, l, scope)
	for name, secret := range secrets {
		if secret == "" {
			t.Fatalf("%s is empty; the test proves nothing", name)
		}
		if strings.Contains(history, secret) {
			t.Errorf("%s appears in the audit log or an event payload", name)
		}
	}
}

// historyText renders every audit entry and event as one searchable string.
func historyText(t *testing.T, l *Local, scope core.TenantScope) string {
	t.Helper()
	ctx := context.Background()
	var sb strings.Builder
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 1000}})
		if err != nil {
			return err
		}
		for _, e := range entries {
			sb.WriteString(e.Action + " " + e.SubjectType + " " + e.SubjectID + " ")
			sb.Write(e.Before)
			sb.Write(e.After)
			sb.WriteString("\n")
		}
		events, err := tx.ReadEvents(ctx, 0, 1000)
		if err != nil {
			return err
		}
		for _, e := range events {
			sb.WriteString(string(e.Type) + " " + e.SubjectID + " ")
			for k, v := range e.Payload {
				fmt.Fprintf(&sb, "%s=%v ", k, v)
			}
			sb.WriteString("\n")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading history: %v", err)
	}
	return sb.String()
}

// sessionCredentials reads sessions for the verifier used in tests.
type sessionCredentials struct {
	local *Local
	scope core.TenantScope
}

func newStoreCredentials(l *Local, scope core.TenantScope) *sessionCredentials {
	return &sessionCredentials{local: l, scope: scope}
}

func (c *sessionCredentials) SessionByHash(ctx context.Context, hash string) (*auth.StoredSession, error) {
	var out *auth.StoredSession
	err := c.local.store.View(ctx, c.scope, func(tx store.Tx) error {
		actorID, expires, err := tx.GetSessionByHash(ctx, hash)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		out = &auth.StoredSession{
			TokenHash: hash,
			ActorID:   actorID,
			TenantID:  c.scope.TenantID,
			ExpiresAt: expires,
		}
		return nil
	})
	return out, err
}
