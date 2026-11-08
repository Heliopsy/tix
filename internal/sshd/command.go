// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/ssh"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
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

// DefaultWriteStallTimeout is how long a command's output may wait on a
// client that is not reading before the command is stopped. Exports are
// spooled before they are written, so a stalled client holds no store
// connection; what it still holds is a session slot.
const DefaultWriteStallTimeout = 2 * time.Minute

// DefaultRecheckInterval is how often a running command's key is resolved
// again, so a revocation reaches a long command quickly.
const DefaultRecheckInterval = 30 * time.Second

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
//
// Standard input and output are the session's bytes, untouched, whether or
// not a pseudo-terminal was asked for: an export redirected to a file under
// `ssh -t` is still the file. Only standard error, which is for a person, gets
// the carriage returns the ssh package's emulated terminal adds.
func (s *Server) runCommand(sess ssh.Session, ch *channel, args []string) {
	actor := ch.actor
	if s.opts.Commands == nil || s.opts.Demo {
		fatalf(sess, "this listener runs no commands; it serves %s", commandUsage())
		return
	}
	if err := checkWords(args); err != nil {
		refuse(sess, core.ExitUsage, err.Error())
		return
	}
	ctx, stop := context.WithCancelCause(sessionCtx(sess, actor))
	cancel := func() { stop(context.Canceled) }
	defer cancel()

	_, _, hasPty := sess.Pty()
	stderr := io.Writer(sess.Stderr())
	stdout := &progressWriter{w: rawChannel(sess), clk: s.opts.Clock}
	s.register(sess, ch, func(reason string) error {
		if reason != "" {
			_, _ = io.WriteString(stderr, "\n"+reason+"\n")
		}
		cancel()
		return sess.Close()
	})
	go s.watch(sess, cancel, nil, ch.done)
	go s.police(sess, ch, stderr, stdout, stop)

	code := s.runGuarded(ctx, sess, stderr, Invocation{
		Args:     args,
		Stdin:    &cappedInput{r: sess, limit: s.opts.MaxInputBytes, terminal: hasPty},
		Stdout:   stdout,
		Stderr:   stderr,
		Service:  s.opts.Service,
		Actor:    actor,
		Via:      core.ViaFrom(ctx),
		Tenant:   s.tenantKey(ctx, actor),
		Terminal: hasPty,
	})
	if cause := context.Cause(ctx); code == core.ExitOK && (errors.Is(cause, errTimedOut) || errors.Is(cause, errStalled)) {
		code = core.ExitError
	}
	if errors.Is(context.Cause(ctx), errStanding) {
		code = core.ExitPermission
	}
	_ = sess.Exit(code)
}

// runGuarded runs the command, turning a panic in it into a failed command
// rather than the end of every session the process serves.
func (s *Server) runGuarded(ctx context.Context, sess ssh.Session, stderr io.Writer, inv Invocation) (code int) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("a command run over ssh panicked",
				"fingerprint", auth.Fingerprint(sess.PublicKey()), "command", firstWord(inv.Args), "panic", r)
			_, _ = io.WriteString(stderr, "tix: the command failed inside the server; try again shortly\n")
			code = core.ExitError
		}
	}()
	return s.opts.Commands(ctx, inv)
}

