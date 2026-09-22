package sshd

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// NeutralUser is the username meaning "the only tenant this key is enrolled
// in". SSH always sends a username and tix has no use for one otherwise, so it
// carries the tenant when a key is enrolled in more than one.
const NeutralUser = "tix"

// enrolled resolves a presented fingerprint to the actor a registered key
// speaks for. It is the second auth.PublicKeyLookup, beside the sandbox
// provisioner, and it is what a listener serving real users runs.
type enrolled struct {
	store store.Store
	clk   clock.Clock
	log   *slog.Logger
}

var _ auth.PublicKeyLookup = (*enrolled)(nil)

// refused is the single answer every unresolved key gets.
//
// It is one sentence for every failure on purpose. An unenrolled key, a
// username naming a tenant that does not exist, and a username naming one that
// does but holds no enrolment of this key are all the same refusal, so a
// stranger cannot use the listener to discover which tenants exist.
func refused() error {
	return core.Unauthenticated(
		"this key is not enrolled here: ask an operator to enrol it with `tix user key add`")
}

// ActorByFingerprint resolves a proven fingerprint, and the username that came
// with it, to the actor the session runs as.
//
// The lookup across tenants is the one unscoped read tix makes, and it is
// narrowed to a single tenant here, before an actor is read or a session is
// opened. An ambiguous fingerprint is refused rather than guessed: guessing
// would drop somebody into the wrong tenant.
func (e *enrolled) ActorByFingerprint(ctx context.Context, fingerprint string) (*core.Actor, error) {
	if strings.TrimSpace(fingerprint) == "" {
		return nil, refused()
	}
	var found []core.SSHKey
	if err := e.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		var err error
		found, err = u.FindSSHKeysByFingerprint(ctx, fingerprint)
		return err
	}); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, refused()
	}

	key, err := e.narrow(ctx, found, usernameFrom(ctx))
	if err != nil {
		return nil, err
	}
	actor, err := e.actor(ctx, *key)
	if err != nil {
		return nil, err
	}
	e.touch(ctx, *key)
	return actor, nil
}

// narrow picks the one enrolment this connection runs as, or refuses.
func (e *enrolled) narrow(ctx context.Context, found []core.SSHKey, username string) (*core.SSHKey, error) {
	if username != "" && username != NeutralUser {
		return e.byTenantKey(ctx, found, username)
	}
	if len(found) == 1 {
		return &found[0], nil
	}
	return nil, e.ambiguous(ctx, found)
}

// byTenantKey resolves a named tenant to the enrolment held there.
//
// A tenant that does not exist and a tenant holding no enrolment of this key
// both end here, and both leave by the same door.
func (e *enrolled) byTenantKey(ctx context.Context, found []core.SSHKey, username string) (*core.SSHKey, error) {
	var tenant *core.Tenant
	if err := e.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		t, err := u.GetTenantByKey(ctx, username)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		tenant = t
		return nil
	}); err != nil {
		return nil, err
	}
	if tenant == nil || tenant.DeletedAt != nil {
		return nil, refused()
	}
	for i := range found {
		if found[i].TenantID == tenant.ID {
			return &found[i], nil
		}
	}
	return nil, refused()
}

// ambiguous refuses a fingerprint enrolled in several tenants, naming them.
//
// Naming them discloses nothing: every tenant listed is one whose administrator
// already enrolled this very key, so its holder can reach it already.
func (e *enrolled) ambiguous(ctx context.Context, found []core.SSHKey) error {
	keys, err := e.tenantKeys(ctx, found)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return refused()
	}
	return core.Unauthenticated(
		"this key is enrolled in more than one tenant: connect as one of %s, for example `ssh %s@host`",
		strings.Join(keys, ", "), keys[0])
}

// tenantKeys renders the tenant keys behind a set of enrolments, sorted.
func (e *enrolled) tenantKeys(ctx context.Context, found []core.SSHKey) ([]string, error) {
	var keys []string
	if err := e.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		keys = keys[:0]
		for _, k := range found {
			t, err := u.GetTenantByID(ctx, k.TenantID)
			if core.IsKind(err, core.KindNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if t.DeletedAt == nil {
				keys = append(keys, t.Key)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(keys)
	return keys, nil
}

// actor reads the identity and the authority behind an enrolment, inside that
// enrolment's tenant and no other.
func (e *enrolled) actor(ctx context.Context, key core.SSHKey) (*core.Actor, error) {
	scope := core.TenantScope{TenantID: key.TenantID}
	var found *core.Actor
	if err := e.store.View(ctx, scope, func(tx store.Tx) error {
		a, err := tx.GetActor(ctx, key.ActorID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		// The role is per tenant and lives on the membership, so an actor read
		// on its own carries no authority at all. It is read here rather than
		// stamped on the key, which is what makes a demotion land on the next
		// connection instead of at the end of the key's life.
		if m, err := tx.GetMember(ctx, key.ActorID); err == nil && m != nil {
			a.Role = m.Role
		}
		found = a
		return nil
	}); err != nil {
		return nil, err
	}
	if found == nil {
		return nil, refused()
	}
	if found.Role != "" && !found.Role.Valid() {
		return nil, refused()
	}
	found.TenantID = key.TenantID
	// The key grants nothing of its own: whatever authority this session holds
	// is the actor's membership, set above and nowhere else.
	found.Scopes = nil
	return found, nil
}

// touch records that a key authenticated. It runs outside every transaction
// the decision was made in, and its failure is logged rather than returned: an
// audit convenience must never become an availability dependency.
func (e *enrolled) touch(ctx context.Context, key core.SSHKey) {
	scope := core.TenantScope{TenantID: key.TenantID}
	err := e.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.TouchSSHKey(ctx, key.ID, e.clk.Now())
	})
	if err != nil && e.log != nil {
		e.log.Warn("recording the last use of an ssh key failed",
			"key", key.ID, "error", err.Error())
	}
}

// usernameFrom reads the username the SSH client offered. It is read from the
// context value the ssh package sets, so it survives any wrapping between the
// connection and this lookup.
func usernameFrom(ctx context.Context) string {
	name, _ := ctx.Value(ssh.ContextKeyUser).(string)
	return strings.TrimSpace(name)
}
