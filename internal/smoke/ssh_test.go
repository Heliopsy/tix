//go:build smoke

package smoke

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"

	"github.com/heliopsy/tix/internal/core"
)

// board is a terminal big enough for the interface to draw a board in.
var board = &pty.Winsize{Cols: 100, Rows: 30}

// TestSSHWithoutADatabaseCreatesNothing is this morning's bug, kept caught: a
// demo listener refuses the store holding somebody's real work, and a command
// that refuses must leave nothing behind. The refusal used to arrive after the
// store had already been created and migrated.
func TestSSHWithoutADatabaseCreatesNothing(t *testing.T) {
	s := newScratch(t)

	got := s.run("ssh", "--demo")
	if got.code != core.KindInvalid.ExitCode() {
		t.Fatalf("tix ssh --demo exited %d, want %d\nstdout:\n%s\nstderr:\n%s",
			got.code, core.KindInvalid.ExitCode(), got.out, got.err)
	}
	if !strings.Contains(got.err, "give the demo a database of its own with --db") {
		t.Fatalf("the refusal does not say what to do instead:\n%s", got.err)
	}
	if found := findDatabases(t, s.dir); len(found) != 0 {
		t.Fatalf("a refused command left databases behind: %v", found)
	}
}

// findDatabases lists every store under dir, which after a refusal is none.
func findDatabases(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".db") {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the scratch home: %v", err)
	}
	return found
}

// TestTUIRefusesATerminalTooSmallToDraw checks the message a person gets
// instead of a broken frame.
func TestTUIRefusesATerminalTooSmallToDraw(t *testing.T) {
	requirePTY(t)
	s := newScratch(t)
	db := s.path("tiny.db")
	s.mustRun("--db", db, "task", "add", "buy milk")

	// Fifty columns and six rows: too short to draw a board, wide enough for
	// the whole refusal to be readable rather than cut off mid-word.
	term := startTerminal(t, s.env(), s.dir, &pty.Winsize{Cols: 50, Rows: 6}, tixBin, "--db", db, "tui")
	term.waitForScreen("terminal is 50x6; tix tui needs at least 24x8")
	term.quit()
}

// TestSSHDemoServesASeededBoardToAnyKey drives the public listener with a real
// client: any key is accepted and lands on the seeded demo board.
func TestSSHDemoServesASeededBoardToAnyKey(t *testing.T) {
	requireSSHClient(t)
	requirePTY(t)
	s := newScratch(t)

	addr := freePort(t)
	listener := s.start("ssh", "--demo", "--db", s.path("demo.db"), "--listen", addr)
	waitForLog(t, listener, "tix ssh listening on")

	term := dialSSH(t, s, addr, generateKey(t, s, "visitor"))
	// Every one of these is asynchronous: the board arrives from loadProjects
	// and "live" from the event subscription. Asserting before both have
	// landed reads as an empty board on a dead stream, which is not a bug.
	screen := term.waitForScreen("Demo board", "live", "connected")
	if !strings.Contains(screen, "demo-") {
		t.Fatalf("the demo board carried no seeded tasks:\n%s", screen)
	}

	term.quit()
}

// TestSSHServesEnrolledKeysAndRefusesTheRest is the default listener: a key
// somebody enrolled reaches that actor's own board, and a key nobody enrolled
// reaches nothing.
func TestSSHServesEnrolledKeysAndRefusesTheRest(t *testing.T) {
	requireSSHClient(t)
	requirePTY(t)
	s := newScratch(t)
	db := s.path("enrolled.db")

	s.mustRun("--db", db, "task", "add", "bootstrap the tenant")
	s.mustRun("--db", db, "user", "create", "smoke@example.test", "--handle", "smoke", "--role", "admin")
	s.mustRun("--db", db, "project", "create", "mine", "Mine")

	enrolled := generateKey(t, s, "enrolled")
	s.mustRun("--db", db, "user", "key", "add", "--file", enrolled+".pub", "--actor", "smoke", "--label", "laptop")
	stranger := generateKey(t, s, "stranger")

	addr := freePort(t)
	listener := s.start("--db", db, "ssh", "--listen", addr)
	waitForLog(t, listener, "tix ssh listening on")

	t.Run("an enrolled key lands on its own board", func(t *testing.T) {
		term := dialSSH(t, s, addr, enrolled)
		screen := term.waitForScreen("as smoke", "live", "connected", "mine")
		if !strings.Contains(screen, "Mine") {
			t.Fatalf("the board did not show the actor's own projects:\n%s", screen)
		}
		term.quit()
	})

	t.Run("an unenrolled key is refused", func(t *testing.T) {
		out, code := runSSH(t, s, addr, stranger)
		if code == 0 {
			t.Fatalf("a key nobody enrolled was let in:\n%s", out)
		}
		if !strings.Contains(out, "not enrolled") {
			t.Fatalf("the refusal does not say why:\n%s", out)
		}
	})
}

// generateKey writes a throwaway key pair into the scratch home and returns
// the path of its private half.
func generateKey(t *testing.T, s *scratch, name string) string {
	t.Helper()
	path := filepath.Join(s.dir, name)
	cmd := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", name, "-f", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generating %s: %v\n%s", name, err, out)
	}
	return path
}

// sshArgs are what every connection here needs: this key and no other, and a
// host key that is new every run and belongs in nobody's known_hosts.
func sshArgs(addr, key string) []string {
	host, port, _ := strings.Cut(addr, ":")
	return []string{
		"-p", port,
		"-i", key,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"-o", "LogLevel=ERROR",
		"tix@" + host,
	}
}

// dialSSH opens an interactive session on a real terminal.
func dialSSH(t *testing.T, s *scratch, addr, key string) *terminal {
	t.Helper()
	return startTerminal(t, s.env(), s.dir, board, "ssh", sshArgs(addr, key)...)
}

// runSSH connects without expecting a session, for the refusal.
func runSSH(t *testing.T, s *scratch, addr, key string) (string, int) {
	t.Helper()
	cmd := exec.Command("ssh", append([]string{"-T"}, sshArgs(addr, key)...)...)
	cmd.Env = s.env()
	cmd.Dir = s.dir
	out, err := cmd.CombinedOutput()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("running ssh: %v", err)
	}
	return string(out), code
}