// firstWord names a command for a log line without its arguments, which may
// carry a task's text.
func firstWord(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// farewellWait bounds the attempt to tell a client why its command stopped. A
// client that stopped reading has a full window, and stderr shares it, so the
// line may never leave; the session is closed either way.
const farewellWait = time.Second

// The causes a command is stopped for, each with its own line to the client.
var (
	errTimedOut = errors.New("the command ran past its timeout")
	errStalled  = errors.New("the client stopped reading the command's output")
	errStanding = errors.New("the key's standing changed while the command ran")
)

// police stops a command that ran past its timeout, whose client stopped
// reading its output, or whose key lost its standing, until the channel ends.
//
// Cancelling the context stops every service call, but not a read of standard
// input the client is holding open nor a write it is not reading, so the
// session is closed as well, which is what unblocks either and gives the slot
// back.
func (s *Server) police(sess ssh.Session, ch *channel, stderr io.Writer, out *progressWriter,
	stop context.CancelCauseFunc) {
	deadline := s.opts.Clock.After(s.opts.CommandTimeout)
	tick := s.opts.Clock.NewTicker(min(s.opts.RecheckInterval, s.opts.WriteStallTimeout))
	defer tick.Stop()
	end := func(cause error, line string) {
		stop(cause)
		code := core.ExitError
		if errors.Is(cause, errStanding) {
			code = core.ExitPermission
		}
		said := make(chan struct{})
		go func() {
			defer close(said)
			_, _ = io.WriteString(stderr, "tix: "+line+"\n")
			_ = sess.Exit(code)
		}()
		select {
		case <-said:
		case <-s.opts.Clock.After(farewellWait):
		}
		_ = sess.Close()
	}
	lastCheck := s.opts.Clock.Now()
	for {
		select {
		case <-ch.done:
			return
		case <-sess.Context().Done():
			return
		case <-deadline:
			end(errTimedOut, fmt.Sprintf("the command ran past %s and was stopped", short(s.opts.CommandTimeout)))
			return
		case now := <-tick.C():
			if out.stalledFor(now) >= s.opts.WriteStallTimeout {
				end(errStalled, fmt.Sprintf("nothing read the output for %s; the command was stopped",
					short(s.opts.WriteStallTimeout)))
				return
			}
			if now.Sub(lastCheck) < s.opts.RecheckInterval {
				continue
			}
			lastCheck = now
			if err := s.stillStanding(sess, ch.actor); err != nil {
				s.log.Info("stopping an ssh command whose key lost its standing",
					"fingerprint", auth.Fingerprint(sess.PublicKey()), "error", err.Error())
				end(errStanding, "this key's access changed while the command ran; it was stopped")
				return
			}
		}
	}
}

// stillStanding resolves the session's key again and refuses when it no
// longer resolves to the same actor, in the same tenant, with the same role.
// A revoked key, a disabled or removed person and a demotion all land within
// one recheck interval rather than at the end of a long command.
func (s *Server) stillStanding(sess ssh.Session, was *core.Actor) error {
	if s.standing == nil {
		return nil
	}
	now, err := s.standing(sess.Context(), auth.Fingerprint(sess.PublicKey()))
	if err != nil {
		return err
	}
	if now.ID != was.ID || now.TenantID != was.TenantID || now.Role != was.Role {
		return core.Forbidden("the key now resolves to a different standing")
	}
	return nil
}

// progressWriter is a command's standard output. It notes when a write has
// been waiting on the client, so a client that stopped reading can be told
// apart from a command that is simply quiet.
type progressWriter struct {
	w   io.Writer
	clk clock.Clock

	mu      sync.Mutex
	pending time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.pending = p.clk.Now()
	p.mu.Unlock()
	n, err := p.w.Write(b)
	p.mu.Lock()
	p.pending = time.Time{}
	p.mu.Unlock()
	return n, err
}

// stalledFor reports how long the write in progress has waited, or zero.
func (p *progressWriter) stalledFor(now time.Time) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending.IsZero() {
		return 0
	}
	return now.Sub(p.pending)
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
	r        io.Reader
	limit    int64
	read     int64
	terminal bool
}

// IsTerminal reports whether a person is at a pseudo-terminal on the other
// end, which is what lets a prompt ask them.
func (c *cappedInput) IsTerminal() bool { return c.terminal }

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

// rawChannel is the session's channel without the emulated terminal's
// rewriting of every line feed, which the ssh package applies to a session's
// own Write once any channel on the connection asked for a pseudo-terminal.
// The channel is reached by its exported field name; TestAPseudoTerminalLeavesDataByteExactAndPromptsWorking
// fails if the package stops embedding it, rather than an export silently
// gaining carriage returns.
func rawChannel(sess ssh.Session) io.Writer {
	v := reflect.ValueOf(sess)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		if f := v.FieldByName("Channel"); f.IsValid() && f.CanInterface() {
			if ch, ok := f.Interface().(io.Writer); ok && ch != nil {
				return ch
			}
		}
	}
	return sess
}
