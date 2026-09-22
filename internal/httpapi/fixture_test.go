package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/migrations"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// apiFixture is a live server over a real service on a temporary database.
type apiFixture struct {
	t        *testing.T
	server   *httptest.Server
	store    *sqlite.Store
	local    *service.Local
	clock    *clock.Fake
	tenantA  core.Tenant
	tenantB  core.Tenant
	actorA   core.Actor
	actorB   core.Actor
	tokenA   string
	tokenB   string
	hostA    string
	hostB    string
	projectA core.Project
	svc      *apiService
	live     *connections.Registry
	logs     *bytes.Buffer
}

// apiService completes core.Service with in-memory stand-ins for the groups
// the local service does not implement yet.
type apiService struct {
	*service.Local
	users    map[string]*core.User
	tokens   map[string][]core.APIToken
	hooks    map[string]*core.WebhookEndpoint
	sources  map[string]*core.SyncSource
	sessions map[string]*core.Session
	block    chan struct{}
}

var _ core.Service = (*apiService)(nil)

func newAPIService(l *service.Local) *apiService {
	return &apiService{
		Local:    l,
		users:    map[string]*core.User{},
		tokens:   map[string][]core.APIToken{},
		hooks:    map[string]*core.WebhookEndpoint{},
		sources:  map[string]*core.SyncSource{},
		sessions: map[string]*core.Session{},
	}
}

func (s *apiService) CreateUser(ctx context.Context, in core.CreateUserInput) (*core.User, error) {
	if _, err := core.RequireActor(ctx); err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	u := &core.User{ID: "user-" + in.Email, Email: in.Email, DisplayName: in.DisplayName}
	s.users[u.ID] = u
	return u, nil
}

func (s *apiService) GetUser(_ context.Context, id string) (*core.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, core.NotFound("user %q", id)
	}
	return u, nil
}

func (s *apiService) ListUsers(_ context.Context, _ core.Page) ([]core.User, string, error) {
	out := make([]core.User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, *u)
	}
	return out, "", nil
}

func (s *apiService) UpdateUser(_ context.Context, id string, in core.UpdateUserInput) (*core.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, core.NotFound("user %q", id)
	}
	if in.DisplayName != nil {
		u.DisplayName = *in.DisplayName
	}
	return u, nil
}

func (s *apiService) DeleteUser(_ context.Context, id string) error {
	if _, ok := s.users[id]; !ok {
		return core.NotFound("user %q", id)
	}
	delete(s.users, id)
	return nil
}

func (s *apiService) Login(_ context.Context, email, password string) (*core.Session, error) {
	if email == "" || password == "" {
		return nil, core.Invalid("email and password are required")
	}
	if password != "correct-horse-battery" {
		return nil, core.Unauthenticated("invalid credentials")
	}
	sess := &core.Session{
		Token:     "session-secret",
		ActorID:   "actor-" + email,
		TenantID:  "tenant",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	s.sessions[sess.Token] = sess
	return sess, nil
}

func (s *apiService) Logout(ctx context.Context) error {
	_, err := core.RequireActor(ctx)
	return err
}

func (s *apiService) CreateToken(ctx context.Context, in core.CreateTokenInput) (*core.IssuedToken, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	tok := core.APIToken{ID: "tok-" + in.Name, TenantID: actor.TenantID, ActorID: actor.ID,
		Name: in.Name, Scopes: in.Scopes}
	s.tokens[actor.ID] = append(s.tokens[actor.ID], tok)
	return &core.IssuedToken{APIToken: tok, Token: "tix_pat_secret"}, nil
}

func (s *apiService) ListTokens(_ context.Context, actorID string) ([]core.APIToken, error) {
	return s.tokens[actorID], nil
}

func (s *apiService) RevokeToken(_ context.Context, id string) error {
	for actorID, list := range s.tokens {
		for i, tok := range list {
			if tok.ID == id {
				s.tokens[actorID] = append(list[:i], list[i+1:]...)
				return nil
			}
		}
	}
	return core.NotFound("token %q", id)
}

func (s *apiService) PutWebhook(ctx context.Context, in core.WebhookInput) (*core.WebhookEndpoint, error) {
	if _, err := core.RequireActor(ctx); err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	id := in.ID
	if id == "" {
		id = "hook-1"
	}
	hook := &core.WebhookEndpoint{ID: id, URL: in.URL, EventTypes: in.EventTypes, Active: in.Active}
	s.hooks[id] = hook
	return hook, nil
}

func (s *apiService) ListWebhooks(_ context.Context) ([]core.WebhookEndpoint, error) {
	out := make([]core.WebhookEndpoint, 0, len(s.hooks))
	for _, h := range s.hooks {
		out = append(out, *h)
	}
	return out, nil
}

func (s *apiService) DeleteWebhook(_ context.Context, id string) error {
	if _, ok := s.hooks[id]; !ok {
		return core.NotFound("webhook %q", id)
	}
	delete(s.hooks, id)
	return nil
}

func (s *apiService) ListDeliveries(_ context.Context, _ core.DeliveryFilter) ([]core.WebhookDelivery, string, error) {
	return nil, "", nil
}

func (s *apiService) RedeliverWebhook(_ context.Context, id string) error {
	return core.NotFound("delivery %q", id)
}

func (s *apiService) ExportTo(_ context.Context, _ core.ExportInput, w io.Writer) error {
	return json.NewEncoder(w).Encode(core.SnapshotRecord{Kind: core.RecordHeader,
		Header: &core.SnapshotHeader{Version: core.SnapshotVersion, TenantKey: "acme"}})
}

func (s *apiService) ImportFrom(_ context.Context, r io.Reader, in core.ImportInput) (*core.ImportResult, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, core.Invalid("reading snapshot: %v", err)
	}
	if len(b) == 0 {
		return nil, core.Invalid("snapshot is empty")
	}
	return &core.ImportResult{Created: map[string]int{"task": 1}, DryRun: in.DryRun}, nil
}

