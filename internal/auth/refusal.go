// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"errors"
	"sync"
)

// Reasons a credential or a sign-in was refused, as the refusal log names
// them. They are stable: an operator greps for them and alerts on them, and
// none of them is ever sent to the client.
const (
	// RefusedNoCredentials is a request under the API that presented nothing.
	RefusedNoCredentials = "no_credentials"
	// RefusedUnsupportedScheme is an Authorization header that is not Bearer.
	RefusedUnsupportedScheme = "unsupported_scheme"
	// RefusedMalformedToken is a bearer value that is not a tix token's shape.
	RefusedMalformedToken = "malformed_token"
	// RefusedUnknownToken is a well formed token no row in the tenant matches.
	RefusedUnknownToken = "unknown_token"
	// RefusedTokenExpired is a token past its expiry.
	RefusedTokenExpired = "token_expired"
	// RefusedTokenRevoked is a revoked token, or a rotated one past its grace.
	RefusedTokenRevoked = "token_revoked"
	// RefusedActorDisabled is a credential whose actor or account is disabled.
	RefusedActorDisabled = "actor_disabled"
	// RefusedSCIMOnly is a SCIM provisioning token used off the SCIM endpoints.
	RefusedSCIMOnly = "scim_only"
	// RefusedWrongTenant is a credential of another tenant than the host's.
	RefusedWrongTenant = "wrong_tenant"
	// RefusedUnknownSession is a session cookie no row matches. Logout and
	// revocation delete the row, so an ended session is refused as unknown.
	RefusedUnknownSession = "unknown_session"
	// RefusedSessionExpired is a session past its expiry.
	RefusedSessionExpired = "session_expired"
	// RefusedSessionIdle is a session unused for longer than its idle timeout.
	RefusedSessionIdle = "session_idle"
	// RefusedSessionInvalid is a session whose stored role is not one tix knows.
	RefusedSessionInvalid = "session_invalid"
	// RefusedBadPassword is a password sign-in for an account that exists, is
	// usable here, and whose password did not match.
	RefusedBadPassword = "bad_password"
	// RefusedUnknownAccount is a sign-in for an email with no usable account
	// in the tenant: none, disabled, not a member, or without a password.
	RefusedUnknownAccount = "unknown_account"
	// RefusedSignInExpired is a second-factor step whose sign-in ticket is
	// unreadable, expired, or for another tenant or surface.
	RefusedSignInExpired = "signin_expired"
	// RefusedTOTPInvalid is a second-factor code that did not verify.
	RefusedTOTPInvalid = "totp_invalid"
	// RefusedTOTPLocked is a second-factor code refused while verification is
	// locked after too many failures.
	RefusedTOTPLocked = "totp_locked"
	// RefusedScopeDenied is an authenticated call its scopes or role do not cover.
	RefusedScopeDenied = "scope_denied"
	// RefusedProjectDenied is a project-pinned token reaching outside its project.
	RefusedProjectDenied = "project_denied"
	// RefusedRequireClaim is a require_claim token changing a task it has not claimed.
	RefusedRequireClaim = "require_claim"
	// RefusedSecondFactorRequired is a caller the tenant's second-factor policy
	// holds back until it enrols one.
	RefusedSecondFactorRequired = "second_factor_required"
	// RefusedSSHUnknownKey is an SSH key enrolled nowhere.
	RefusedSSHUnknownKey = "ssh_unknown_key"
	// RefusedSSHWrongTenant is an SSH username naming no tenant, or one in
	// which this key is not enrolled.
	RefusedSSHWrongTenant = "ssh_wrong_tenant"
	// RefusedSSHAmbiguousKey is an SSH key enrolled in several tenants, offered
	// without a username choosing one.
	RefusedSSHAmbiguousKey = "ssh_ambiguous_key"
	// RefusedSSHNoActor is an SSH enrolment whose actor is gone or holds a
	// role tix does not know.
	RefusedSSHNoActor = "ssh_no_actor"
	// RefusedSSHSecondFactor is an SSH session that did not prove its second
	// factor, other than by being locked.
	RefusedSSHSecondFactor = "ssh_second_factor"
	// RefusedSSHKeyRevoked is an SSH sign-in with a key whose enrolment was revoked.
	RefusedSSHKeyRevoked = "ssh_key_revoked"
	// RefusedSSHAgentKey is an SSH sign-in with a key enrolled for an agent.
	RefusedSSHAgentKey = "ssh_agent_key"
	// RefusedSSHRateLimited is an SSH connection over the per-source rate limit.
	RefusedSSHRateLimited = "ssh_rate_limited"
	// RefusedSSHCapacity is an SSH connection over the concurrency cap.
	RefusedSSHCapacity = "ssh_capacity"
)

