// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// gatedService holds CreateTask at a gate, or fails it, so a test can see a
// request while it runs and after it broke.
type gatedService struct {
	*apiService
	entered chan struct{}
	gate    chan struct{}
	fail    error
	// writeFirst commits the task before failing.
	writeFirst bool
}

func (s *gatedService) CreateTask(ctx context.Context, in core.CreateTaskInput) (*core.Task, error) {
	if s.entered != nil {
		s.entered <- struct{}{}
		<-s.gate
	}
	if s.fail != nil {
		if s.writeFirst {
			if _, err := s.apiService.CreateTask(ctx, in); err != nil {
				return nil, err
			}
		}
		return nil, s.fail
	}
	return s.apiService.CreateTask(ctx, in)
}

// forgetful never keeps a response, as a server that stopped right after the
// write committed would not.
type forgetful struct{ httpapi.Idempotency }

func (forgetful) FinishIdempotent(context.Context, *service.IdempotencyClaim, service.IdempotentResponse) error {
	return nil
}

// sendKeyed posts a body with an Idempotency-Key as the given host and token.
func (f *apiFixture) sendKeyed(method, path, host, token string, body any, keys ...string) *http.Response {
	f.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		f.t.Fatal(err)
	}
	req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(raw))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Host = host
	req.Header.Set(wire.HeaderContentType, wire.ContentJSON)
	req.Header.Set(wire.HeaderAuth, "Bearer "+token)
	for _, k := range keys {
		req.Header.Add(wire.HeaderIdempotencyKey, k)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp
}

// createKeyed creates a task in tenant A's infra project with a key.
func (f *apiFixture) createKeyed(title string, keys ...string) *http.Response {
	f.t.Helper()
	return f.sendKeyed(http.MethodPost, wire.RouteTasks, f.hostA, f.tokenA,
		core.CreateTaskInput{ProjectRef: "infra", Title: title}, keys...)
}

// tasksIn counts the live tasks of a tenant.
func (f *apiFixture) tasksIn(tenantID string) int {
	f.t.Helper()
	ctx := context.Background()
	var n int
	if err := f.store.View(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		got, err := tx.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: 100}})
		n = len(got)
		return err
	}); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func errorCode(t *testing.T, resp *http.Response) core.Kind {
	t.Helper()
	var env wire.ErrorEnvelope
	decodeBody(t, resp, &env)
	return env.Error.Code
}

func TestARepeatedCreateIsReplayedAndCreatesOneTask(t *testing.T) {
	f := newFixture(t)
	first := f.createKeyed("deploy", "key-1")
	mustStatus(t, first, http.StatusCreated)
	if first.Header.Get(wire.HeaderIdempotentReplayed) != "" {
		t.Fatal("the first answer claims to be a replay")
	}
	firstBody := readBody(t, first)

	again := f.createKeyed("deploy", "key-1")
	mustStatus(t, again, http.StatusCreated)
	if got := again.Header.Get(wire.HeaderIdempotentReplayed); got != "true" {
		t.Fatalf("%s = %q, want true", wire.HeaderIdempotentReplayed, got)
	}
	if ct := again.Header.Get(wire.HeaderContentType); ct != first.Header.Get(wire.HeaderContentType) {
		t.Fatalf("replayed content type %q, first was %q", ct, first.Header.Get(wire.HeaderContentType))
	}
	if body := readBody(t, again); body != firstBody {
		t.Fatalf("replayed body differs:\n%s\nfirst:\n%s", body, firstBody)
	}
	if n := f.tasksIn(f.tenantA.ID); n != 1 {
		t.Fatalf("%d tasks exist, want 1", n)
	}
}

