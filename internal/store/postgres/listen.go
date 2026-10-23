// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/heliopsy/tix/internal/core"
)

// relistenDelay is how long the fan-out waits before taking a new listener
// connection after the previous one failed or dropped. Readers are polling
// their ticker meanwhile, so this delay costs latency and no event.
const relistenDelay = time.Second

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

// WakeOnCommit returns a channel signalled when a transaction appending events
// to tenantID commits, anywhere, including from another process. It is fed by
// one LISTEN connection shared by every reader in this process, started on the
// first call and kept alive until the store is closed.
func (s *Store) WakeOnCommit(ctx context.Context, tenantID string) (<-chan struct{}, error) {
	s.listenMu.Lock()
	defer s.listenMu.Unlock()
	if !s.listening {
		listenCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		// The first LISTEN is issued here, synchronously, and not in the
		// goroutine below.
		//
		// PostgreSQL delivers a notification only to the sessions that are
		// already listening at the moment it fires; there is no queue for a
		// session that starts listening afterwards. Starting the fan-out
		// goroutine and returning immediately therefore left a window where
		// the caller could subscribe, commit, and have its own NOTIFY land
		// before LISTEN had been issued. Nothing resends it, so that commit
		// woke nobody, and the subscriber sat until its ticker came round.
		//
		// Returning only once the connection is listening closes that window
		// for every caller, which is what makes "subscribe, then write" safe
		// to do back to back.
		notifications, err := s.Listen(listenCtx)
		if err != nil {
			cancel()
			return nil, err
		}
		s.listening, s.stopListen = true, cancel
		go s.fanOut(listenCtx, notifications)
	}
	return s.wakers.Add(ctx, tenantID), nil
}

// fanOut relays notifications to the readers until the store closes, retaking
// the listener connection whenever it fails. A gap here is a gap in wake-ups
// and not in the log, so it is ridden out rather than reported upwards.
// fanOut turns notifications into wakes, taking a new listener connection each
// time the current one drops. It is handed the first connection already
// listening, because WakeOnCommit establishes that one before it returns; see
// the note there for why that ordering is load-bearing.
func (s *Store) fanOut(ctx context.Context, notifications <-chan Notification) {
	for ctx.Err() == nil {
		for n := range notifications {
			s.wakers.Wake(n.TenantID)
		}
		select {
		case <-time.After(relistenDelay):
		case <-ctx.Done():
			return
		}
		next, err := s.Listen(ctx)
		if err != nil {
			notifications = noNotifications
			continue
		}
		notifications = next
	}
}

// noNotifications stands in for a listener connection that could not be taken,
// so the loop above falls through to its delay and tries again. Ranging over a
// nil channel would block for ever instead.
var noNotifications = func() <-chan Notification {
	ch := make(chan Notification)
	close(ch)
	return ch
}()
