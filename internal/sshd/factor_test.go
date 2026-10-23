// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	gossh "golang.org/x/crypto/ssh"
)

// factorTestKey is the sealing key the listener under test is configured with.
// It is fixed so that a test asserting enrolment works is asserting this value
// and not a failure to generate one.
const factorTestKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

// boardMarker is the project title a session reaches only once the second
// factor is proved. It is the tenant's own data, so its appearance on the
// terminal is exactly what the prompt has to come before.
const boardMarker = "tenant-only-board"

// person is one enrolled human: a global user row, the actor it owns in one
// tenant, and the key that speaks for it.
type person struct {
	tenantID string
	userID   string
	actor    *core.Actor
	signer   gossh.Signer
}

// watchedService records the order in which a session reaches the service, so
// a test can assert that nothing belonging to the tenant was read before the
// code was accepted. Asserting the absence of the board on the terminal alone
// would not catch a read whose result was simply never painted.
type watchedService struct {
	core.Service

	mu    sync.Mutex
	calls []string
}

func (w *watchedService) record(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, name)
}

// seen returns the calls recorded so far.
func (w *watchedService) seen() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.calls...)
}

// boardReadBefore reports whether any read of the tenant's work happened
// before the named call, and whether the named call happened at all.
func (w *watchedService) boardReadBefore(call string) (bool, bool) {
	for _, seen := range w.seen() {
		switch seen {
		case call:
			return false, true
		case "ListProjects", "ListTasks", "GetTask", "ListWorkflows":
			return true, false
		}
	}
	return false, false
}

func (w *watchedService) ListProjects(ctx context.Context, f core.ProjectFilter) ([]core.Project, string, error) {
	w.record("ListProjects")
	return w.Service.ListProjects(ctx, f)
}

func (w *watchedService) ListTasks(ctx context.Context, f core.TaskFilter) (core.TaskPage, error) {
	w.record("ListTasks")
	return w.Service.ListTasks(ctx, f)
}

func (w *watchedService) ListWorkflows(ctx context.Context) ([]core.Workflow, error) {
	w.record("ListWorkflows")
	return w.Service.ListWorkflows(ctx)
}

func (w *watchedService) VerifySecondFactor(ctx context.Context, code string) error {
	w.record("VerifySecondFactor")
	return w.Service.VerifySecondFactor(ctx, code)
}

func (w *watchedService) BeginTOTPEnrolment(ctx context.Context) (*core.TOTPEnrolment, error) {
	w.record("BeginTOTPEnrolment")
	return w.Service.BeginTOTPEnrolment(ctx)
}

func (w *watchedService) ConfirmTOTPEnrolment(ctx context.Context, code string) (*core.TOTPRecoveryCodes, error) {
	w.record("ConfirmTOTPEnrolment")
	return w.Service.ConfirmTOTPEnrolment(ctx, code)
}

