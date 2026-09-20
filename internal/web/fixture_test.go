package web_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/service"
	"github.com/thereisnotime/tix/internal/store"
	"github.com/thereisnotime/tix/internal/store/sqlite"
	"github.com/thereisnotime/tix/internal/web"
)

// webService completes core.Service with stand-ins for the transfer and sync
// groups, which other packages are still implementing.
type webService struct {
	*service.Local
	sources    map[string]*core.SyncSource
	exported   string
	lastImport core.ImportInput
}

var _ core.Service = (*webService)(nil)

// newWebService wraps the real local service.
func newWebService(l *service.Local) *webService {
	return &webService{Local: l, sources: map[string]*core.SyncSource{},
		exported: `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n"}
}

func (s *webService) ExportTo(ctx context.Context, _ core.ExportInput, w io.Writer) error {
	if _, err := core.RequireActor(ctx); err != nil {
		return err
	}
	_, err := io.WriteString(w, s.exported)
	return err
}

func (s *webService) ImportFrom(ctx context.Context, r io.Reader, in core.ImportInput) (*core.ImportResult, error) {
	if _, err := core.RequireActor(ctx); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, core.Invalid("reading snapshot: %v", err)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, core.Invalid("snapshot is empty")
	}
	s.lastImport = in
	return &core.ImportResult{Created: map[string]int{"task": 2}, DryRun: in.DryRun}, nil
}

func (s *webService) PutSyncSource(ctx context.Context, in core.SyncSourceInput) (*core.SyncSource, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	id := in.ID
	if id == "" {
		id = "src-" + in.Name
	}
	src := &core.SyncSource{ID: id, TenantID: actor.TenantID, System: in.System, Name: in.Name}
	s.sources[id] = src
	return src, nil
}

func (s *webService) ListSyncSources(ctx context.Context) ([]core.SyncSource, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	out := []core.SyncSource{}
	for _, src := range s.sources {
		if src.TenantID == actor.TenantID {
			out = append(out, *src)
		}
	}
	return out, nil
}

func (s *webService) DeleteSyncSource(ctx context.Context, id string) error {
	if _, err := core.RequireActor(ctx); err != nil {
		return err
	}
	if _, ok := s.sources[id]; !ok {
		return core.NotFound("sync source %q", id)
	}
	delete(s.sources, id)
	return nil
}

func (s *webService) RunSync(ctx context.Context, in core.RunSyncInput) (*core.SyncResult, error) {
	if _, err := core.RequireActor(ctx); err != nil {
		return nil, err
	}
	src, ok := s.sources[in.SourceID]
	if !ok {
		return nil, core.NotFound("sync source %q", in.SourceID)
	}
	return &core.SyncResult{System: src.System, Source: src.ID,
		ImportResult: core.ImportResult{Created: map[string]int{"task": 1}, DryRun: in.DryRun}}, nil
}

// fixture is the browser interface over a real service on a temporary database.
type fixture struct {
	t       *testing.T
	server  *httptest.Server
	svc     *webService
	store   *sqlite.Store
	clock   *clock.Fake
	tenantA core.Tenant
	tenantB core.Tenant
	actorA  core.Actor
	actorB  core.Actor
	project core.Project
}

// contextActor is the actor a test request speaks for, chosen by a header the
// fixture's stand-in for the API middleware reads.
const actorHeader = "X-Test-Actor"

// newFixture builds two tenants, each with a project, behind the web handler.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFakeAt()

	st, err := sqlite.Open(filepath.Join(t.TempDir(), "tix.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	f := &fixture{t: t, store: st, clock: clk}
	f.tenantA = seedTenant(t, st, "acme", "Acme Works")
	f.tenantB = seedTenant(t, st, "other", "Other Ltd")
	local := service.New(st, service.WithClock(clk), service.WithHooks(service.HookOff))
	f.svc = newWebService(local)
	f.actorA = seedActor(t, st, f.tenantA.ID, "alice", core.RoleAdmin)
	f.actorB = seedActor(t, st, f.tenantB.ID, "bob", core.RoleAdmin)
	f.project = seedProject(t, st, f.tenantA.ID, "infra")
	seedProject(t, st, f.tenantB.ID, "other")

	handler := web.Handler(f.svc, web.WithEventsPath("/api/v1/events"))
	f.server = httptest.NewServer(f.authenticate(handler))
	t.Cleanup(f.server.Close)
	return f
}

// authenticate stands in for the API middleware, which resolves the tenant and
// the actor before the browser interface sees a request.
func (f *fixture) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithTenant(r.Context(), core.TenantScope{TenantID: f.tenantA.ID})
		if actor := f.actorFor(r.Header.Get(actorHeader)); actor != nil {
			ctx = core.WithActor(ctx, actor)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// actorFor resolves the actor a test request names.
func (f *fixture) actorFor(name string) *core.Actor {
	switch name {
	case "":
		return nil
	case "alice":
		return copyActor(f.actorA, []core.Scope{core.ScopeAll}, core.RoleAdmin)
	case "bob":
		return copyActor(f.actorB, []core.Scope{core.ScopeAll}, core.RoleAdmin)
	case "viewer":
		return copyActor(f.actorA, core.RoleViewer.Scopes(), core.RoleViewer)
	default:
		return nil
	}
}

// copyActor builds a request actor with the given authority.
func copyActor(a core.Actor, scopes []core.Scope, role core.Role) *core.Actor {
	out := a
	out.Scopes = scopes
	out.Role = role
	return &out
}

func seedTenant(t *testing.T, st *sqlite.Store, key, name string) core.Tenant {
	t.Helper()
	ctx := context.Background()
	tenant := core.Tenant{Key: key, Name: name}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating tenant %q: %v", key, err)
	}
	return tenant
}

func seedActor(t *testing.T, st *sqlite.Store, tenantID, handle string, role core.Role) core.Actor {
	t.Helper()
	ctx := context.Background()
	actor := core.Actor{Kind: core.ActorUser, Handle: handle, Role: role,
		Scopes: []core.Scope{core.ScopeAll}}
	if err := st.Update(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("creating actor %q: %v", handle, err)
	}
	actor.TenantID = tenantID
	return actor
}

func seedProject(t *testing.T, st *sqlite.Store, tenantID, key string) core.Project {
	t.Helper()
	ctx := context.Background()
	wf := core.Workflow{Key: "default", Name: "Default", Definition: service.BuiltinWorkflow()}
	project := core.Project{Key: key, Name: strings.ToUpper(key)}
	if err := st.Update(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		project.WorkflowID = wf.ID
		return tx.CreateProject(ctx, &project)
	}); err != nil {
		t.Fatalf("seeding project %q: %v", key, err)
	}
	return project
}

// browser is one test client with its own cookie jar, as a browser has.
type browser struct {
	t      *testing.T
	fix    *fixture
	client *http.Client
	actor  string
}

// as returns a browser signed in as the named actor.
func (f *fixture) as(actor string) *browser {
	f.t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		f.t.Fatalf("building cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	return &browser{t: f.t, fix: f, client: client, actor: actor}
}

// get fetches one page.
func (b *browser) get(path string) *http.Response {
	b.t.Helper()
	req, err := http.NewRequest(http.MethodGet, b.fix.server.URL+path, nil)
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("getting %s: %v", path, err)
	}
	return resp
}

// page fetches one page and returns its body.
func (b *browser) page(path string) string {
	b.t.Helper()
	resp := b.get(path)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b.t.Fatalf("GET %s = %d, want 200", path, resp.StatusCode)
	}
	return readAll(b.t, resp)
}

// post submits a form the way a browser without JavaScript does, carrying the
// CSRF value the page it was rendered from issued.
func (b *browser) post(path string, form url.Values) *http.Response {
	b.t.Helper()
	form.Set("csrf_token", b.csrf())
	return b.postRaw(path, form)
}

// postRaw submits a form exactly as given, without adding a CSRF value.
func (b *browser) postRaw(path string, form url.Values) *http.Response {
	b.t.Helper()
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+path,
		strings.NewReader(form.Encode()))
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("posting %s: %v", path, err)
	}
	return resp
}

// postMultipart submits a form carrying an uploaded file.
func (b *browser) postMultipart(path string, fields map[string]string, filename, content string) *http.Response {
	b.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields["csrf_token"] = b.csrf()
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			b.t.Fatalf("writing field %q: %v", name, err)
		}
	}
	if filename != "" {
		part, err := writer.CreateFormFile("snapshot", filename)
		if err != nil {
			b.t.Fatalf("creating file part: %v", err)
		}
		if _, err := io.WriteString(part, content); err != nil {
			b.t.Fatalf("writing file part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		b.t.Fatalf("closing multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+path, &body)
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("posting %s: %v", path, err)
	}
	return resp
}

// stamp names the actor the request speaks for.
func (b *browser) stamp(req *http.Request) {
	if b.actor != "" {
		req.Header.Set(actorHeader, b.actor)
	}
}

// csrf returns the value the browser's current CSRF cookie carries, fetching a
// page first when it holds none.
func (b *browser) csrf() string {
	b.t.Helper()
	if value := b.cookie(web.CSRFCookieName); value != "" {
		return value
	}
	resp := b.get("/login")
	_ = resp.Body.Close()
	return b.cookie(web.CSRFCookieName)
}

// cookie reads one cookie from the browser's jar.
func (b *browser) cookie(name string) string {
	parsed, err := url.Parse(b.fix.server.URL)
	if err != nil {
		b.t.Fatalf("parsing server url: %v", err)
	}
	for _, c := range b.client.Jar.Cookies(parsed) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// readAll drains a response body.
func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(body)
}

// body drains a response body and closes it.
func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	return readAll(t, resp)
}

// wantStatus fails unless the response carries the expected status.
func wantStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, want, body(t, resp))
	}
}

// createTask creates a task through the browser and returns its reference.
func (b *browser) createTask(project, title string) string {
	b.t.Helper()
	resp := b.post("/tasks", url.Values{
		"project_ref": {project}, "title": {title}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(b.t, resp, http.StatusSeeOther)
	location := resp.Header.Get("Location")
	ref := strings.TrimPrefix(location, "/tasks/")
	if idx := strings.Index(ref, "?"); idx >= 0 {
		ref = ref[:idx]
	}
	if ref == "" {
		b.t.Fatalf("no task reference in %q", location)
	}
	return ref
}

// refPattern finds a task reference in a redirect target.
