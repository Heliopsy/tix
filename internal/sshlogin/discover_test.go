// SPDX-License-Identifier: AGPL-3.0-or-later

package sshlogin

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/heliopsy/tix/internal/core"
)

// keyFile writes a fresh ed25519 private key under dir, encrypted when
// passphrase is non-empty, and returns its path and public key.
func keyFile(t *testing.T, dir, name, passphrase string) (string, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "test")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "test", []byte(passphrase))
	}
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("public: %v", err)
	}
	return path, sshPub
}

func TestFromFileLoadsPlainAndEncryptedKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	plain, plainPub := keyFile(t, dir, "plain", "")
	s, err := FromFile(plain, nil).Signer()
	if err != nil || string(s.PublicKey().Marshal()) != string(plainPub.Marshal()) {
		t.Fatalf("plain key: signer %v, err %v", s, err)
	}

	locked, lockedPub := keyFile(t, dir, "locked", "hunter2")
	asked := ""
	s, err = FromFile(locked, func(p string) ([]byte, error) { asked = p; return []byte("hunter2"), nil }).Signer()
	if err != nil || string(s.PublicKey().Marshal()) != string(lockedPub.Marshal()) || asked != locked {
		t.Fatalf("encrypted key: signer %v, err %v, asked %q", s, err, asked)
	}

	if _, err := FromFile(locked, nil).Signer(); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("encrypted key with no prompt = %v, want a precondition refusal", err)
	}
	refused := errors.New("cancelled")
	if _, err := FromFile(locked, func(string) ([]byte, error) { return nil, refused }).Signer(); !errors.Is(err, refused) {
		t.Fatalf("a cancelled prompt = %v, want the prompt's own error", err)
	}
	if _, err := FromFile(locked, func(string) ([]byte, error) { return []byte("wrong"), nil }).Signer(); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("a wrong passphrase = %v, want invalid", err)
	}
	if _, err := FromFile(filepath.Join(dir, "missing"), nil).Signer(); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("a missing file = %v, want invalid", err)
	}
	junk := filepath.Join(dir, "junk")
	if err := os.WriteFile(junk, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FromFile(junk, nil).Signer(); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("a file that is not a key = %v, want invalid", err)
	}
}

func TestDiscoverListsAgentKeysThenDefaultFiles(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	keyFile(t, filepath.Join(home, ".ssh"), "id_ed25519", "")
	keyFile(t, filepath.Join(home, ".ssh"), "id_rsa", "")

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: priv}); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { _ = agent.ServeAgent(keyring, conn); _ = conn.Close() }()
		}
	}()

	got, closer, err := Discover(socket, home, nil)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = closer() }()
	var origins []string
	for _, c := range got {
		origins = append(origins, filepath.Base(c.Origin))
	}
	if want := "agent id_ed25519 id_rsa"; strings.Join(origins, " ") != want {
		t.Fatalf("candidates = %v, want %s in that order", origins, want)
	}
	if s, err := got[0].Signer(); err != nil || s == nil {
		t.Fatalf("agent candidate signer = %v, %v", s, err)
	}
}

func TestDiscoverWithNothingToOfferSaysSo(t *testing.T) {
	t.Parallel()
	if _, _, err := Discover("", t.TempDir(), nil); !core.IsKind(err, core.KindPrecondition) || !strings.Contains(err.Error(), "--ssh-key") {
		t.Fatalf("Discover with no keys = %v, want a refusal pointing at --ssh-key", err)
	}
	if _, _, err := Discover(filepath.Join(t.TempDir(), "absent.sock"), "", nil); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("Discover with an unreachable agent = %v, want a precondition refusal", err)
	}
}

func TestLoginMovesPastUnenrolledKeysAndListsEveryRefusal(t *testing.T) {
	t.Parallel()
	notEnrolled := &core.Error{Kind: core.KindUnauthenticated, Message: "key not enrolled",
		Details: map[string]any{"reason": core.ReasonSSHKeyNotEnrolled}}

	calls := 0
	srv := &stubServer{namespace: core.SSHLoginNamespace("tix.example"), answer: func(core.SSHLoginInput) (*core.LoginResult, error) {
		calls++
		if calls == 1 {
			return nil, notEnrolled
		}
		return &core.LoginResult{}, nil
	}}
	broken := Candidate{Origin: "broken", Signer: func() (ssh.Signer, error) { return nil, core.Invalid("unreadable") }}
	if _, err := Login(context.Background(), srv, "tix.example", "", []Candidate{broken, candidate(t), candidate(t)}); err != nil {
		t.Fatalf("Login = %v, want the third key accepted after an unreadable and an unenrolled one", err)
	}
	if len(srv.sent) != 2 {
		t.Fatalf("signed attempts = %d, want 2", len(srv.sent))
	}

	all := &stubServer{namespace: core.SSHLoginNamespace("tix.example"),
		answer: func(core.SSHLoginInput) (*core.LoginResult, error) { return nil, notEnrolled }}
	_, err := Login(context.Background(), all, "tix.example", "", []Candidate{broken, candidate(t)})
	if !core.IsKind(err, core.KindUnauthenticated) || !strings.Contains(err.Error(), "broken: unreadable") ||
		!strings.Contains(err.Error(), "test: key not enrolled") {
		t.Fatalf("Login with no key accepted = %v, want every refusal listed", err)
	}

	if _, err := Login(context.Background(), all, "tix.example", "", []Candidate{broken}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("Login with one unreadable key = %v, want that key's own error", err)
	}

	stop := &stubServer{namespace: core.SSHLoginNamespace("tix.example"),
		answer: func(core.SSHLoginInput) (*core.LoginResult, error) { return nil, errors.New("server down") }}
	if _, err := Login(context.Background(), stop, "tix.example", "", []Candidate{candidate(t), candidate(t)}); err == nil ||
		len(stop.sent) != 1 {
		t.Fatalf("Login after a refusal that is not about the key = %v with %d attempts, want it to stop at once", err, len(stop.sent))
	}
}
