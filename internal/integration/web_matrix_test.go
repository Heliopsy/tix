// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	urlpkg "net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/web"
)

// localWebTarget serves the browser interface straight over the harness's
// own server, the same handler a real "tix serve" mounts. This is the
// baseline: no server-fronting-a-server involved.
func localWebTarget(h *matrixHarness) (base string, svc core.Service, ctx context.Context) {
	return h.baseURL, h.local, h.adminCtx
}

// remoteWebTarget serves the browser interface from a SECOND process image,
// over a client.Client that itself talks HTTP to the harness's server.
//
// This is the cell the task named as worth confirming rather than assuming,
// and confirming it surfaced a real, worth-reporting gap: `web.Handler` has
// no authentication logic of its own. Every screen it serves is gated by
// `core.ActorFrom(r.Context())` (internal/web/middleware.go), and nothing
// inside the web package ever sets that; it is put there by
// `internal/httpapi.Router`'s authenticate/resolveTenant middleware, which is
// only ever wired in by `internal/server.Assemble`, and only over
// `server.NewAuthenticator(opts.Store, opts.Clock)` — a verifier that reads
// tokens and sessions straight out of a `store.Store` (internal/server/auth.go),
// not through `core.Service`. `cmd/serve.go`'s refusal to front a remote
// target (`conn.Info.Target.Mode != connect.ModeLocal`) is therefore not
// merely a CLI policy choice: nothing in the shipped assembly path can
// authenticate a browser session without a local store to check it against.
//
// So this target does not reuse server.Assemble or web's own (nonexistent)
// auth. It builds an httpapi.Router directly, which needs no Store, with an
// Authenticator written for this test: it takes whatever bearer token or
// session cookie the browser presented and asks the REAL remote server who
// that credential belongs to, via a client.Client built from it and one call
// to WhoAmI. That is authentication over core.Service rather than over a
// store, and it is enough to prove the cell is reachable in principle - but
// it is test-only scaffolding, not a claim that this is how tix should wire
// it in production. See the report for the one-line production change this
// points at (a store-free, service-based Authenticator implementation).
func remoteWebTarget(t *testing.T, h *matrixHarness) (base string, svc core.Service, ctx context.Context) {
	t.Helper()
	c, err := client.New(h.baseURL, h.adminToken)
	if err != nil {
		t.Fatalf("building the remote client the browser server fronts: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	router, err := httpapi.New(httpapi.Config{
		Service:         c,
		Authenticator:   &serviceAuthenticator{baseURL: h.baseURL},
		WebHandler:      web.Handler(c),
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		DefaultTenantID: h.tenantID,
	})
	if err != nil {
		t.Fatalf("assembling the browser-facing router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv.URL, c, context.Background()
}

// serviceAuthenticator resolves the actor behind a bearer token or session
// cookie by presenting it to the real remote server and asking WhoAmI,
// rather than by looking it up in a local store this front has none of.
type serviceAuthenticator struct{ baseURL string }

func (a *serviceAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*core.Actor, error) {
	if v := r.Header.Get("Authorization"); v != "" {
		if scheme, token, ok := strings.Cut(v, " "); ok && strings.EqualFold(scheme, "bearer") {
			if token = strings.TrimSpace(token); token != "" {
				return a.whoAmI(ctx, token, "")
			}
		}
	}
	if c, err := r.Cookie(auth.SessionCookieName); err == nil && c.Value != "" {
		return a.whoAmI(ctx, "", c.Value)
	}
	return nil, auth.ErrNoCredential
}

func (a *serviceAuthenticator) whoAmI(ctx context.Context, bearer, cookie string) (*core.Actor, error) {
	var opts []client.Option
	if cookie != "" {
		opts = append(opts, client.WithSessionCookie(cookie))
	}
	c, err := client.New(a.baseURL, bearer, opts...)
	if err != nil {
		return nil, core.Internal("building a verification client").Wrap(err)
	}
	defer func() { _ = c.Close() }()
	return c.WhoAmI(ctx)
}

// webScenario is one row of the browser transport-equivalence table. run
// creates whatever the scenario needs directly through svc/ctx, then drives
// a signed-in browser against base and asserts what it sees.
type webScenario struct {
	name   string
	exempt string
	run    func(t *testing.T, targetName, base string, svc core.Service, ctx context.Context)
}

var webScenarios = []webScenario{
	{
		name: "the task list shows a task written directly through this target",
		run: func(t *testing.T, targetName, base string, svc core.Service, ctx context.Context) {
			password := "correct-horse-battery-staple-" + randSuffix()
			email := "browser-" + randSuffix() + "@example.test"
			if _, err := svc.CreateUser(ctx, core.CreateUserInput{
				Email: email, Password: password, Handle: "browser-" + randSuffix(), Role: core.RoleAdmin,
			}); err != nil {
				t.Fatalf("[%s] creating the browser user: %v", targetName, err)
			}
			title := "seen in the browser " + randSuffix()
			if _, err := svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "default", Title: title}); err != nil {
				t.Fatalf("[%s] creating the task directly: %v", targetName, err)
			}

			browser := signIn(t, base, email, password)
			res, err := browser.Get(base + web.RouteTasks)
			if err != nil {
				t.Fatalf("[%s] GET %s: %v", targetName, web.RouteTasks, err)
			}
			body := readAll(t, res)
			if !strings.Contains(body, title) {
				t.Fatalf("[%s] task list does not show %q, written directly through this target", targetName, title)
			}
		},
	},
	{
		name: "creating a task through the browser form writes through this target",
		run: func(t *testing.T, targetName, base string, svc core.Service, ctx context.Context) {
			password := "correct-horse-battery-staple-" + randSuffix()
			email := "browser-" + randSuffix() + "@example.test"
			if _, err := svc.CreateUser(ctx, core.CreateUserInput{
				Email: email, Password: password, Handle: "browser-" + randSuffix(), Role: core.RoleAdmin,
			}); err != nil {
				t.Fatalf("[%s] creating the browser user: %v", targetName, err)
			}

			browser := signIn(t, base, email, password)
			page, err := browser.Get(base + web.RouteTasks)
			if err != nil {
				t.Fatalf("[%s] GET %s: %v", targetName, web.RouteTasks, err)
			}
			csrf := betweenQuotes(readAll(t, page), `name="`+web.CSRFFieldName+`" value="`)
			if csrf == "" {
				t.Fatalf("[%s] the tasks page carried no csrf token", targetName)
			}

			title := "written from the browser form " + randSuffix()
			form := urlpkg.Values{
				"title": {title}, "project_ref": {"default"}, "priority": {"3"},
				web.CSRFFieldName: {csrf},
			}
			res, err := browser.PostForm(base+web.RouteTasks, form)
			if err != nil {
				t.Fatalf("[%s] posting the new-task form: %v", targetName, err)
			}
			_ = readAll(t, res)

			listed, err := svc.ListTasks(ctx, core.TaskFilter{ProjectKeys: []string{"default"}})
			if err != nil {
				t.Fatalf("[%s] listing tasks directly after the browser write: %v", targetName, err)
			}
			found := false
			for _, tsk := range listed.Tasks {
				if tsk.Title == title {
					found = true
				}
			}
			if !found {
				t.Fatalf("[%s] no task titled %q found directly after a browser-form write", targetName, title)
			}
		},
	},
}

func TestWebTransportEquivalence(t *testing.T) {
	for _, sc := range webScenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			if sc.exempt != "" {
				t.Skipf("exempt: %s", sc.exempt)
			}

			localH := newMatrixHarness(t)
			localBase, localSvc, localCtx := localWebTarget(localH)
			sc.run(t, "browser against direct service.Local", localBase, localSvc, localCtx)

			remoteH := newMatrixHarness(t)
			remoteBase, remoteSvc, remoteCtx := remoteWebTarget(t, remoteH)
			sc.run(t, "browser against a server fronting a client.Client", remoteBase, remoteSvc, remoteCtx)
		})
	}
}
