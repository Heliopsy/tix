// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/ssh"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/tui"
)

// Limits on a hosted command's words. A command line is a handful of short
// words; these bound what a client can make the server parse and hold.
const (
	MaxCommandWords     = 128
	MaxCommandWordBytes = 4 << 10
	MaxCommandBytes     = 32 << 10
)

// DefaultCommandTimeout is how long one hosted command may run. It is longer
// than any command a person waits on and shorter than the idle timeout, so a
// stalled import cannot hold its slot for the life of the listener.
const DefaultCommandTimeout = 15 * time.Minute

// Invocation is one tix command a session asked the listener to run.
type Invocation struct {
	// Args are the words after the program name, as the client sent them.
	Args []string
	// Stdin, Stdout and Stderr are the session's own streams.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Service is the listener's service, Actor the session's identity, Via
	// its ssh channel and Tenant the key of the tenant the actor is in.
	Service core.Service
	Actor   *core.Actor
	Via     core.Via
	Tenant  string
	// Terminal reports whether the client allocated a pseudo-terminal.
	Terminal bool
}

// CommandRunner runs one hosted command in-process and returns its exit code.
// The command tree lives above this package, which may not import it, so the
// process wiring the listener up hands the runner in.
type CommandRunner func(ctx context.Context, inv Invocation) int

// commandWords are the first words a session's command is answered as a
// command for: every hosted command's, and every word the command line knows.
func (s *Server) commandWords() map[string]bool {
	out := map[string]bool{}
	for _, path := range capability.SSHCommands() {
		word, _, _ := strings.Cut(path, " ")
		out[word] = true
	}
	for _, word := range s.opts.CommandWords {
		out[word] = true
	}
	return out
}

// isCommand reports whether a session's command asks for a command rather
// than an interface. Interface names win, so `watch` stays the event stream;
// command words are compared exactly, as the command line compares them, so
// `ssh host TASK` still opens the project keyed TASK.
func (s *Server) isCommand(command []string) bool {
	if len(command) == 0 {
		return false
	}
	if _, ok := tui.LookupInterface(command[0]); ok {
		return false
	}
	return s.commandWords()[command[0]]
}

// checkWords refuses a command line no person types: too many words, a word
// too long, or a byte that is not text.
func checkWords(args []string) error {
	if len(args) > MaxCommandWords {
		return core.Invalid("a command may carry at most %d words", MaxCommandWords)
	}
	total := 0
	for _, w := range args {
		total += len(w)
		if len(w) > MaxCommandWordBytes {
			return core.Invalid("a command word may be at most %d bytes", MaxCommandWordBytes)
		}
		if !utf8.ValidString(w) {
			return core.Invalid("a command word is not valid UTF-8")
		}
		for _, r := range w {
			if (r < 0x20 && r != '\t' && r != '\n') || r == 0x7f || (r >= 0x80 && r < 0xa0) {
				return core.Invalid("a command word carries a control character")
			}
		}
	}
	if total > MaxCommandBytes {
		return core.Invalid("a command may be at most %d bytes", MaxCommandBytes)
	}
	return nil
}

// runCommand runs one hosted command as the session's actor and ends the
// session with its exit code.
func (s *Server) runCommand(sess ssh.Session, actor *core.Actor, args []string) {
	if s.opts.Commands == nil || s.opts.Demo {
		fatalf(sess, "this listener runs no commands; it serves %s", commandUsage())
		return
	}
	if err := checkWords(args); err != nil {
		refuse(sess, core.ExitUsage, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(sessionCtx(sess, actor), s.opts.CommandTimeout)
	defer cancel()

	_, _, hasPty := sess.Pty()
	var stdout, stderr io.Writer = sess, sess.Stderr()
	var stdin io.Reader = sess
	if hasPty {
		stdout, stderr = crlf{sess}, crlf{sess.Stderr()}
		stdin = &terminalInput{r: sess}
	}
	sess.Context().SetValue(handleKey{}, s.opts.Connections.Register(connections.Entry{
		Surface:     core.ConnectionSSH,
		TenantID:    actor.TenantID,
		ActorID:     actor.ID,
		ActorHandle: actor.Handle,
		Remote:      sourceOf(sess.RemoteAddr()),
		Fingerprint: auth.Fingerprint(sess.PublicKey()),
	}, func(reason string) error {
		if reason != "" {
			_, _ = io.WriteString(stderr, "\n"+reason+"\n")
		}
		cancel()
		return sess.Close()
	}))
	go s.watch(sess, cancel, nil)

	code := s.opts.Commands(ctx, Invocation{
		Args:     args,
		Stdin:    &cappedInput{r: stdin, limit: s.opts.MaxInputBytes},
		Stdout:   stdout,
		Stderr:   stderr,
		Service:  s.opts.Service,
		Actor:    actor,
		Via:      core.ViaFrom(ctx),
		Tenant:   s.tenantKey(ctx, actor),
		Terminal: hasPty,
	})
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		_, _ = fmt.Fprintf(stderr, "tix: the command ran past %s and was stopped\n", short(s.opts.CommandTimeout))
		if code == core.ExitOK {
			code = core.ExitError
		}
	}
	_ = sess.Exit(code)
}

// tenantKey names the actor's tenant for the commands that describe where
// they ran. A failure leaves it empty rather than refusing the command.
func (s *Server) tenantKey(ctx context.Context, actor *core.Actor) string {
	t, err := s.opts.Service.GetTenant(ctx, actor.TenantID)
	if err != nil || t == nil {
		return ""
	}
	return t.Key
}

// refuse reports a refusal and ends the session with code.
func refuse(sess ssh.Session, code int, msg string) {
	_, _ = fmt.Fprintf(sess.Stderr(), "tix: %s\n", msg)
	_ = sess.Exit(code)
	_ = sess.Close()
}

// cappedInput refuses to hand a command more of standard input than an import
// over HTTP may carry. It fails rather than truncating: a file cut short at the
// limit would import as if it were whole.
type cappedInput struct {
	r     io.Reader
	limit int64
	read  int64
}

func (c *cappedInput) Read(p []byte) (int, error) {
	if c.read > c.limit {
		return 0, core.Invalid("%s", config.ImportLimitRefusal(c.limit))
	}
	if room := c.limit - c.read + 1; int64(len(p)) > room {
		p = p[:room]
	}
	n, err := c.r.Read(p)
	c.read += int64(n)
	if c.read > c.limit {
		return 0, core.Invalid("%s", config.ImportLimitRefusal(c.limit))
	}
	return n, err
}

// crlf turns each line feed into the carriage return and line feed a client
// terminal in raw mode needs, which a real pseudo-terminal would have done.
type crlf struct{ w io.Writer }

func (c crlf) Write(p []byte) (int, error) {
	if !strings.ContainsRune(string(p), '\n') {
		return c.w.Write(p)
	}
	out := strings.ReplaceAll(strings.ReplaceAll(string(p), "\r\n", "\n"), "\n", "\r\n")
	if _, err := io.WriteString(c.w, out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// terminalInput is standard input from a client terminal in raw mode: the
// carriage return the enter key sends reads as a line feed, and it tells a
// prompt that a person is there to answer it.
type terminalInput struct {
	mu sync.Mutex
	r  io.Reader
}

// IsTerminal reports that a person is on the other end.
func (t *terminalInput) IsTerminal() bool { return true }

func (t *terminalInput) Read(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, err := t.r.Read(p)
	for i := range p[:n] {
		if p[i] == '\r' {
			p[i] = '\n'
		}
	}
	return n, err
}
