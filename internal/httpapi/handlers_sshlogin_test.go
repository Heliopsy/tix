// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// sshLoginFixture is a person in tenant A with an enrolled throwaway key.
func sshLoginFixture(t *testing.T) (*apiFixture, ssh.Signer) {
	t.Helper()
	f := newFixture(t)
	ctx := core.WithActor(context.Background(), &f.actorA)
	user, err := f.local.CreateUser(ctx, core.CreateUserInput{
		Email: "ada@example.com", Password: "correct horse battery", Role: core.RoleMember,
	})
	if err != nil {
		t.Fatalf("creating the user: %v", err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("wrapping the key: %v", err)
	}
	if _, err := f.local.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{
		ActorID: user.ID, PublicKey: string(ssh.MarshalAuthorizedKey(signer.PublicKey())),
	}); err != nil {
		t.Fatalf("enrolling: %v", err)
	}
	return f, signer
}

// signedOver asks host for a challenge, anonymously, and signs it.
func (f *apiFixture) signedOver(host string, s ssh.Signer) core.SSHLoginInput {
	f.t.Helper()
	resp := f.do(http.MethodPost, wire.RouteSSHChallenge, host, "", nil)
	mustStatus(f.t, resp, http.StatusOK)
	var challenge core.SSHChallenge
	decodeBody(f.t, resp, &challenge)
	if challenge.Namespace != "tix-login@"+host {
		f.t.Fatalf("namespace = %q, want it built from the Host %q", challenge.Namespace, host)
	}
	sig, err := auth.SignSSH(s, challenge.Namespace, []byte(challenge.Nonce))
	if err != nil {
		f.t.Fatalf("signing: %v", err)
	}
	return core.SSHLoginInput{
		PublicKey: string(ssh.MarshalAuthorizedKey(s.PublicKey())), Signature: sig, Nonce: challenge.Nonce,
	}
}

func bodyText(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}
	return string(b)
}

func TestSSHLoginRoutesIssueASessionWithoutACredential(t *testing.T) {
	f, s := sshLoginFixture(t)
	answer := f.signedOver(f.hostA, s)

	resp := f.do(http.MethodPost, wire.RouteSSHVerify, f.hostA, "", answer)
	mustStatus(t, resp, http.StatusOK)
	cookie := false
	for _, c := range resp.Cookies() {
		cookie = cookie || (c.Name == wire.SessionCookieName && c.Value != "")
	}
	var result core.LoginResult
	decodeBody(t, resp, &result)
	if result.Session == nil || result.Session.TenantID != f.tenantA.ID {
		t.Fatalf("result = %+v, want a session for tenant A", result)
	}
	if !cookie {
		t.Error("the session was not set as a cookie")
	}

	f.clock.Advance(time.Second)
	replay := f.do(http.MethodPost, wire.RouteSSHVerify, f.hostA, "", answer)
	if replay.StatusCode != http.StatusUnauthorized || !strings.Contains(bodyText(t, replay), "already used") {
		t.Fatalf("a replay answered %d, want 401 already used", replay.StatusCode)
	}
}

func TestSSHLoginRoutesRefuseWhatTheServiceRefuses(t *testing.T) {
	f, s := sshLoginFixture(t)

	stranger, err := ssh.NewSignerFromKey(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)))
	if err != nil {
		t.Fatalf("wrapping the key: %v", err)
	}
	resp := f.do(http.MethodPost, wire.RouteSSHVerify, f.hostA, "", f.signedOver(f.hostA, stranger))
	if text := bodyText(t, resp); resp.StatusCode != http.StatusUnauthorized ||
		!strings.Contains(text, core.ReasonSSHKeyNotEnrolled) {
		t.Fatalf("an unenrolled key answered %d %s, want 401 with reason %s",
			resp.StatusCode, text, core.ReasonSSHKeyNotEnrolled)
	}

	other := f.do(http.MethodPost, wire.RouteSSHVerify, f.hostB, "", f.signedOver(f.hostA, s))
	if text := bodyText(t, other); other.StatusCode != http.StatusUnauthorized ||
		!strings.Contains(text, "not valid for this server") {
		t.Fatalf("tenant A's challenge on tenant B's host answered %d %s, want 401", other.StatusCode, text)
	}

	alias := core.Domain{TenantID: f.tenantA.ID, Hostname: "alias.acme.test", CertMode: core.CertNone}
	if err := f.store.Update(context.Background(), core.TenantScope{TenantID: f.tenantA.ID},
		func(tx store.Tx) error { return tx.AddDomain(context.Background(), &alias) }); err != nil {
		t.Fatalf("adding a second domain: %v", err)
	}
	relayed := f.do(http.MethodPost, wire.RouteSSHVerify, alias.Hostname, "", f.signedOver(f.hostA, s))
	if text := bodyText(t, relayed); relayed.StatusCode != http.StatusUnauthorized ||
		!strings.Contains(text, "not valid for this server") {
		t.Fatalf("a challenge for one host of the tenant on another answered %d %s, want 401",
			relayed.StatusCode, text)
	}
}
