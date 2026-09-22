package sshd

import (
	"context"
	"errors"

	"github.com/charmbracelet/ssh"

	"github.com/heliopsy/tix/internal/core"
)

// Drain stops accepting new connections and waits for the sessions already
// open, up to the deadline carried by ctx.
//
// Close cuts every live session at once, which is the wrong ending for a
// terminal interface: somebody typing loses their screen with no idea whether
// the server went away or their network did. Drain lets them finish instead,
// and only closes what is still open once the deadline has passed.
//
// It is the method internal/server prefers when shutting a listener down
// alongside the HTTP server, so both share one shutdown budget.
//
// A session still open when the deadline passes is told why before it goes.
// A terminal that simply stops is indistinguishable from a dropped network,
// and this one was deliberate.
func (s *Server) Drain(ctx context.Context) error {
	if s == nil || s.ssh == nil {
		return nil
	}
	err := s.ssh.Shutdown(ctx)
	if err == nil || errors.Is(err, ssh.ErrServerClosed) {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		s.endLingering()
		if closeErr := s.ssh.Close(); closeErr != nil && !errors.Is(closeErr, ssh.ErrServerClosed) {
			return closeErr
		}
		return nil
	}
	return err
}

// endLingering tells every session this listener still holds that the server
// is going, and closes it.
//
// It reaches them through the connection registry, which is the same place an
// administrator ending one connection reaches it, so there is one way to close
// a session with a reason rather than two that could drift apart.
func (s *Server) endLingering() {
	reg := s.opts.Connections
	if reg == nil {
		return
	}
	for _, c := range reg.BySurface(core.ConnectionSSH) {
		if err := reg.End(c.TenantID, c.ID, shutdownNotice); err != nil {
			s.log.Debug("ending a lingering ssh session", "connection", c.ID, "error", err.Error())
		}
	}
}

// shutdownNotice is what a session is told when it outlives the drain.
const shutdownNotice = "the tix server is shutting down; this session is ending"
