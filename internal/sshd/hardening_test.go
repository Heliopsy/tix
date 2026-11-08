// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd_test

import (
	"bytes"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sshd"
	gossh "golang.org/x/crypto/ssh"
)

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s never happened", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// startHeld starts a command that waits on standard input the test holds
// open, and returns the input, its stderr and a channel carrying its end.
func startHeld(t *testing.T, client *gossh.Client, line string, stdout io.Writer) (io.WriteCloser, *bytes.Buffer, chan error) {
	t.Helper()
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	errw := &bytes.Buffer{}
	sess.Stderr = errw
	if stdout != nil {
		sess.Stdout = stdout
	}
	done := make(chan error, 1)
	go func() { done <- sess.Run(line) }()
	return stdin, errw, done
}

// exitOf reads a session's exit status from what Run returned.
func exitOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit *gossh.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the session ended without a status: %v", err)
	}
	return exit.ExitStatus()
}

// Every command started goroutines that waited for the connection to end,
// and a client keeping one connection alive collected them without bound.
func TestCommandsOnOneConnectionLeaveNoGoroutinesBehind(t *testing.T) {
	_, _, client := enrolled(t, nil)
	run(t, client, "task ls -p tob", nil, nil)
	before := runtime.NumGoroutine()
	for range 60 {
		if r := run(t, client, "task ls -p tob", nil, nil); r.code != 0 {
			t.Fatalf("task ls exited %d: %s", r.code, r.err)
		}
	}
	var after int
	waitFor(t, "the goroutines settling", func() bool {
		after = runtime.NumGoroutine()
		return after < before+15
	})
}

// Two channels on one connection each register and give back their own
// entry; a handle kept on the connection's context was clobbered by the
// second and left the first behind as a ghost.
func TestChannelsOnOneConnectionKeepTheirOwnRegistration(t *testing.T) {
	reg := connections.New()
	_, p, client := enrolled(t, func(o *sshd.Options) { o.Connections = reg })
	held, _, done := startHeld(t, client, "import tasks -p tob", nil)
	waitFor(t, "the held command registering", func() bool { return len(reg.List(p.TenantID)) == 1 })
	if r := run(t, client, "task ls -p tob", nil, nil); r.code != 0 {
		t.Fatalf("task ls exited %d: %s", r.code, r.err)
	}
	if n := len(reg.List(p.TenantID)); n != 1 {
		t.Fatalf("after the quick command ended %d connections are listed, want the held one", n)
	}
	_ = held.Close()
	<-done
	waitFor(t, "the held command unregistering", func() bool { return len(reg.List(p.TenantID)) == 0 })
}

// A client that stops reading is stopped at the write stall timeout, rather
// than holding its slot until the command timeout. Its window is full, and
// stderr shares it, so all it can be sure to see is the session closing.
func TestAClientThatStopsReadingIsStopped(t *testing.T) {
	l, p, client := enrolled(t, func(o *sshd.Options) {
		o.WriteStallTimeout = 300 * time.Millisecond
		o.RecheckInterval = time.Hour
	})
	body := strings.Repeat("payload ", 8<<10)
	for i := range 60 {
		if _, err := l.svc.CreateTask(sshd.SessionContext(p.Actor), core.CreateTaskInput{
			ProjectRef: "tob", Title: "bulk", Body: body + string(rune('a'+i%26)),
		}); err != nil {
			t.Fatal(err)
		}
	}
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	if _, err := sess.StdoutPipe(); err != nil {
		t.Fatal(err)
	}
	var errw bytes.Buffer
	sess.Stderr = &errw
	done := make(chan error, 1)
	go func() { done <- sess.Run("export --comments") }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a stalled export reported success")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("a client that stopped reading held the command open")
	}
}

// A key revoked while its command runs stops the command within a recheck.
func TestARevokedKeyStopsItsRunningCommand(t *testing.T) {
	reg := connections.New()
	l, p, client := enrolled(t, func(o *sshd.Options) {
		o.RecheckInterval = 100 * time.Millisecond
		o.Connections = reg
	})
	held, errw, done := startHeld(t, client, "import tasks -p tob", nil)
	defer func() { _ = held.Close() }()
	waitFor(t, "the command starting", func() bool { return len(reg.List(p.TenantID)) == 1 })
	sshd.RevokeKeysOf(t, l.st, p)
	select {
	case err := <-done:
		if exitOf(t, err) != core.ExitPermission || !strings.Contains(errw.String(), "access changed") {
			t.Fatalf("the revoked key's command ended %v, stderr %q", err, errw.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("a revoked key's command kept running")
	}
}

// Under a pseudo-terminal the data streams are the bytes, untouched, and a
// prompt still finds a person to ask.
func TestAPseudoTerminalLeavesDataByteExactAndPromptsWorking(t *testing.T) {
	_, _, client := enrolled(t, nil)
	for _, title := range []string{"one", "two"} {
		if r := run(t, client, "task add "+title+" -p tob", nil, nil); r.code != 0 {
			t.Fatalf("task add exited %d: %s", r.code, r.err)
		}
	}
	plain := run(t, client, "export tasks -p tob", nil, nil)
	withPty := runPty(t, client, "export tasks -p tob", nil)
	if withPty.out != plain.out {
		t.Fatalf("the export under a pty differs:\n%q\n%q", withPty.out, plain.out)
	}
	csv := "title\r\nfrom a pty\r\n"
	if r := runPty(t, client, "import tasks -p tob --dry-run -o json", strings.NewReader(csv)); r.code != 0 ||
		!strings.Contains(r.out, "from a pty") {
		t.Fatalf("an import under a pty: exit %d, out %q, err %q", r.code, r.out, r.err)
	}
	enrol := runPty(t, client, "user totp enrol", strings.NewReader("\r"))
	if strings.Contains(enrol.err+enrol.out, "needs a terminal") || !strings.Contains(enrol.err+enrol.out, "otpauth://") {
		t.Fatalf("a prompt under a pty found nobody: exit %d, %q", enrol.code, enrol.err+enrol.out)
	}
}

// runPty is run with a pseudo-terminal, as `ssh -t` asks for one.
func runPty(t *testing.T, client *gossh.Client, line string, stdin io.Reader) result {
	t.Helper()
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	if err := sess.RequestPty("xterm", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	sess.Stdout, sess.Stderr = &out, &errw
	if stdin != nil {
		sess.Stdin = stdin
	}
	code := exitOf(t, sess.Run(line))
	return result{out: out.String(), err: errw.String(), code: code}
}
