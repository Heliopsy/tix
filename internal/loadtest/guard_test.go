// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeServer answers every request with an empty listing and records what it saw.
type fakeServer struct {
	mu       sync.Mutex
	requests []string
	projects map[string]bool
}

func newFakeServer(t *testing.T) (*fakeServer, *httptest.Server) {
	f := &fakeServer{projects: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		if key, ok := strings.CutPrefix(r.URL.Path, "/api/v1/projects/"); ok && r.Method == http.MethodGet {
			f.mu.Lock()
			exists := f.projects[key]
			f.mu.Unlock()
			if !exists {
				http.Error(w, `{"code":"not_found"}`, http.StatusNotFound)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks" {
			_, _ = io.WriteString(w, `{"ref":"loadtestabc-7","status":"todo","version":1}`)
			return
		}
		_, _ = io.WriteString(w, `{"items":[{"ref":"web-1","key":"web","status":"todo","version":1}]}`)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeServer) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func TestReadOnlyClientRefusesWritesWithoutSendingThem(t *testing.T) {
	f, srv := newFakeServer(t)
	rec := NewRecorder()
	c := newClient(Target{Base: srv.URL, Remote: true}, Account{}, rec, time.Second)
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
		_, err := c.api(context.Background(), "write", method, "/tasks", map[string]string{"title": "x"}, nil)
		if !errors.Is(err, ErrWritesRefused) {
			t.Errorf("%s on a read-only client: error %v, want ErrWritesRefused", method, err)
		}
	}
	if got := f.seen(); len(got) != 0 {
		t.Fatalf("a refused write still reached the server: %v", got)
	}
	if _, notes := rec.Snapshot(); notes["writes refused"] != 4 {
		t.Fatalf("refusals counted %d, want 4", notes["writes refused"])
	}
	if _, err := c.api(context.Background(), "read", http.MethodGet, "/tasks", nil, nil); err != nil {
		t.Fatalf("a read on a read-only client failed: %v", err)
	}
	if got := f.seen(); len(got) != 1 || got[0] != "GET /api/v1/tasks" {
		t.Fatalf("the read did not reach the server as one GET: %v", got)
	}
}

func TestReadOnlyAgentOnlyReads(t *testing.T) {
	f, srv := newFakeServer(t)
	tgt := Target{Base: srv.URL, Remote: true}
	u := &user{kind: KindAgent, tgt: tgt, c: newClient(tgt, Account{Token: "t"}, NewRecorder(), time.Second),
		rng: rand.New(rand.NewPCG(1, 2)), acct: Account{Token: "t"}}
	ctx := context.Background()
	if err := u.discover(ctx); err != nil {
		t.Fatal(err)
	}
	for range 200 {
		u.agentStep(ctx)
	}
	reqs := f.seen()
	if len(reqs) < 200 {
		t.Fatalf("the agent sent %d requests in 200 steps; its reads are not running", len(reqs))
	}
	for _, r := range reqs {
		if !strings.HasPrefix(r, "GET ") {
			t.Fatalf("a read-only agent sent %q", r)
		}
	}
}

func TestWritingAgentOnRemoteWritesOnlyToItsProject(t *testing.T) {
	f, srv := newFakeServer(t)
	tgt := Target{Base: srv.URL, Remote: true, AllowWrites: true, Project: "loadtestabc"}
	u := &user{kind: KindAgent, tgt: tgt, c: newClient(tgt, Account{Token: "t"}, NewRecorder(), time.Second),
		rng: rand.New(rand.NewPCG(3, 4)), acct: Account{Token: "t"}}
	ctx := context.Background()
	if err := u.discover(ctx); err != nil {
		t.Fatal(err)
	}
	for range 200 {
		u.agentStep(ctx)
	}
	creates, changes := 0, 0
	for _, r := range f.seen() {
		switch {
		case strings.HasPrefix(r, "GET "):
		case r == "POST /api/v1/tasks":
			creates++
		case strings.Contains(r, "/tasks/loadtestabc-7"):
			changes++
		default:
			t.Fatalf("a remote agent sent %q, a write to a task it did not create", r)
		}
	}
	if creates == 0 || changes == 0 {
		t.Fatalf("creates %d, changes %d: an agent allowed to write should do both", creates, changes)
	}
}

func TestCheckRemote(t *testing.T) {
	tests := []struct {
		name string
		o    RemoteOptions
		want string
	}{
		{"read-only with a token", RemoteOptions{URL: "https://tix.example", Token: "t"}, ""},
		{"writes with a token", RemoteOptions{URL: "https://tix.example", Token: "t", AllowWrites: true, Project: "scratch"}, ""},
		{"a project without --allow-writes", RemoteOptions{URL: "https://tix.example", Token: "t", Project: "web"}, "--allow-writes"},
		{"no credential", RemoteOptions{URL: "https://tix.example"}, "--token"},
		{"not a url", RemoteOptions{URL: "tix.example", Token: "t"}, "not an http"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckRemote(tt.o)
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("CheckRemote = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseRefusesWriteFlagsOnALocalRun(t *testing.T) {
	env := func(string) string { return "" }
	for _, args := range [][]string{{"--allow-writes"}, {"--project", "web"}, {"--token", "t"}} {
		if _, err := parse(args, io.Discard, env); err == nil {
			t.Errorf("parse(%v) accepted a remote-only flag without --target", args)
		}
	}
	if _, err := parse([]string{"--target", "https://tix.example", "--token", "t", "--project", "web"}, io.Discard, env); err == nil {
		t.Error("a remote run named a project to write to without --allow-writes and was accepted")
	}
	o, err := parse([]string{"--target", "https://tix.example", "--token", "t"}, io.Discard, env)
	if err != nil {
		t.Fatal(err)
	}
	if o.remote.AllowWrites || o.cfg.Mix[KindBrowser] != 0 {
		t.Fatalf("a remote run with only a token: writes %v, browser weight %d; want read-only and no browsers",
			o.remote.AllowWrites, o.cfg.Mix[KindBrowser])
	}
}

func TestScratchProject(t *testing.T) {
	f, srv := newFakeServer(t)
	f.projects["web"] = true
	ctx := context.Background()
	acct := Account{Token: "t"}

	if key, err := ScratchProject(ctx, srv.URL, acct, "web"); err != nil || key != "web" {
		t.Fatalf("a named existing project: %q, %v; want it used", key, err)
	}
	if key, err := ScratchProject(ctx, srv.URL, acct, "scratch"); err != nil || key != "scratch" {
		t.Fatalf("a named missing project: %q, %v; want it created", key, err)
	}
	if got := f.seen(); got[len(got)-1] != "POST /api/v1/projects" {
		t.Fatalf("the missing project was not created: %v", got)
	}

	key, err := ScratchProject(ctx, srv.URL, acct, "")
	if err != nil || !strings.HasPrefix(key, "loadtest") {
		t.Fatalf("an unnamed scratch project: %q, %v", key, err)
	}

	saved := scratchKey
	t.Cleanup(func() { scratchKey = saved })
	scratchKey = func() (string, error) { return "web", nil }
	before := len(f.seen())
	if _, err := ScratchProject(ctx, srv.URL, acct, ""); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("an unnamed key that collides with an existing project: %v, want a refusal", err)
	}
	for _, r := range f.seen()[before:] {
		if !strings.HasPrefix(r, "GET ") {
			t.Fatalf("a refused scratch project still sent %q", r)
		}
	}
}
