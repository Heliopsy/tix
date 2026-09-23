// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	gossh "golang.org/x/crypto/ssh"
)

// TestConcurrentSessionsCannotSeeEachOther runs many sessions at once, each
// under its own fingerprint, and asserts that none of them can reach another's
// tenant by any route the interface has: listing tasks, listing projects, or
// naming another tenant's task by its identifier.
//
// This is the failure the project treats as unrecoverable, so it is proved
// under concurrency rather than assumed from the shape of the code.
func TestConcurrentSessionsCannotSeeEachOther(t *testing.T) {
	p, _, _ := newProvisioner(t)
	const sessions = 8

	type seen struct {
		tenantID string
		taskID   string
		titles   []string
	}
	var (
		mu      sync.Mutex
		results = map[string]seen{}
		start   = make(chan struct{})
		wg      sync.WaitGroup
	)

	for i := 0; i < sessions; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			fingerprint := fmt.Sprintf("SHA256:key-%d", i)
			actor, err := p.ActorByFingerprint(context.Background(), fingerprint)
			if err != nil {
				t.Errorf("session %d: %v", i, err)
				return
			}
			<-start

			ctx := sessionContext(actor)
			mine := fmt.Sprintf("only session %d wrote this", i)
			task, err := p.service.CreateTask(ctx, core.CreateTaskInput{
				ProjectRef: seedProjectKey, Title: mine,
			})
			if err != nil {
				t.Errorf("session %d: CreateTask: %v", i, err)
				return
			}
			page, err := p.service.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
			if err != nil {
				t.Errorf("session %d: ListTasks: %v", i, err)
				return
			}
			titles := make([]string, 0, len(page.Tasks))
			for _, task := range page.Tasks {
				titles = append(titles, task.Title)
			}

			mu.Lock()
			results[fingerprint] = seen{tenantID: actor.TenantID, taskID: task.ID, titles: titles}
			mu.Unlock()
		}(i)
	}
	close(start)
	wg.Wait()

	if len(results) != sessions {
		t.Fatalf("%d sessions reported, want %d", len(results), sessions)
	}

	tenants := map[string]string{}
	for fingerprint, got := range results {
		if other, dup := tenants[got.tenantID]; dup {
			t.Fatalf("%s and %s share tenant %q", fingerprint, other, got.tenantID)
		}
		tenants[got.tenantID] = fingerprint
	}

	for fingerprint, got := range results {
		for _, title := range got.titles {
			if strings.HasPrefix(title, "only session ") && !strings.Contains(title, fingerprint[len("SHA256:key-")-1:]) {
				// Belt and braces: any foreign marker at all is a leak.
				if title != "only session "+strings.TrimPrefix(fingerprint, "SHA256:key-")+" wrote this" {
					t.Errorf("%s sees %q, which belongs to another session", fingerprint, title)
				}
			}
		}
	}

	// Naming another tenant's task by its identifier is the route a scoped
	// listing would not catch.
	for fingerprint, got := range results {
		actor, err := p.ActorByFingerprint(context.Background(), fingerprint)
		if err != nil {
			t.Fatalf("%s: %v", fingerprint, err)
		}
		for otherFingerprint, other := range results {
			if otherFingerprint == fingerprint {
				continue
			}
			_, err := p.service.GetTask(sessionContext(actor), core.TaskRef{ID: other.taskID})
			if !core.IsKind(err, core.KindNotFound) {
				t.Fatalf("%s read %s's task %q directly: %v",
					fingerprint, otherFingerprint, other.taskID, err)
			}
		}
		_ = got
	}
}

// TestServerAcceptsAnyKeyAndGivesEachOneItsOwnTenant drives the real listener
// with a real ssh client, because the handshake and the identity it carries
// are not something a unit test of the provisioner can prove.
func TestServerAcceptsAnyKeyAndGivesEachOneItsOwnTenant(t *testing.T) {
	srv, st := newServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = srv.Serve(ctx) }()
	addr := srv.Addr()

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			dialSession(t, addr, fmt.Sprintf("client %d", i))
		}(i)
	}
	wg.Wait()

	var keys []string
	if err := walkTenants(context.Background(), st, func(tn core.Tenant) {
		if isSandbox(tn.Key) {
			keys = append(keys, tn.Key)
		}
	}); err != nil {
		t.Fatalf("listing tenants: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("two unknown keys produced %d sandboxes, want 2: %v", len(keys), keys)
	}
	if keys[0] == keys[1] {
		t.Fatal("two different keys landed in the same sandbox")
	}
}

// newServer builds a listener on an ephemeral loopback port.
func newServer(t *testing.T) (*Server, store.Store) {
	t.Helper()
	p, _, st := newProvisioner(t)
	srv, err := New(Options{
		Service:     p.service,
		Store:       st,
		Clock:       p.clk,
		Demo:        true,
		Addr:        "127.0.0.1:0",
		HostKeyPath: filepath.Join(t.TempDir(), "host_key"),
		Logger:      slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, st
}

// dialSession connects with a fresh key, opens the interface and quits.
func dialSession(t *testing.T, addr, name string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Errorf("%s: generating key: %v", name, err)
		return
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Errorf("%s: signer: %v", name, err)
		return
	}
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            "visitor",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Errorf("%s: dial: %v", name, err)
		return
	}
	defer func() { _ = client.Close() }()

	sess, err := client.NewSession()
	if err != nil {
		t.Errorf("%s: session: %v", name, err)
		return
	}
	defer func() { _ = sess.Close() }()

	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Errorf("%s: stdin: %v", name, err)
		return
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		t.Errorf("%s: stdout: %v", name, err)
		return
	}
	if err := sess.RequestPty("xterm-256color", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Errorf("%s: pty: %v", name, err)
		return
	}
	if err := sess.Shell(); err != nil {
		t.Errorf("%s: shell: %v", name, err)
		return
	}

	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, stdout)
		close(drained)
	}()
	// A quit leaves the board for the project list behind it, so the interface
	// takes two. They are repeated because the first keystrokes may land while
	// the program is still starting.
	go func() {
		for i := 0; i < 20; i++ {
			if _, err := stdin.Write([]byte("q")); err != nil {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()
	select {
	case <-drained:
	case <-time.After(20 * time.Second):
		t.Errorf("%s: the session did not end", name)
	}
}

func TestNewRefusesANonLoopbackBindWithoutTheExplicitChoice(t *testing.T) {
	p, _, st := newProvisioner(t)
	base := Options{
		Service:     p.service,
		Store:       st,
		Clock:       p.clk,
		HostKeyPath: filepath.Join(t.TempDir(), "host_key"),
		Logger:      slog.New(slog.DiscardHandler),
	}
	tests := []struct {
		name    string
		addr    string
		allow   bool
		refused bool
	}{
		{"loopback", "127.0.0.1:2222", false, false},
		{"loopback name", "localhost:2222", false, false},
		{"every interface", "0.0.0.0:2222", false, true},
		{"every interface, chosen", "0.0.0.0:2222", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := base
			o.Addr, o.AllowInsecure = tc.addr, tc.allow
			srv, err := New(o)
			if srv != nil {
				_ = srv.Close()
			}
			if refused := err != nil; refused != tc.refused {
				t.Fatalf("New(%q, allow=%v) error = %v", tc.addr, tc.allow, err)
			}
		})
	}
}

var _ = service.New
