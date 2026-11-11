// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

const loginHost = "tix.example"

// newSigner is a throwaway ed25519 key.
func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("wrapping the key: %v", err)
	}
	return s
}

func authorizedOf(s ssh.Signer) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(s.PublicKey())))
}

// sshLoginContext is the anonymous, tenant-pinned context a sign-in arrives
// on, naming the host as the HTTP transport does.
func sshLoginContext(scope core.TenantScope) context.Context {
	return core.WithAudience(loginContext(scope), loginHost)
}

// enrolKeyFor enrols the signer's public half against an actor.
func enrolKeyFor(t *testing.T, l *Local, admin *core.Actor, actorID string, s ssh.Signer) *core.SSHKey {
	t.Helper()
	key, err := l.EnrolSSHKey(authContext(admin), core.EnrolSSHKeyInput{ActorID: actorID, PublicKey: authorizedOf(s)})
	if err != nil {
		t.Fatalf("enrolling a key: %v", err)
	}
	return key
}

// signedAnswer asks for a challenge and signs it the way the client does.
func signedAnswer(t *testing.T, l *Local, ctx context.Context, s ssh.Signer, namespace string) core.SSHLoginInput {
	t.Helper()
	challenge, err := l.SSHLoginChallenge(ctx)
	if err != nil {
		t.Fatalf("SSHLoginChallenge: %v", err)
	}
	if namespace == "" {
		namespace = challenge.Namespace
	}
	sig, err := auth.SignSSH(s, namespace, []byte(challenge.Nonce))
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	return core.SSHLoginInput{PublicKey: authorizedOf(s), Signature: sig, Nonce: challenge.Nonce}
}

func TestSSHLoginChallengeNamesTheHost(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	challenge, err := l.SSHLoginChallenge(sshLoginContext(scope))
	if err != nil {
		t.Fatalf("SSHLoginChallenge: %v", err)
	}
	if challenge.Namespace != "tix-login@tix.example" {
		t.Errorf("namespace = %q, want tix-login@tix.example", challenge.Namespace)
	}
	local, err := l.SSHLoginChallenge(loginContext(scope))
	if err != nil {
		t.Fatalf("SSHLoginChallenge: %v", err)
	}
	if local.Namespace != "tix-login@local" {
		t.Errorf("namespace without a host = %q, want tix-login@local", local.Namespace)
	}
}

func TestSSHLoginIssuesASessionAndRecordsTheMethod(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	s := newSigner(t)
	enrolKeyFor(t, l, admin, user.ID, s)
	ctx := sshLoginContext(scope)

	result, err := l.SSHLogin(ctx, signedAnswer(t, l, ctx, s, ""))
	if err != nil {
		t.Fatalf("SSHLogin: %v", err)
	}
	if result.Session == nil || result.Session.ActorID != user.ID || result.Session.TenantID != scope.TenantID {
		t.Fatalf("result = %+v, want a session for the key's person in this tenant", result)
	}
	if got := result.Session.ExpiresAt.Sub(l.clock.Now()); got != core.DefaultSessionTTL.D() {
		t.Errorf("session lives %v, want %v like a password sign-in", got, core.DefaultSessionTTL.D())
	}
	var logins []string
	for _, e := range auditEntries(t, l, admin) {
		if e.Action == auditUserLogin {
			logins = append(logins, string(e.After))
		}
	}
	if len(logins) != 1 || !strings.Contains(logins[0], `"method":"ssh-key"`) {
		t.Fatalf("user.login entries = %v, want one recording method ssh-key", logins)
	}
}

func TestPasswordLoginRecordsItsMethod(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	seedUser(t, l, admin, "ada@example.com")
	if _, err := signIn(l, scope, "ada@example.com", testPassword); err != nil {
		t.Fatalf("signIn: %v", err)
	}
	found := false
	for _, e := range auditEntries(t, l, admin) {
		if e.Action == auditUserLogin && strings.Contains(string(e.After), `"method":"password"`) {
			found = true
		}
	}
	if !found {
		t.Fatal("a password sign-in did not record method password")
	}
}

