package sshd

import (
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/heliopsy/tix/internal/clock"
	gossh "golang.org/x/crypto/ssh"
)

// keepaliveRequest is the request every SSH client answers, with success or
// with failure. Either answer proves the client is still there, which is the
// only thing being asked.
const keepaliveRequest = "keepalive@openssh.com"

// activity is when the interface last saw a person.
//
// It is deliberately not a byte counter on the connection. A keepalive is
// traffic, and so is its reply, so a timer fed by traffic is reset by the very
// mechanism meant to detect an absent client, and an abandoned session would
// then live forever. This is fed by key and mouse messages only: protocol
// traffic never produces one.
type activity struct{ at atomic.Int64 }

// touch records t as the last thing a person did.
func (a *activity) touch(t time.Time) { a.at.Store(t.UnixNano()) }

// idleFor reports how long it has been since the last key.
func (a *activity) idleFor(now time.Time) time.Duration {
	return now.Sub(time.Unix(0, a.at.Load()))
}

// watched is the interface's model with an activity tap on its input. It
// wraps rather than changes the model, so the interface stays unaware that it
// is being served over a network.
type watched struct {
	tea.Model
	act *activity
	clk clock.Clock
}

// Update records a person's input and delegates everything else untouched.
func (w watched) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		w.act.touch(w.clk.Now())
	}
	model, cmd := w.Model.Update(msg)
	w.Model = model
	return w, cmd
}

// watch drops a session whose client has gone, and quits one whose person has.
//
// The two are independent on purpose. A keepalive refreshes every deadline the
// transport keeps, so it cannot also be what the idle decision reads: the idle
// decision reads the activity tap, which only a keystroke moves.
func (s *Server) watch(sess ssh.Session, program *tea.Program, act *activity) {
	ctx := sess.Context()
	conn, ok := ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn)
	if !ok {
		return
	}
	ping := s.opts.Clock.NewTicker(s.opts.KeepaliveInterval)
	defer ping.Stop()
	idle := s.opts.Clock.NewTicker(min(s.opts.KeepaliveInterval, s.opts.IdleTimeout))
	defer idle.Stop()

	var missed atomic.Int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-idle.C():
			if act.idleFor(s.opts.Clock.Now()) < s.opts.IdleTimeout {
				continue
			}
			s.log.Info("closing an idle ssh session",
				"source", sourceOf(sess.RemoteAddr()), "idle", s.opts.IdleTimeout)
			program.Quit()
			return
		case <-ping.C():
			if missed.Load() >= int64(s.opts.KeepaliveMaxMissed) {
				s.log.Info("dropping an ssh session whose client stopped answering",
					"source", sourceOf(sess.RemoteAddr()),
					"missed", s.opts.KeepaliveMaxMissed)
				// Closing the connection rather than the session is what
				// releases the slot and any lease promptly: a write to a
				// vanished client blocks until TCP gives up, which is far
				// longer than the sandbox lease this demo is about.
				_ = conn.Close()
				return
			}
			missed.Add(1)
			go func() {
				// A request to a client that is gone never returns, which is
				// why the count is kept here rather than read from an error.
				if _, _, err := conn.SendRequest(keepaliveRequest, true, nil); err == nil {
					missed.Add(-1)
				}
			}()
		}
	}
}
