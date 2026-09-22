package sshd

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	gossh "golang.org/x/crypto/ssh"
)

// enrolment is one key registered against one actor of one tenant.
type enrolment struct {
	tenant core.Tenant
	actor  core.Actor
	key    core.SSHKey
}

// newEnrolled returns the enrolled lookup over a real temporary database.
func newEnrolled(t *testing.T) (*enrolled, *clock.Fake, store.Store) {
	t.Helper()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "hosted.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return &enrolled{store: st, clk: clk}, clk, st
}

// enrol registers a fingerprint against a fresh actor of a tenant, creating
// the tenant when it is new.
func enrol(t *testing.T, st store.Store, tenantKey, handle string, role core.Role, fingerprint string) enrolment {
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

	scope := core.TenantScope{TenantID: tenant.ID}
	actor := &core.Actor{TenantID: tenant.ID, Kind: core.ActorUser, Handle: handle, DisplayName: handle}
	key := &core.SSHKey{
		ActorID:     "",
		Fingerprint: fingerprint,
		PublicKey:   "ssh-ed25519 AAAA" + fingerprint,
		Label:       handle,
	}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.CreateActor(ctx, actor); err != nil {
			return err
		}
		if role != "" {
			if err := tx.AddMember(ctx, &core.Membership{
				TenantID: tenant.ID, ActorID: actor.ID, Role: role,
			}); err != nil {
				return err
			}
		}
		key.ActorID = actor.ID
		return tx.CreateSSHKey(ctx, key)
	}); err != nil {
		t.Fatalf("enrolling %q in %q: %v", fingerprint, tenantKey, err)
	}
	return enrolment{tenant: *tenant, actor: *actor, key: *key}
}

// asUser returns a context carrying the username an ssh client offered.
func asUser(name string) context.Context {
	return context.WithValue(context.Background(), ssh.ContextKeyUser, name)
}

// revoke stops a key authenticating.
func revoke(t *testing.T, st store.Store, e enrolment, at time.Time) {
	t.Helper()
	ctx := context.Background()
	scope := core.TenantScope{TenantID: e.tenant.ID}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		return tx.RevokeSSHKey(ctx, e.key.ID, at)
	}); err != nil {
		t.Fatalf("revoking %q: %v", e.key.ID, err)
	}
}

func TestEnrolledInOneTenantNeedsNoUsername(t *testing.T) {
	e, _, st := newEnrolled(t)
	acme := enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")

	for _, username := range []string{NeutralUser, "", "acme"} {
		actor, err := e.ActorByFingerprint(asUser(username), "SHA256:alice")
		if err != nil {
			t.Fatalf("username %q: %v", username, err)
		}
		if actor.ID != acme.actor.ID || actor.TenantID != acme.tenant.ID {
			t.Fatalf("username %q resolved to %+v, want actor %q of tenant %q",
				username, actor, acme.actor.ID, acme.tenant.ID)
		}
		if actor.Role != core.RoleMember {
			t.Fatalf("username %q gave role %q, want the membership's own", username, actor.Role)
		}
	}
}

func TestTheUsernameSelectsAmongSeveralTenants(t *testing.T) {
	e, _, st := newEnrolled(t)
	acme := enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")
	beta := enrol(t, st, "beta", "alice", core.RoleAdmin, "SHA256:alice")

	for _, tc := range []struct {
		username string
		want     enrolment
		role     core.Role
	}{
		{"acme", acme, core.RoleMember},
		{"beta", beta, core.RoleAdmin},
	} {
		actor, err := e.ActorByFingerprint(asUser(tc.username), "SHA256:alice")
		if err != nil {
			t.Fatalf("username %q: %v", tc.username, err)
		}
		if actor.TenantID != tc.want.tenant.ID || actor.ID != tc.want.actor.ID {
			t.Fatalf("username %q resolved into tenant %q, want %q",
				tc.username, actor.TenantID, tc.want.tenant.ID)
		}
		if actor.Role != tc.role {
			t.Fatalf("username %q gave role %q, want %q", tc.username, actor.Role, tc.role)
		}
	}
}

