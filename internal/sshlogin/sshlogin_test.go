// SPDX-License-Identifier: AGPL-3.0-or-later

package sshlogin

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/core"
)

// stubServer answers the two sign-in calls and records what it was sent.
type stubServer struct {
	core.Service
	namespace string
	answer    func(in core.SSHLoginInput) (*core.LoginResult, error)
	sent      []core.SSHLoginInput
}

func (s *stubServer) SSHLoginChallenge(context.Context) (*core.SSHChallenge, error) {
	return &core.SSHChallenge{Nonce: "n", Namespace: s.namespace}, nil
}

func (s *stubServer) SSHLogin(_ context.Context, in core.SSHLoginInput) (*core.LoginResult, error) {
	s.sent = append(s.sent, in)
	return s.answer(in)
}

func candidate(t *testing.T) Candidate {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return Candidate{Origin: "test", Signer: func() (ssh.Signer, error) { return s, nil }}
}

func TestLoginRefusesToSignForAnotherServer(t *testing.T) {
	srv := &stubServer{
		namespace: "tix-login@real.example",
		answer:    func(core.SSHLoginInput) (*core.LoginResult, error) { return &core.LoginResult{}, nil },
	}
	_, err := Login(context.Background(), srv, "evil.example", "", []Candidate{candidate(t)})
	if !core.IsKind(err, core.KindPrecondition) || !strings.Contains(err.Error(), "tix-login@evil.example") {
		t.Fatalf("Login = %v, want a refusal naming the namespace this client signs for", err)
	}
	if len(srv.sent) != 0 {
		t.Fatalf("a signature was sent to a server announcing another namespace: %+v", srv.sent)
	}
}

func TestLoginStopsOnARefusalTheNextKeyWouldMeetToo(t *testing.T) {
	srv := &stubServer{
		namespace: "tix-login@a.example",
		answer: func(core.SSHLoginInput) (*core.LoginResult, error) {
			return nil, core.Unauthenticated("the login challenge has expired; start again")
		},
	}
	_, err := Login(context.Background(), srv, "a.example", "acme", []Candidate{candidate(t), candidate(t)})
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("Login = %v, want the expiry", err)
	}
	if len(srv.sent) != 1 || srv.sent[0].Tenant != "acme" {
		t.Fatalf("sent %+v, want one attempt naming the tenant", srv.sent)
	}
}

func TestDiscoverFindsTheDefaultKeyFiles(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "id_rsa"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	found, closer, err := Discover("", home, nil)
	defer func() { _ = closer() }()
	if err != nil || len(found) != 1 || found[0].Origin != filepath.Join(home, ".ssh", "id_rsa") {
		t.Fatalf("Discover = %+v, %v, want the one default file present", found, err)
	}
	if _, _, err := Discover("", t.TempDir(), nil); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("Discover with nothing = %v, want a precondition naming --ssh-key", err)
	}
}