func (s *apiService) PutSyncSource(ctx context.Context, in core.SyncSourceInput) (*core.SyncSource, error) {
	if _, err := core.RequireActor(ctx); err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	id := in.ID
	if id == "" {
		id = "src-1"
	}
	src := &core.SyncSource{ID: id, System: in.System, Name: in.Name}
	s.sources[id] = src
	return src, nil
}

func (s *apiService) ListSyncSources(_ context.Context) ([]core.SyncSource, error) {
	out := make([]core.SyncSource, 0, len(s.sources))
	for _, src := range s.sources {
		out = append(out, *src)
	}
	return out, nil
}

func (s *apiService) DeleteSyncSource(_ context.Context, id string) error {
	if _, ok := s.sources[id]; !ok {
		return core.NotFound("sync source %q", id)
	}
	delete(s.sources, id)
	return nil
}

func (s *apiService) RunSync(_ context.Context, in core.RunSyncInput) (*core.SyncResult, error) {
	src, ok := s.sources[in.SourceID]
	if !ok {
		return nil, core.NotFound("sync source %q", in.SourceID)
	}
	return &core.SyncResult{System: src.System, Source: src.ID,
		ImportResult: core.ImportResult{DryRun: in.DryRun}}, nil
}

// mapTokens resolves minted tokens for the test authenticator.
type mapTokens struct {
	byHash map[string]*core.APIToken
}

func (m mapTokens) TokenByHash(_ context.Context, hash string) (*core.APIToken, error) {
	return m.byHash[hash], nil
}

// ListTags blocks while the fixture is exercising the request timeout.
func (s *apiService) ListTags(ctx context.Context) ([]core.Tag, error) {
	if s.block != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.block:
		}
	}
	return s.Local.ListTags(ctx)
}

// newFixture builds two tenants, an agent token in each, and a live server.
func newFixture(t *testing.T) *apiFixture { return newFixtureWith(t) }

