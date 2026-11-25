// SPDX-License-Identifier: AGPL-3.0-or-later

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/wire"
)

// lockedBuffer is a log sink the server's goroutines can share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// davServer is a served process with CalDAV on and one member, ana, holding
// an app password, an account password, a session and a personal token.
type davServer struct {
	conn     *connect.Conn
	st       store.Store
	base     string
	logs     *lockedBuffer
	scope    core.TenantScope
	ana      *core.Actor
	password string
	app      *core.IssuedDAVPassword
	session  string
	personal string
}

const anaPassword = "correct horse battery staple"

func serveDAV(t *testing.T, enabled bool) *davServer {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tix.db")
	cfg := config.Defaults()
	cfg.CalDAV.Enabled = enabled
	conn, err := connect.Dial(ctx, &config.Resolved{Config: cfg}, connect.Overrides{DB: path})
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	st, err := sqlite.Open(path, clock.New())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	logs := &lockedBuffer{}
	srv, err := server.Assemble(server.Options{
		Service: conn.Service, Store: st, TenantID: conn.Info.TenantID, Addr: "127.0.0.1:0",
		PruneInterval: time.Hour, CalDAV: enabled, TrustedProxies: []string{"127.0.0.1/32"},
		Logger: slog.New(slog.NewTextHandler(logs, nil)), DisableRegistry: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(runCtx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	d := &davServer{conn: conn, st: st, base: "http://" + srv.Addr(), logs: logs,
		scope: core.TenantScope{TenantID: conn.Info.TenantID}}
	admin := conn.Context(ctx)
	user, err := conn.Service.CreateUser(admin, core.CreateUserInput{Email: "ana@example.com",
		Password: anaPassword, Role: core.RoleMember})
	if err != nil {
		t.Fatal(err)
	}
	d.ana = &core.Actor{ID: user.ID, TenantID: d.scope.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	login, err := conn.Service.Login(core.WithTenant(ctx, d.scope),
		core.LoginInput{Email: "ana@example.com", Password: anaPassword})
	if err != nil || login.Session == nil {
		t.Fatalf("signing in: %v", err)
	}
	d.session = login.Session.Token
	tok, err := conn.Service.CreateToken(admin, core.CreateTokenInput{Name: "ana-cli", ActorID: user.ID,
		Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatal(err)
	}
	d.personal = tok.Token
	d.app, err = conn.Service.CreateDAVPassword(core.WithActor(ctx, d.ana), core.CreateDAVPasswordInput{Name: "iPhone"})
	if err != nil {
		t.Fatal(err)
	}
	d.password = d.app.Password
	return d
}

// send makes one request; setup adds its credential.
func (d *davServer) send(t *testing.T, method, path, body string, setup func(*http.Request)) (*http.Response, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, d.base+path, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set(wire.HeaderContentType, "application/xml; charset=utf-8")
	}
	if setup != nil {
		setup(req)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func basic(user, pass string) func(*http.Request) {
	return func(r *http.Request) { r.SetBasicAuth(user, pass) }
}

const propfindPrincipal = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`

// Every credential but an app password over Basic is refused under /dav/ with
// the Basic challenge, and each refusal is logged with its reason.
func TestDAVSignsInWithAnAppPasswordAlone(t *testing.T) {
	d := serveDAV(t, true)
	cases := []struct {
		name   string
		setup  func(*http.Request)
		status int
		reason string
	}{
		{"nothing", nil, http.StatusUnauthorized, auth.RefusedNoCredentials},
		{"a session cookie", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: d.session})
		}, http.StatusUnauthorized, auth.RefusedDAVPasswordRequired},
		{"the app password as a bearer", func(r *http.Request) { r.Header.Set(wire.HeaderAuth, "Bearer "+d.password) },
			http.StatusUnauthorized, auth.RefusedDAVPasswordRequired},
		{"the account password", basic("ana@example.com", anaPassword), http.StatusUnauthorized,
			auth.RefusedDAVPasswordRequired},
		{"a personal token", basic("ana@example.com", d.personal), http.StatusUnauthorized,
			auth.RefusedDAVPasswordRequired},
		{"somebody else's name", basic("raj@example.com", d.password), http.StatusUnauthorized,
			auth.RefusedDAVUserMismatch},
		{"the app password", basic("ANA@example.com", d.password), http.StatusMultiStatus, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := d.send(t, "PROPFIND", "/dav/", propfindPrincipal, func(r *http.Request) {
				r.Header.Set("Depth", "0")
				if tc.setup != nil {
					tc.setup(r)
				}
			})
			if resp.StatusCode != tc.status {
				t.Fatalf("status %d, want %d: %s", resp.StatusCode, tc.status, body)
			}
			if tc.status == http.StatusUnauthorized {
				if got := resp.Header.Get("WWW-Authenticate"); got != auth.DAVChallenge {
					t.Errorf("challenge %q", got)
				}
				if strings.Contains(body, "ana") || strings.Contains(body, d.scope.TenantID) {
					t.Errorf("a refusal names somebody: %q", body)
				}
				if !strings.Contains(d.logs.String(), "auth_reason="+tc.reason) {
					t.Errorf("no refusal logged as %s", tc.reason)
				}
				return
			}
			if !strings.Contains(body, "/principals/"+strings.ToLower(d.ana.ID)+"/") {
				t.Errorf("no principal in %s", body)
			}
		})
	}
}

// An app password is refused off /dav/, while the API keeps working for the
// credentials that belong there.
func TestAnAppPasswordIsRefusedOnTheAPI(t *testing.T) {
	d := serveDAV(t, true)
	resp, _ := d.send(t, http.MethodGet, wire.RouteTasks, "", func(r *http.Request) {
		r.Header.Set(wire.HeaderAuth, "Bearer "+d.password)
	})
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(d.logs.String(), "auth_reason="+auth.RefusedCaldavOnly) {
		t.Errorf("an app password on the API: %d", resp.StatusCode)
	}
	resp, _ = d.send(t, http.MethodGet, wire.RouteTasks, "", func(r *http.Request) {
		r.Header.Set(wire.HeaderAuth, "Bearer "+d.personal)
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("a personal token on the API: %d", resp.StatusCode)
	}
}

func TestTheWellKnownAddressRedirectsWithoutACredential(t *testing.T) {
	d := serveDAV(t, true)
	for _, method := range []string{"PROPFIND", http.MethodGet} {
		resp, _ := d.send(t, method, core.DAVWellKnownPath, "", nil)
		if resp.StatusCode != http.StatusMovedPermanently || !strings.HasSuffix(resp.Header.Get("Location"), "/dav/") {
			t.Errorf("%s: %d to %q", method, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
}

func TestDisabledCalDAVIsNotThere(t *testing.T) {
	d := serveDAV(t, false)
	for _, path := range []string{"/dav/", "/dav", core.DAVWellKnownPath} {
		resp, _ := d.send(t, "PROPFIND", path, propfindPrincipal, basic("ana@example.com", d.password))
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, resp.StatusCode)
		}
	}
	resp, body := d.send(t, http.MethodGet, wire.RouteDAV, "", func(r *http.Request) {
		r.Header.Set(wire.HeaderAuth, "Bearer "+d.personal)
	})
	var info core.DAVInfo
	if err := json.Unmarshal([]byte(body), &info); err != nil || resp.StatusCode != http.StatusOK ||
		info.Enabled || info.Reason == "" {
		t.Errorf("dav.info while off: %d %s", resp.StatusCode, body)
	}
}

// A revoked password stops at once and the owner's sign-in is untouched,
// however often a stale phone retries it.
func TestARevokedPasswordStopsAndLocksNothing(t *testing.T) {
	d := serveDAV(t, true)
	if err := d.conn.Service.RevokeDAVPassword(core.WithActor(context.Background(), d.ana), d.app.ID); err != nil {
		t.Fatal(err)
	}
	for range 100 {
		resp, _ := d.send(t, "PROPFIND", "/dav/", propfindPrincipal, basic("ana@example.com", d.password))
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("a revoked password answered %d", resp.StatusCode)
		}
	}
	if !strings.Contains(d.logs.String(), "auth_reason="+auth.RefusedTokenRevoked) {
		t.Error("the revoked password's refusals were not logged")
	}
	if _, err := d.conn.Service.Login(core.WithTenant(context.Background(), d.scope),
		core.LoginInput{Email: "ana@example.com", Password: anaPassword}); err != nil {
		t.Fatalf("the owner's sign-in after a hundred refusals: %v", err)
	}
	resp, _ := d.send(t, http.MethodGet, wire.RouteTasks, "", func(r *http.Request) {
		r.Header.Set(wire.HeaderAuth, "Bearer "+d.personal)
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("the owner's other token: %d", resp.StatusCode)
	}
}

// The management routes answer with absolute addresses built as every other
// one, through a trusted proxy's scheme, and never list a secret.
func TestAppPasswordsOverHTTP(t *testing.T) {
	d := serveDAV(t, true)
	cookie := func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: d.session})
		r.Header.Set("X-Forwarded-Proto", "https")
	}
	resp, body := d.send(t, http.MethodGet, wire.RouteDAV, "", cookie)
	var info core.DAVInfo
	if err := json.Unmarshal([]byte(body), &info); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("dav.info: %d %s", resp.StatusCode, body)
	}
	if !info.Enabled || !info.Write || info.Username != "ana@example.com" ||
		!strings.HasPrefix(info.URL, "https://") || !strings.HasSuffix(info.URL, "/dav/") {
		t.Errorf("dav.info %+v", info)
	}

	create := func(name string) (*http.Response, string) {
		return d.send(t, http.MethodPost, wire.RouteDAVPasswords, `{"name":"`+name+`"}`, func(r *http.Request) {
			cookie(r)
			r.Header.Set(wire.HeaderContentType, wire.ContentJSON)
			r.Header.Set("X-CSRF-Token", "")
		})
	}
	resp, body = create("Mac")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var issued map[string]any
	_ = json.Unmarshal([]byte(body), &issued)
	for _, key := range []string{"password", "username", "url", "scopes", "collections", "id", "created_at"} {
		if _, ok := issued[key]; !ok {
			t.Errorf("create answer lacks %q: %s", key, body)
		}
	}
	if issued["collections"] != "assigned" || !strings.HasPrefix(issued["url"].(string), "https://") {
		t.Errorf("create answer %s", body)
	}
	if resp, body := create("mac"); resp.StatusCode != http.StatusConflict {
		t.Errorf("a duplicate name: %d %s", resp.StatusCode, body)
	}
	resp, body = d.send(t, http.MethodGet, wire.RouteDAVPasswords, "", cookie)
	if resp.StatusCode != http.StatusOK || strings.Contains(body, `"password"`) ||
		strings.Contains(body, issued["password"].(string)) || !strings.Contains(body, `"Mac"`) {
		t.Errorf("list: %d %s", resp.StatusCode, body)
	}
	resp, _ = d.send(t, http.MethodDelete, strings.Replace(wire.RouteDAVPassword, "{id}", issued["id"].(string), 1), "", cookie)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("revoke: %d", resp.StatusCode)
	}
}

// A write from a phone goes through the router's Basic sign-in and lands as
// an ordinary change: its audit entry and its event name the caldav channel
// and the app password's token, and the task's version moves by one.
func TestADAVWriteIsRecordedOnTheCalDAVChannel(t *testing.T) {
	d := serveDAV(t, true)
	ctx := context.Background()
	admin := d.conn.Context(ctx)
	task, err := d.conn.Service.CreateTask(admin, core.CreateTaskInput{Title: "Rotate keys", AssigneeActorID: d.ana.ID})
	if err != nil {
		t.Fatal(err)
	}
	var key string
	if err := d.st.View(ctx, d.scope, func(tx store.Tx) error {
		tenant, err := tx.GetTenant(ctx)
		key = tenant.Key
		return err
	}); err != nil {
		t.Fatal(err)
	}
	obj := "/dav/" + key + "/calendars/" + strings.ToLower(d.ana.ID) + "/assigned/" + strings.ToLower(task.ID) + ".ics"
	resp, body := d.send(t, http.MethodGet, obj, "", basic("ana@example.com", d.password))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET answered %d: %s", resp.StatusCode, body)
	}
	todo := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:task-" + strings.ToLower(task.ID) +
		"@tix\r\nSUMMARY:Rotate keys today\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	resp, body = d.send(t, http.MethodPut, obj, todo, func(r *http.Request) {
		r.SetBasicAuth("ana@example.com", d.password)
		r.Header.Set(wire.HeaderContentType, "text/calendar; charset=utf-8")
		r.Header.Set("If-Match", resp.Header.Get("ETag"))
	})
	if resp.StatusCode != http.StatusNoContent || resp.Header.Get("ETag") != "" {
		t.Fatalf("PUT answered %d with ETag %q: %s", resp.StatusCode, resp.Header.Get("ETag"), body)
	}
	got, err := d.conn.Service.GetTask(admin, core.TaskRef{ID: task.ID})
	if err != nil || got.Title != "Rotate keys today" || got.Version != task.Version+1 {
		t.Fatalf("the task after the PUT: %+v %v", got, err)
	}
	entries, _, err := d.conn.Service.ListAudit(admin, core.AuditFilter{Vias: []core.ViaKind{core.ViaCalDAV}})
	if err != nil || len(entries) != 1 || entries[0].Action != "task.update" ||
		entries[0].Via.TokenName != d.app.TokenName || entries[0].ActorID != d.ana.ID {
		t.Fatalf("audit on the caldav channel: %+v %v", entries, err)
	}
	if err := d.st.View(ctx, d.scope, func(tx store.Tx) error {
		events, err := tx.ReadEvents(ctx, 0, 1000)
		var found bool
		for _, e := range events {
			if e.Type == core.EventTaskUpdated && e.SubjectID == task.ID {
				found = e.Via != nil && e.Via.Kind == core.ViaCalDAV && e.Via.TokenID == d.app.ID
			}
		}
		if !found {
			t.Errorf("no task.updated event on the caldav channel in %+v", events)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