// TestAmbiguityIsRefusedNamingOnlyTheCallersOwnTenants covers both halves of
// step 3: no tenant is chosen on the holder's behalf, and the message that
// says so names the tenants this very key is enrolled in and no others.
func TestAmbiguityIsRefusedNamingOnlyTheCallersOwnTenants(t *testing.T) {
	e, _, st := newEnrolled(t)
	enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")
	enrol(t, st, "beta", "alice", core.RoleAdmin, "SHA256:alice")
	// A third tenant this key has nothing to do with. It must not be named.
	enrol(t, st, "secret-client", "bob", core.RoleAdmin, "SHA256:bob")

	actor, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:alice")
	if err == nil {
		t.Fatalf("an ambiguous fingerprint opened a session in %q", actor.TenantID)
	}
	if !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("ambiguity gave %v, want an unauthenticated refusal", err)
	}
	msg := err.Error()
	for _, want := range []string{"acme", "beta"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("the ambiguity message %q does not name %q", msg, want)
		}
	}
	if strings.Contains(msg, "secret-client") {
		t.Fatalf("the ambiguity message %q names a tenant this key is not enrolled in", msg)
	}
}

func TestAnUnenrolledKeyIsRefused(t *testing.T) {
	e, _, st := newEnrolled(t)
	enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")

	if _, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:stranger"); err == nil {
		t.Fatal("an unenrolled fingerprint authenticated")
	} else if !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("refusal = %v, want unauthenticated", err)
	}
}

// TestAnUnenrolledKeyCannotTellAnExistingTenantFromAnAbsentOne is the
// disclosure requirement. A stranger must not be able to use the listener as a
// directory of who is hosted here.
func TestAnUnenrolledKeyCannotTellAnExistingTenantFromAnAbsentOne(t *testing.T) {
	e, _, st := newEnrolled(t)
	enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")

	// A key enrolled nowhere at all.
	_, real := e.ActorByFingerprint(asUser("acme"), "SHA256:stranger")
	_, absent := e.ActorByFingerprint(asUser("no-such-tenant"), "SHA256:stranger")
	_, neutral := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:stranger")
	if real == nil || absent == nil || neutral == nil {
		t.Fatalf("a stranger authenticated: %v / %v / %v", real, absent, neutral)
	}
	if real.Error() != absent.Error() || real.Error() != neutral.Error() {
		t.Fatalf("refusals differ and so disclose what exists:\n existing: %q\n absent:   %q\n neutral:  %q",
			real, absent, neutral)
	}

	// A key enrolled somewhere else, which is the case that actually reaches
	// the tenant lookup: naming a tenant that exists and naming one that does
	// not must still be the same answer, and the same answer a stranger gets.
	enrol(t, st, "beta", "bob", core.RoleAdmin, "SHA256:bob")
	_, elsewhere := e.ActorByFingerprint(asUser("acme"), "SHA256:bob")
	_, nowhere := e.ActorByFingerprint(asUser("no-such-tenant"), "SHA256:bob")
	if elsewhere == nil || nowhere == nil {
		t.Fatalf("a key enrolled elsewhere authenticated: %v / %v", elsewhere, nowhere)
	}
	if elsewhere.Error() != nowhere.Error() || elsewhere.Error() != real.Error() {
		t.Fatalf("refusals differ and so disclose what exists:\n wrong tenant: %q\n absent:       %q\n stranger:     %q",
			elsewhere, nowhere, real)
	}
}

func TestARevokedKeyIsRefused(t *testing.T) {
	e, clk, st := newEnrolled(t)
	acme := enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")

	if _, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:alice"); err != nil {
		t.Fatalf("before revocation: %v", err)
	}
	revoke(t, st, acme, clk.Now())
	if _, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:alice"); err == nil {
		t.Fatal("a revoked key authenticated")
	}
	// Naming the tenant must not route around revocation either.
	if _, err := e.ActorByFingerprint(asUser("acme"), "SHA256:alice"); err == nil {
		t.Fatal("a revoked key authenticated when its tenant was named")
	}
}

func TestAUsernameNamingATenantTheKeyIsNotEnrolledInIsRefused(t *testing.T) {
	e, _, st := newEnrolled(t)
	enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")
	enrol(t, st, "beta", "bob", core.RoleAdmin, "SHA256:bob")

	actor, err := e.ActorByFingerprint(asUser("beta"), "SHA256:alice")
	if err == nil {
		t.Fatalf("alice's key opened a session in %q", actor.TenantID)
	}
	if !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("refusal = %v, want unauthenticated", err)
	}
}

