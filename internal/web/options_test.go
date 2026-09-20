package web_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/web"
)

func TestSecureCookiesOption(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	handler := web.Handler(f.svc, web.WithSecureCookies(true),
		web.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, nil))))

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	cookies := recorder.Result().Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatalf("no csrf cookie was issued")
	}
	for _, cookie := range cookies {
		if strings.Contains(cookie, web.CSRFCookieName) && !strings.Contains(cookie, "Secure") {
			t.Fatalf("the csrf cookie is not marked Secure: %s", cookie)
		}
		if !strings.Contains(cookie, "HttpOnly") {
			t.Fatalf("the csrf cookie is not marked HttpOnly: %s", cookie)
		}
	}
}

func TestEventsPathDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/activity")
	if !strings.Contains(page, `data-events="/api/v1/events"`) {
		t.Fatalf("the feed does not name the event stream")
	}

	handler := web.Handler(f.svc, web.WithEventsPath(""), web.WithLogger(nil))
	req := httptest.NewRequest(http.MethodGet, "/activity", nil)
	req = req.WithContext(core.WithActor(req.Context(), &core.Actor{
		ID: f.actorA.ID, TenantID: f.tenantA.ID, Kind: core.ActorUser,
		Handle: "alice", Role: core.RoleAdmin, Scopes: []core.Scope{core.ScopeAll}}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if !strings.Contains(recorder.Body.String(), web.DefaultEventsPath) {
		t.Fatalf("an empty events path did not fall back to the default")
	}
}

func TestMethodNotAllowedOnAScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/activity", url.Values{})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestWorkflowMigrationsAreParsed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/workflows", url.Values{"key": {"phases"}, "name": {"Phases"},
		"initial": {"new"}, "states": {"new|New|open\nshipped|Shipped|terminal"},
		"transitions": {"new>shipped"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	migrated := b.post("/workflows", url.Values{"key": {"phases"}, "name": {"Phases"},
		"initial": {"queued"}, "states": {"queued|Queued|open\nshipped|Shipped|terminal"},
		"transitions": {"queued>shipped"}, "migrate": {"new=queued\nignored"}})
	_ = migrated.Body.Close()
	wantStatus(t, migrated, http.StatusSeeOther)

	page := b.page("/workflows/phases")
	if !strings.Contains(page, "queued") {
		t.Fatalf("the migrated workflow is not shown:\n%s", page)
	}

	deleted := b.post("/workflows/phases/delete", url.Values{})
	defer func() { _ = deleted.Body.Close() }()
	wantStatus(t, deleted, http.StatusSeeOther)
}

func TestTypedCustomFieldsAcceptEveryType(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	defs := []url.Values{
		{"key": {"count"}, "label": {"Count"}, "type": {"int"}},
		{"key": {"ratio"}, "label": {"Ratio"}, "type": {"float"}},
		{"key": {"urgent"}, "label": {"Urgent"}, "type": {"bool"}},
		{"key": {"extra"}, "label": {"Extra"}, "type": {"json"}},
		{"key": {"note"}, "label": {"Note"}, "type": {"string"}},
	}
	for _, def := range defs {
		resp := b.post("/projects/infra/fields", def)
		wantStatus(t, resp, http.StatusSeeOther)
		_ = resp.Body.Close()
	}

	ref := b.createTask("infra", "typed fields")
	saved := b.post("/tasks/"+ref, url.Values{"title": {"typed fields"},
		"field.count": {"3"}, "field.ratio": {"1.5"}, "field.urgent": {"true"},
		"field.extra": {`{"a":1}`}, "field.note": {"plain"}})
	wantStatus(t, saved, http.StatusSeeOther)
	_ = saved.Body.Close()

	page := b.page("/tasks/" + ref)
	for _, want := range []string{"1.5", "plain"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the detail screen does not show %q", want)
		}
	}

	bad := []url.Values{
		{"title": {"typed fields"}, "field.count": {"many"}},
		{"title": {"typed fields"}, "field.ratio": {"half"}},
		{"title": {"typed fields"}, "field.urgent": {"perhaps"}},
		{"title": {"typed fields"}, "field.extra": {"{"}},
	}
	for _, form := range bad {
		resp := b.post("/tasks/"+ref, form)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for %v", resp.StatusCode, form)
		}
		_ = resp.Body.Close()
	}
}
