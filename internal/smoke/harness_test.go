//go:build smoke

package smoke

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/heliopsy/tix/internal/testenv"
)

// Bounds are generous rather than tuned: a smoke run is a sanity pass, so
// anything slower than these is a real failure and not a busy machine.
const (
	pollEvery  = 50 * time.Millisecond
	readyWait  = 30 * time.Second
	screenWait = 30 * time.Second
	exitWait   = 30 * time.Second
)

// maxCell is the widest a rendered column may be. The fingerprint column is
// fifty characters and legitimate; an ssh-ed25519 public key is eighty and is
// the regression this bound exists to catch.
const maxCell = 64

// tixBin is the binary under test, built once for the whole package.
var tixBin string

func TestMain(m *testing.M) {
	code := build()
	if code == 0 {
		code = m.Run()
	}
	testenv.AppendLog()
	testenv.Report(os.Stderr)
	os.Exit(code)
}

// build compiles the real binary from the repository root, which is what
// separates this suite from one calling cmd.Execute in process.
func build() int {
	dir, err := os.MkdirTemp("", "tix-smoke-bin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoke: scratch directory: %v\n", err)
		return 1
	}
	tixBin = filepath.Join(dir, "tix")
	cmd := exec.Command("go", "build", "-o", tixBin, ".")
	cmd.Dir = "../.."
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "smoke: building the binary: %v\n", err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------- capabilities

// sshClientCapability names the real SSH client the ssh surface is driven
// with. The CI toolbox image carries no openssh, so those tests skip there and
// the run says so rather than passing in silence.
func sshClientCapability() testenv.Capability {
	return testenv.Capability{
		Name: "ssh-client",
		Why:  "ssh and ssh-keygen are not on PATH, so nothing drove `tix ssh` as a real client would",
		How:  "install openssh (apk add openssh-client) and rerun: just smoke",
	}
}

// ptyCapability names the pseudo-terminal the terminal interface needs.
func ptyCapability(why string) testenv.Capability {
	return testenv.Capability{
		Name: "pty",
		Why:  why,
		How:  "run where /dev/ptmx is available (a container needs no special flag on Linux) and rerun: just smoke",
	}
}

// requireSSHClient skips through testenv when no SSH client is installed.
func requireSSHClient(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ssh", "ssh-keygen"} {
		if _, err := exec.LookPath(bin); err != nil {
			testenv.Skip(t, sshClientCapability())
		}
	}
}

// requirePTY skips through testenv when a pseudo-terminal cannot be opened.
func requirePTY(t *testing.T) {
	t.Helper()
	p, tty, err := pty.Open()
	if err != nil {
		testenv.Skip(t, ptyCapability("opening a pseudo-terminal failed: "+err.Error()))
	}
	_ = tty.Close()
	_ = p.Close()
}

// ---------------------------------------------------------------- running it

// result is one finished run of the binary.
type result struct {
	out  string
	err  string
	code int
}

// scratch is a home nobody else shares: HOME and XDG_DATA_HOME point inside
// it, so a command with no --db resolves to a store this test owns. The real
// one at the operator's XDG_DATA_HOME is never reachable from here.
type scratch struct {
	t    *testing.T
	dir  string
	envs []string
}

func newScratch(t *testing.T) *scratch {
	t.Helper()
	dir := t.TempDir()
	return &scratch{t: t, dir: dir, envs: []string{
		"HOME=" + dir,
		"XDG_DATA_HOME=" + filepath.Join(dir, "data"),
		"XDG_CONFIG_HOME=" + filepath.Join(dir, "config"),
		"XDG_STATE_HOME=" + filepath.Join(dir, "state"),
		"TIX_NO_COLOR=1",
		"NO_COLOR=1",
	}}
}

// env returns the environment every subprocess of this scratch home runs with.
func (s *scratch) env() []string {
	return append(os.Environ(), s.envs...)
}

// path names a file inside the scratch home.
func (s *scratch) path(name string) string { return filepath.Join(s.dir, name) }

// run executes the binary and returns what it printed and how it exited.
func (s *scratch) run(args ...string) result {
	s.t.Helper()
	cmd := exec.Command(tixBin, args...)
	cmd.Env = s.env()
	cmd.Dir = s.dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		s.t.Fatalf("running tix %s: %v", strings.Join(args, " "), err)
	}
	return result{out: out.String(), err: errb.String(), code: code}
}

