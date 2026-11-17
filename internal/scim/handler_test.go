// SPDX-License-Identifier: AGPL-3.0-or-later

package scim_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/scim"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
	"github.com/heliopsy/tix/internal/wire"
)

// tokens resolves a token inside one tenant.
type tokens struct {
	st    store.Store
	scope core.TenantScope
}

func (l tokens) TokenByHash(ctx context.Context, hash string) (*core.APIToken, error) {
	var out *core.APIToken
	err := l.st.View(ctx, l.scope, func(tx store.Tx) error {
		tok, err := tx.GetTokenByHash(ctx, hash)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		out = tok
		return err
	})
	return out, err
}

// harness is a handler over a real service with SCIM enabled, and the
// provider's actor.
type harness struct {
	h        *scim.Handler
	provider *core.Actor
	admin    context.Context
	svc      *service.Local
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(sqlitetest.Path(t), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	tenant := core.Tenant{Key: "acme", Name: "Acme"}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, &tenant) }); err != nil {
		t.Fatal(err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	admin := core.Actor{Kind: core.ActorUser, Handle: "alice", Scopes: []core.Scope{core.ScopeAll}}
	if err := st.Update(ctx, scope, func(tx store.Tx) error { return tx.CreateActor(ctx, &admin) }); err != nil {
		t.Fatal(err)
	}
	admin.TenantID = tenant.ID
	svc := service.New(st, service.WithClock(clk), service.WithHooks(service.HookOff),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams())))
	adminCtx := core.WithActor(ctx, &admin)
	s, err := svc.EnableSCIM(adminCtx)
	if err != nil {
		t.Fatal(err)
	}
	actor, err := auth.NewTokenVerifier(tokens{st: st, scope: scope}, clk).Verify(core.WithSCIMRequest(ctx), s.Token)
	if err != nil {
		t.Fatal(err)
	}
	origin := func(*http.Request) string { return "https://tix.example" }
	return &harness{h: scim.New(svc, origin), provider: actor, admin: adminCtx, svc: svc}
}

func (h *harness) do(t *testing.T, method, path, contentType, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		r.Header.Set(wire.HeaderContentType, contentType)
	}
	r = r.WithContext(core.WithActor(r.Context(), h.provider))
	w := httptest.NewRecorder()
	h.h.ServeHTTP(w, r)
	var out map[string]any
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %d %q", method, path, w.Code, w.Body)
		}
	}
	return w, out
}

func TestHandlerServesDiscoveryAndRefusesWhatItDoesNotKnow(t *testing.T) {
	h := newHarness(t)
	p := core.SCIMPath
	tests := []struct {
		method, path, ct, body string
		status                 int
	}{
		{http.MethodGet, p + "/ServiceProviderConfig", "", "", 200},
		{http.MethodGet, p + "/ResourceTypes", "", "", 200},
		{http.MethodGet, p + "/ResourceTypes/Group", "", "", 200},
		{http.MethodGet, p + "/ResourceTypes/Widget", "", "", 404},
		{http.MethodGet, p + "/Schemas", "", "", 200},
		{http.MethodGet, p + "/Schemas/" + scim.SchemaGroup, "", "", 200},
		{http.MethodGet, p + "/Schemas/urn:nope", "", "", 404},
		{http.MethodPost, p + "/ServiceProviderConfig", "", "", 405},
		{http.MethodGet, p + "/Me", "", "", 404},
		{http.MethodPost, p + "/Users", "text/plain", "x", 415},
		{http.MethodPost, p + "/Users", wire.ContentSCIM, "{", 400},
		{http.MethodPost, p + "/Users", wire.ContentSCIM, "", 400},
		{http.MethodPost, p + "/Users", wire.ContentJSON, `{"userName":"x@example.com","active":"maybe"}`, 400},
		{http.MethodPost, p + "/Users", wire.ContentJSON, `{"userName":"nomail"}`, 400},
		{http.MethodGet, p + "/Users?count=x", "", "", 400},
		{http.MethodGet, p + "/Groups?filter=title%20eq%20%22x%22", "", "", 400},
		{http.MethodPut, p + "/Users/nobody", wire.ContentSCIM, `{"userName":"x"}`, 404},
		{http.MethodPatch, p + "/Users/nobody", wire.ContentSCIM, `{"Operations":[{"op":"replace","path":"active","value":false}]}`, 404},
		{http.MethodDelete, p + "/Users/nobody", "", "", 404},
		{http.MethodGet, p + "/Groups/nobody", "", "", 404},
		{http.MethodPut, p + "/Groups/nobody", wire.ContentSCIM, `{"displayName":"x"}`, 404},
		{http.MethodPatch, p + "/Groups/nobody", wire.ContentSCIM, `{"Operations":[{"op":"add","path":"members","value":[]}]}`, 404},
		{http.MethodDelete, p + "/Groups/nobody", "", "", 404},
		{http.MethodPost, p + "/Groups", wire.ContentSCIM, `{"displayName":""}`, 400},
	}
	for _, tc := range tests {
		w, body := h.do(t, tc.method, tc.path, tc.ct, tc.body)
		if w.Code != tc.status {
			t.Errorf("%s %s = %d %v, want %d", tc.method, tc.path, w.Code, body, tc.status)
		}
		if ct := w.Header().Get(wire.HeaderContentType); ct != wire.ContentSCIM {
			t.Errorf("%s %s content type %q", tc.method, tc.path, ct)
		}
	}
}

