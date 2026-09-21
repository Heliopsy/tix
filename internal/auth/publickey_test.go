package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"golang.org/x/crypto/ssh"
)

// testKey returns a fresh public key.
func testKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("wrapping key: %v", err)
	}
	return key
}

// stubLookup resolves one fingerprint and nothing else.
type stubLookup struct {
	fingerprint string
	actor       *core.Actor
	err         error
}

func (s stubLookup) ActorByFingerprint(_ context.Context, fingerprint string) (*core.Actor, error) {
	if s.err != nil {
		return nil, s.err
	}
	if fingerprint != s.fingerprint {
		return nil, nil
	}
	return s.actor, nil
}

func TestFingerprintMatchesWhatAnSSHClientPrints(t *testing.T) {
	key := testKey(t)
	if got := Fingerprint(key); !strings.HasPrefix(got, "SHA256:") {
		t.Errorf("Fingerprint = %q, want the SHA256 form", got)
	}
	if got := FingerprintID(key); len(got) != 64 {
		t.Errorf("FingerprintID = %q, want 64 hex characters", got)
	}
	if Fingerprint(nil) != "" || FingerprintID(nil) != "" {
		t.Error("a nil key produced a fingerprint")
	}
	other := testKey(t)
	if Fingerprint(key) == Fingerprint(other) {
		t.Error("two different keys share a fingerprint")
	}
}

func TestPublicKeyVerifier(t *testing.T) {
	known := testKey(t)
	stranger := testKey(t)
	actor := &core.Actor{ID: "a", TenantID: "t", Handle: "visitor"}
	v := NewPublicKeyVerifier(stubLookup{fingerprint: Fingerprint(known), actor: actor})

	tests := []struct {
		name string
		key  ssh.PublicKey
		kind core.Kind
	}{
		{"a known key resolves", known, ""},
		{"an unknown key is refused", stranger, core.KindUnauthenticated},
		{"no key abstains", nil, core.KindUnauthenticated},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := v.Verify(context.Background(), tc.key)
			if tc.kind == "" {
				if err != nil || got != actor {
					t.Fatalf("Verify = %v, %v", got, err)
				}
				return
			}
			if !core.IsKind(err, tc.kind) {
				t.Fatalf("Verify error = %v, want kind %q", err, tc.kind)
			}
		})
	}
}

func TestPublicKeyVerifierPassesTheLookupFailureThrough(t *testing.T) {
	want := core.Precondition("full")
	v := NewPublicKeyVerifier(stubLookup{err: want})
	if _, err := v.Verify(context.Background(), testKey(t)); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("Verify error = %v, want the lookup's own failure", err)
	}
}