func TestSSHLoginRefusals(t *testing.T) {
	type fixture struct {
		l     *Local
		ctx   context.Context
		admin *core.Actor
		user  *core.User
		key   ssh.Signer
	}
	cases := []struct {
		name   string
		answer func(t *testing.T, f fixture) core.SSHLoginInput
		kind   core.Kind
		says   string
		reason string
	}{
		{
			name: "a key nobody enrolled",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				return signedAnswer(t, f.l, f.ctx, newSigner(t), "")
			},
			kind: core.KindUnauthenticated, says: "not enrolled", reason: core.ReasonSSHKeyNotEnrolled,
		},
		{
			name: "a revoked key",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				keys, err := f.l.ListSSHKeys(authContext(f.admin), f.user.ID)
				if err != nil || len(keys) != 1 {
					t.Fatalf("listing keys: %v %v", keys, err)
				}
				if err := f.l.RevokeSSHKey(authContext(f.admin), keys[0].ID); err != nil {
					t.Fatalf("revoking: %v", err)
				}
				return signedAnswer(t, f.l, f.ctx, f.key, "")
			},
			kind: core.KindUnauthenticated, says: "revoked", reason: core.ReasonSSHKeyRevoked,
		},
		{
			name: "a signature by another key over the enrolled key",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				in := signedAnswer(t, f.l, f.ctx, newSigner(t), "")
				in.PublicKey = authorizedOf(f.key)
				return in
			},
			kind: core.KindUnauthenticated, says: "does not verify",
		},
		{
			name: "a signature made for git",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				return signedAnswer(t, f.l, f.ctx, f.key, "git")
			},
			kind: core.KindUnauthenticated, says: `not "tix-login@tix.example"`,
		},
		{
			name: "a signature made for another server",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				return signedAnswer(t, f.l, f.ctx, f.key, "tix-login@other.example")
			},
			kind: core.KindUnauthenticated, says: `not "tix-login@tix.example"`,
		},
		{
			name: "a challenge issued for another host",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				other := core.WithAudience(f.ctx, "other.example")
				return signedAnswer(t, f.l, other, f.key, "tix-login@tix.example")
			},
			kind: core.KindUnauthenticated, says: "not valid for this server",
		},
		{
			name: "an altered challenge",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				in := signedAnswer(t, f.l, f.ctx, f.key, "")
				in.Nonce = "A" + in.Nonce[1:]
				return in
			},
			kind: core.KindUnauthenticated, says: "not valid for this server",
		},
		{
			name: "a challenge minted under another key",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				in := signedAnswer(t, f.l, f.ctx, f.key, "")
				foreign, err := auth.SSHNonceKey("")
				if err != nil {
					t.Fatalf("drawing a key: %v", err)
				}
				forged, _, err := auth.MintSSHNonce(foreign, auth.SSHNonce{
					TenantID: f.admin.TenantID, Audience: loginHost, IssuedAt: f.l.clock.Now(),
				})
				if err != nil {
					t.Fatalf("forging: %v", err)
				}
				sig, err := auth.SignSSH(f.key, "tix-login@"+loginHost, []byte(forged))
				if err != nil {
					t.Fatalf("signing: %v", err)
				}
				in.Nonce, in.Signature = forged, sig
				return in
			},
			kind: core.KindUnauthenticated, says: "not valid for this server",
		},
		{
			name: "an expired challenge",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				in := signedAnswer(t, f.l, f.ctx, f.key, "")
				f.l.clock.(interface{ Advance(time.Duration) }).Advance(auth.SSHNonceTTL + time.Second)
				return in
			},
			kind: core.KindUnauthenticated, says: "expired",
		},
		{
			name: "a key enrolled against an agent",
			answer: func(t *testing.T, f fixture) core.SSHLoginInput {
				agent := core.Actor{Kind: core.ActorAgent, Handle: "bot"}
				if err := f.l.store.Update(context.Background(), core.TenantScope{TenantID: f.admin.TenantID},
					func(tx store.Tx) error { return tx.CreateActor(context.Background(), &agent) }); err != nil {
					t.Fatalf("creating an agent: %v", err)
				}
				s := newSigner(t)
				enrolKeyFor(t, f.l, f.admin, agent.ID, s)
				return signedAnswer(t, f.l, f.ctx, s, "")
			},
			kind: core.KindUnauthenticated, says: "API token",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, _, scope, admin := newLocal(t)
			user := seedUser(t, l, admin, "ada@example.com")
			s := newSigner(t)
			enrolKeyFor(t, l, admin, user.ID, s)
			f := fixture{l: l, ctx: sshLoginContext(scope), admin: admin, user: user, key: s}

			result, err := l.SSHLogin(f.ctx, tc.answer(t, f))
			if err == nil {
				t.Fatalf("SSHLogin succeeded with %+v", result)
			}
			if !core.IsKind(err, tc.kind) || !strings.Contains(err.Error(), tc.says) {
				t.Fatalf("SSHLogin = %v, want %s saying %q", err, tc.kind, tc.says)
			}
			var domain *core.Error
			if tc.reason != "" && (!asError(err, &domain) || domain.Details["reason"] != tc.reason) {
				t.Fatalf("SSHLogin = %#v, want reason %q", err, tc.reason)
			}
		})
	}
}