// factorListener binds a listener serving enrolled keys over a service that
// can seal a second factor, and returns everything a test drives it with.
func factorListener(t *testing.T, idle time.Duration) (*Server, *watchedService, store.Store) {
	t.Helper()
	ctx := context.Background()
	clk := clock.New()

	st, err := sqlite.Open(filepath.Join(t.TempDir(), "hosted.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	svc := service.New(st, service.WithClock(clk), service.WithTOTPKey(factorTestKey))
	t.Cleanup(func() { _ = svc.Close() })
	watched := &watchedService{Service: svc}

	srv, err := New(Options{
		Service:     watched,
		Store:       st,
		Clock:       clk,
		Addr:        "127.0.0.1:0",
		HostKeyPath: filepath.Join(t.TempDir(), "host_key"),
		IdleTimeout: idle,
		Logger:      slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Serve(runCtx); close(done) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return srv, watched, st
}

// seedPerson creates the tenant, the user, the actor they share an identifier
// with, the membership that carries the role, and the enrolled key.
//
// The user and the actor are written with one identifier because that is the
// shape the service reads: a second factor hangs off the global user row, and
// the actor behind a key is how a session finds it.
func seedPerson(t *testing.T, st store.Store, tenantKey, handle, email string) person {
	t.Helper()
	ctx := context.Background()

	tenant := &core.Tenant{Key: tenantKey, Name: tenantKey}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		existing, err := u.GetTenantByKey(ctx, tenantKey)
		if err == nil {
			tenant = existing
			return nil
		}
		if !core.IsKind(err, core.KindNotFound) {
			return err
		}
		return u.CreateTenant(ctx, tenant)
	}); err != nil {
		t.Fatalf("creating tenant %q: %v", tenantKey, err)
	}

	signer, fingerprint := newSigner(t)
	scope := core.TenantScope{TenantID: tenant.ID}
	actor := &core.Actor{TenantID: tenant.ID, Kind: core.ActorUser, Handle: handle, DisplayName: handle}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.CreateActor(ctx, actor); err != nil {
			return err
		}
		if err := tx.CreateUser(ctx, &core.User{ID: actor.ID, Email: email}, "x"); err != nil {
			return err
		}
		if err := tx.AddMember(ctx, &core.Membership{
			TenantID: tenant.ID, ActorID: actor.ID, Role: core.RoleAdmin,
		}); err != nil {
			return err
		}
		return tx.CreateSSHKey(ctx, &core.SSHKey{
			ActorID:     actor.ID,
			Fingerprint: fingerprint,
			PublicKey:   "ssh-ed25519 AAAA" + fingerprint,
			Label:       handle,
		})
	}); err != nil {
		t.Fatalf("seeding %q in %q: %v", handle, tenantKey, err)
	}
	actor.Role = core.RoleAdmin
	return person{tenantID: tenant.ID, userID: actor.ID, actor: actor, signer: signer}
}

// seedBoard gives the tenant its default state machine and one project, which
// is the data a session must not see before it has proved a factor.
func seedBoard(t *testing.T, st store.Store, svc core.Service, p person) {
	t.Helper()
	ctx := context.Background()
	scope := core.TenantScope{TenantID: p.tenantID}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		return tx.PutWorkflow(ctx, &core.Workflow{
			Key: builtinWorkflowKey, Name: "Default", Builtin: true,
			Definition: demoWorkflow(DefaultLeaseTTL),
		})
	}); err != nil {
		t.Fatalf("seeding the workflow: %v", err)
	}
	if _, err := svc.CreateProject(sessionContext(p.actor), core.CreateProjectInput{
		Key: "tob", Name: boardMarker,
	}); err != nil {
		t.Fatalf("seeding a board: %v", err)
	}
}

// requireFactor turns the SSH requirement on for one tenant.
func requireFactor(t *testing.T, st store.Store, tenantID string, surface core.Surface) {
	t.Helper()
	ctx := context.Background()
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		tenant, err := u.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		switch surface {
		case core.SurfaceWeb:
			tenant.RequireTOTPWeb = true
		case core.SurfaceCLI:
			tenant.RequireTOTPCLI = true
		case core.SurfaceSSH:
			tenant.RequireTOTPSSH = true
		}
		return u.UpdateTenant(ctx, tenant)
	}); err != nil {
		t.Fatalf("requiring a factor on %s: %v", surface, err)
	}
}

// enrolFactor takes a person through enrolment and returns the secret their
// authenticator holds and the recovery codes issued once.
func enrolFactor(t *testing.T, svc core.Service, p person) (string, []string) {
	t.Helper()
	ctx := sessionContext(p.actor)
	enrolment, err := svc.BeginTOTPEnrolment(ctx)
	if err != nil {
		t.Fatalf("beginning enrolment: %v", err)
	}
	// The enrolment is confirmed with the previous step's code, which the
	// window accepts and which leaves the current step above the replay
	// high-water mark. Confirming with the current one would make every code
	// the test then types a replay, and the test would be asserting the
	// replay guard rather than the prompt.
	codes, err := svc.ConfirmTOTPEnrolment(ctx, totpAt(t, enrolment.Secret, -1))
	if err != nil {
		t.Fatalf("confirming enrolment: %v", err)
	}
	return enrolment.Secret, codes.Codes
}

// totpNow produces the code an authenticator holding secret shows now.
func totpNow(t *testing.T, secret string) string {
	t.Helper()
	return totpAt(t, secret, 0)
}

