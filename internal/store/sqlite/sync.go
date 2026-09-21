package sqlite

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var syncSourceColumns = []string{
	"id", "tenant_id", "system", "name", "cursor", "last_run_at", "last_status", "created_at",
}

func scanSyncSource(s scanner) (core.SyncSource, error) {
	var (
		src     core.SyncSource
		lastRun sql.NullString
		created sql.NullString
	)
	if err := s.Scan(&src.ID, &src.TenantID, &src.System, &src.Name, &src.Cursor,
		&lastRun, &src.LastStatus, &created); err != nil {
		return core.SyncSource{}, mapErr(err, "scanning sync source")
	}
	var err error
	if src.LastRunAt, err = sqlb.ScanNullTime(lastRun); err != nil {
		return core.SyncSource{}, err
	}
	if src.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.SyncSource{}, err
	}
	return src, nil
}

// PutSyncSource creates or replaces an import source and its cursor.
func (t *tx) PutSyncSource(ctx context.Context, s *core.SyncSource) error {
	s.TenantID = t.scope.TenantID
	upd := t.builder("sync_sources").
		Where("system = ?", s.System).
		Where("name = ?", s.Name).
		Set("cursor", s.Cursor).
		Set("last_run_at", sqlb.NullTimeText(s.LastRunAt)).
		Set("last_status", s.LastStatus)
	n, err := t.execUpdate(ctx, upd, "updating sync source %q", s.Name)
	if err != nil {
		return err
	}
	if n > 0 {
		if s.ID == "" {
			existing, err := t.findSyncSource(ctx, s.System, s.Name)
			if err != nil {
				return err
			}
			s.ID = existing.ID
			s.CreatedAt = existing.CreatedAt
		}
		return nil
	}

	if s.ID == "" {
		s.ID = id.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = t.store.clock.Now()
	}
	ins := t.insert("sync_sources").
		Set("id", s.ID).
		Set("system", s.System).
		Set("name", s.Name).
		Set("cursor", s.Cursor).
		Set("last_run_at", sqlb.NullTimeText(s.LastRunAt)).
		Set("last_status", s.LastStatus).
		Set("created_at", sqlb.TimeText(s.CreatedAt))
	_, err = t.execInsert(ctx, ins, "creating sync source %q", s.Name)
	return err
}

func (t *tx) findSyncSource(ctx context.Context, system, name string) (*core.SyncSource, error) {
	b := t.builder("sync_sources").
		Select(syncSourceColumns...).
		Where("system = ?", system).
		Where("name = ?", name).
		Limit(1)
	q, args := b.SelectQuery()
	s, err := scanSyncSource(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("sync source %q of system %q", name, system)
		}
		return nil, err
	}
	return &s, nil
}

// GetSyncSource returns one import source.
func (t *tx) GetSyncSource(ctx context.Context, sourceID string) (*core.SyncSource, error) {
	b := t.builder("sync_sources").Select(syncSourceColumns...).Where("id = ?", sourceID).Limit(1)
	q, args := b.SelectQuery()
	s, err := scanSyncSource(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("sync source %q", sourceID)
		}
		return nil, err
	}
	return &s, nil
}

// ListSyncSources returns this tenant's import sources.
func (t *tx) ListSyncSources(ctx context.Context) ([]core.SyncSource, error) {
	b := t.builder("sync_sources").
		Select(syncSourceColumns...).
		OrderBy("system", core.Ascending).
		OrderBy("name", core.Ascending)
	rows, err := t.query(ctx, b, "listing sync sources")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.SyncSource{}
	for rows.Next() {
		v, err := scanSyncSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing sync sources")
}

// DeleteSyncSource removes an import source.
func (t *tx) DeleteSyncSource(ctx context.Context, sourceID string) error {
	b := t.builder("sync_sources").Where("id = ?", sourceID)
	n, err := t.execDelete(ctx, b, "deleting sync source %q", sourceID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("sync source %q", sourceID)
	}
	return nil
}

var externalRefColumns = []string{
	"tenant_id", "entity_type", "entity_id", "system", "external_id",
	"external_url", "external_version", "last_synced_at",
}

func scanExternalRef(s scanner) (core.ExternalRef, error) {
	var (
		r      core.ExternalRef
		synced sql.NullString
	)
	if err := s.Scan(&r.TenantID, &r.EntityType, &r.EntityID, &r.System, &r.ExternalID,
		&r.ExternalURL, &r.ExternalVersion, &synced); err != nil {
		return core.ExternalRef{}, mapErr(err, "scanning external reference")
	}
	var err error
	if r.LastSyncedAt, err = sqlb.ScanTime(synced); err != nil {
		return core.ExternalRef{}, err
	}
	return r, nil
}

// PutExternalRef ties a tix entity to its counterpart in an external system.
func (t *tx) PutExternalRef(ctx context.Context, r *core.ExternalRef) error {
	r.TenantID = t.scope.TenantID
	if r.LastSyncedAt.IsZero() {
		r.LastSyncedAt = t.store.clock.Now()
	}
	upd := t.builder("external_refs").
		Where("system = ?", r.System).
		Where("entity_type = ?", r.EntityType).
		Where("external_id = ?", r.ExternalID).
		Set("entity_id", r.EntityID).
		Set("external_url", r.ExternalURL).
		Set("external_version", r.ExternalVersion).
		Set("last_synced_at", sqlb.TimeText(r.LastSyncedAt))
	n, err := t.execUpdate(ctx, upd, "updating external reference %q", r.ExternalID)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	ins := t.insert("external_refs").
		Set("entity_type", r.EntityType).
		Set("entity_id", r.EntityID).
		Set("system", r.System).
		Set("external_id", r.ExternalID).
		Set("external_url", r.ExternalURL).
		Set("external_version", r.ExternalVersion).
		Set("last_synced_at", sqlb.TimeText(r.LastSyncedAt))
	_, err = t.execInsert(ctx, ins, "creating external reference %q", r.ExternalID)
	return err
}

// GetExternalRef returns the reference for one external identifier.
func (t *tx) GetExternalRef(ctx context.Context, system, externalID, entityType string) (*core.ExternalRef, error) {
	b := t.builder("external_refs").
		Select(externalRefColumns...).
		Where("system = ?", system).
		Where("external_id = ?", externalID).
		Where("entity_type = ?", entityType).
		Limit(1)
	q, args := b.SelectQuery()
	r, err := scanExternalRef(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("external reference %q in system %q", externalID, system)
		}
		return nil, err
	}
	return &r, nil
}

// ListExternalRefs returns every reference belonging to one external system.
func (t *tx) ListExternalRefs(ctx context.Context, system string) ([]core.ExternalRef, error) {
	b := t.builder("external_refs").
		Select(externalRefColumns...).
		Where("system = ?", system).
		OrderBy("entity_type", core.Ascending).
		OrderBy("external_id", core.Ascending)
	rows, err := t.query(ctx, b, "listing external references")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.ExternalRef{}
	for rows.Next() {
		v, err := scanExternalRef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing external references")
}
