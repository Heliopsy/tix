// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// sshWait bounds every read from a live SSH session. It is a failure timeout
// rather than a latency budget: an assertion returns the instant its marker
// arrives, so the ceiling costs nothing when the listener works.
const sshWait = 30 * time.Second

// demoBoardMarker is text the seeded sandbox board paints and nothing else
// does, so seeing it proves the terminal interface came up rather than that
// some bytes arrived.
const demoBoardMarker = "Demo board"

// sshHarness is a real SSH listener over a temporary database, reachable on
// an ephemeral loopback port.
type sshHarness struct {
	t     *testing.T
	addr  string
	svc   *service.Local
	store store.Store
	conns *connections.Registry

	tenantID    string
	actorID     string
	actorHandle string
}

// newSSHHarness boots the listener in the mode the test needs. In enrolled
// mode it also creates a user to enrol keys against.
func newSSHHarness(t *testing.T, demo bool) *sshHarness {
	t.Helper()
	ctx := context.Background()
	clk := clock.New()
	path := filepath.Join(t.TempDir(), "tix.db")

	st, err := sqlite.Open(path, clk)
	if err != nil {
		t.Fatalf("opening the store at %q: %v", path, err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating %q: %v", path, err)
	}

	svc := service.New(st,
		service.WithClock(clk),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams())),
		service.WithoutStarterProjects())
	t.Cleanup(func() { _ = svc.Close() })

	tenant, err := svc.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("bootstrapping defaults: %v", err)
	}
	adminCtx := core.WithSource(core.WithActor(ctx, core.SystemActor(tenant.ID)), core.SourceCLI)

	h := &sshHarness{
		t: t, svc: svc, store: st,
		conns:    connections.New(),
		tenantID: tenant.ID,
	}
	if !demo {
		user, err := svc.CreateUser(adminCtx, core.CreateUserInput{
			Email: "member@example.test", Handle: "member", Role: core.RoleMember,
		})
		if err != nil {
			t.Fatalf("creating the enrolled user: %v", err)
		}
		h.actorID, h.actorHandle = user.ID, "member"
	}

	srv, err := sshd.New(sshd.Options{
		Service:     svc,
		Store:       st,
		Clock:       clk,
		Demo:        demo,
		Addr:        "127.0.0.1:0",
		HostKeyPath: filepath.Join(t.TempDir(), "ssh_host_ed25519_key"),
		Connections: h.conns,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("building the ssh listener: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("binding the ssh listener: %v", err)
	}
	serveCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(serveCtx)
	}()
	t.Cleanup(func() {
		stop()
		_ = srv.Close()
		wg.Wait()
	})
	h.addr = srv.Addr()
	return h
}

// enrol registers a public key against the harness's user.
func (h *sshHarness) enrol(pub gossh.PublicKey) {
	h.t.Helper()
	actor := &core.Actor{
		ID: h.actorID, TenantID: h.tenantID, Kind: core.ActorUser,
		Handle: h.actorHandle, Role: core.RoleAdmin, Scopes: []core.Scope{core.ScopeAll},
	}
	ctx := core.WithSource(core.WithActor(context.Background(), actor), core.SourceCLI)
	line := string(gossh.MarshalAuthorizedKey(pub))
	if _, err := h.svc.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{
		ActorID: h.actorID, PublicKey: line, Label: "integration",
	}); err != nil {
		h.t.Fatalf("enrolling a key: %v", err)
	}
}