func TestAKeyReusedForADifferentRequestIs422(t *testing.T) {
	f := newFixture(t)
	resp := f.createKeyed("first", "key-1")
	mustStatus(t, resp, http.StatusCreated)
	_ = resp.Body.Close()

	resp = f.createKeyed("second", "key-1")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", resp.StatusCode, readBody(t, resp))
	}
	if code := errorCode(t, resp); code != core.KindInvalid {
		t.Fatalf("code = %q, want invalid", code)
	}
	if n := f.tasksIn(f.tenantA.ID); n != 1 {
		t.Fatalf("%d tasks exist, want 1", n)
	}
}

func TestWithoutAKeyEveryRequestRuns(t *testing.T) {
	f := newFixture(t)
	for range 2 {
		resp := f.createKeyed("same")
		mustStatus(t, resp, http.StatusCreated)
		_ = resp.Body.Close()
	}
	if n := f.tasksIn(f.tenantA.ID); n != 2 {
		t.Fatalf("%d tasks exist, want 2", n)
	}
}

func TestAMalformedKeyIs400AndRunsNothing(t *testing.T) {
	f := newFixture(t)
	for name, keys := range map[string][]string{
		"too long":    {strings.Repeat("k", 201)},
		"not ascii":   {"café"},
		"two headers": {"a", "b"},
	} {
		t.Run(name, func(t *testing.T) {
			resp := f.createKeyed("deploy", keys...)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", resp.StatusCode, readBody(t, resp))
			}
			if code := errorCode(t, resp); code != core.KindInvalid {
				t.Fatalf("code = %q, want invalid", code)
			}
		})
	}
	if n := f.tasksIn(f.tenantA.ID); n != 0 {
		t.Fatalf("%d tasks exist, want 0", n)
	}
}

func TestKeysBelongToOneActorInOneTenant(t *testing.T) {
	f := newFixture(t)
	seedProject(t, f.store, f.tenantB.ID, "infra")
	carol := seedActor(t, f.store, f.tenantA.ID, "carol")
	carolToken := mintToken(t, f.clock, f.lookup, f.tenantA.ID, carol.ID)

	body := core.CreateTaskInput{ProjectRef: "infra", Title: "deploy"}
	for _, who := range []struct{ host, token string }{
		{f.hostA, f.tokenA}, {f.hostA, carolToken}, {f.hostB, f.tokenB},
	} {
		resp := f.sendKeyed(http.MethodPost, wire.RouteTasks, who.host, who.token, body, "shared-key")
		mustStatus(t, resp, http.StatusCreated)
		if resp.Header.Get(wire.HeaderIdempotentReplayed) != "" {
			t.Fatal("another actor's request was answered from a key it never sent")
		}
		_ = resp.Body.Close()
	}
	if a, b := f.tasksIn(f.tenantA.ID), f.tasksIn(f.tenantB.ID); a != 2 || b != 1 {
		t.Fatalf("tenant A has %d tasks and tenant B %d, want 2 and 1", a, b)
	}
}

func TestARetryWhileTheFirstRunsIs409WithRetryAfter(t *testing.T) {
	gated := &gatedService{entered: make(chan struct{}), gate: make(chan struct{})}
	f := newFixtureWith(t, func(f *apiFixture, cfg *httpapi.Config) {
		gated.apiService = f.svc
		cfg.Service = gated
	})
	done := make(chan *http.Response)
	go func() { done <- f.createKeyed("deploy", "key-1") }()
	<-gated.entered

	gated.entered = nil
	resp := f.createKeyed("deploy", "key-1")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", resp.StatusCode, readBody(t, resp))
	}
	if ra := resp.Header.Get(wire.HeaderRetryAfter); ra != "60" {
		t.Fatalf("%s = %q, want 60, twice the request timeout", wire.HeaderRetryAfter, ra)
	}
	if code := errorCode(t, resp); code != core.KindConflict {
		t.Fatalf("code = %q, want conflict", code)
	}

	close(gated.gate)
	first := <-done
	mustStatus(t, first, http.StatusCreated)
	_ = first.Body.Close()
	replayed := f.createKeyed("deploy", "key-1")
	mustStatus(t, replayed, http.StatusCreated)
	if replayed.Header.Get(wire.HeaderIdempotentReplayed) != "true" {
		t.Fatal("the finished request was not replayed")
	}
	_ = replayed.Body.Close()
	if n := f.tasksIn(f.tenantA.ID); n != 1 {
		t.Fatalf("%d tasks exist, want 1", n)
	}
}

