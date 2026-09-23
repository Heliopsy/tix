// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var sshKeyColumns = []string{
	"id", "tenant_id", "actor_id", "fingerprint", "public_key", "label",
	"created_at", "last_used_at", "revoked_at",
}

func scanSSHKey(s scanner) (core.SSHKey, error) {
	var (
		k        core.SSHKey
		created  sql.NullTime
		lastUsed sql.NullTime
		revoked  sql.NullTime
	)
	if err := s.Scan(&k.ID, &k.TenantID, &k.ActorID, &k.Fingerprint, &k.PublicKey,
		&k.Label, &created, &lastUsed, &revoked); err != nil {
		return core.SSHKey{}, mapErr(err, "scanning ssh key")
	}
	k.CreatedAt = scanTime(created)
	k.LastUsedAt = scanNullTime(lastUsed)
	k.RevokedAt = scanNullTime(revoked)
	return k, nil
}

// CreateSSHKey enrols a public key against an actor in this tenant.
func (t *tx) CreateSSHKey(ctx context.Context, k *core.SSHKey) error {
	if k.ID == "" {
		k.ID = id.New()
	}
	k.TenantID = t.scope.TenantID
	if k.CreatedAt.IsZero() {
		k.CreatedAt = t.store.clock.Now()
	}
	ins := t.insert("ssh_keys").
		Set("id", k.ID).
		Set("actor_id", k.ActorID).
		Set("fingerprint", k.Fingerprint).
		Set("public_key", k.PublicKey).
		Set("label", k.Label).
		Set("created_at", timeArg(k.CreatedAt)).
		Set("last_used_at", nullTimeArg(k.LastUsedAt)).
		Set("revoked_at", nullTimeArg(k.RevokedAt))
	_, err := t.execInsert(ctx, ins, "enrolling ssh key %q", k.Fingerprint)
	return err
}

// GetSSHKey returns an enrolled key by identifier.
func (t *tx) GetSSHKey(ctx context.Context, keyID string) (*core.SSHKey, error) {
	b := t.builder("ssh_keys").Select(sshKeyColumns...).Where("id = ?", keyID).Limit(1)
	q, args := b.SelectQuery()
	k, err := scanSSHKey(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("ssh key %q", keyID)
		}
		return nil, err
	}
	return &k, nil
}

// ListSSHKeys returns an actor's keys, newest first. Revoked keys are included:
// a key that stopped working is what an operator is most often looking for.
func (t *tx) ListSSHKeys(ctx context.Context, actorID string) ([]core.SSHKey, error) {
	b := t.builder("ssh_keys").
		Select(sshKeyColumns...).
		Where("actor_id = ?", actorID).
		OrderBy("created_at", core.Descending).
		OrderBy("id", core.Descending)
	rows, err := t.query(ctx, b, "listing ssh keys")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.SSHKey{}
	for rows.Next() {
		v, err := scanSSHKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing ssh keys")
}

// RevokeSSHKey stops a key authenticating from the given instant.
func (t *tx) RevokeSSHKey(ctx context.Context, keyID string, at time.Time) error {
	b := t.builder("ssh_keys").
		Where("id = ?", keyID).
		Set("revoked_at", timeArg(at))
	n, err := t.execUpdate(ctx, b, "revoking ssh key %q", keyID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("ssh key %q", keyID)
	}
	return nil
}

// TouchSSHKey records the last time a key authenticated.
func (t *tx) TouchSSHKey(ctx context.Context, keyID string, at time.Time) error {
	b := t.builder("ssh_keys").
		Where("id = ?", keyID).
		Set("last_used_at", timeArg(at))
	n, err := t.execUpdate(ctx, b, "touching ssh key %q", keyID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("ssh key %q", keyID)
	}
	return nil
}

// FindSSHKeysByFingerprint returns every live enrolment of a fingerprint,
// across tenants, for the authentication path that runs before a tenant is known.
//
// The two engines reach the same rows by different means, because only this one
// has row-level security to satisfy. SQLite runs the statement as written;
// here ssh_keys carries FORCE ROW LEVEL SECURITY and a policy admitting only the
// transaction's tenant, so an unscoped SELECT would return nothing at all rather
// than failing loudly. Raising the ssh auth flag adds the SELECT-only policy that
// admits this one read, and it is lowered again before the caller regains
// control. It stays one statement whatever the tenant count: this runs for any
// connection presenting any key, so its cost must not grow with the deployment.
func (t *tx) FindSSHKeysByFingerprint(ctx context.Context, fingerprint string) ([]core.SSHKey, error) {
	if err := t.sshAuthLookup(ctx, true); err != nil {
		return nil, err
	}
	defer func() { _ = t.sshAuthLookup(ctx, false) }()

	q, args := sqlb.SSHKeysByFingerprintQuery(dialect, sshKeyColumns, fingerprint)
	rows, err := t.ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err, "finding ssh keys by fingerprint %q", fingerprint)
	}
	defer func() { _ = rows.Close() }()

	out := []core.SSHKey{}
	for rows.Next() {
		v, err := scanSSHKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "finding ssh keys by fingerprint")
}

// sshAuthLookup raises or lowers the flag sshAuthPolicy reads. It is
// transaction-local, so it cannot outlive the lookup that raised it, and this is
// deliberately the only place in the tree that writes it.
func (t *tx) sshAuthLookup(ctx context.Context, on bool) error {
	value := ""
	if on {
		value = sshAuthOn
	}
	if _, err := t.ex.ExecContext(ctx,
		`SELECT set_config($1, $2, true)`, sshAuthSetting, value); err != nil {
		return mapErr(err, "admitting the ssh key lookup")
	}
	return nil
}
