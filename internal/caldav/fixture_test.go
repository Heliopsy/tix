// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
)

const testOrigin = "https://tix.example"

// tenantFixture is one tenant: its administrator, a project and a member.
type tenantFixture struct {
	tenant  core.Tenant
	scope   core.TenantScope
	admin   *core.Actor
	project *core.Project
	web     *core.Project
	bob     *core.Actor
}

// fixture is a real service over a temporary SQLite store with two tenants
// that share project keys, handles and one account.
type fixture struct {
	t      *testing.T
	st     store.Store
	svc    *service.Local
	clk    *clock.Fake
	h      *Handler
	acme   tenantFixture
	globex tenantFixture
	userID string
}

// workflow reaches every state from every other and has a cancelled state.
func workflow() core.WorkflowDefinition {
	keys := []string{"todo", "doing", "blocked", "done", "cancelled"}
	def := core.WorkflowDefinition{Initial: "todo", States: []core.State{
		{Key: "todo", Label: "To do", Category: core.CategoryTodo},
		{Key: "doing", Label: "Doing", Category: core.CategoryInProgress},
		{Key: "blocked", Label: "Blocked", Category: core.CategoryBlocked},
		{Key: "done", Label: "Done", Terminal: true, Category: core.CategoryDone},
		{Key: "cancelled", Label: "Cancelled", Terminal: true, Category: core.CategoryCancelled},
	}}
	for _, from := range keys {
		for _, to := range keys {
			if from != to {
				def.Transitions = append(def.Transitions, core.Transition{From: from, To: to})
			}
		}
	}
	return def
}

func newFixture(t *testing.T, opts ...service.Option) *fixture {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(sqlitetest.Path(t), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	all := append([]service.Option{service.WithClock(clk),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams()))}, opts...)
	f := &fixture{t: t, st: st, svc: service.New(st, all...), clk: clk}
	f.h = New(f.svc, func(*http.Request) string { return testOrigin })
	f.acme = f.tenant("acme")
	f.globex = f.tenant("globex")

	user, err := f.svc.CreateUser(core.WithActor(ctx, f.acme.admin), core.CreateUserInput{Email: "bob@example.com",
		Handle: "bob", DisplayName: "Bob", Role: core.RoleMember})
	if err != nil {
		t.Fatalf("creating bob: %v", err)
	}
	f.userID = user.ID
	f.acme.bob = &core.Actor{ID: user.ID, TenantID: f.acme.scope.TenantID, Kind: core.ActorUser, Handle: "bob",
		Role: core.RoleMember}
	linked := core.Actor{UserID: user.ID, Kind: core.ActorUser, Handle: "bob", DisplayName: "Bob"}
	if err := st.Update(ctx, f.globex.scope, func(tx store.Tx) error {
		if err := tx.CreateActor(ctx, &linked); err != nil {
			return err
		}
		return tx.AddMember(ctx, &core.Membership{ActorID: linked.ID, Role: core.RoleMember})
	}); err != nil {
		t.Fatalf("linking bob into globex: %v", err)
	}
	f.globex.bob = &core.Actor{ID: linked.ID, TenantID: f.globex.scope.TenantID, Kind: core.ActorUser,
		Handle: "bob", Role: core.RoleMember}
	return f
}

// tenant creates a tenant with an administrator and two projects.
func (f *fixture) tenant(key string) tenantFixture {
	f.t.Helper()
	ctx := context.Background()
	out := tenantFixture{tenant: core.Tenant{Key: key, Name: strings.ToUpper(key)}}
	if err := f.st.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, &out.tenant) }); err != nil {
		f.t.Fatalf("creating tenant %s: %v", key, err)
	}
	out.scope = core.TenantScope{TenantID: out.tenant.ID}
	admin := core.Actor{Kind: core.ActorUser, Handle: "alice", Scopes: []core.Scope{core.ScopeAll}}
	wf := core.Workflow{Key: "wf", Name: "Workflow", Definition: workflow()}
	infra := core.Project{Key: "infra", Name: "Infra", Color: "#336699"}
	web := core.Project{Key: "web", Name: "Web"}
	if err := f.st.Update(ctx, out.scope, func(tx store.Tx) error {
		if err := tx.CreateActor(ctx, &admin); err != nil {
			return err
		}
		if err := tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		infra.WorkflowID, web.WorkflowID = wf.ID, wf.ID
		if err := tx.CreateProject(ctx, &infra); err != nil {
			return err
		}
		return tx.CreateProject(ctx, &web)
	}); err != nil {
		f.t.Fatalf("seeding tenant %s: %v", key, err)
	}
	admin.TenantID = out.tenant.ID
	out.admin, out.project, out.web = &admin, &infra, &web
	return out
}

// adminCtx is a context acting as a tenant's administrator.
func adminCtx(tf tenantFixture) context.Context {
	return core.WithSource(core.WithActor(context.Background(), tf.admin), core.SourceCLI)
}