// liveSSHConnection waits for the listener to register a session and returns
// the identity it registered it under, which is the actor the session is
// actually running as.
func (h *sshHarness) liveSSHConnection() core.Connection {
	h.t.Helper()
	deadline := time.Now().Add(sshWait)
	for time.Now().Before(deadline) {
		if live := h.conns.BySurface(core.ConnectionSSH); len(live) > 0 {
			return live[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("no ssh session was registered within %s", sshWait)
	return core.Connection{}
}

// tenantKeyOf reads a tenant's key, which is how a test tells a sandbox from
// a real tenant without reaching into the listener.
func (h *sshHarness) tenantKeyOf(tenantID string) string {
	h.t.Helper()
	var key string
	err := h.store.Unscoped(context.Background(), func(u store.UnscopedTx) error {
		t, err := u.GetTenantByID(context.Background(), tenantID)
		if err != nil {
			return err
		}
		key = t.Key
		return nil
	})
	if err != nil {
		h.t.Fatalf("reading tenant %q: %v", tenantID, err)
	}
	return key
}

// newSigner returns a fresh key nobody has seen before.
func newSigner(t *testing.T) gossh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("building a signer: %v", err)
	}
	return signer
}

// dialSSH opens an authenticated connection to the listener.
func dialSSH(t *testing.T, addr string, signer gossh.Signer, user string) *gossh.Client {
	t.Helper()
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         sshWait,
	})
	if err != nil {
		t.Fatalf("dialling %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// syncBuffer collects a session's output from the reader goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// terminal is one interactive session with its output collected.
type terminal struct {
	t     *testing.T
	sess  *gossh.Session
	stdin io.WriteCloser
	out   *syncBuffer
	err   *syncBuffer
	done  chan error
}

// openTerminal asks for a pseudo-terminal and starts the session, passing
// command when one is given so the exec path is exercised rather than shell.
func openTerminal(t *testing.T, client *gossh.Client, command string) *terminal {
	t.Helper()
	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("opening a session: %v", err)
	}
	out, errOut := &syncBuffer{}, &syncBuffer{}
	sess.Stdout, sess.Stderr = out, errOut
	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if err := sess.RequestPty("xterm-256color", 48, 160, gossh.TerminalModes{}); err != nil {
		t.Fatalf("requesting a pty: %v", err)
	}
	term := &terminal{t: t, sess: sess, stdin: stdin, out: out, err: errOut, done: make(chan error, 1)}
	if command == "" {
		err = sess.Shell()
	} else {
		err = sess.Start(command)
	}
	if err != nil {
		t.Fatalf("starting the session: %v", err)
	}
	go func() { term.done <- sess.Wait() }()
	t.Cleanup(func() { _ = sess.Close() })
	return term
}

// waitFor blocks until the session's output carries want, failing with
// everything seen so far when it does not.
func (term *terminal) waitFor(want string) {
	term.t.Helper()
	deadline := time.Now().Add(sshWait)
	for time.Now().Before(deadline) {
		if strings.Contains(term.out.String(), want) {
			return
		}
		select {
		case err := <-term.done:
			if strings.Contains(term.out.String(), want) {
				return
			}
			term.t.Fatalf("the session ended before %q appeared (%v)\nstdout:\n%s\nstderr:\n%s",
				want, err, term.out.String(), term.err.String())
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	term.t.Fatalf("%q never appeared within %s\nstdout:\n%s\nstderr:\n%s",
		want, sshWait, term.out.String(), term.err.String())
}

// waitEnded blocks until the session is over and returns its error.
func (term *terminal) waitEnded() error {
	term.t.Helper()
	select {
	case err := <-term.done:
		return err
	case <-time.After(sshWait):
		term.t.Fatalf("the session did not end within %s\nstdout:\n%s\nstderr:\n%s",
			sshWait, term.out.String(), term.err.String())
		return nil
	}
}

// quit presses q until the interface gives the terminal back. The first
// presses may land while the program is still starting, hence the repeat.
func (term *terminal) quit() {
	term.t.Helper()
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := term.stdin.Write([]byte("q")); err != nil {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()
	defer close(stop)
	select {
	case <-term.done:
	case <-time.After(sshWait):
		term.t.Errorf("the session did not end after quitting\nstdout:\n%s", term.out.String())
	}
}

// TestSSHOffersTheTerminalInterfaceAndNothingElse pins what the network-facing
// surface exposes. Everything the listener does NOT declare is true today by
// omission, so each omission is asserted here against a real listener.
func TestSSHOffersTheTerminalInterfaceAndNothingElse(t *testing.T) {
	t.Run("a command is not executed, the interface opens instead", func(t *testing.T) {
		h := newSSHHarness(t, true)
		sentinel := filepath.Join(t.TempDir(), "executed")
		client := dialSSH(t, h.addr, newSigner(t), sshd.NeutralUser)

		// A real side effect rather than a marker in the output: an exec path
		// added later would run this through a shell and leave the file
		// behind, whatever it chose to print.
		term := openTerminal(t, client, "touch "+sentinel)
		term.waitFor(demoBoardMarker)
		if _, err := os.Stat(sentinel); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("the command passed to ssh ran: %q exists (%v)", sentinel, err)
		}
		if got := term.out.String(); strings.Contains(got, "touch "+sentinel) {
			t.Errorf("the session echoed the command back:\n%s", got)
		}
		term.quit()
	})

	t.Run("a subsystem request is refused", func(t *testing.T) {
		h := newSSHHarness(t, true)
		client := dialSSH(t, h.addr, newSigner(t), sshd.NeutralUser)

		sess, err := client.NewSession()
		if err != nil {
			t.Fatalf("opening a session: %v", err)
		}
		subsystemErr := sess.RequestSubsystem("sftp")
		_ = sess.Close()
		if subsystemErr == nil {
			t.Fatal("the listener accepted an sftp subsystem request")
		}

		// The refusal must be the subsystem request's own, not a connection
		// that had already died for some other reason: the same connection
		// still serves the interface immediately afterwards.
		term := openTerminal(t, client, "")
		term.waitFor(demoBoardMarker)
		term.quit()
	})

	t.Run("a direct-tcpip channel is refused", func(t *testing.T) {
		h := newSSHHarness(t, true)
		client := dialSSH(t, h.addr, newSigner(t), sshd.NeutralUser)

		// A listener the test owns, so the assertion is that the SSH server
		// never made the outbound connection. Aiming at a dead port instead
		// would pass even with forwarding enabled, because the refusal would
		// then come from the far end rather than from the channel being
		// rejected.
		reached := make(chan struct{}, 1)
		target := listenAndRecord(t, reached)

		conn, err := client.Dial("tcp", target)
		if err == nil {
			_ = conn.Close()
			t.Fatal("the listener opened a direct-tcpip channel")
		}
		var openErr *gossh.OpenChannelError
		if !errors.As(err, &openErr) || openErr.Reason != gossh.UnknownChannelType {
			t.Fatalf("direct-tcpip failed with %v, want the channel rejected as an unknown type", err)
		}
		select {
		case <-reached:
			t.Fatalf("the ssh listener connected out to %s on the client's behalf", target)
		case <-time.After(250 * time.Millisecond):
		}

		term := openTerminal(t, client, "")
		term.waitFor(demoBoardMarker)
		term.quit()
	})

	t.Run("a remote forward is refused", func(t *testing.T) {
		h := newSSHHarness(t, true)
		client := dialSSH(t, h.addr, newSigner(t), sshd.NeutralUser)

		ln, err := client.Listen("tcp", "127.0.0.1:0")
		if err == nil {
			_ = ln.Close()
			t.Fatal("the listener accepted a tcpip-forward request")
		}

		term := openTerminal(t, client, "")
		term.waitFor(demoBoardMarker)
		term.quit()
	})

	// The three requests above are the ones a client can send. A door opened
	// under a name no test dials cannot be driven, so it is caught by name
	// instead: these fields are how every other surface is added to a
	// charmbracelet/ssh server, and none of them is set anywhere in the tree.
	t.Run("no second surface is wired onto the server", func(t *testing.T) {
		forbidden := []string{
			"SubsystemHandlers",
			"ChannelHandlers",
			"RequestHandlers",
			"LocalPortForwardingCallback",
			"ReversePortForwardingCallback",
			"SessionRequestCallback",
		}
		for _, field := range forbidden {
			if named := filesNamingIdentifier(t, field); len(named) > 0 {
				t.Errorf("%s is named in %v; the ssh surface is the terminal interface and nothing else",
					field, named)
			}
		}
	})

	// One construction is what makes every assertion above cover both ways in:
	// `tix ssh` and `tix serve --ssh-listen` each call sshd.New, and sshd.New
	// builds the only ssh.Server in the tree.
	t.Run("both entry points share one server", func(t *testing.T) {
		if named := filesNamingIdentifier(t, "ssh.Server{"); !equalStrings(named, []string{"internal/sshd/sshd.go"}) {
			t.Fatalf("ssh.Server is constructed in %v, want only internal/sshd/sshd.go: a second "+
				"construction would face the network without these assertions", named)
		}
		if named := filesNamingIdentifier(t, "sshd.New("); !equalStrings(named, []string{"cmd/serve.go", "cmd/ssh.go"}) {
			t.Fatalf("sshd.New is called from %v, want only cmd/serve.go and cmd/ssh.go", named)
		}
	})
}

// TestSSHRefusesAnUnenrolledKeyWithTheEnrolmentMessage asserts the refusal a
// stranger actually receives on a listener serving real users.
func TestSSHRefusesAnUnenrolledKeyWithTheEnrolmentMessage(t *testing.T) {
	h := newSSHHarness(t, false)
	client := dialSSH(t, h.addr, newSigner(t), sshd.NeutralUser)
	term := openTerminal(t, client, "")

	err := term.waitEnded()
	var exit *gossh.ExitError
	if !errors.As(err, &exit) || exit.ExitStatus() != 1 {
		t.Errorf("the refused session ended with %v, want exit status 1", err)
	}
	const want = "tix: this key is not enrolled here: ask an operator to enrol it with `tix user key add`"
	if got := strings.TrimSpace(term.err.String()); got != want {
		t.Fatalf("the refusal said %q, want %q", got, want)
	}
	if got := term.out.String(); strings.Contains(got, demoBoardMarker) {
		t.Errorf("an unenrolled key still saw a board:\n%s", got)
	}
	if live := h.conns.BySurface(core.ConnectionSSH); len(live) != 0 {
		t.Errorf("a refused key registered %d connections, want none", len(live))
	}
}

// TestDemoAndEnrolledAdmitDifferentPeopleAsDifferentActors asserts the two
// modes differ where the product says they do: who is let in, and whose
// identity the session then runs as.
func TestDemoAndEnrolledAdmitDifferentPeopleAsDifferentActors(t *testing.T) {
	t.Run("demo gives an unknown key its own sandbox visitor", func(t *testing.T) {
		h := newSSHHarness(t, true)
		client := dialSSH(t, h.addr, newSigner(t), sshd.NeutralUser)
		term := openTerminal(t, client, "")
		term.waitFor(demoBoardMarker)

		conn := h.liveSSHConnection()
		if conn.ActorHandle != "visitor" {
			t.Errorf("the demo session runs as %q, want the sandbox visitor", conn.ActorHandle)
		}
		if key := h.tenantKeyOf(conn.TenantID); !strings.HasPrefix(key, "sandbox-") {
			t.Errorf("the demo session runs in tenant %q, want a provisioned sandbox", key)
		}
		if conn.TenantID == h.tenantID {
			t.Errorf("the demo session landed in the installation's own tenant %q", conn.TenantID)
		}
		term.quit()
	})

	t.Run("enrolled runs as the actor the key is enrolled against", func(t *testing.T) {
		h := newSSHHarness(t, false)
		signer := newSigner(t)
		h.enrol(signer.PublicKey())
		client := dialSSH(t, h.addr, signer, sshd.NeutralUser)
		term := openTerminal(t, client, "")

		conn := h.liveSSHConnection()
		if conn.ActorID != h.actorID || conn.ActorHandle != h.actorHandle {
			t.Errorf("the enrolled session runs as %q/%q, want %q/%q",
				conn.ActorID, conn.ActorHandle, h.actorID, h.actorHandle)
		}
		if conn.TenantID != h.tenantID {
			t.Errorf("the enrolled session runs in tenant %q, want the actor's own %q",
				conn.TenantID, h.tenantID)
		}
		if key := h.tenantKeyOf(conn.TenantID); strings.HasPrefix(key, "sandbox-") {
			t.Errorf("an enrolled session was given a sandbox %q", key)
		}
		if sandboxes := h.sandboxTenants(); len(sandboxes) != 0 {
			t.Errorf("enrolled mode provisioned %v, want no sandbox at all", sandboxes)
		}
		term.quit()
	})

	// The two grants are made in two files and must stay that way. A visitor's
	// scope set is generous because a sandbox holds nobody else's work; an
	// enrolled session's authority is its membership, which is why the
	// enrolled path clears Scopes rather than filling them. Neither fact is
	// observable from a client, so both are asserted by name.
	t.Run("the sandbox grant cannot reach the enrolled path", func(t *testing.T) {
		if named := filesNamingIdentifier(t, "visitorScopes"); !equalStrings(named, []string{"internal/sshd/sandbox.go"}) {
			t.Errorf("visitorScopes is named in %v, want only internal/sshd/sandbox.go", named)
		}
		enrolled := readRepoFile(t, "internal/sshd/enrolled.go")
		if !strings.Contains(enrolled, "found.Scopes = nil") {
			t.Error("the enrolled path no longer clears the actor's scopes; a session's authority " +
				"must be its membership and nothing the key carries")
		}
	})
}

// TestADemoVisitorCannotBeSteeredIntoAnotherSandbox drives the one route
// internal/sshd/isolation_test.go leaves open. That test proves two
// fingerprints get two tenants and cannot read across them; it never sends a
// username. The username IS a tenant selector on the enrolled path, so a demo
// visitor naming somebody else's sandbox is the obvious thing to try.
func TestADemoVisitorCannotBeSteeredIntoAnotherSandbox(t *testing.T) {
	h := newSSHHarness(t, true)

	first := dialSSH(t, h.addr, newSigner(t), sshd.NeutralUser)
	firstTerm := openTerminal(t, first, "")
	firstTerm.waitFor(demoBoardMarker)
	victim := h.liveSSHConnection()
	victimKey := h.tenantKeyOf(victim.TenantID)
	firstTerm.quit()

	if !strings.HasPrefix(victimKey, "sandbox-") {
		t.Fatalf("the first visitor landed in %q, which is not a sandbox", victimKey)
	}

	// Same listener, different key, and a username naming the sandbox that
	// already exists.
	second := dialSSH(t, h.addr, newSigner(t), victimKey)
	secondTerm := openTerminal(t, second, "")
	secondTerm.waitFor(demoBoardMarker)
	intruder := h.liveSSHConnection()
	if intruder.TenantID == victim.TenantID {
		t.Fatalf("naming %q as the ssh username put a second key in the first visitor's sandbox %q",
			victimKey, victim.TenantID)
	}
	if key := h.tenantKeyOf(intruder.TenantID); !strings.HasPrefix(key, "sandbox-") {
		t.Errorf("the second visitor landed in %q, want a sandbox of its own", key)
	}
	secondTerm.quit()

	if got := h.sandboxTenants(); len(got) != 2 {
		t.Errorf("two keys produced %d sandboxes (%v), want 2", len(got), got)
	}
}

// listenAndRecord starts a TCP listener that reports every connection it
// accepts, and returns its address.
func listenAndRecord(t *testing.T, reached chan<- struct{}) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting the forward target: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			select {
			case reached <- struct{}{}:
			default:
			}
			_ = conn.Close()
		}
	}()
	return ln.Addr().String()
}

