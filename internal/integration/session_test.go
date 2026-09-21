package integration

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/web"
)

// signIn drives the real browser sign-in: fetch the login page for a CSRF
// value, post the credentials, and keep whatever cookies come back.
func signIn(t *testing.T, base, email, password string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	page, err := client.Get(base + web.RouteLogin)
	if err != nil {
		t.Fatalf("loading the login page: %v", err)
	}
	body := readAll(t, page)
	token := betweenQuotes(body, `name="`+web.CSRFFieldName+`" value="`)
	if token == "" {
		t.Fatal("the login page carried no csrf token")
	}

	form := url.Values{"email": {email}, "password": {password}, web.CSRFFieldName: {token}}
	res, err := client.PostForm(base+web.RouteLogin, form)
	if err != nil {
		t.Fatalf("posting credentials: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("sign in = %d, want 303: %s", res.StatusCode, readAll(t, res))
	}
	return client
}

func readAll(t *testing.T, res *http.Response) string {
	t.Helper()
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(b)
}

func betweenQuotes(body, prefix string) string {
	i := strings.Index(body, prefix)
	if i < 0 {
		return ""
	}
	rest := body[i+len(prefix):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// A signed-in browser must actually reach its pages. Two separate faults each
// broke this while every unit test stayed green, because the unit tests put an
// actor into the context by hand and so exercised neither the middleware that
// resolves the session cookie nor the lookup that reads the tenant role.
func TestSignedInBrowserReachesItsPages(t *testing.T) {
	h := newHarness(t)
	const password = "correct-horse-battery-staple"
	if _, err := h.server.CreateUser(h.adminCtx, core.CreateUserInput{
		Email: "human@example.test", Handle: "human", Password: password, Role: core.RoleAdmin,
	}); err != nil {
		t.Fatalf("creating the browser user: %v", err)
	}

	client := signIn(t, h.baseURL, "human@example.test", password)

	for _, path := range []string{web.RouteProjects, web.RouteTasks, web.RouteWorkflows} {
		res, err := client.Get(h.baseURL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		status := res.StatusCode
		body := readAll(t, res)
		switch status {
		case http.StatusOK:
		case http.StatusSeeOther:
			t.Errorf("GET %s redirected a signed-in browser back to sign-in; the session did not survive the request", path)
		case http.StatusForbidden:
			t.Errorf("GET %s = 403 for an admin; the session carries no role", path)
		default:
			t.Errorf("GET %s = %d, want 200: %s", path, status, body)
		}
	}
}

// An anonymous browser is redirected to the login page, while the API answers
// a credential-free request with an error rather than a redirect.
func TestAnonymousVisitorIsRedirectedAndTheAPIIsRefused(t *testing.T) {
	h := newHarness(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	res, err := client.Get(h.baseURL + web.RouteProjects)
	if err != nil {
		t.Fatalf("anonymous GET: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("anonymous browser GET = %d, want a redirect to sign in", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); !strings.HasPrefix(loc, web.RouteLogin) {
		t.Errorf("anonymous browser was sent to %q, want the login page", loc)
	}

	api, err := client.Get(h.baseURL + httpapi.RouteTasks)
	if err != nil {
		t.Fatalf("anonymous API GET: %v", err)
	}
	_ = api.Body.Close()
	if api.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous API GET = %d, want 401", api.StatusCode)
	}
}
