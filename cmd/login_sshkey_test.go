// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/migrations"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// loginKey is a throwaway ed25519 key: the private half for the client, the
// public half for enrolment.
type loginKey struct {
	priv ed25519.PrivateKey
	pub  string
}

func newLoginKey(t *testing.T) loginKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("wrapping the key: %v", err)
	}
	return loginKey{priv: priv, pub: string(gossh.MarshalAuthorizedKey(signer.PublicKey()))}
}

// write puts the private key in OpenSSH form at path, encrypted when a
// passphrase is given.
func (k loginKey) write(t *testing.T, path, passphrase string) {
	t.Helper()
	var (
		block *pem.Block
		err   error
	)
	if passphrase == "" {
		block, err = gossh.MarshalPrivateKey(k.priv, "")
	} else {
		block, err = gossh.MarshalPrivateKeyWithPassphrase(k.priv, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatalf("marshalling the key: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("making the key directory: %v", err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("writing the key: %v", err)
	}
}

// enrolLoginKey creates ada and enrols the key against her, through the CLI
// on the test's own database.
func (c *cli) enrolLoginKey(k loginKey) {
	c.t.Helper()
	c.mustRun("user", "create", "ada@example.com", "--password", "correct horse battery", "--handle", "ada")
	c.mustRun("user", "key", "add", "--file", c.writeKeyFile("ada.pub", strings.TrimSpace(k.pub)), "--actor", "ada")
}

// loginServer serves the API over the test's database in this process and
// returns its address.
func (c *cli) loginServer() string {
	c.t.Helper()
	ctx := context.Background()
	st, err := sqlite.Open(c.dbPath(), clock.New())
	if err != nil {
		c.t.Fatalf("opening the store: %v", err)
	}
	c.t.Cleanup(func() { _ = st.Close() })
	var tenant *core.Tenant
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		tenant, err = u.GetTenantByKey(ctx, service.DefaultTenantKey)
		return err
	}); err != nil {
		c.t.Fatalf("reading the default tenant: %v", err)
	}
	latest, err := migrations.Latest()
	if err != nil {
		c.t.Fatalf("reading migrations: %v", err)
	}
	router, err := httpapi.New(httpapi.Config{
		Service:         service.New(st),
		Authenticator:   server.NewAuthenticator(st, clock.New()),
		DefaultTenantID: tenant.ID,
		Probe:           st,
		ExpectedSchema:  latest,
	})
	if err != nil {
		c.t.Fatalf("building the router: %v", err)
	}
	srv := httptest.NewServer(router)
	c.t.Cleanup(srv.Close)
	return srv.URL
}

// fakeAgent serves an in-memory ssh-agent holding keys and returns its
// socket. The directory is short because a unix socket path is limited to
// about a hundred bytes and t.TempDir names are long.
func fakeAgent(t *testing.T, keys ...loginKey) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tixagent")
	if err != nil {
		t.Fatalf("making the socket directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	ring := agent.NewKeyring()
	for _, k := range keys {
		if err := ring.Add(agent.AddedKey{PrivateKey: k.priv}); err != nil {
			t.Fatalf("adding a key to the agent: %v", err)
		}
	}
	socket := filepath.Join(dir, "agent.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listening on the agent socket: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = agent.ServeAgent(ring, conn)
				_ = conn.Close()
			}()
		}
	}()
	return socket
}

func mustSession(t *testing.T, got result) core.Session {
	t.Helper()
	if got.code != core.ExitOK {
		t.Fatalf("login exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
	var session core.Session
	if err := json.Unmarshal([]byte(got.out), &session); err != nil || session.Token == "" {
		t.Fatalf("login printed no session: %v\n%s", err, got.out)
	}
	return session
}

func TestLoginWithAnSSHKeyFile(t *testing.T) {
	c := newCLI(t)
	k := newLoginKey(t)
	c.enrolLoginKey(k)
	url := c.loginServer()
	path := filepath.Join(c.home, "keys", "id_ed25519")
	k.write(t, path, "")

	mustSession(t, c.run("--server", url, "login", "--ssh-key="+path, "-o", "json"))
}

func TestLoginWithAnEncryptedKeyAsksForThePassphrase(t *testing.T) {
	c := newCLI(t)
	k := newLoginKey(t)
	c.enrolLoginKey(k)
	url := c.loginServer()
	path := filepath.Join(c.home, "keys", "id_ed25519")
	k.write(t, path, "open sesame")

	got := c.runTTYEnv(nil, func(string) string { return "open sesame\n" },
		"--server", url, "login", "--ssh-key="+path, "-o", "json")
	mustSession(t, got)
	if !strings.Contains(got.err, "passphrase for "+path) {
		t.Errorf("stderr = %q, want a passphrase prompt naming the key", got.err)
	}
	if strings.Contains(got.out+got.err, "open sesame") {
		t.Errorf("the passphrase was printed:\nstdout: %s\nstderr: %s", got.out, got.err)
	}

	refused := c.run("--server", url, "login", "--ssh-key="+path)
	if refused.code == core.ExitOK || !strings.Contains(refused.err, "encrypted") {
		t.Fatalf("an encrypted key with no terminal exited %d: %s", refused.code, refused.err)
	}
}

func TestLoginWithTheAgentSkipsKeysThatAreNotEnrolled(t *testing.T) {
	c := newCLI(t)
	enrolled, stranger := newLoginKey(t), newLoginKey(t)
	c.enrolLoginKey(enrolled)
	url := c.loginServer()
	socket := fakeAgent(t, stranger, enrolled)

	mustSession(t, runEnv(c, []string{"SSH_AUTH_SOCK=" + socket}, "--server", url, "login", "--ssh-key", "-o", "json"))
}

func TestLoginWithNoAcceptedKeyNamesWhatItTried(t *testing.T) {
	c := newCLI(t)
	c.enrolLoginKey(newLoginKey(t))
	url := c.loginServer()
	stranger := newLoginKey(t)
	signer, err := gossh.NewSignerFromKey(stranger.priv)
	if err != nil {
		t.Fatalf("wrapping the key: %v", err)
	}
	socket := fakeAgent(t, stranger)

	got := runEnv(c, []string{"SSH_AUTH_SOCK=" + socket}, "--server", url, "login", "--ssh-key")
	if got.code != core.ExitPermission {
		t.Fatalf("login exited %d, want %d\n%s", got.code, core.ExitPermission, got.err)
	}
	if !strings.Contains(got.err, gossh.FingerprintSHA256(signer.PublicKey())) || !strings.Contains(got.err, "not enrolled") {
		t.Fatalf("stderr = %q, want it to name the key it tried and why", got.err)
	}
}

func TestLoginWithAnSSHKeyAgainstTheDatabase(t *testing.T) {
	c := newCLI(t)
	k := newLoginKey(t)
	c.enrolLoginKey(k)
	path := filepath.Join(c.home, "keys", "id_ed25519")
	k.write(t, path, "")

	mustSession(t, c.run("login", "--ssh-key="+path, "-o", "json"))
}

func TestLoginWithAnSSHKeyRefusesAPassword(t *testing.T) {
	c := newCLI(t)
	got := c.run("login", "--ssh-key", "--email", "ada@example.com")
	if got.code != core.ExitUsage {
		t.Fatalf("--ssh-key with --email exited %d, want %d", got.code, core.ExitUsage)
	}
}