func TestSuccessfulAuthenticationRecordsTheLastUse(t *testing.T) {
	e, clk, st := newEnrolled(t)
	acme := enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")

	clk.Advance(time.Hour)
	if _, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:alice"); err != nil {
		t.Fatalf("ActorByFingerprint: %v", err)
	}

	ctx := context.Background()
	scope := core.TenantScope{TenantID: acme.tenant.ID}
	var got *core.SSHKey
	if err := st.View(ctx, scope, func(tx store.Tx) error {
		var err error
		got, err = tx.GetSSHKey(ctx, acme.key.ID)
		return err
	}); err != nil {
		t.Fatalf("GetSSHKey: %v", err)
	}
	if got.LastUsedAt == nil || !got.LastUsedAt.Equal(clk.Now()) {
		t.Fatalf("last_used_at = %v, want %v", got.LastUsedAt, clk.Now())
	}
}

// TestRecordingUseCannotFailTheConnection closes the store before the touch
// can run, which is the bluntest possible failure of that write. The
// authentication decision was already made, and it must stand.
func TestRecordingUseCannotFailTheConnection(t *testing.T) {
	e, _, st := newEnrolled(t)
	enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")

	broken := &failingTouchStore{Store: st}
	e.store = broken
	actor, err := e.ActorByFingerprint(asUser(NeutralUser), "SHA256:alice")
	if err != nil {
		t.Fatalf("a failed touch refused the connection: %v", err)
	}
	if actor == nil || actor.TenantID == "" {
		t.Fatalf("actor = %+v, want the session's own tenant", actor)
	}
	if !broken.attempted {
		t.Fatal("the last use was never recorded at all, so this proves nothing")
	}
}

// failingTouchStore fails every write, leaving reads alone.
type failingTouchStore struct {
	store.Store
	attempted bool
}

func (s *failingTouchStore) Update(context.Context, core.TenantScope, func(store.Tx) error) error {
	s.attempted = true
	return core.Internal("the database is gone")
}

// TestEveryResolutionNarrowsToOneTenant asserts the invariant the whole design
// rests on: whatever the lookup returns carries a tenant, and whatever it
// refuses returns no actor at all.
func TestEveryResolutionNarrowsToOneTenant(t *testing.T) {
	e, _, st := newEnrolled(t)
	enrol(t, st, "acme", "alice", core.RoleMember, "SHA256:alice")
	enrol(t, st, "beta", "alice", core.RoleAdmin, "SHA256:alice")
	enrol(t, st, "acme", "carol", core.RoleViewer, "SHA256:carol")

	for _, tc := range []struct {
		name        string
		username    string
		fingerprint string
	}{
		{"one tenant, neutral", NeutralUser, "SHA256:carol"},
		{"one tenant, named", "acme", "SHA256:carol"},
		{"several, named", "beta", "SHA256:alice"},
		{"several, ambiguous", NeutralUser, "SHA256:alice"},
		{"wrong tenant", "beta", "SHA256:carol"},
		{"unenrolled", NeutralUser, "SHA256:nobody"},
		{"no fingerprint at all", NeutralUser, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor, err := e.ActorByFingerprint(asUser(tc.username), tc.fingerprint)
			if err != nil {
				if actor != nil {
					t.Fatalf("a refusal returned an actor: %+v", actor)
				}
				return
			}
			if actor.TenantID == "" {
				t.Fatalf("actor %+v runs without a tenant scope", actor)
			}
			if len(actor.Scopes) != 0 {
				t.Fatalf("actor %+v carries scopes of its own; a key grants nothing", actor)
			}
		})
	}
}

