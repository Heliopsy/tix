package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/heliopsy/tix/internal/core"
)

// eventsChannel is the NOTIFY channel a committing transaction announces on.
const eventsChannel = "tix_events"

// Notification names the tenant whose outbox grew.
type Notification struct {
	TenantID string
}

// Listen delivers a notification each time a transaction that appended events
// commits, so a subscriber wakes on the commit rather than polling. The returned
// channel closes when ctx ends or the connection drops.
func (s *Store) Listen(ctx context.Context) (<-chan Notification, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, mapErr(err, "acquiring a listener connection")
	}
	if _, err := conn.ExecContext(ctx, "LISTEN "+eventsChannel); err != nil {
		_ = conn.Close()
		return nil, mapErr(err, "listening on %q", eventsChannel)
	}

	out := make(chan Notification)
	go func() {
		defer close(out)
		defer func() { _ = conn.Close() }()
		for {
			tenant, err := waitForNotification(ctx, conn)
			if err != nil {
				return
			}
			select {
			case out <- Notification{TenantID: tenant}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// waitForNotification blocks on the pgx connection underneath the pooled one,
// which is the only place this package reaches past database/sql.
func waitForNotification(ctx context.Context, conn rawConn) (string, error) {
	var payload string
	err := conn.Raw(func(driverConn any) error {
		c, ok := driverConn.(*stdlib.Conn)
		if !ok {
			return core.Internal("listener connection is not a pgx connection")
		}
		n, err := c.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		payload = n.Payload
		return nil
	})
	if err != nil {
		return "", err
	}
	return payload, nil
}

// rawConn is the part of *sql.Conn the listener needs.
type rawConn interface {
	Raw(f func(driverConn any) error) error
}