// password issues an app password for a person.
func (f *fixture) password(person *core.Actor, collections core.DAVCollections) *core.IssuedDAVPassword {
	f.t.Helper()
	ctx := core.WithSource(core.WithActor(context.Background(), person), core.SourceCLI)
	p, err := f.svc.CreateDAVPassword(ctx, core.CreateDAVPasswordInput{Name: "phone " + string(collections),
		Collections: collections})
	if err != nil {
		f.t.Fatalf("creating app password: %v", err)
	}
	return p
}

// davCtx is the context the middleware builds for a request signed in with
// an app password.
func davCtx(tenantID string, p *core.IssuedDAVPassword) context.Context {
	a := &core.Actor{ID: p.ActorID, TenantID: tenantID, Kind: core.ActorAgent, Handle: p.TokenName,
		Scopes: []core.Scope{core.ScopeCaldav}, TokenID: p.ID, TokenName: p.TokenName}
	return core.WithDAVRequest(core.WithSource(core.WithActor(context.Background(), a), core.SourceAPI))
}

// task creates a task in a tenant's infra project, assigned to whoever is
// named.
func (f *fixture) task(tf tenantFixture, title, assignee string) *core.Task {
	f.t.Helper()
	task, err := f.svc.CreateTask(adminCtx(tf), core.CreateTaskInput{Title: title, ProjectRef: tf.project.Key,
		AssigneeActorID: assignee})
	if err != nil {
		f.t.Fatalf("creating task %q: %v", title, err)
	}
	return task
}

// move transitions a task.
func (f *fixture) move(tf tenantFixture, task *core.Task, to string) {
	f.t.Helper()
	if _, err := f.svc.TransitionTask(adminCtx(tf), core.TaskRef{ID: task.ID}, core.TransitionInput{To: to}); err != nil {
		f.t.Fatalf("moving %s to %s: %v", task.Ref, to, err)
	}
}

// reply is a recorded answer.
type reply struct {
	status int
	header http.Header
	body   string
}

// serve sends one request into the handler with ctx as its context.
func (f *fixture) serve(ctx context.Context, method, path string, header map[string]string, body string) reply {
	f.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequestWithContext(ctx, method, path, rd)
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return reply{status: w.Code, header: w.Header(), body: w.Body.String()}
}

// name is a task's resource name.
func name(t *core.Task) string { return strings.ToLower(t.ID) }

// home is a person's calendar home path.
func home(tf tenantFixture, a *core.Actor) string {
	return "/dav/" + tf.tenant.Key + "/calendars/" + strings.ToLower(a.ID) + "/"
}

// msDoc is a multistatus as a test reads it.
type msDoc struct {
	Responses []msResponse `xml:"DAV: response"`
	SyncToken string       `xml:"DAV: sync-token"`
}

type msResponse struct {
	Href      string       `xml:"DAV: href"`
	Status    string       `xml:"DAV: status"`
	Propstats []msPropstat `xml:"DAV: propstat"`
}

type msPropstat struct {
	Prop struct {
		Props []msProp `xml:",any"`
	} `xml:"DAV: prop"`
	Status string `xml:"DAV: status"`
}

type msProp struct {
	XMLName xml.Name
	Inner   string `xml:",innerxml"`
	Text    string `xml:",chardata"`
}

// parseMS reads a 207 body, failing on anything else.
func parseMS(t *testing.T, r reply) msDoc {
	t.Helper()
	if r.status != http.StatusMultiStatus {
		t.Fatalf("status %d, want 207: %s", r.status, r.body)
	}
	if ct := r.header.Get("Content-Type"); ct != xmlContentType {
		t.Errorf("content type %q", ct)
	}
	var d msDoc
	if err := xml.Unmarshal([]byte(r.body), &d); err != nil {
		t.Fatalf("parsing multistatus: %v\n%s", err, r.body)
	}
	return d
}

// response is the one response for an href.
func (d msDoc) response(t *testing.T, href string) msResponse {
	t.Helper()
	var found []msResponse
	for _, r := range d.Responses {
		if r.Href == href {
			found = append(found, r)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d responses for %s in %+v", len(found), href, d.Responses)
	}
	return found[0]
}

// hrefs lists the responses' hrefs.
func (d msDoc) hrefs() []string {
	out := make([]string, len(d.Responses))
	for i, r := range d.Responses {
		out[i] = r.Href
	}
	return out
}

// prop finds a property and the status of the propstat holding it.
func (r msResponse) prop(n xml.Name) (msProp, string, bool) {
	for _, ps := range r.Propstats {
		for _, p := range ps.Prop.Props {
			if p.XMLName == n {
				return p, ps.Status, true
			}
		}
	}
	return msProp{}, "", false
}

// okProp is a property's value, failing unless it is in a 200 propstat.
func (r msResponse) okProp(t *testing.T, n xml.Name) msProp {
	t.Helper()
	p, status, ok := r.prop(n)
	if !ok || status != "HTTP/1.1 200 OK" {
		t.Fatalf("%s of %s: found %v with %q", n.Local, r.Href, ok, status)
	}
	return p
}

// propfindBody asks for the named properties.
func propfindBody(props string) string {
	return `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" ` +
		`xmlns:cs="http://calendarserver.org/ns/" xmlns:ical="http://apple.com/ns/ical/"><d:prop>` + props +
		`</d:prop></d:propfind>`
}