func TestHandlerReplacesUsersAndGroups(t *testing.T) {
	h := newHarness(t)
	p := core.SCIMPath
	w, user := h.do(t, http.MethodPost, p+"/Users", wire.ContentSCIM,
		`{"userName":"ann@example.com","name":{"givenName":"Ann","familyName":"Lee"},"active":true}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user %d %v", w.Code, user)
	}
	id := user["id"].(string)
	if loc := w.Header().Get("Location"); loc != "https://tix.example/scim/v2/Users/"+id {
		t.Errorf("location %q", loc)
	}
	w, user = h.do(t, http.MethodPut, p+"/Users/"+id, wire.ContentSCIM,
		`{"userName":"ann.lee@example.com","displayName":"A. Lee","active":false}`)
	if w.Code != http.StatusOK || user["userName"] != "ann.lee@example.com" || user["active"] != false {
		t.Fatalf("put user %d %v", w.Code, user)
	}
	if emails, _ := user["emails"].([]any); len(emails) != 1 {
		t.Errorf("a put without emails kept %v, want the address kept", user["emails"])
	}
	w, group := h.do(t, http.MethodPost, p+"/Groups", wire.ContentSCIM,
		`{"displayName":"Eng","externalId":"g1","members":[{"value":"`+id+`"}]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create group %d %v", w.Code, group)
	}
	gid := group["id"].(string)
	w, group = h.do(t, http.MethodPut, p+"/Groups/"+gid, wire.ContentSCIM, `{"displayName":"Ops"}`)
	if w.Code != http.StatusOK || group["displayName"] != "Ops" || group["members"] != nil {
		t.Fatalf("put group %d %v", w.Code, group)
	}
	w, group = h.do(t, http.MethodGet, p+"/Groups/"+gid+"?excludedAttributes=members", "", "")
	if w.Code != http.StatusOK || group["externalId"] != nil {
		t.Errorf("get group %d %v", w.Code, group)
	}
	w, list := h.do(t, http.MethodGet, p+"/Groups", "", "")
	if w.Code != http.StatusOK || list["totalResults"] != float64(1) {
		t.Errorf("list groups %d %v", w.Code, list)
	}
	w, list = h.do(t, http.MethodGet, p+"/Users?filter=userName%20eq%20%22ANN.LEE@example.com%22", "", "")
	if w.Code != http.StatusOK || list["totalResults"] != float64(1) {
		t.Errorf("filter users %d %v", w.Code, list)
	}
	if _, err := h.svc.DisableSCIM(h.admin); err != nil {
		t.Fatal(err)
	}
	if w, _ := h.do(t, http.MethodGet, p+"/ServiceProviderConfig", "", ""); w.Code != http.StatusNotFound {
		t.Errorf("discovery while off = %d", w.Code)
	}
}
