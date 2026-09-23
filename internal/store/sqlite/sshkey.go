// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

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
		created  sql.NullString
		lastUsed sql.NullString
		revoked  sql.NullString
	)
	if err := s.Scan(&k.ID, &k.TenantID, &k.ActorID, &k.Fingerprint, &k.PublicKey,
		&k.Label, &created, &lastUsed, &revoked); err != nil {
		return core.SSHKey{}, mapErr(err, "scanning ssh key")
	}
	var err error
	if k.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.SSHKey{}, err
	}
	if k.LastUsedAt, err = sqlb.ScanNullTime(lastUsed); err != nil {
		return core.SSHKey{}, err
	}
	if k.RevokedAt, err = sqlb.ScanNullTime(revoked); err != nil {
		return core.SSHKey{}, err
	}
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
		Set("created_at", sqlb.TimeText(k.CreatedAt)).
		Set("last_used_at", sqlb.NullTimeText(k.LastUsedAt)).
		Set("revoked_at", sqlb.NullTimeText(k.RevokedAt))
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
		Set("revoked_at", sqlb.TimeText(at))
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
		Set("last_used_at", sqlb.TimeText(at))
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
func (t *tx) FindSSHKeysByFingerprint(ctx context.Context, fingerprint string) ([]core.SSHKey, error) {
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