// Refusal is why a credential was refused, in terms safe to log. It never
// holds a secret: a token is named by its id and name once resolved, and
// otherwise by a reference derived from its stored hash.
type Refusal struct {
	// Reason is one of the Refused constants.
	Reason string
	// TokenID and TokenName name the stored token, when one was found.
	TokenID   string
	TokenName string
	// TokenRef names a well formed token that matched no row, so repeated
	// attempts with one value can be grouped. See CredentialRef.
	TokenRef string
	// Session names the browser session, as SessionRef does in the audit log.
	Session string
	// Fingerprint is the SSH public key's fingerprint, which is not a secret.
	Fingerprint string
}

// RefusalNote is where the verifiers leave the reason a request was refused
// for the request log to read once the request is answered.
type RefusalNote struct {
	mu  sync.Mutex
	r   Refusal
	set bool
}

type refusalKey struct{}

// WithRefusalNote returns a context carrying an empty note and the note.
func WithRefusalNote(ctx context.Context) (context.Context, *RefusalNote) {
	n := &RefusalNote{}
	return context.WithValue(ctx, refusalKey{}, n), n
}

// NoteRefusal records why the request in ctx was refused. A later note
// replaces an earlier one, because the chain tries the bearer token before
// the cookie and the last refusal is the one the caller got.
func NoteRefusal(ctx context.Context, r Refusal) {
	if n, ok := ctx.Value(refusalKey{}).(*RefusalNote); ok {
		n.mu.Lock()
		n.r, n.set = r, true
		n.mu.Unlock()
	}
}

// NoteRefusalUnlessNoted records r only when nothing more specific was noted.
func NoteRefusalUnlessNoted(ctx context.Context, r Refusal) {
	if n, ok := ctx.Value(refusalKey{}).(*RefusalNote); ok {
		n.mu.Lock()
		if !n.set {
			n.r, n.set = r, true
		}
		n.mu.Unlock()
	}
}

// ClearRefusal forgets a note, for a request that went on to authenticate.
func ClearRefusal(ctx context.Context) {
	if n, ok := ctx.Value(refusalKey{}).(*RefusalNote); ok {
		n.mu.Lock()
		n.r, n.set = Refusal{}, false
		n.mu.Unlock()
	}
}

// Get returns the refusal noted, if any.
func (n *RefusalNote) Get() (Refusal, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.r, n.set
}

// CredentialRef is the reference a refusal names an unmatched token by: a
// digest of its stored hash under a label of its own, as SessionRef is, so
// it groups attempts with one value and is neither the value nor the hash a
// lookup accepts. It is computed only for a value of the secret's shape,
// whose 256 bits make the digest useless for guessing; a malformed value may
// be a password pasted in by mistake and gets no reference.
func CredentialRef(value string) string {
	return "t-" + HashToken("token-ref:" + HashToken(value))[:16]
}

// withdrawnError is what a lookup answers for a credential whose actor lost
// its standing. It reads as invalid credentials to every caller that does not
// look for it, so a lookup that returns it refuses exactly as returning nil did.
type withdrawnError struct{ tokenID, tokenName string }

func (e *withdrawnError) Error() string { return "credential's actor is disabled" }

// Withdrawn reports a stored credential whose actor or account is disabled.
// tokenID and tokenName name a token; both are empty for a session.
func Withdrawn(tokenID, tokenName string) error {
	return &withdrawnError{tokenID: tokenID, tokenName: tokenName}
}

// IsWithdrawn reports whether err is a lookup's answer for a credential whose
// actor or account is disabled.
func IsWithdrawn(err error) bool {
	var w *withdrawnError
	return errors.As(err, &w)
}
