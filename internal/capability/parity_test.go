package capability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/cmd"
	"github.com/thereisnotime/tix/internal/auth"
	"github.com/thereisnotime/tix/internal/capability"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
	"github.com/thereisnotime/tix/internal/web"
)

// serviceMethods returns every exported method of the frozen service contract.
func serviceMethods() []string {
	iface := reflect.TypeOf((*core.Service)(nil)).Elem()
	out := make([]string, 0, iface.NumMethod())
	for i := range iface.NumMethod() {
		out = append(out, iface.Method(i).Name)
	}
	return out
}

func TestRegistryCoversEveryServiceMethod(t *testing.T) {
	t.Parallel()
	declared := map[string]int{}
	for _, op := range capability.Operations() {
		declared[op.Method]++
	}
	for _, name := range serviceMethods() {
		switch declared[name] {
		case 1:
		case 0:
			t.Errorf("service method %q has no capability registry entry", name)
		default:
			t.Errorf("service method %q has %d registry entries", name, declared[name])
		}
	}
	known := map[string]bool{}
	for _, name := range serviceMethods() {
		known[name] = true
	}
	for _, op := range capability.Operations() {
		if !known[op.Method] {
			t.Errorf("operation %q names method %q, which the service does not declare", op.Name, op.Method)
		}
	}
	if len(capability.Operations()) != len(serviceMethods()) {
		t.Errorf("registry holds %d operations for %d service methods",
			len(capability.Operations()), len(serviceMethods()))
	}
}

func TestEveryOperationBindsOrIsExempt(t *testing.T) {
	t.Parallel()
	for _, op := range capability.Operations() {
		for _, surface := range capability.Bound {
			exemption, exempted := op.ExemptionFor(surface)
			switch {
			case op.Binds(surface) && exempted:
				t.Errorf("%s: %s binding is both declared and exempted", op.Name, surface)
			case op.Binds(surface):
			case !exempted:
				t.Errorf("%s: no %s binding and no exemption", op.Name, surface)
			case strings.TrimSpace(exemption.Reason) == "":
				t.Errorf("%s: %s exemption carries no justification", op.Name, surface)
			}
		}
	}
}

func TestOperationsAreWellFormed(t *testing.T) {
	t.Parallel()
	names := map[string]bool{}
	for _, op := range capability.Operations() {
		if op.Name == "" {
			t.Errorf("operation for %q has no name", op.Method)
		}
		if names[op.Name] {
			t.Errorf("operation name %q is declared twice", op.Name)
		}
		names[op.Name] = true
		if op.CLI != "" && !strings.HasPrefix(op.CLI, "tix ") {
			t.Errorf("%s: cli binding %q is not a tix command path", op.Name, op.CLI)
		}
		for _, route := range []capability.Route{op.HTTP, op.Web} {
			if route.Zero() {
				continue
			}
			if route.Method == "" || !strings.HasPrefix(route.Pattern, "/") {
				t.Errorf("%s: route %q %q is malformed", op.Name, route.Method, route.Pattern)
			}
		}
		if op.HTTP.Template != "" {
			t.Errorf("%s: an http route renders no template", op.Name)
		}
		for _, e := range op.Exempt {
			if e.Gap && !strings.HasPrefix(e.Reason, "GAP:") {
				t.Errorf("%s: gap on %s is not marked in its reason", op.Name, e.Surface)
			}
			if !e.Gap && strings.HasPrefix(e.Reason, "GAP:") {
				t.Errorf("%s: %s reason reads as a gap but is not flagged", op.Name, e.Surface)
			}
		}
	}
	if len(capability.Gaps()) > len(capability.Exemptions()) {
		t.Fatal("gaps outnumber the exemptions they are drawn from")
	}
}

func TestLookupByMethod(t *testing.T) {
	t.Parallel()
	op, ok := capability.ByMethod("CreateTask")
	if !ok || op.Name != "task.create" {
		t.Fatalf("ByMethod(CreateTask) = %q, %v", op.Name, ok)
	}
	if _, ok := capability.ByMethod("NoSuchMethod"); ok {
		t.Fatal("ByMethod resolved a method the service does not declare")
	}
	if op.Binds(capability.Surface("carrier pigeon")) {
		t.Fatal("an unknown surface reported a binding")
	}
	if _, ok := op.ExemptionFor(capability.SurfaceWeb); ok {
		t.Fatal("task.create reported a web exemption")
	}
}

func TestEveryCLIBindingResolvesAgainstTheCommandTree(t *testing.T) {
	t.Parallel()
	for _, op := range capability.Operations() {
		if op.CLI == "" {
			continue
		}
		if code, usage := runCLI(t, op.CLI); code != core.ExitOK || !strings.Contains(usage, op.CLI) {
			t.Errorf("%s: cli binding %q is not a command (exit %d)", op.Name, op.CLI, code)
		}
	}
	for _, invented := range []string{"tix no-such-command", "tix task append"} {
		if code, usage := runCLI(t, invented); code == core.ExitOK && strings.Contains(usage, invented) {
			t.Fatalf("invented command %q resolved against the command tree", invented)
		}
	}
}

