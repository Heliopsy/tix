package httpapi_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

func TestUnauthenticatedRequestIsRejected(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodGet, httpapi.RouteTasks, f.hostA, "", nil)
	mustStatus(t, resp, http.StatusUnauthorized)
	var env httpapi.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindUnauthenticated {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindUnauthenticated)
	}
}

func TestInvalidCredentialIsRejected(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodGet, httpapi.RouteTasks, f.hostA, "tix_pat_bogus", nil)
	mustStatus(t, resp, http.StatusUnauthorized)
	_ = resp.Body.Close()
}

func TestCrossTenantCredentialIsRejected(t *testing.T) {
	f := newFixture(t)
	f.createTask("tenant a task")

	resp := f.do(http.MethodGet, httpapi.RouteTasks, f.hostA, f.tokenB, nil)
	mustStatus(t, resp, http.StatusUnauthorized)
	body := readBody(t, resp)
	if strings.Contains(body, "tenant a task") {
		t.Fatal("a cross-tenant request returned data")
	}
}

func TestMatchingTenantCredentialProceeds(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodGet, httpapi.RouteTasks, f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
}

func TestUnknownHostIsNotFoundWithoutADefaultTenant(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodGet, httpapi.RouteTasks, "nowhere.example", f.tokenA, nil)
	mustStatus(t, resp, http.StatusNotFound)
	var env httpapi.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindNotFound {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindNotFound)
	}
}

func TestUnknownHostFallsBackToTheDefaultTenant(t *testing.T) {
	f := newFixtureWith(t, func(f *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = f.tenantA.ID
	})

	resp := f.do(http.MethodGet, httpapi.RouteTasks, "nowhere.example", f.tokenA, nil)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
}

func TestTenantResolutionPrecedesAuthentication(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodGet, httpapi.RouteTasks, "nowhere.example", "", nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
}

func TestBodySizeLimitIsEnforced(t *testing.T) {
	f := newFixture(t)
	huge := core.CreateTaskInput{ProjectRef: "infra", Title: strings.Repeat("x", 8192)}

	resp := f.call(http.MethodPost, httpapi.RouteTasks, huge)
	mustStatus(t, resp, http.StatusRequestEntityTooLarge)
	var env httpapi.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Message == "" {
		t.Error("oversized body was rejected without a message")
	}
}

func TestBodyWithinTheLimitIsAccepted(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, httpapi.RouteTasks,
		core.CreateTaskInput{ProjectRef: "infra", Title: strings.Repeat("x", 400)})
	mustStatus(t, resp, http.StatusCreated)
	_ = resp.Body.Close()
}

func TestUnsupportedMediaTypeIsRejected(t *testing.T) {
	f := newFixture(t)

	req := f.newRequest(http.MethodPost, httpapi.RouteTasks, bytes.NewReader([]byte("title=x")))
	req.Header.Set(httpapi.HeaderContentType, "application/x-www-form-urlencoded")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	mustStatus(t, resp, http.StatusUnsupportedMediaType)
	_ = resp.Body.Close()
}

func TestMalformedJSONIsRejected(t *testing.T) {
	f := newFixture(t)

	req := f.newRequest(http.MethodPost, httpapi.RouteTasks, bytes.NewReader([]byte("{not json")))
	req.Header.Set(httpapi.HeaderContentType, httpapi.ContentJSON)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	mustStatus(t, resp, http.StatusBadRequest)
	var env httpapi.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindInvalid {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindInvalid)
	}
}

func TestMissingBodyIsRejected(t *testing.T) {
	f := newFixture(t)

	req := f.newRequest(http.MethodPost, httpapi.RouteTasks, bytes.NewReader(nil))
	req.Header.Set(httpapi.HeaderContentType, httpapi.ContentJSON)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()
}

func TestRequestIDIsReturnedAndHonoured(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodGet, httpapi.RouteTasks, nil)
	mustStatus(t, resp, http.StatusOK)
	if resp.Header.Get(httpapi.HeaderRequestID) == "" {
		t.Error("no request id was assigned")
	}
	_ = resp.Body.Close()

	req := f.newRequest(http.MethodGet, httpapi.RouteTasks, nil)
	req.Header.Set(httpapi.HeaderRequestID, "caller-supplied")
	echoed, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	if got := echoed.Header.Get(httpapi.HeaderRequestID); got != "caller-supplied" {
		t.Errorf("request id = %q, want the caller's", got)
	}
	_ = echoed.Body.Close()
}

func TestLogsCarryTheRequestAndNoCredential(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, httpapi.RouteTasks,
		core.CreateTaskInput{ProjectRef: "infra", Title: "logged"})
	mustStatus(t, resp, http.StatusCreated)
	_ = resp.Body.Close()

	logs := f.logs.String()
	for _, want := range []string{"method=POST", httpapi.RouteTasks, "status=201", "tenant="} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs missing %q: %s", want, logs)
		}
	}
	if strings.Contains(logs, f.tokenA) {
		t.Error("the api token appeared in the logs")
	}
	if strings.Contains(strings.ToLower(logs), "authorization") {
		t.Error("the authorization header appeared in the logs")
	}
}

