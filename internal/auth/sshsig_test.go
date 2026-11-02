// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/testenv"
)

func ed25519Signer(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return s, priv
}

func TestSSHSignatureRoundTripsAndIsBoundToItsNamespace(t *testing.T) {
	s, _ := ed25519Signer(t)
	msg := []byte("nonce")
	sig, err := SignSSH(s, "tix-login@a.example", msg)
	if err != nil {
		t.Fatalf("SignSSH: %v", err)
	}
	if err := VerifySSH(s.PublicKey(), sig, "tix-login@a.example", msg); err != nil {
		t.Fatalf("VerifySSH: %v", err)
	}
	for _, ns := range []string{"git", "file", "tix-login@b.example"} {
		err := VerifySSH(s.PublicKey(), sig, ns, msg)
		if !core.IsKind(err, core.KindUnauthenticated) || !strings.Contains(err.Error(), "namespace") {
			t.Errorf("verifying under %q = %v, want a namespace refusal", ns, err)
		}
	}
	if err := VerifySSH(s.PublicKey(), sig, "tix-login@a.example", []byte("other")); err == nil {
		t.Error("a signature verified over a different message")
	}
}

// TestSHA1RSASignaturesAreRefused holds the sshsig rule that ssh-rsa, which
// hashes with SHA-1, is not an acceptable signature algorithm.
func TestSHA1RSASignaturesAreRefused(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	good, err := SignSSH(s, "tix-login@a.example", []byte("n"))
	if err != nil {
		t.Fatalf("SignSSH: %v", err)
	}
	if err := VerifySSH(s.PublicKey(), good, "tix-login@a.example", []byte("n")); err != nil {
		t.Fatalf("an rsa-sha2-512 signature was refused: %v", err)
	}
	sha1 := sha1Signer{s.(ssh.AlgorithmSigner)}
	bad, err := SignSSH(sha1, "tix-login@a.example", []byte("n"))
	if err != nil {
		t.Fatalf("SignSSH: %v", err)
	}
	if err := VerifySSH(s.PublicKey(), bad, "tix-login@a.example", []byte("n")); err == nil ||
		!strings.Contains(err.Error(), "SHA-1") {
		t.Fatalf("an ssh-rsa signature = %v, want it refused as SHA-1", err)
	}
}

// sha1Signer hides SignWithAlgorithm so SignSSH falls back to ssh-rsa.
type sha1Signer struct{ s ssh.AlgorithmSigner }

func (w sha1Signer) PublicKey() ssh.PublicKey { return w.s.PublicKey() }
func (w sha1Signer) Sign(r io.Reader, data []byte) (*ssh.Signature, error) {
	return w.s.SignWithAlgorithm(r, data, ssh.KeyAlgoRSA)
}

// TestSignaturesInteroperateWithSSHKeygen proves the format is the one
// `ssh-keygen -Y` reads and writes, in both directions.
func TestSignaturesInteroperateWithSSHKeygen(t *testing.T) {
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		testenv.Skip(t, testenv.Capability{
			Name: "ssh-keygen", Why: "ssh-keygen is not on PATH, so sshsig interoperability was not checked",
			How: "install openssh-client",
		})
	}
	dir := t.TempDir()
	s, priv := ed25519Signer(t)
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	keyPath := filepath.Join(dir, "id")
	write(t, keyPath, pem.EncodeToMemory(block), 0o600)
	msgPath := filepath.Join(dir, "msg")
	write(t, msgPath, []byte("challenge"), 0o600)

	out, err := exec.Command(keygen, "-Y", "sign", "-f", keyPath, "-n", "tix-login@a.example", msgPath).CombinedOutput() // #nosec G204 -- fixed arguments
	if err != nil {
		t.Fatalf("ssh-keygen -Y sign: %v\n%s", err, out)
	}
	theirs, err := os.ReadFile(msgPath + ".sig") // #nosec G304 -- the test's own file
	if err != nil {
		t.Fatalf("reading the signature: %v", err)
	}
	if err := VerifySSH(s.PublicKey(), string(theirs), "tix-login@a.example", []byte("challenge")); err != nil {
		t.Fatalf("an ssh-keygen signature did not verify: %v", err)
	}

	ours, err := SignSSH(s, "tix-login@a.example", []byte("challenge"))
	if err != nil {
		t.Fatalf("SignSSH: %v", err)
	}
	sigPath := filepath.Join(dir, "ours.sig")
	write(t, sigPath, []byte(ours), 0o600)
	allowed := filepath.Join(dir, "allowed")
	write(t, allowed, []byte("ada "+string(ssh.MarshalAuthorizedKey(s.PublicKey()))), 0o600)
	verify := exec.Command(keygen, "-Y", "verify", "-f", allowed, "-I", "ada", "-n", "tix-login@a.example", "-s", sigPath) // #nosec G204 -- fixed arguments
	verify.Stdin = strings.NewReader("challenge")
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen -Y verify refused our signature: %v\n%s", err, out)
	}
}

func write(t *testing.T, path string, b []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestSSHNonceIsBoundToItsKeyAndExpires(t *testing.T) {
	key, err := SSHNonceKey("")
	if err != nil {
		t.Fatalf("SSHNonceKey: %v", err)
	}
	other, _ := SSHNonceKey("")
	now := time.Date(2026, 1, 1, 12, 0, 0, 123456789, time.UTC)
	nonce, minted, err := MintSSHNonce(key, SSHNonce{TenantID: "t1", Audience: "a.example", IssuedAt: now})
	if err != nil {
		t.Fatalf("MintSSHNonce: %v", err)
	}
	if !minted.IssuedAt.Equal(now.Truncate(time.Microsecond)) {
		t.Errorf("issued at %v, want it truncated to microseconds", minted.IssuedAt)
	}
	got, err := ParseSSHNonce(key, nonce, now.Add(time.Minute))
	if err != nil || got.TenantID != "t1" || got.Audience != "a.example" || !got.IssuedAt.Equal(minted.IssuedAt) {
		t.Fatalf("ParseSSHNonce = %+v, %v", got, err)
	}
	if _, err := ParseSSHNonce(other, nonce, now); err == nil {
		t.Error("a nonce verified under another key")
	}
	if _, err := ParseSSHNonce(key, nonce, now.Add(SSHNonceTTL)); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("a nonce at its expiry = %v, want it expired", err)
	}
	derived, err := SSHNonceKey("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	again, _ := SSHNonceKey("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil || string(derived) != string(again) {
		t.Errorf("a configured key derived %x and %x, want one stable key", derived, again)
	}
}