func asError(err error, target **core.Error) bool {
	return errors.As(err, target)
}

func TestSSHLoginChallengeIsSingleUsePerKey(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	s := newSigner(t)
	enrolKeyFor(t, l, admin, user.ID, s)
	ctx := sshLoginContext(scope)

	answer := signedAnswer(t, l, ctx, s, "")
	if _, err := l.SSHLogin(ctx, answer); err != nil {
		t.Fatalf("first SSHLogin: %v", err)
	}
	clk.Advance(time.Second)
	if _, err := l.SSHLogin(ctx, answer); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("replayed SSHLogin = %v, want it refused as already used", err)
	}
	logins := 0
	for _, e := range auditEntries(t, l, admin) {
		if e.Action == auditUserLogin {
			logins++
		}
	}
	if logins != 1 {
		t.Fatalf("%d sign-ins recorded, want the replay to leave none", logins)
	}
	clk.Advance(time.Second)
	if _, err := l.SSHLogin(ctx, signedAnswer(t, l, ctx, s, "")); err != nil {
		t.Fatalf("a fresh challenge after a replay: %v", err)
	}
}

func TestSSHLoginSignsInToTheTenantTheRequestNames(t *testing.T) {
	l, clk, scope, admin := newLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	s := newSigner(t)
	enrolKeyFor(t, l, admin, user.ID, s)

	other := otherTenantActor(t, l)
	otherScope := core.TenantScope{TenantID: other.TenantID}
	otherUser, err := l.CreateUser(authContext(other), core.CreateUserInput{
		Email: "ada@other.example", Password: testPassword, Role: core.RoleMember,
	})
	if err != nil {
		t.Fatalf("seeding the other tenant's user: %v", err)
	}
	enrolKeyFor(t, l, other, otherUser.ID, s)

	for _, tc := range []struct {
		scope core.TenantScope
		actor string
	}{{scope, user.ID}, {otherScope, otherUser.ID}} {
		ctx := sshLoginContext(tc.scope)
		result, err := l.SSHLogin(ctx, signedAnswer(t, l, ctx, s, ""))
		if err != nil {
			t.Fatalf("SSHLogin in %s: %v", tc.scope.TenantID, err)
		}
		if result.Session.TenantID != tc.scope.TenantID || result.Session.ActorID != tc.actor {
			t.Fatalf("session = %+v, want tenant %s actor %s", result.Session, tc.scope.TenantID, tc.actor)
		}
	}

	clk.Advance(time.Second)
	ctx := sshLoginContext(scope)
	answer := signedAnswer(t, l, ctx, s, "")
	answer.Tenant = "other"
	if _, err := l.SSHLogin(ctx, answer); !core.IsKind(err, core.KindInvalid) ||
		!strings.Contains(err.Error(), `"acme"`) {
		t.Fatalf("naming another tenant = %v, want invalid naming acme", err)
	}
	answer.Tenant = "acme"
	if _, err := l.SSHLogin(ctx, answer); err != nil {
		t.Fatalf("naming the resolved tenant: %v", err)
	}
}

func TestSSHLoginStillAsksForTheSecondFactor(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user := seedUser(t, l, admin, "ada@example.com")
	secret, _ := enrol(t, l, clk, userActor(t, l, user, core.RoleMember))
	s := newSigner(t)
	enrolKeyFor(t, l, admin, user.ID, s)
	ctx := sshLoginContext(scope)

	answer := signedAnswer(t, l, ctx, s, "")
	result, err := l.SSHLogin(ctx, answer)
	if err != nil {
		t.Fatalf("SSHLogin: %v", err)
	}
	if result.Session != nil || result.Challenge == nil || result.Challenge.Kind != core.ChallengeCode {
		t.Fatalf("result = %+v, want a code challenge and no session", result)
	}
	if _, err := l.SSHLogin(ctx, answer); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("replaying into a second ticket = %v, want it refused", err)
	}
	clk.Advance(30 * time.Second)
	done, err := l.Login(loginContext(scope), core.LoginInput{
		Surface: core.SurfaceCLI, Ticket: result.Challenge.Ticket, Code: codeFor(t, secret, clk),
	})
	if err != nil || done.Session == nil {
		t.Fatalf("completing with the code = %+v, %v", done, err)
	}
	found := false
	for _, e := range auditEntries(t, l, admin) {
		if e.Action == auditUserLogin && strings.Contains(string(e.After), `"method":"ssh-key"`) {
			found = true
		}
	}
	if !found {
		t.Fatal("the sign-in completed by code was not recorded as method ssh-key")
	}
}
