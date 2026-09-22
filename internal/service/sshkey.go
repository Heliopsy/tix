package service

import (
	"context"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Events emitted for enrolled SSH keys.
const (
	eventSSHKeyEnrolled core.EventType = "sshkey.enrolled"
	eventSSHKeyRevoked  core.EventType = "sshkey.revoked"
)

// Audit actions recorded for enrolled SSH keys.
const (
	auditSSHKeyEnrol  = "sshkey.enrol"
	auditSSHKeyRevoke = "sshkey.revoke"
)

// parsedKey is what enrolment keeps from a submission: the key itself and
// nothing that came with it.
type parsedKey struct {
	canonical   string
	fingerprint string
}

// parsePublicKey reduces a submission to the key it contains.
//
// An authorized_keys line may carry options ahead of the key type, and
// ssh.ParseAuthorizedKey accepts them and hands them back separately. Anything
// storing the submitted bytes would therefore store attacker-supplied
// directives such as command="..." or no-pty, which would then be shown in the
// web interface, travel in exports, and be honoured by anything that ever wrote
// these rows to a real authorized_keys file. So the parsed key is marshalled
// back out and that is what is stored: no options, no comment, no whitespace.
//
// Storing the canonical form is also what makes the uniqueness constraint mean
// what it should. Two submissions of one key differing only in comment would
// otherwise become two rows for a single identity.
func parsePublicKey(text string) (parsedKey, error) {
	key, _, _, rest, err := ssh.ParseAuthorizedKey([]byte(text))
	if err != nil {
		return parsedKey{}, core.Invalid("that is not an ssh public key; expected the contents of a .pub file")
	}
	// A submission carrying a second key is refused rather than silently
	// enrolling the first: the submitter and the server must agree on which
	// key was registered.
	if strings.TrimSpace(string(rest)) != "" {
		return parsedKey{}, core.Invalid("enrol one key at a time; the submission carried more than one")
	}
	return parsedKey{
		canonical:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
		fingerprint: ssh.FingerprintSHA256(key),
	}, nil
}

// EnrolSSHKey registers a public key against an actor of this tenant.
//
// Authority is the same as for an API token, because a key is the same kind of
// thing: a credential that speaks for an actor. An empty ActorID enrols against
// the caller; naming another actor resolves it within this tenant, so enrolment
// cannot reach an actor of another tenant or invent one.
func (l *Local) EnrolSSHKey(ctx context.Context, in core.EnrolSSHKeyInput) (*core.SSHKey, error) {
	actor, err := l.authorize(ctx, authz.ActionTokenAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	parsed, err := parsePublicKey(in.PublicKey)
	if err != nil {
		return nil, err
	}

	var out *core.SSHKey
	err = l.write(ctx, actor, func(m *mutation) error {
		target, err := tokenActor(ctx, m.tx, actor, in.ActorID)
		if err != nil {
			return err
		}
		key := &core.SSHKey{
			ActorID:     target.ID,
			Fingerprint: parsed.fingerprint,
			PublicKey:   parsed.canonical,
			Label:       strings.TrimSpace(in.Label),
			CreatedAt:   m.now,
		}
		if err := m.tx.CreateSSHKey(ctx, key); err != nil {
			// The engines report the uniqueness violation in their own words,
			// and neither is worth showing: one names the constraint and its
			// columns. What the caller needs is that this key is already here.
			if core.KindOf(err) == core.KindConflict {
				return core.Conflict("that key is already enrolled in this tenant")
			}
			return err
		}
		out = key
		return m.Record(auditSSHKeyEnrol, eventSSHKeyEnrolled, "ssh_key", key.ID, "", nil,
			map[string]any{
				"id":          key.ID,
				"actor_id":    key.ActorID,
				"fingerprint": key.Fingerprint,
				"label":       key.Label,
			},
			map[string]any{"ssh_key_id": key.ID, "fingerprint": key.Fingerprint})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListSSHKeys returns an actor's enrolled keys, revoked ones included: a key
// that stopped working is the thing an operator most wants to see.
func (l *Local) ListSSHKeys(ctx context.Context, actorID string) ([]core.SSHKey, error) {
	actor, err := l.authorize(ctx, authz.ActionTokenAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := []core.SSHKey{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		target, err := tokenActor(ctx, tx, actor, actorID)
		if err != nil {
			return err
		}
		found, err := tx.ListSSHKeys(ctx, target.ID)
		if err != nil {
			return err
		}
		out = found
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeSSHKey stops a key authenticating.
//
// Sessions the key already holds are not cut. Watching for revocations of keys
// currently being served would put a second subscription and a second failure
// mode on the connection path; the exposure is bounded instead by the listener's
// idle timeout, and an operator who needs a hard cut restarts the listener.
func (l *Local) RevokeSSHKey(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionTokenAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return core.Invalid("ssh key identifier is required")
	}
	return l.write(ctx, actor, func(m *mutation) error {
		if err := m.tx.RevokeSSHKey(ctx, id, m.now); err != nil {
			return err
		}
		return m.Record(auditSSHKeyRevoke, eventSSHKeyRevoked, "ssh_key", id, "", nil,
			map[string]any{"id": id, "revoked_at": m.now}, map[string]any{"ssh_key_id": id})
	})
}