// mustRun fails the test when the binary does not exit cleanly.
func (s *scratch) mustRun(args ...string) result {
	s.t.Helper()
	got := s.run(args...)
	if got.code != 0 {
		s.t.Fatalf("tix %s exited %d\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), got.code, got.out, got.err)
	}
	return got
}

// daemon is a long-running subprocess: a server or a listener.
type daemon struct {
	t   *testing.T
	cmd *exec.Cmd
	log *lockedBuffer
}

// start launches the binary in the background and stops it when the test ends.
func (s *scratch) start(args ...string) *daemon {
	s.t.Helper()
	cmd := exec.Command(tixBin, args...)
	cmd.Env = s.env()
	cmd.Dir = s.dir
	log := &lockedBuffer{}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		s.t.Fatalf("starting tix %s: %v", strings.Join(args, " "), err)
	}
	d := &daemon{t: s.t, cmd: cmd, log: log}
	s.t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	return d
}

// signal asks the process to stop the way an operator would.
func (d *daemon) signal(sig os.Signal) {
	d.t.Helper()
	if err := d.cmd.Process.Signal(sig); err != nil {
		d.t.Fatalf("signalling: %v", err)
	}
}

// waitExit returns the exit code, failing when the process outstays the bound.
func (d *daemon) waitExit() int {
	d.t.Helper()
	done := make(chan error, 1)
	go func() { done <- d.cmd.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		if err != nil {
			d.t.Fatalf("waiting for exit: %v\nlog:\n%s", err, d.log.String())
		}
		return 0
	case <-time.After(exitWait):
		d.t.Fatalf("did not exit within %s\nlog:\n%s", exitWait, d.log.String())
		return -1
	}
}

// lockedBuffer collects a subprocess's output while a test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// ---------------------------------------------------------------- waiting

// freePort returns a loopback address nothing is listening on.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("releasing the port: %v", err)
	}
	return addr
}

// waitForHTTP blocks until the url answers.
func waitForHTTP(t *testing.T, url string, d *daemon) {
	t.Helper()
	deadline := time.Now().Add(readyWait)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec // a loopback address this test chose
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(pollEvery)
	}
	t.Fatalf("%s never answered\nlog:\n%s", url, d.log.String())
}

// waitForPortFree blocks until the address can be bound again, which is how a
// clean shutdown differs from a process that merely stopped printing.
func waitForPortFree(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(exitWait)
	for time.Now().Before(deadline) {
		l, err := net.Listen("tcp", addr)
		if err == nil {
			_ = l.Close()
			return
		}
		time.Sleep(pollEvery)
	}
	t.Fatalf("%s was never released", addr)
}

// waitForLog blocks until the daemon has printed want.
func waitForLog(t *testing.T, d *daemon, want string) {
	t.Helper()
	deadline := time.Now().Add(readyWait)
	for time.Now().Before(deadline) {
		if strings.Contains(d.log.String(), want) {
			return
		}
		time.Sleep(pollEvery)
	}
	t.Fatalf("never printed %q\nlog:\n%s", want, d.log.String())
}

// ---------------------------------------------------------------- terminals

// terminal is a subprocess holding a real pseudo-terminal, which is the only
// way to drive an interface that draws itself.
type terminal struct {
	t    *testing.T
	cmd  *exec.Cmd
	file *os.File
	out  *lockedBuffer
}

// startTerminal runs name under a pty of the given size.
func startTerminal(t *testing.T, env []string, dir string, size *pty.Winsize, name string, args ...string) *terminal {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Dir = dir
	f, err := pty.StartWithSize(cmd, size)
	if err != nil {
		testenv.Skip(t, ptyCapability("starting a process on a pseudo-terminal failed: "+err.Error()))
	}
	term := &terminal{t: t, cmd: cmd, file: f, out: &lockedBuffer{}}
	go term.pump()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		_ = f.Close()
	})
	return term
}

// pump collects what is drawn and answers the questions a real terminal
// answers. The interface asks for the background colour and the cursor
// position on startup and waits five seconds for a reply, so a pty that stays
// silent turns every terminal assertion into a five-second one.
func (term *terminal) pump() {
	buf := make([]byte, 4096)
	for {
		n, err := term.file.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = term.out.Write(chunk)
			term.answer(chunk)
		}
		if err != nil {
			return
		}
	}
}