func TestPasswordBodiesAreNotLogged(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodPost, httpapi.RouteLogin, f.hostA, "",
		httpapi.LoginRequest{Email: "a@example.com", Password: "correct-horse-battery"})
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()

	if strings.Contains(f.logs.String(), "correct-horse-battery") {
		t.Error("a password reached the logs")
	}
}

func TestLoginSetsASessionCookie(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodPost, httpapi.RouteLogin, f.hostA, "",
		httpapi.LoginRequest{Email: "a@example.com", Password: "correct-horse-battery"})
	mustStatus(t, resp, http.StatusOK)
	defer func() { _ = resp.Body.Close() }()

	for _, c := range resp.Cookies() {
		if c.Name == httpapi.SessionCookieName {
			if !c.HttpOnly {
				t.Error("session cookie is not HttpOnly")
			}
			return
		}
	}
	t.Fatal("login set no session cookie")
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	f := newFixture(t)

	resp := f.do(http.MethodPost, httpapi.RouteLogin, f.hostA, "",
		httpapi.LoginRequest{Email: "a@example.com", Password: "wrong-password"})
	mustStatus(t, resp, http.StatusUnauthorized)
	_ = resp.Body.Close()
}

func TestRequestTimeoutAnswersInTheEnvelope(t *testing.T) {
	f := newFixtureWith(t, func(f *apiFixture, cfg *httpapi.Config) {
		cfg.RequestTimeout = 20 * time.Millisecond
		f.svc.block = make(chan struct{})
	})
	t.Cleanup(func() { close(f.svc.block) })

	resp := f.call(http.MethodGet, httpapi.RouteLabels, nil)
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504: %s", resp.StatusCode, readBody(t, resp))
	}
	var env httpapi.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Message == "" {
		t.Error("timeout was reported without a message")
	}
}

func TestPanicIsRecovered(t *testing.T) {
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) {
		cfg.EventHandler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("handler exploded")
		})
	})

	resp := f.call(http.MethodGet, httpapi.RouteEvents, nil)
	mustStatus(t, resp, http.StatusInternalServerError)
	var env httpapi.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindInternal {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindInternal)
	}
	if strings.Contains(env.Error.Message, "exploded") {
		t.Error("the panic value leaked to the client")
	}
}

func TestReadinessFailsWhenMigrationsAreBehind(t *testing.T) {
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) {
		cfg.ExpectedSchema = 9999
	})

	resp := f.do(http.MethodGet, httpapi.RouteReady, f.hostA, "", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", resp.StatusCode, readBody(t, resp))
	}
	var body httpapi.ReadyBody
	decodeBody(t, resp, &body)
	if body.Ready {
		t.Error("readiness reports ready with the schema behind")
	}
	if !hasFailingCheck(body.Checks, "migrations") {
		t.Errorf("checks = %+v, want the migration check to fail", body.Checks)
	}
}

func TestRouterRejectsAnIncompleteConfiguration(t *testing.T) {
	if _, err := httpapi.New(httpapi.Config{}); err == nil {
		t.Error("a router without a service was accepted")
	}
	if _, err := httpapi.New(httpapi.Config{Service: &apiService{}}); err == nil {
		t.Error("a router without an authenticator was accepted")
	}
}

// The browser surface redirects an anonymous visitor to its own login page, so
// an anonymous browser request must reach the web handler rather than being
// answered with a JSON envelope. Refusing here made signing in impossible.
// Anything under the API prefix must still be refused.
func TestAnonymousBrowserRequestsReachTheWebHandler(t *testing.T) {
	reached := false
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) {
		cfg.Authenticator = abstainingAuthenticator{}
		cfg.WebHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusOK)
		})
	})

	for _, path := range []string{"/", "/projects", "/assets/app.css"} {
		reached = false
		res := f.do(http.MethodGet, path, f.hostA, "", nil)
		_ = res.Body.Close()
		if !reached {
			t.Errorf("anonymous GET %q never reached the web handler (status %d)", path, res.StatusCode)
		}
	}

	for _, path := range []string{"/api/v1/tasks", "/api/v1/users"} {
		res := f.do(http.MethodGet, path, f.hostA, "", nil)
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous GET %q = %d, want 401: %s", path, res.StatusCode, body)
		}
	}
}

// abstainingAuthenticator presents no credential, as an anonymous visitor does.
type abstainingAuthenticator struct{}

func (abstainingAuthenticator) Authenticate(context.Context, *http.Request) (*core.Actor, error) {
	return nil, auth.ErrNoCredential
}
