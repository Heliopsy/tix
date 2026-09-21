package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/heliopsy/tix/internal/core"
	"golang.org/x/crypto/ssh"
)

// PublicKeyLookup resolves the actor a presented key speaks for. The demo
// listener satisfies it by provisioning a sandbox on first sight; key
// enrolment will satisfy it from the keys a user has registered, and neither
// implementation needs the other to change.
type PublicKeyLookup interface {
	ActorByFingerprint(ctx context.Context, fingerprint string) (*core.Actor, error)
}

// Fingerprint renders a public key as the SHA-256 fingerprint an ssh client
// prints, so what tix records is what the operator can compare by eye.
func Fingerprint(key ssh.PublicKey) string {
	if key == nil {
		return ""
	}
	return ssh.FingerprintSHA256(key)
}

// FingerprintID renders a public key as lowercase hex, for the places that
// need the same identity as an identifier rather than as a display string.
func FingerprintID(key ssh.PublicKey) string {
	if key == nil {
		return ""
	}
	sum := sha256.Sum256(key.Marshal())
	return hex.EncodeToString(sum[:])
}

// PublicKeyVerifier resolves a presented public key into an actor.
type PublicKeyVerifier struct {
	lookup PublicKeyLookup
}

// NewPublicKeyVerifier returns a verifier backed by lookup.
func NewPublicKeyVerifier(lookup PublicKeyLookup) *PublicKeyVerifier {
	return &PublicKeyVerifier{lookup: lookup}
}

// Verify returns the actor the key speaks for, or an unauthenticated error.
func (v *PublicKeyVerifier) Verify(ctx context.Context, key ssh.PublicKey) (*core.Actor, error) {
	if key == nil {
		return nil, ErrNoCredential
	}
	actor, err := v.lookup.ActorByFingerprint(ctx, Fingerprint(key))
	if err != nil {
		return nil, err
	}
	if actor == nil {
		return nil, core.Unauthenticated("invalid credentials")
	}
	return actor, nil
}