// totpAt produces the code for a step relative to the current one.
func totpAt(t *testing.T, secret string, offset int64) string {
	t.Helper()
	code, err := auth.TOTPCode(secret, auth.TOTPStep(time.Now())+offset)
	if err != nil {
		t.Fatalf("producing a code: %v", err)
	}
	return code
}

// promptSession is one live session a test types at.
type promptSession struct {
	stdin  io.WriteCloser
	stdout *syncBuffer
	stderr *syncBuffer
	closed chan struct{}
}

// openPromptSession authenticates, asks for a pseudo-terminal and starts the
// interface, leaving both streams collected and standard input open.
func openPromptSession(t *testing.T, addr, user string, signer gossh.Signer) *promptSession {
	t.Helper()
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            user,
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
	t.Cleanup(func() { _ = sess.Close() })

	out := &promptSession{stdout: &syncBuffer{}, stderr: &syncBuffer{}, closed: make(chan struct{})}
	if out.stdin, err = sess.StdinPipe(); err != nil {
		t.Fatalf("stdin: %v", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		t.Fatalf("stderr: %v", err)
	}
	if err := sess.RequestPty("xterm-256color", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Fatalf("pty: %v", err)
	}
	if err := sess.Shell(); err != nil {
		t.Fatalf("shell: %v", err)
	}

	var streams sync.WaitGroup
	streams.Add(2)
	go func() { defer streams.Done(); _, _ = io.Copy(out.stdout, stdout) }()
	go func() { defer streams.Done(); _, _ = io.Copy(out.stderr, stderr) }()
	go func() { streams.Wait(); close(out.closed) }()
	return out
}

// waitFor blocks until want appears on either stream.
func (p *promptSession) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if strings.Contains(p.text(), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%q never arrived; the session said %q", want, p.text())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitClosed blocks until the listener has let the session go.
func (p *promptSession) waitClosed(t *testing.T) {
	t.Helper()
	select {
	case <-p.closed:
	case <-time.After(30 * time.Second):
		t.Fatalf("the session is still open; it said %q", p.text())
	}
}

// answer types one line at the prompt.
func (p *promptSession) answer(t *testing.T, line string) {
	t.Helper()
	if _, err := p.stdin.Write([]byte(line + "\r")); err != nil {
		t.Fatalf("typing %q: %v", line, err)
	}
}

// text is everything the session has written, both streams together.
func (p *promptSession) text() string { return p.stdout.String() + p.stderr.String() }

// TestTheCodeIsAskedForBeforeAnyTenantDataIsDrawn is the requirement this
// whole surface exists for. A key is a single factor; a prompt drawn after the
// board is on screen is not a second one.
func TestTheCodeIsAskedForBeforeAnyTenantDataIsDrawn(t *testing.T) {
	srv, watched, st := factorListener(t, time.Hour)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	secret, _ := enrolFactor(t, watched.Service, alice)
	requireFactor(t, st, alice.tenantID, core.SurfaceSSH)

	sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
	sess.waitFor(t, "second factor")

	// Both halves, because either alone passes while the other is broken: the
	// board is not on the terminal, and the service was never asked for it.
	if strings.Contains(sess.text(), boardMarker) {
		t.Fatalf("the board was drawn before the code was asked for: %q", sess.text())
	}
	if before, _ := watched.boardReadBefore("VerifySecondFactor"); before {
		t.Fatalf("the tenant's work was read before the prompt: %v", watched.seen())
	}

	sess.answer(t, totpNow(t, secret))
	sess.waitFor(t, boardMarker)

	before, verified := watched.boardReadBefore("VerifySecondFactor")
	if !verified {
		t.Fatalf("the board opened without the code being verified at all: %v", watched.seen())
	}
	if before {
		t.Fatalf("the tenant's work was read before the code was accepted: %v", watched.seen())
	}
}

// TestAWrongCodeEndsTheSessionWithNothingDrawn covers the refusal: the attempts
// run out, the session closes, and no tenant data was ever read.
func TestAWrongCodeEndsTheSessionWithNothingDrawn(t *testing.T) {
	srv, watched, st := factorListener(t, time.Hour)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	enrolFactor(t, watched.Service, alice)
	requireFactor(t, st, alice.tenantID, core.SurfaceSSH)
	watched.calls = nil

	sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
	for i := 0; i < maxCodeAttempts; i++ {
		sess.waitFor(t, "second factor")
		sess.answer(t, "000000")
	}
	sess.waitClosed(t)

	if strings.Contains(sess.text(), boardMarker) {
		t.Fatalf("a refused session saw the board: %q", sess.text())
	}
	for _, call := range watched.seen() {
		switch call {
		case "ListProjects", "ListTasks", "ListWorkflows", "GetTask":
			t.Fatalf("a refused session read the tenant's work: %v", watched.seen())
		}
	}
}

// TestTheRequirementAndAVoluntaryEnrolmentBothDrawThePrompt pins the rule the
// service already made: a factor is owed when the tenant asks for one here,
// and also when the person holds one of their own, since an enrolment that
// protected no sign-in would protect nothing.
func TestTheRequirementAndAVoluntaryEnrolmentBothDrawThePrompt(t *testing.T) {
	tests := []struct {
		name     string
		enrolled bool
		require  core.Surface
		prompt   bool
	}{
		{"required here, enrolled", true, core.SurfaceSSH, true},
		{"required nowhere, enrolled anyway", true, "", true},
		{"required on the browser only, not enrolled", false, core.SurfaceWeb, false},
		{"required nowhere, not enrolled", false, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, watched, st := factorListener(t, time.Hour)
			alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
			seedBoard(t, st, watched.Service, alice)
			var secret string
			if tc.enrolled {
				secret, _ = enrolFactor(t, watched.Service, alice)
			}
			if tc.require != "" {
				requireFactor(t, st, alice.tenantID, tc.require)
			}

			sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
			if !tc.prompt {
				sess.waitFor(t, boardMarker)
				if strings.Contains(sess.text(), "second factor") {
					t.Fatalf("a session owing nothing was prompted: %q", sess.text())
				}
				return
			}
			sess.waitFor(t, "second factor")
			if strings.Contains(sess.text(), boardMarker) {
				t.Fatalf("the board was drawn before the prompt: %q", sess.text())
			}
			sess.answer(t, totpNow(t, secret))
			sess.waitFor(t, boardMarker)
		})
	}
}

// TestARecoveryCodeIsAcceptedAtThePrompt keeps the escape from a lost phone
// working on the one surface that has no other way in.
func TestARecoveryCodeIsAcceptedAtThePrompt(t *testing.T) {
	srv, watched, st := factorListener(t, time.Hour)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	_, codes := enrolFactor(t, watched.Service, alice)
	requireFactor(t, st, alice.tenantID, core.SurfaceSSH)

	sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
	sess.waitFor(t, "second factor")
	sess.answer(t, codes[0])
	sess.waitFor(t, boardMarker)
}

// TestALockedVerifierIsToldToWait keeps the distinction the design asks for.
// Telling somebody their code is wrong when the real answer is that they are
// rate limited sends them to look for a clock problem they do not have.
func TestALockedVerifierIsToldToWait(t *testing.T) {
	srv, watched, st := factorListener(t, time.Hour)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	enrolFactor(t, watched.Service, alice)
	requireFactor(t, st, alice.tenantID, core.SurfaceSSH)

	ctx := sessionContext(alice.actor)
	for i := 0; i < auth.TOTPMaxFailures; i++ {
		if err := watched.Service.VerifySecondFactor(ctx, "000000"); err == nil {
			t.Fatalf("attempt %d: a wrong code was accepted", i+1)
		}
	}

	sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
	sess.waitClosed(t)
	if !strings.Contains(sess.text(), "locked until") {
		t.Fatalf("a locked verifier said %q, want the wait", sess.text())
	}
	if strings.Contains(sess.text(), "was not accepted") {
		t.Fatalf("a locked verifier was reported as a wrong code: %q", sess.text())
	}
	if strings.Contains(sess.text(), "Code from your authenticator") {
		t.Fatalf("a locked verifier was asked for a code it cannot verify: %q", sess.text())
	}
	if strings.Contains(sess.text(), boardMarker) {
		t.Fatalf("a locked session saw the board: %q", sess.text())
	}
}

// TestAnUnenrolledPersonIsTakenThroughEnrolment is what keeps turning the flag
// on from locking out the tenant that turned it on, administrator first.
func TestAnUnenrolledPersonIsTakenThroughEnrolment(t *testing.T) {
	srv, watched, st := factorListener(t, time.Hour)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	requireFactor(t, st, alice.tenantID, core.SurfaceSSH)

	sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
	sess.waitFor(t, "otpauth://totp/")
	if strings.Contains(sess.text(), boardMarker) {
		t.Fatalf("enrolment was offered after the board: %q", sess.text())
	}

	secret := secretFromURI(t, sess.text())
	sess.answer(t, totpNow(t, secret))

	// The codes are shown exactly once and nothing can produce them again, so
	// a terminal that did not print them has lost them.
	sess.waitFor(t, "recovery codes")
	status, err := watched.TOTPStatus(sessionContext(alice.actor))
	if err != nil {
		t.Fatalf("reading status: %v", err)
	}
	if !status.Enrolled {
		t.Fatal("the enrolment never confirmed")
	}
	if printed := countRecoveryCodes(sess.text()); printed != status.RecoveryCodesLeft {
		t.Fatalf("the terminal printed %d recovery codes, want the %d that were issued",
			printed, status.RecoveryCodesLeft)
	}
	sess.answer(t, "")
	sess.waitFor(t, boardMarker)
}

// secretFromURI pulls the shared secret out of the enrolment URI the terminal
// printed, which is what an authenticator would read from it.
func secretFromURI(t *testing.T, screen string) string {
	t.Helper()
	_, after, ok := strings.Cut(screen, "secret=")
	if !ok {
		t.Fatalf("no secret in the enrolment the terminal printed: %q", screen)
	}
	secret := after
	for i, r := range after {
		if r == '&' || r == '\r' || r == '\n' || r == ' ' {
			secret = after[:i]
			break
		}
	}
	if secret == "" {
		t.Fatalf("the enrolment URI carries an empty secret: %q", screen)
	}
	return secret
}

// countRecoveryCodes counts the codes on screen by the marker each is printed
// with, so a count cannot be satisfied by any other text in the block.
func countRecoveryCodes(screen string) int {
	return strings.Count(screen, recoveryCodeMarker)
}

// TestAnUnenrolledKeyIsRefusedBeforeAnyPrompt holds the ordering the spec is
// explicit about: the key is proved first, so a key nobody enrolled learns
// nothing, not even that this tenant asks for a second factor.
func TestAnUnenrolledKeyIsRefusedBeforeAnyPrompt(t *testing.T) {
	srv, watched, st := factorListener(t, time.Hour)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	enrolFactor(t, watched.Service, alice)
	requireFactor(t, st, alice.tenantID, core.SurfaceSSH)

	stranger, _ := newSigner(t)
	sess := openPromptSession(t, srv.Addr(), "acme", stranger)
	sess.waitFor(t, "not enrolled")
	sess.waitClosed(t)
	if strings.Contains(sess.text(), "second factor") {
		t.Fatalf("a stranger was shown the code prompt: %q", sess.text())
	}
}

// TestAnUnansweredPromptCannotHoldTheConnection closes the hole the placement
// opens. The watchdog that bounds an idle session is started with the
// interface, and the interface does not exist yet, so an unanswered prompt
// would otherwise hold a session slot and a key's capacity for as long as the
// client cared to wait.
func TestAnUnansweredPromptCannotHoldTheConnection(t *testing.T) {
	srv, watched, st := factorListener(t, 2*time.Second)
	alice := seedPerson(t, st, "acme", "alice", "alice@example.com")
	seedBoard(t, st, watched.Service, alice)
	enrolFactor(t, watched.Service, alice)
	requireFactor(t, st, alice.tenantID, core.SurfaceSSH)

	sess := openPromptSession(t, srv.Addr(), "acme", alice.signer)
	sess.waitFor(t, "second factor")
	sess.waitClosed(t)

	fingerprint := fingerprintOf(alice.signer)
	deadline := time.Now().Add(30 * time.Second)
	for {
		perKey, live := srv.live.counts(fingerprint)
		if perKey == 0 && live == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("an abandoned prompt still holds %d of this key's slots and %d of the listener's",
				perKey, live)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
