// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	gossh "golang.org/x/crypto/ssh"
)

// fakeEvents is the real service with its event source replaced by a channel
// the test feeds, so a line arrives when the test says and not when a commit
// happens to land.
type fakeEvents struct {
	core.Service
	feed    chan core.Event
	filters chan core.EventFilter
	actors  chan *core.Actor
	ended   chan struct{}
	once    sync.Once
}

func newFakeEvents(svc core.Service) *fakeEvents {
	return &fakeEvents{
		Service: svc,
		feed:    make(chan core.Event),
		filters: make(chan core.EventFilter, 4),
		actors:  make(chan *core.Actor, 4),
		ended:   make(chan struct{}),
	}
}

func (f *fakeEvents) Subscribe(ctx context.Context, filter core.EventFilter) (<-chan core.Event, error) {
	actor, _ := core.ActorFrom(ctx)
	f.actors <- actor
	f.filters <- filter
	out := make(chan core.Event)
	go func() {
		defer f.once.Do(func() { close(f.ended) })
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-f.feed:
				select {
				case out <- e:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

// send hands the stream one event, failing when nothing is reading.
func (f *fakeEvents) send(t *testing.T, e core.Event) {
	t.Helper()
	select {
	case f.feed <- e:
	case <-time.After(30 * time.Second):
		t.Fatal("no subscription took the event")
	}
}

// watchListener is a paced sandbox listener whose events come from a fake.
func watchListener(t *testing.T, with ...func(*Options)) (*Server, *fakeEvents, *clock.Fake) {
	t.Helper()
	var events *fakeEvents
	srv, clk := newPacedServer(t, append([]func(*Options){func(o *Options) {
		events = newFakeEvents(o.Service)
		o.Service = events
	}}, with...)...)
	return srv, events, clk
}

// notifyBuffer is a syncBuffer that says when something was written, so a
// test waits on output rather than on a timer.
type notifyBuffer struct {
	syncBuffer
	wrote chan struct{}
}

func newNotifyBuffer() *notifyBuffer { return &notifyBuffer{wrote: make(chan struct{}, 1)} }

func (b *notifyBuffer) Write(p []byte) (int, error) {
	n, err := b.syncBuffer.Write(p)
	select {
	case b.wrote <- struct{}{}:
	default:
	}
	return n, err
}

// waitFor blocks until the buffer holds want.
func (b *notifyBuffer) waitFor(t *testing.T, want string) string {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		if got := b.String(); strings.Contains(got, want) {
			return got
		}
		select {
		case <-b.wrote:
		case <-deadline:
			t.Fatalf("waited 30s for %q; got %q", want, b.String())
		}
	}
}

// watchSession is a client running one command.
type watchSession struct {
	client *gossh.Client
	stdout *notifyBuffer
	stderr *notifyBuffer
	stdin  io.WriteCloser
	ended  chan struct{}
	exit   error
}

// dialWatch runs command on the listener at addr with a fresh key, with or
// without a pseudo-terminal.
func dialWatch(t *testing.T, addr, command string, pty bool) *watchSession {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            "visitor",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         30 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	w := &watchSession{client: client, stdout: newNotifyBuffer(), stderr: newNotifyBuffer(), ended: make(chan struct{})}
	sess.Stdout, sess.Stderr = w.stdout, w.stderr
	if w.stdin, err = sess.StdinPipe(); err != nil {
		t.Fatalf("stdin: %v", err)
	}
	if pty {
		if err := sess.RequestPty("xterm-256color", 40, 120, gossh.TerminalModes{}); err != nil {
			t.Fatalf("pty: %v", err)
		}
	}
	if err := sess.Start(command); err != nil {
		t.Fatalf("start %q: %v", command, err)
	}
	go func() {
		w.exit = sess.Wait()
		close(w.ended)
	}()
	return w
}

// waitEnded blocks until the session is over.
func (w *watchSession) waitEnded(t *testing.T, why string) {
	t.Helper()
	select {
	case <-w.ended:
	case <-time.After(30 * time.Second):
		t.Fatalf("the watch session did not end on %s; it printed %q", why, w.stdout.String())
	}
}

// claimed is an event whose line is easy to find.
func claimed(seq int64) core.Event {
	return core.Event{
		Seq: seq, Type: core.EventTaskClaimed, ActorID: "agent-pax", SubjectType: "task",
		SubjectID: "tsk_1", Payload: map[string]any{"ref": "demo-1"},
		OccurredAt: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
	}
}

// TestWatchOverSSHStreamsLinesAndEndsWhenTheClientGoes runs the command over
// a real connection: the line arrives as the session's actor, plain because
// nothing asked for a terminal, and closing the client ends the subscription
// and gives the slot back.
func TestWatchOverSSHStreamsLinesAndEndsWhenTheClientGoes(t *testing.T) {
	srv, events, _ := watchListener(t)
	w := dialWatch(t, srv.Addr(), "watch", false)
	w.stderr.waitFor(t, "ctrl-c to stop")
	if actor := <-events.actors; actor == nil || actor.Handle != "visitor" {
		t.Fatalf("the stream subscribed as %+v, want the sandbox's visitor", actor)
	}

	events.send(t, claimed(7))
	got := w.stdout.waitFor(t, "000007")
	_, line, _ := strings.Cut(got, "000007")
	if !strings.Contains(line, "agent-pax") || !strings.Contains(line, "DEMO-1") {
		t.Errorf("the line %q is not the one tix watch draws for the event", line)
	}
	if strings.Contains(got, "\x1b[") || strings.Contains(got, "\r\n") || !strings.HasSuffix(got, "\n") {
		t.Errorf("a stream with no terminal wrote %q, want one plain newline-terminated line", got)
	}

	_ = w.client.Close()
	select {
	case <-events.ended:
	case <-time.After(30 * time.Second):
		t.Fatal("closing the client left the subscription running")
	}
	waitForLiveSessions(t, srv, 0)
}

// TestWatchUnderATerminalIsColouredAndCtrlCEndsIt is the other rendering: a
// client with a pseudo-terminal gets colour and raw-mode line ends, and its
// ctrl-c arrives as a byte the listener has to act on.
func TestWatchUnderATerminalIsColouredAndCtrlCEndsIt(t *testing.T) {
	srv, events, _ := watchListener(t)
	w := dialWatch(t, srv.Addr(), "watch", true)
	w.stderr.waitFor(t, "ctrl-c to stop")
	events.send(t, claimed(8))
	got := w.stdout.waitFor(t, "\r\n")
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("an xterm-256color terminal was sent %q, want colour", got)
	}
	if strings.Count(got, "\n") != strings.Count(got, "\r\n") {
		t.Errorf("a raw-mode terminal was sent a bare newline in %q", got)
	}

	if _, err := w.stdin.Write([]byte{0x03}); err != nil {
		t.Fatalf("pressing ctrl-c: %v", err)
	}
	w.waitEnded(t, "ctrl-c")
	if w.exit != nil {
		t.Errorf("ctrl-c ended the watch with %v, want exit status zero", w.exit)
	}
	waitForLiveSessions(t, srv, 0)
}

// TestWatchResolvesProjectKeysAndRefusesOneItCannotRead asserts both halves of
// a refusal: the reason reaches the client and no subscription was opened.
func TestWatchResolvesProjectKeysAndRefusesOneItCannotRead(t *testing.T) {
	srv, events, _ := watchListener(t)
	w := dialWatch(t, srv.Addr(), "watch demo --type task.*", false)
	w.stderr.waitFor(t, "project=demo type=task.*")
	f := <-events.filters
	if len(f.ProjectIDs) != 1 || f.ProjectIDs[0] == "demo" || f.ProjectIDs[0] == "" {
		t.Errorf("watch demo subscribed with project ids %q, want the seeded project's identifier", f.ProjectIDs)
	}
	if len(f.Types) != 1 || f.Types[0] != "task.*" {
		t.Errorf("watch --type task.* subscribed with types %q", f.Types)
	}

	refused := dialWatch(t, srv.Addr(), "watch nosuch", false)
	refused.waitEnded(t, "a project it cannot read")
	if !strings.Contains(refused.stderr.String(), `tix: watch: no project "nosuch" that you can read`) {
		t.Errorf("the refusal said %q, want the reason", refused.stderr.String())
	}
	if refused.exit == nil {
		t.Error("a refused watch exited zero")
	}
	select {
	case f := <-events.filters:
		t.Fatalf("a refused watch still subscribed with %+v", f)
	default:
	}
}

// TestShutdownEndsAWatchAndSaysWhy drains past the deadline with a watch
// streaming, which is the one session that never ends on its own.
func TestShutdownEndsAWatchAndSaysWhy(t *testing.T) {
	srv, _, _ := watchListener(t)
	w := dialWatch(t, srv.Addr(), "watch", false)
	w.stderr.waitFor(t, "ctrl-c to stop")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := srv.Drain(ctx); err != nil {
		t.Fatalf("drain: %v", err)
	}
	w.waitEnded(t, "server shutdown")
	if !strings.Contains(w.stderr.String(), shutdownNotice) {
		t.Errorf("the watch ended saying %q, want the shutdown notice", w.stderr.String())
	}
}

// nextDecision is the watchdog's next logged decision, failing rather than
// hanging when it makes none.
func nextDecision(t *testing.T, decisions <-chan string) string {
	t.Helper()
	select {
	case got := <-decisions:
		return got
	case <-time.After(30 * time.Second):
		t.Fatal("the watchdog made no decision in 30s")
		return ""
	}
}

// TestAWatchIsNotClosedForIdleness pins the watchdog's other half: a stream
// has no keystrokes by design, so a full idle timeout passing must leave it
// streaming, while the keepalive still runs.
func TestAWatchIsNotClosedForIdleness(t *testing.T) {
	const idle = 20 * time.Minute
	decisions := make(chan string, 64)
	srv, events, clk := watchListener(t, func(o *Options) {
		o.IdleTimeout, o.KeepaliveInterval = idle, idle
		o.Logger = slog.New(&keepaliveDecisions{seen: decisions})
	})
	before := clk.Tickers()
	w := dialWatch(t, srv.Addr(), "watch", false)
	w.stderr.waitFor(t, "ctrl-c to stop")
	waitForWatchdog(t, clk, before+1)
	for range 2 {
		clk.Advance(idle)
		if got := nextDecision(t, decisions); got != keepaliveSent {
			t.Fatalf("the watchdog logged %q, want a keepalive", got)
		}
	}
	events.send(t, claimed(9))
	w.stdout.waitFor(t, "000009")
}

// TestAWatchWhoseClientVanishesIsDropped runs the keepalive limit against a
// stream, which has no interface to quit and must still give its slot back.
func TestAWatchWhoseClientVanishesIsDropped(t *testing.T) {
	const interval = time.Minute
	decisions := make(chan string, 64)
	srv, _, clk := watchListener(t, func(o *Options) {
		o.KeepaliveInterval, o.KeepaliveMaxMissed = interval, 1
		o.Logger = slog.New(&keepaliveDecisions{seen: decisions})
	})
	before := clk.Tickers()
	r := newRelay(t, srv.Addr())
	w := dialWatch(t, r.addr(), "watch", false)
	w.stderr.waitFor(t, "ctrl-c to stop")
	waitForWatchdog(t, clk, before+1)
	r.freeze()
	clk.Advance(interval)
	if got := nextDecision(t, decisions); got != keepaliveSent {
		t.Fatalf("the first tick logged %q, want a keepalive", got)
	}
	clk.Advance(interval)
	if got := nextDecision(t, decisions); got != clientDropped {
		t.Fatalf("after the limit the listener logged %q, want the client dropped", got)
	}
	<-r.dropped
	waitForLiveSessions(t, srv, 0)
}
