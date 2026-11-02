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

// Ways a person can sign in, recorded on the audit entry and the event.
const (
	LoginMethodPassword = "password"
	LoginMethodSSHKey   = "ssh-key"
)

// SSHLoginNamespacePrefix opens the sshsig namespace a sign-in signature is
// made under. The audience follows it, so a signature names the server it was
// made for and is worthless for git, for file signing and on any other host.
const SSHLoginNamespacePrefix = "tix-login@"

// AudienceLocal is the audience of a sign-in that reaches a database directly,
// where there is no host to name.
const AudienceLocal = "local"

// SSHLoginNamespace returns the namespace a signature for audience is made under.
func SSHLoginNamespace(audience string) string {
	if strings.TrimSpace(audience) == "" {
		audience = AudienceLocal
	}
	return SSHLoginNamespacePrefix + strings.ToLower(strings.TrimSpace(audience))
}

// Reasons an SSH sign-in refusal carries, so a client walking several keys
// knows the next one may still work.
const (
	ReasonSSHKeyNotEnrolled = "ssh_key_not_enrolled"
	ReasonSSHKeyRevoked     = "ssh_key_revoked"
)

// SSHChallenge is what a client signs to prove it holds a key. Nonce is opaque
// and signed by the server; Namespace is what the server expects the signature
// to be made under, which a client checks against its own idea of the host
// rather than adopting.
type SSHChallenge struct {
	Nonce     string    `json:"nonce" yaml:"nonce"`
	Namespace string    `json:"namespace" yaml:"namespace"`
	ExpiresAt time.Time `json:"expires_at" yaml:"expires_at"`
}

// SSHLoginInput is the signed answer to a challenge.
type SSHLoginInput struct {
	// PublicKey is the key in authorized_keys form.
	PublicKey string `json:"public_key" yaml:"public_key"`
	// Signature is the armored sshsig block over the nonce.
	Signature string `json:"signature" yaml:"signature"`
	Nonce     string `json:"nonce" yaml:"nonce"`
	// Tenant, when set, is the tenant key the caller means. A server that
	// resolved another one refuses rather than signing in somewhere else.
	Tenant string `json:"tenant,omitempty" yaml:"tenant,omitempty"`
}

// Validate checks the shape of the input.
func (in SSHLoginInput) Validate() error {
	switch {
	case strings.TrimSpace(in.PublicKey) == "":
		return Invalid("a public key is required")
	case strings.TrimSpace(in.Signature) == "":
		return Invalid("a signature is required")
	case strings.TrimSpace(in.Nonce) == "":
		return Invalid("a challenge nonce is required")
	}
	return nil
}
