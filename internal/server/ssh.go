package server

import "context"

// SSHListener is the SSH surface the serve command runs beside the HTTP
// server. It is an interface rather than the concrete listener because
// internal/sshd imports this package for its bind guard, so the dependency
// cannot run the other way.
type SSHListener interface {
	// Listen binds the address without accepting on it.
	Listen() error
	// Serve accepts until ctx is cancelled.
	Serve(ctx context.Context) error
	// Close stops the listener, dropping live sessions.
	Close() error
	// Addr reports the bound address, empty before Listen.
	Addr() string
}

// SSHDrainer is an SSHListener that can stop accepting and wait for the
// sessions it is already holding. Shutdown prefers it, because an SSH session
// is long-lived and cutting one mid-edit is the shape a shutdown timeout
// exists to avoid. A listener without it is closed at once instead.
type SSHDrainer interface {
	// Drain stops accepting and waits for live sessions, reporting the
	// context's error if they outlive it.
	Drain(ctx context.Context) error
}

// sshWorker runs the SSH listener as one more member of the worker set, next
// to the sweeper, dispatcher and pruner.
//
// It ignores the context it is handed. Every other worker stops when the
// signal arrives; this one has to outlive it by the shutdown timeout, because
// that is the window its sessions drain in.
func (s *Server) sshWorker() Worker {
	return FuncWorker{WorkerName: "ssh-listener", Fn: func(context.Context) error {
		return s.ssh.Serve(s.sshCtx)
	}}
}

// SSHAddr reports the address the SSH listener bound, empty when none is
// configured or it has not bound yet.
func (s *Server) SSHAddr() string {
	if s.ssh == nil {
		return ""
	}
	return s.ssh.Addr()
}

// stopSSH ends the listener's own lifetime, which is what lets the worker set
// finish waiting.
func (s *Server) stopSSH() {
	if s.sshStop != nil {
		s.sshStop()
	}
}

// drainSSH gives live sessions the shutdown timeout and closes whatever is
// left when it expires. A listener that cannot drain is closed at once and
// says so, because waiting would only postpone the same cut.
func (s *Server) drainSSH(ctx context.Context) {
	if s.ssh == nil {
		return
	}
	defer s.stopSSH()

	drainer, ok := s.ssh.(SSHDrainer)
	if !ok {
		s.cfg.Logger.Warn("ssh listener cannot drain; closing live sessions",
			"addr", s.ssh.Addr())
		_ = s.ssh.Close()
		return
	}
	if err := drainer.Drain(ctx); err != nil {
		s.cfg.Logger.Error("ssh sessions were still open when shutdown expired",
			"timeout", s.cfg.ShutdownTimeout.String(), "error", err.Error())
		_ = s.ssh.Close()
	}
}