// newFixtureWith builds a fixture whose router configuration the caller may
// adjust before the server starts.
func newFixtureWith(t *testing.T, mutators ...func(*apiFixture, *httpapi.Config)) *apiFixture {
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

	f := &apiFixture{t: t, store: st, clock: clk, hostA: "acme.test", hostB: "other.test"}
	f.tenantA = seedTenant(t, st, "acme", "Acme", f.hostA)
	f.tenantB = seedTenant(t, st, "other", "Other", f.hostB)

	f.live = connections.New(connections.WithClock(clk), connections.WithServerID("srv-test"))
	f.local = service.New(st, service.WithClock(clk), service.WithHooks(service.HookOff),
		service.WithConnections(f.live))
	f.actorA = seedActor(t, st, f.tenantA.ID, "alice")
	f.actorB = seedActor(t, st, f.tenantB.ID, "bob")

	lookup := mapTokens{byHash: map[string]*core.APIToken{}}
	f.tokenA = mintToken(t, clk, lookup, f.tenantA.ID, f.actorA.ID)
	f.tokenB = mintToken(t, clk, lookup, f.tenantB.ID, f.actorB.ID)

	svc := newAPIService(f.local)
	latest, err := migrations.Latest()
	if err != nil {
		t.Fatalf("reading migrations: %v", err)
	}
	f.svc = svc
	f.logs = &bytes.Buffer{}
	cfg := httpapi.Config{
		Service:        svc,
		Authenticator:  auth.NewChain(auth.NewBearerAuthenticator(auth.NewTokenVerifier(lookup, clk))),
		Probe:          st,
		ExpectedSchema: latest,
		MaxBodyBytes:   4096,
		Logger:         slog.New(slog.NewTextHandler(f.logs, nil)),
	}
	for _, m := range mutators {
		m(f, &cfg)
	}
	router, err := httpapi.New(cfg)
	if err != nil {
		t.Fatalf("building router: %v", err)
	}
	f.server = httptest.NewServer(router)
	t.Cleanup(f.server.Close)

	f.projectA = seedProject(t, st, f.tenantA.ID, "infra")
	return f
}

func seedTenant(t *testing.T, st *sqlite.Store, key, name, host string) core.Tenant {
	t.Helper()
	ctx := context.Background()
	tenant := core.Tenant{Key: key, Name: name}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating tenant %q: %v", key, err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	domain := core.Domain{TenantID: tenant.ID, Hostname: host, CertMode: core.CertNone}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		return tx.AddDomain(ctx, &domain)
	}); err != nil {
		t.Fatalf("adding domain %q: %v", host, err)
	}
	return tenant
}

func seedActor(t *testing.T, st *sqlite.Store, tenantID, handle string) core.Actor {
	t.Helper()
	ctx := context.Background()
	actor := core.Actor{Kind: core.ActorUser, Handle: handle, Role: core.RoleAdmin,
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
	project := core.Project{Key: key, Name: key}
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

func mintToken(t *testing.T, clk *clock.Fake, lookup mapTokens, tenantID, actorID string) string {
	t.Helper()
	minted, err := auth.MintAPIToken(clk, tenantID, core.CreateTokenInput{
		Name: "agent", ActorID: actorID, Scopes: []core.Scope{core.ScopeAll},
	})
	if err != nil {
		t.Fatalf("minting token: %v", err)
	}
	stored := minted.Issued.APIToken
	lookup.byHash[minted.Hash] = &stored
	return minted.Issued.Token
}

// do sends a request to the fixture server with the given host and credential.
func (f *apiFixture) do(method, path, host, token string, body any) *http.Response {
	f.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			f.t.Fatalf("marshalling body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, f.server.URL+path, reader)
	if err != nil {
		f.t.Fatalf("building request: %v", err)
	}
	req.Host = host
	if body != nil {
		req.Header.Set(httpapi.HeaderContentType, httpapi.ContentJSON)
	}
	if token != "" {
		req.Header.Set(httpapi.HeaderAuth, "Bearer "+token)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatalf("sending %s %s: %v", method, path, err)
	}
	return resp
}

// call sends an authenticated request as tenant A.
func (f *apiFixture) call(method, path string, body any) *http.Response {
	f.t.Helper()
	return f.do(method, path, f.hostA, f.tokenA, body)
}

// decodeBody reads a JSON response body into v.
func decodeBody(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
}

// readBody drains a response body as text.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	return string(b)
}

// mustStatus fails unless the response carries the expected status.
func mustStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, want, readBody(t, resp))
	}
}

// createTask creates a task through the API and returns it.
func (f *apiFixture) createTask(title string) core.Task {
	f.t.Helper()
	resp := f.call(http.MethodPost, httpapi.RouteTasks,
		core.CreateTaskInput{ProjectRef: "infra", Title: title})
	defer func() { _ = resp.Body.Close() }()
	mustStatus(f.t, resp, http.StatusCreated)
	var task core.Task
	decodeBody(f.t, resp, &task)
	return task
}

// newRequest builds an authenticated request against tenant A's host.
func (f *apiFixture) newRequest(method, path string, body io.Reader) *http.Request {
	f.t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, body)
	if err != nil {
		f.t.Fatalf("building request: %v", err)
	}
	req.Host = f.hostA
	req.Header.Set(httpapi.HeaderAuth, "Bearer "+f.tokenA)
	return req
}