// sandboxTenants lists the sandbox tenant keys this listener provisioned.
func (h *sshHarness) sandboxTenants() []string {
	h.t.Helper()
	ctx := context.Background()
	var keys []string
	page := core.Page{Limit: core.MaxPageLimit, Sort: "created_at", Direction: core.Ascending}
	err := h.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		tenants, err := u.ListTenants(ctx, page)
		if err != nil {
			return err
		}
		for _, t := range tenants {
			if strings.HasPrefix(t.Key, "sandbox-") && t.DeletedAt == nil {
				keys = append(keys, t.Key)
			}
		}
		return nil
	})
	if err != nil {
		h.t.Fatalf("listing tenants: %v", err)
	}
	sort.Strings(keys)
	return keys
}

// filesNamingIdentifier returns every non-test Go file in the repository whose
// text contains the identifier, relative to the repository root and sorted.
func filesNamingIdentifier(t *testing.T, identifier string) []string {
	t.Helper()
	root := repoRoot(t)
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "dist", "bin", "openspec", "docs":
				return filepath.SkipDir
			}
			// Any dot-directory, not just .git: a nested checkout under
			// .claude/worktrees/ holds a second copy of every file counted here.
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path) // #nosec G304 -- the walk names every file it reads
		if err != nil {
			return err
		}
		if !strings.Contains(string(body), identifier) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	sort.Strings(found)
	return found
}

// readRepoFile returns a repository file's text by its path from the root.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel))) // #nosec G304 -- the caller names a repository file
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(body)
}

// repoRoot locates the module root from this package.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("%q is not the repository root: %v", dir, err)
	}
	return dir
}

// equalStrings compares two sorted string slices.
func equalStrings(got, want []string) bool {
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
