package sshd

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/tui"
	"github.com/muesli/termenv"
)

// noticeKey carries the farewell line from the program handler to the handler
// that runs once the interface has given the terminal back.
type noticeKey struct{}

// handle serves one connection: the wish middleware owns the terminal, the
// program handler owns the identity.
//
// The cap on live sessions is taken here, before anything has been looked up,
// so a full listener refuses every key the same way and a refusal says nothing
// about whether this one had been seen before.
func (s *Server) handle(sess ssh.Session) {
	if key := sess.PublicKey(); key != nil {
		fingerprint := auth.Fingerprint(key)
		if err := s.live.acquire(fingerprint); err != nil {
			s.log.Warn("refusing an ssh session over the concurrency cap",
				"source", sourceOf(sess.RemoteAddr()), "error", err.Error())
			fatalf(sess, "%s", message(err))
			return
		}
		defer s.live.release(fingerprint)
	}
	bm.MiddlewareWithProgramHandler(s.program, termenv.ANSI256)(s.farewell)(sess)
}

// program resolves the connecting key into its own sandbox and returns the
// interface bound to that session's streams.
//
// Every value it builds is per connection: the actor, the context carrying it,
// the capped service and the model. Nothing is shared between sessions but the
// service and the store beneath them, and both take their tenant from the
// context they are handed, so two sessions cannot reach each other's data.
func (s *Server) program(sess ssh.Session) *tea.Program {
	key := sess.PublicKey()
	if key == nil {
		fatalf(sess, "tix needs a public key to know which sandbox is yours; ssh with a key, not a password")
		return nil
	}
	actor, err := s.verifier.Verify(sess.Context(), key)
	if err != nil {
		s.log.Warn("refusing an ssh session",
			"fingerprint", auth.Fingerprint(key),
			"source", sourceOf(sess.RemoteAddr()),
			"error", err.Error())
		fatalf(sess, "%s", message(err))
		return nil
	}

	ctx := core.WithSource(core.WithActor(sess.Context(), actor), core.SourceTUI)
	environ := sess.Environ()
	color := colorFor(environ)
	act := &activity{}
	act.touch(s.opts.Clock.Now())
	model := tui.New(tui.Config{
		Service:   capped{Service: s.opts.Service, limit: s.opts.MaxTasks},
		Context:   ctx,
		Actor:     actor,
		Environ:   environ,
		Color:     &color,
		Project:   seedProjectKey,
		TimeStyle: s.opts.TimeStyle,
		Now:       s.opts.Clock.Now,
	})
	sess.Context().SetValue(noticeKey{}, fmt.Sprintf(
		"This was a demo sandbox owned by your ssh key. Reconnect with the same key to find it as you left it; "+
			"it is deleted after %s without a visit.\r\n", short(s.opts.TenantTTL)))

	opts := append([]tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithContext(sess.Context()),
	}, bm.MakeOptions(sess)...)
	program := tea.NewProgram(watched{Model: model, act: act, clk: s.opts.Clock}, opts...)
	go s.watch(sess, program, act)
	return program
}

// farewell prints the sandbox notice once the alternate screen is gone, which
// is the one moment a line is certain to be read rather than painted over.
func (s *Server) farewell(sess ssh.Session) {
	notice, ok := sess.Context().Value(noticeKey{}).(string)
	if !ok {
		return
	}
	_, _ = fmt.Fprint(sess, notice)
}

// fatalf reports a refusal to the client and closes the session.
func fatalf(sess ssh.Session, format string, args ...any) {
	_, _ = fmt.Fprintf(sess.Stderr(), "tix: "+format+"\r\n", args...)
	_ = sess.Exit(1)
	_ = sess.Close()
}

// message renders an error for a stranger, who is owed the reason a listener
// refused them but not the shape of what is behind it.
func message(err error) string {
	var e *core.Error
	if errors.As(err, &e) && e.Kind != core.KindInternal {
		return e.Message
	}
	return "this demo could not open a sandbox for you; try again shortly"
}

// colorFor decides whether a session is drawn in colour, from what the client
// itself said.
//
// The interface's own probe ends in a character-device test, which a network
// stream can never pass, so the answer over SSH has to be worked out rather
// than measured. The rest of that probe still applies and is applied here:
// either NO_COLOR spelling opts out, and a dumb terminal opts out. What
// replaces the device test is the terminal type: a session that named no
// terminal is not assumed to take ANSI, since nothing is left to catch it if
// the assumption is wrong. A pseudo-terminal request always carries a terminal
// type, and the session appends it last, so it wins over anything the client
// set with an env request.
func colorFor(environ []string) bool {
	for _, name := range []string{tui.EnvNoColor, tui.EnvTixNoColor} {
		if value := lookupEnv(environ, name); value != "" {
			return false
		}
	}
	switch term := lookupEnv(environ, "TERM"); term {
	case "", "dumb":
		return false
	default:
		return true
	}
}

// lookupEnv returns the last value of name in an environ slice, which is the
// one the interface would read.
func lookupEnv(environ []string, name string) string {
	prefix := name + "="
	for i := len(environ) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(environ[i], prefix); ok {
			return value
		}
	}
	return ""
}