func TestAServerErrorBeforeAnyWriteIsNotKept(t *testing.T) {
	gated := &gatedService{fail: core.Internal("disk on fire")}
	f := newFixtureWith(t, func(f *apiFixture, cfg *httpapi.Config) {
		gated.apiService = f.svc
		cfg.Service = gated
	})
	resp := f.createKeyed("deploy", "key-1")
	mustStatus(t, resp, http.StatusInternalServerError)
	_ = resp.Body.Close()

	gated.fail = nil
	resp = f.createKeyed("deploy", "key-1")
	mustStatus(t, resp, http.StatusCreated)
	if resp.Header.Get(wire.HeaderIdempotentReplayed) != "" {
		t.Fatal("the retry was answered from the failed attempt")
	}
	_ = resp.Body.Close()
	if n := f.tasksIn(f.tenantA.ID); n != 1 {
		t.Fatalf("%d tasks exist, want 1", n)
	}
}

func TestAServerErrorAfterAWriteIsKeptSoTheWriteIsNotRepeated(t *testing.T) {
	gated := &gatedService{fail: core.Internal("rendering failed"), writeFirst: true}
	f := newFixtureWith(t, func(f *apiFixture, cfg *httpapi.Config) {
		gated.apiService = f.svc
		cfg.Service = gated
	})
	resp := f.createKeyed("deploy", "key-1")
	mustStatus(t, resp, http.StatusInternalServerError)
	_ = resp.Body.Close()

	gated.fail = nil
	resp = f.createKeyed("deploy", "key-1")
	mustStatus(t, resp, http.StatusInternalServerError)
	if resp.Header.Get(wire.HeaderIdempotentReplayed) != "true" {
		t.Fatal("the retry ran again instead of replaying")
	}
	_ = resp.Body.Close()
	if n := f.tasksIn(f.tenantA.ID); n != 1 {
		t.Fatalf("%d tasks exist, want 1", n)
	}
}

func TestAWriteWhoseResponseWasLostIsNeverRunAgain(t *testing.T) {
	f := newFixtureWith(t, func(f *apiFixture, cfg *httpapi.Config) {
		cfg.Idempotency = forgetful{f.local}
	})
	resp := f.createKeyed("deploy", "key-1")
	mustStatus(t, resp, http.StatusCreated)
	_ = resp.Body.Close()

	f.clock.Advance(2*httpapi.DefaultRequestTimeout + time.Second)
	resp = f.createKeyed("deploy", "key-1")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", resp.StatusCode, readBody(t, resp))
	}
	if ra := resp.Header.Get(wire.HeaderRetryAfter); ra != "" {
		t.Fatalf("%s = %q on a request that will never finish", wire.HeaderRetryAfter, ra)
	}
	_ = resp.Body.Close()
	if n := f.tasksIn(f.tenantA.ID); n != 1 {
		t.Fatalf("%d tasks exist, want 1", n)
	}
}

func TestAValidationErrorIsNotReplayed(t *testing.T) {
	f := newFixture(t)
	for range 2 {
		resp := f.createKeyed("", "key-1")
		mustStatus(t, resp, http.StatusBadRequest)
		if resp.Header.Get(wire.HeaderIdempotentReplayed) != "" {
			t.Fatal("a refused request that changed nothing was replayed")
		}
		_ = resp.Body.Close()
	}
}

func TestReadsIgnoreTheKey(t *testing.T) {
	f := newFixture(t)
	req := f.newRequest(http.MethodGet, wire.RouteTasks, nil)
	req.Header.Set(wire.HeaderIdempotencyKey, strings.Repeat("k", 300))
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
}