// runCLI asks the real command tree for a command's help, which cobra answers
// from the nearest command it can resolve, so the usage line has to name the
// whole path for the command to exist.
func runCLI(t *testing.T, path string) (int, string) {
	t.Helper()
	fields := strings.Fields(path)[1:]
	args := make([]string, 0, len(fields)+1)
	args = append(append(args, fields...), "--help")
	var out, errw strings.Builder
	code := cmd.Run(args, strings.NewReader(""), &out, &errw, []string{}, t.TempDir())
	return code, out.String()
}

func TestEveryHTTPBindingResolvesAgainstTheServedMux(t *testing.T) {
	t.Parallel()
	router := newRouter(t)
	for _, op := range capability.Operations() {
		if op.HTTP.Zero() {
			continue
		}
		status := probe(router, op.HTTP.Method, concretePath(op.HTTP.Pattern))
		switch status {
		case http.StatusNotFound:
			t.Errorf("%s: http binding %s %s is served by no route", op.Name, op.HTTP.Method, op.HTTP.Pattern)
		case http.StatusMethodNotAllowed:
			t.Errorf("%s: http binding %s %s is served under another method",
				op.Name, op.HTTP.Method, op.HTTP.Pattern)
		}
	}
	if status := probe(router, http.MethodGet, "/api/v1/nowhere"); status != http.StatusNotFound {
		t.Fatalf("an unregistered path answered %d, so routing is not being checked", status)
	}
}

// newRouter builds the real router over a service that is never reached, so a
// request reports only whether it was routed.
func newRouter(t *testing.T) *httpapi.Router {
	t.Helper()
	actor := &core.Actor{ID: "u1", TenantID: "t1", Kind: core.ActorUser, Scopes: core.AllScopes}
	router, err := httpapi.New(httpapi.Config{
		Service:         stubService{},
		Authenticator:   auth.NewStaticAuthenticator(actor),
		Resolver:        stubResolver{},
		DefaultTenantID: "t1",
		EventHandler:    okHandler(),
	})
	if err != nil {
		t.Fatalf("building router: %v", err)
	}
	return router
}

// okHandler stands in for the WebSocket handler, which the router registers
// only when one is supplied.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

// probe reports the status the handler answers a request with.
func probe(handler http.Handler, method, path string) int {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec.Code
}

// concretePath fills a pattern's wildcards with usable values.
func concretePath(pattern string) string {
	r := strings.NewReplacer(
		"{ref}", "infra-1",
		"{dep}", "infra-2",
		"{key}", "infra",
		"{id}", "abc",
		"{name}", "urgent",
		"{hostname}", "acme.example",
		"{actorID}", "u1",
	)
	return r.Replace(pattern)
}

// stubService satisfies the contract without implementing it; every call
// panics, and the router recovers a panic as an internal error.
type stubService struct{ core.Service }

// stubResolver serves no host, which leaves the configured default tenant.
type stubResolver struct{}

func (stubResolver) ResolveDomain(context.Context, string) (*core.Tenant, error) {
	return nil, core.NotFound("no tenant serves this host")
}

func TestEveryWebBindingResolvesAgainstTheBrowserRoutes(t *testing.T) {
	t.Parallel()
	declared := map[string][]string{}
	for _, b := range web.Bindings() {
		key := b.Method + " " + b.Pattern
		declared[key] = append(declared[key], b.Services...)
		if b.Template != "" && !web.HasTemplate(b.Template) {
			t.Errorf("web route %q names missing template %q", key, b.Template)
		}
	}

	handler := web.Handler(stubService{})
	for _, op := range capability.Operations() {
		if op.Web.Zero() {
			continue
		}
		key := op.Web.Method + " " + op.Web.Pattern
		services, ok := declared[key]
		if !ok {
			t.Errorf("%s: web binding %q is served by no browser route", op.Name, key)
			continue
		}
		if !contains(services, op.Method) {
			t.Errorf("%s: browser route %q does not expose %q", op.Name, key, op.Method)
		}
		if op.Web.Template != "" && !web.HasTemplate(op.Web.Template) {
			t.Errorf("%s: web binding names missing template %q", op.Name, op.Web.Template)
		}
		if status := probe(handler, op.Web.Method, concretePath(op.Web.Pattern)); status == http.StatusNotFound {
			t.Errorf("%s: web binding %q is served by no route", op.Name, key)
		}
	}
	// The browser mux serves "GET /" as the root screen, so only a method it
	// registers for no pattern proves an unbound path is not dispatched.
	if status := probe(handler, http.MethodPost, "/nowhere-at-all"); status < http.StatusBadRequest {
		t.Fatalf("an unregistered browser path answered %d, so routing is not being checked", status)
	}
	if web.HasTemplate("absent.html") {
		t.Fatal("a template that is not embedded reported as present")
	}
}

func TestWebExemptionsAgreeWithTheBrowserInterface(t *testing.T) {
	t.Parallel()
	absent := map[string]bool{}
	for _, e := range web.Exemptions() {
		absent[e.Service] = true
	}
	for _, op := range capability.Operations() {
		_, exempted := op.ExemptionFor(capability.SurfaceWeb)
		if exempted != absent[op.Method] {
			t.Errorf("%s: capability exempts it from the web %v, internal/web %v",
				op.Name, exempted, absent[op.Method])
		}
	}
}

// contains reports whether a slice holds a value.
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
