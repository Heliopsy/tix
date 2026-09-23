// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
	"time"
)

// SSHKey is a public key enrolled against an actor, letting that actor reach
// the terminal interface over SSH without a password.
//
// The key is a credential, so it is tenant-scoped like every other credential
// here: the same public key enrolled in two tenants is two rows, and revoking
// one leaves the other standing. Fingerprint is the SHA-256 form an ssh client
// prints, so what an operator compares by eye is what is stored.
type SSHKey struct {
	ID          string     `json:"id" yaml:"id"`
	TenantID    string     `json:"tenant_id" yaml:"tenant_id"`
	ActorID     string     `json:"actor_id" yaml:"actor_id"`
	Fingerprint string     `json:"fingerprint" yaml:"fingerprint"`
	PublicKey   string     `json:"public_key" yaml:"public_key"`
	Label       string     `json:"label,omitempty" yaml:"label,omitempty"`
	CreatedAt   time.Time  `json:"created_at" yaml:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty" yaml:"last_used_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty" yaml:"revoked_at,omitempty"`
}

// Active reports whether the key may still authenticate. Unlike a token a key
// has no expiry: it stands until somebody revokes it.
func (k SSHKey) Active() bool { return k.RevokedAt == nil }

// EnrolSSHKeyInput enrols one public key against an actor.
type EnrolSSHKeyInput struct {
	// ActorID is the actor the key speaks for. Empty means the calling actor,
	// which is the common case: enrolling one's own key.
	ActorID string `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	// PublicKey is the key in authorized_keys form, as ssh-keygen writes it.
	PublicKey string `json:"public_key" yaml:"public_key"`
	// Label is a human note for telling one key from another.
	Label string `json:"label,omitempty" yaml:"label,omitempty"`
}

// hasKeyTypePrefix reports whether the text opens with an SSH key type.
func hasKeyTypePrefix(key string) bool {
	return strings.HasPrefix(key, "ssh-") ||
		strings.HasPrefix(key, "ecdsa-") ||
		strings.HasPrefix(key, "sk-")
}

// Validate checks the shape of the input.
//
// It deliberately stops at the shape. Parsing a public key needs an SSH
// implementation, and core imports only the standard library, so the authority
// on whether these bytes are a key is the service layer. What is caught here is
// the mistake worth a clear message: a file path, a private key, or nothing.
func (in EnrolSSHKeyInput) Validate() error {
	key := strings.TrimSpace(in.PublicKey)
	if key == "" {
		return Invalid("a public key is required")
	}
	if strings.Contains(key, "PRIVATE KEY") {
		return Invalid("that is a private key; enrol the public half, usually the matching .pub file")
	}
	if !hasKeyTypePrefix(key) {
		// An authorized_keys line may carry options ahead of the key type. Those
		// are refused rather than discarded: a submitter who pasted a line
		// meaning to restrict the key would otherwise be left believing a
		// restriction applied when none was recorded. Saying so is safer than
		// accepting the key and dropping the directive.
		// The message says what is wrong without quoting the submission back.
		// Echoing attacker-supplied text into an error that the web interface,
		// the command line and the JSON API all render is how reflected content
		// gets a foothold, and it can carry whatever the submitter pasted.
		if field, _, ok := strings.Cut(key, " "); ok && strings.ContainsAny(field, "=,\"") {
			return Invalid("authorized_keys options are not supported here; remove the leading options field and enrol the key on its own")
		}
		return Invalid("a public key in authorized_keys form is required, such as the contents of ~/.ssh/id_ed25519.pub")
	}
	if len(in.Label) > 200 {
		return Invalid("label must be 200 characters or fewer")
	}
	return nil
}
