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

var serverColumns = []string{"id", "address", "version", "surfaces", "started_at", "last_seen_at"}

func scanServer(s scanner) (core.Server, error) {
	var (
		srv      core.Server
		surfaces string
		started  sql.NullString
		lastSeen sql.NullString
	)
	if err := s.Scan(&srv.ID, &srv.Address, &srv.Version, &surfaces, &started, &lastSeen); err != nil {
		return core.Server{}, mapErr(err, "scanning server")
	}
	var err error
	if srv.StartedAt, err = sqlb.ScanTime(started); err != nil {
		return core.Server{}, err
	}
	if srv.LastSeenAt, err = sqlb.ScanTime(lastSeen); err != nil {
		return core.Server{}, err
	}
	srv.Surfaces = sqlb.ParseServerSurfaces(surfaces)
	return srv, nil
}

// RegisterServer records a running process. An earlier row under the same
// identifier is replaced rather than merged, because a process that registers
// again has restarted and its start instant is the new one.
func (t *tx) RegisterServer(ctx context.Context, s *core.Server) error {
	if s.ID == "" {
		s.ID = id.New()
	}
	if s.StartedAt.IsZero() {
		s.StartedAt = t.store.clock.Now()
	}
	if s.LastSeenAt.IsZero() {
		s.LastSeenAt = s.StartedAt
	}
	if _, err := t.execDelete(ctx, t.builder("servers").Where("id = ?", s.ID),
		"replacing server %q", s.ID); err != nil {
		return err
	}
	ins := t.insert("servers").
		Set("id", s.ID).
		Set("address", s.Address).
		Set("version", s.Version).
		Set("surfaces", sqlb.JoinServerSurfaces(s.Surfaces)).
		Set("started_at", sqlb.TimeText(s.StartedAt)).
		Set("last_seen_at", sqlb.TimeText(s.LastSeenAt))
	_, err := t.execInsert(ctx, ins, "registering server %q", s.ID)
	return err
}

// HeartbeatServer refreshes a server's last seen instant.
func (t *tx) HeartbeatServer(ctx context.Context, serverID string, at time.Time) error {
	b := t.builder("servers").Where("id = ?", serverID).Set("last_seen_at", sqlb.TimeText(at))
	_, err := t.execUpdate(ctx, b, "heartbeating server %q", serverID)
	return err
}

// DeregisterServer removes a server's row.
func (t *tx) DeregisterServer(ctx context.Context, serverID string) error {
	_, err := t.execDelete(ctx, t.builder("servers").Where("id = ?", serverID),
		"deregistering server %q", serverID)
	return err
}

// ListServers returns registered servers, oldest start first.
func (t *tx) ListServers(ctx context.Context, page core.Page) ([]core.Server, error) {
	spec, err := resolvePage(page, "started_at", map[string]string{
		"started_at":   "started_at",
		"last_seen_at": "last_seen_at",
	})
	if err != nil {
		return nil, err
	}
	b := spec.apply(t.builder("servers").Select(serverColumns...), "id")
	rows, err := t.query(ctx, b, "listing servers")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Server{}
	for rows.Next() {
		v, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing servers")
}

// ForgetServersBefore deletes rows unseen since the cutoff.
func (t *tx) ForgetServersBefore(ctx context.Context, cutoff time.Time) (int, error) {
	n, err := t.execDelete(ctx, t.builder("servers").Where("last_seen_at < ?", sqlb.TimeText(cutoff)),
		"forgetting servers unseen since %s", cutoff)
	return int(n), err
}