// newEnrolledListener binds a listener serving enrolled keys only.
func newEnrolledListener(t *testing.T) (string, store.Store) {
	t.Helper()
	p, _, st := newProvisioner(t)
	srv, err := New(Options{
		Service:     p.service,
		Store:       st,
		Clock:       clock.New(),
		Addr:        "127.0.0.1:0",
		HostKeyPath: filepath.Join(t.TempDir(), "host_key"),
		IdleTimeout: time.Hour,
		Logger:      slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Serve(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return srv.Addr(), st
}

// newSigner returns a fresh key and the fingerprint an ssh client prints for it.
func newSigner(t *testing.T) (gossh.Signer, string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return signer, gossh.FingerprintSHA256(signer.PublicKey())
}

// TestOnlyAPublicKeyAuthenticates proves the listener offers no other
// credential: a password, a keyboard-interactive exchange and an
// authentication carrying nothing are all refused at the handshake.
func TestOnlyAPublicKeyAuthenticates(t *testing.T) {
	addr, st := newEnrolledListener(t)
	signer, fingerprint := newSigner(t)
	enrolSigner(t, st, "acme", "alice", fingerprint)

	tests := []struct {
		name string
		auth []gossh.AuthMethod
	}{
		{"a password", []gossh.AuthMethod{gossh.Password("hunter2")}},
		{"keyboard-interactive", []gossh.AuthMethod{gossh.KeyboardInteractive(
			func(string, string, []string, []bool) ([]string, error) {
				return []string{"hunter2"}, nil
			})}},
		{"nothing at all", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
				User:            "acme",
				Auth:            tc.auth,
				HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
				Timeout:         10 * time.Second,
			})
			if err == nil {
				_ = client.Close()
				t.Fatalf("%s authenticated a session", tc.name)
			}
		})
	}

	// The same listener accepts the enrolled key, so the refusals above are
	// the credential being refused rather than the listener being broken.
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            "acme",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("the enrolled key was refused: %v", err)
	}
	_ = client.Close()
}

// TestTheListenerRefusesAnUnenrolledKeyAndServesAnEnrolledOne drives the whole
// path, from a real handshake to the message the client is left holding.
func TestTheListenerRefusesAnUnenrolledKeyAndServesAnEnrolledOne(t *testing.T) {
	addr, st := newEnrolledListener(t)
	known, fingerprint := newSigner(t)
	stranger, _ := newSigner(t)
	enrolSigner(t, st, "acme", "alice", fingerprint)

	if stderr := runSSHSession(t, addr, "acme", stranger); !strings.Contains(stderr, "not enrolled") {
		t.Fatalf("an unenrolled key was told %q, want the refusal", stderr)
	}
	if stderr := runSSHSession(t, addr, "acme", known); strings.Contains(stderr, "not enrolled") {
		t.Fatalf("an enrolled key was refused: %q", stderr)
	}
}

// enrolSigner registers a real fingerprint against a fresh actor.
func enrolSigner(t *testing.T, st store.Store, tenantKey, handle, fingerprint string) enrolment {
	t.Helper()
	return enrol(t, st, tenantKey, handle, core.RoleAdmin, fingerprint)
}

// runSSHSession opens a session, quits it, and returns whatever the listener
// wrote to the client's standard error.
func runSSHSession(t *testing.T, addr, user string, signer gossh.Signer) string {
	t.Helper()
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = client.Close() }()

	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer func() { _ = sess.Close() }()

	stderr, err := sess.StderrPipe()
	if err != nil {
		t.Fatalf("stderr: %v", err)
	}
	var collected syncBuffer
	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	if err := sess.RequestPty("xterm-256color", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Fatalf("pty: %v", err)
	}
	if err := sess.Shell(); err != nil {
		t.Fatalf("shell: %v", err)
	}

	// Both streams are drained and both are waited for. A refused connection
	// produces no stdout at all, so waiting only on stdout returns before the
	// refusal has been copied off the wire and reports an empty message.
	var streams sync.WaitGroup
	streams.Add(2)
	go func() {
		defer streams.Done()
		_, _ = io.Copy(io.Discard, stdout)
	}()
	go func() {
		defer streams.Done()
		_, _ = io.Copy(&collected, stderr)
	}()
	drained := make(chan struct{})
	go func() {
		streams.Wait()
		close(drained)
	}()
	go func() {
		for i := 0; i < 40; i++ {
			if _, err := stdin.Write([]byte("q")); err != nil {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()
	select {
	case <-drained:
	case <-time.After(20 * time.Second):
		t.Fatal("the session did not end")
	}
	return collected.String()
}

// syncBuffer collects a session's stderr safely.
//
// x/crypto/ssh copies stderr on its own goroutine for as long as the channel
// is open, and the caller reads once stdout has drained. Those are different
// streams, so the read and the copy overlap: handing the library a plain
// strings.Builder is a data race that -race catches and a plain run does not.
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