// answer replies to the terminal queries the interface makes.
func (term *terminal) answer(chunk []byte) {
	replies := []struct{ query, reply string }{
		{"\x1b]11;?", "\x1b]11;rgb:0000/0000/0000\x1b\\"},
		{"\x1b[6n", "\x1b[1;1R"},
	}
	for _, r := range replies {
		if bytes.Contains(chunk, []byte(r.query)) {
			_, _ = term.file.WriteString(r.reply)
		}
	}
}

// screen is everything drawn so far, with the escape sequences removed.
func (term *terminal) screen() string { return stripANSI(term.out.String()) }

// waitForScreen polls until every wanted string has been drawn. Polling rather
// than sleeping is the point: the interface loads its projects and opens its
// event stream asynchronously, so a fixed sleep reads as "zero projects, not
// connected", which looks exactly like a bug and is not one.
func (term *terminal) waitForScreen(want ...string) string {
	term.t.Helper()
	deadline := time.Now().Add(screenWait)
	var seen string
	for time.Now().Before(deadline) {
		seen = term.screen()
		if containsAll(seen, want) {
			return seen
		}
		time.Sleep(pollEvery)
	}
	term.t.Fatalf("the interface never settled; wanted every one of %q\nscreen:\n%s", want, seen)
	return ""
}

// send types into the terminal.
func (term *terminal) send(s string) {
	term.t.Helper()
	if _, err := term.file.WriteString(s); err != nil {
		term.t.Fatalf("typing %q: %v", s, err)
	}
}

// quit leaves the interface the way a person does. One "q" steps out of the
// board to the project list and a second leaves the program, so the key is
// repeated until the process is actually gone rather than pressed once and
// hoped about.
func (term *terminal) quit() {
	term.t.Helper()
	done := make(chan error, 1)
	go func() { done <- term.cmd.Wait() }()
	deadline := time.Now().Add(exitWait)
	for time.Now().Before(deadline) {
		term.send("q")
		select {
		case <-done:
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
	term.t.Fatalf("the interface never quit\nscreen:\n%s", term.screen())
}

func containsAll(s string, want []string) bool {
	for _, w := range want {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}

// ansi matches the escape sequences a drawn interface leaves in a capture.
var ansi = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b[()][AB012]|\x1b[=>]`)

func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }

// ---------------------------------------------------------------- rendering

// goTime matches a Go time.Time printed by fmt: the reflection renderer's
// signature, and never what a formatted column looks like.
var goTime = regexp.MustCompile(`[+-]\d{4} [A-Z]{2,5}\b|m=[+-]\d`)

// assertRendered holds a listing to what a person can read: no zero value
// spelled "<nil>", no raw Go timestamp where a formatted time belongs, the
// columns it is supposed to have, and none of them absurdly wide.
//
// Every one of these fired at once the day `tix user key ls` fell through to
// the reflection renderer, with a service that was entirely correct.
func assertRendered(t *testing.T, label, out string, header ...string) {
	t.Helper()
	if strings.Contains(out, "<nil>") {
		t.Errorf("%s printed <nil> where a value belongs:\n%s", label, out)
	}
	if m := goTime.FindString(out); m != "" {
		t.Errorf("%s printed the raw Go timestamp %q instead of a formatted time:\n%s", label, m, out)
	}
	rows := tableRows(out)
	if len(rows) == 0 {
		t.Fatalf("%s rendered no table at all:\n%s", label, out)
	}
	if got := rows[0]; !sameColumns(got, header) {
		t.Errorf("%s header = %q, want %q:\n%s", label, got, header, out)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len([]rune(cell)) > maxCell {
				t.Errorf("%s column %d is %d characters wide, over the %d a terminal can read: %q",
					label, i, len([]rune(cell)), maxCell, cell)
			}
		}
	}
}

// tableRows splits a rendered table into its cells, headers first.
func tableRows(out string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(stripANSI(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "│") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "│"), "│")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

func sameColumns(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// sleepPoll waits one polling interval.
func sleepPoll() { time.Sleep(pollEvery) }
