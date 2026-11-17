// SPDX-License-Identifier: AGPL-3.0-or-later

package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// scimServer is an assembled serve process with SCIM enabled.
type scimServer struct {
	*assembled
	base  string
	token string
	ctx   context.Context
}

func serveSCIM(t *testing.T) *scimServer {
	t.Helper()
	a := assemble(t, nil)
	if err := a.srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.srv.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	admin := a.conn.Context(context.Background())
	s, err := a.conn.Service.EnableSCIM(admin)
	if err != nil {
		t.Fatalf("EnableSCIM: %v", err)
	}
	return &scimServer{assembled: a, base: "http://" + a.srv.Addr(), token: s.Token, ctx: admin}
}

// call sends one request and decodes the answer into a map.
func (s *scimServer) call(t *testing.T, method, path, token, body string) (int, http.Header, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, s.base+path, r)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set(wire.HeaderAuth, "Bearer "+token)
	}
	if body != "" {
		req.Header.Set(wire.HeaderContentType, wire.ContentSCIM)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s %s answered %d with %q, not JSON", method, path, resp.StatusCode, raw)
		}
	}
	return resp.StatusCode, resp.Header, out
}

func filter(path, expr string, extra ...string) string {
	q := url.Values{"filter": {expr}}
	for i := 0; i+1 < len(extra); i += 2 {
		q.Set(extra[i], extra[i+1])
	}
	return path + "?" + q.Encode()
}

// TestSCIMSpeaksTheProtocolProvidersSend runs the request shapes the Okta SCIM
// test suite and the Entra ID provisioning validator send, in their order: a
// lookup before create, a create, reads, patches and puts in each provider's
// own dialect, group membership, and the clean up.
func TestSCIMSpeaksTheProtocolProvidersSend(t *testing.T) {
	s := serveSCIM(t)
	var userID, groupID string
	idOf := func(b map[string]any) string { id, _ := b["id"].(string); return id }
	steps := []struct {
		name   string
		method string
		path   func() string
		body   func() string
		status int
		check  func(t *testing.T, h http.Header, b map[string]any)
	}{
		{"service provider config", http.MethodGet, func() string { return "/scim/v2/ServiceProviderConfig" }, nil, 200,
			func(t *testing.T, h http.Header, b map[string]any) {
				if ct := h.Get(wire.HeaderContentType); ct != wire.ContentSCIM {
					t.Errorf("content type %q", ct)
				}
				if p, _ := b["patch"].(map[string]any); p["supported"] != true {
					t.Errorf("patch %v", b["patch"])
				}
			}},
		{"resource types", http.MethodGet, func() string { return "/scim/v2/ResourceTypes" }, nil, 200,
			func(t *testing.T, _ http.Header, b map[string]any) {
				if b["totalResults"] != float64(2) {
					t.Errorf("resource types %v", b["totalResults"])
				}
			}},
		{"schemas", http.MethodGet, func() string { return "/scim/v2/Schemas" }, nil, 200, nil},
		{"one schema", http.MethodGet, func() string { return "/scim/v2/Schemas/urn:ietf:params:scim:schemas:core:2.0:User" }, nil, 200, nil},
		{"okta: look up before create", http.MethodGet,
			func() string {
				return filter("/scim/v2/Users", `userName eq "ann@example.com"`, "startIndex", "1", "count", "100")
			},
			nil, 200, func(t *testing.T, _ http.Header, b map[string]any) {
				if b["totalResults"] != float64(0) {
					t.Errorf("found %v before create", b["totalResults"])
				}
			}},
		{"okta: create", http.MethodPost, func() string { return "/scim/v2/Users" }, func() string {
			return `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"ann@example.com",
			"name":{"givenName":"Ann","familyName":"Lee"},"emails":[{"primary":true,"value":"ann@example.com","type":"work"}],
			"displayName":"Ann Lee","locale":"en-US","externalId":"00u1","groups":[],"password":"ignored","active":true}`
		}, 201, func(t *testing.T, h http.Header, b map[string]any) {
			userID = idOf(b)
			if userID == "" || !strings.HasSuffix(h.Get("Location"), "/scim/v2/Users/"+userID) {
				t.Errorf("created %v at %q", b, h.Get("Location"))
			}
		}},
		{"okta: create again", http.MethodPost, func() string { return "/scim/v2/Users" }, func() string {
			return `{"userName":"ann@example.com","emails":[{"value":"ann@example.com"}],"active":true}`
		}, 409, func(t *testing.T, _ http.Header, b map[string]any) {
			if b["scimType"] != "uniqueness" {
				t.Errorf("scimType %v", b["scimType"])
			}
		}},
		{"get by id", http.MethodGet, func() string { return "/scim/v2/Users/" + userID }, nil, 200,
			func(t *testing.T, _ http.Header, b map[string]any) {
				if b["userName"] != "ann@example.com" || b["externalId"] != "00u1" || b["active"] != true {
					t.Errorf("user %v", b)
				}
			}},
		{"entra: look up by externalId", http.MethodGet,
			func() string { return filter("/scim/v2/Users", `externalId eq "00u1"`) }, nil, 200,
			func(t *testing.T, _ http.Header, b map[string]any) {
				if b["totalResults"] != float64(1) {
					t.Errorf("found %v", b["totalResults"])
				}
			}},
		{"entra: deactivate with a string boolean", http.MethodPatch, func() string { return "/scim/v2/Users/" + userID },
			func() string {
				return `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
				"Operations":[{"op":"Replace","path":"active","value":"False"}]}`
			}, 200, func(t *testing.T, _ http.Header, b map[string]any) {
				if b["active"] != false {
					t.Errorf("active %v", b["active"])
				}
			}},
		{"entra: replace the work email and name", http.MethodPatch, func() string { return "/scim/v2/Users/" + userID },
			func() string {
				return `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[
				{"op":"Replace","path":"emails[type eq \"work\"].value","value":"ann.lee@example.com"},
				{"op":"Add","path":"name.familyName","value":"Lee-Park"},
				{"op":"Add","path":"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:department","value":"R&D"}]}`
			}, 200, nil},
		{"filter by work email", http.MethodGet,
			func() string {
				return filter("/scim/v2/Users", `emails[type eq "work"].value eq "ann.lee@example.com"`)
			}, nil, 200,
			func(t *testing.T, _ http.Header, b map[string]any) {
				if b["totalResults"] != float64(1) {
					t.Errorf("found %v", b["totalResults"])
				}
			}},
		{"okta: put reactivates", http.MethodPut, func() string { return "/scim/v2/Users/" + userID }, func() string {
			return `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"id":"` + userID + `","userName":"ann@example.com",
			"name":{"givenName":"Ann","familyName":"Lee"},"emails":[{"primary":true,"value":"ann@example.com","type":"work"}],
			"displayName":"Ann L","externalId":"00u1","active":true}`
		}, 200, func(t *testing.T, _ http.Header, b map[string]any) {
			if b["active"] != true || b["displayName"] != "Ann L" {
				t.Errorf("after put %v", b)
			}
		}},
		{"paging", http.MethodGet, func() string { return "/scim/v2/Users?startIndex=1&count=1" }, nil, 200,
			func(t *testing.T, _ http.Header, b map[string]any) {
				if b["itemsPerPage"] != float64(1) || b["startIndex"] != float64(1) {
					t.Errorf("page %v", b)
				}
			}},
		{"an unsupported filter", http.MethodGet, func() string { return filter("/scim/v2/Users", `userName co "ann"`) },
			nil, 400, func(t *testing.T, _ http.Header, b map[string]any) {
				if b["scimType"] != "invalidFilter" {
					t.Errorf("scimType %v", b["scimType"])
				}
			}},
		{"a user nobody has", http.MethodGet, func() string { return "/scim/v2/Users/nobody" }, nil, 404,
			func(t *testing.T, _ http.Header, b map[string]any) {
				if s, _ := b["schemas"].([]any); len(s) != 1 || s[0] != "urn:ietf:params:scim:api:messages:2.0:Error" {
					t.Errorf("error schemas %v", b["schemas"])
				}
			}},
		{"okta: create group", http.MethodPost, func() string { return "/scim/v2/Groups" }, func() string {
			return `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"Engineering",
			"members":[{"value":"` + userID + `","display":"ann@example.com"}]}`
		}, 201, func(t *testing.T, _ http.Header, b map[string]any) { groupID = idOf(b) }},
		{"entra: look up group without members", http.MethodGet,
			func() string {
				return filter("/scim/v2/Groups", `displayName eq "engineering"`, "excludedAttributes", "members")
			}, nil, 200, func(t *testing.T, _ http.Header, b map[string]any) {
				res, _ := b["Resources"].([]any)
				if len(res) != 1 {
					t.Fatalf("found %v", b)
				}
				if _, has := res[0].(map[string]any)["members"]; has {
					t.Error("members were not excluded")
				}
			}},
		{"okta: remove a member", http.MethodPatch, func() string { return "/scim/v2/Groups/" + groupID }, func() string {
			return `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations":[{"op":"remove","path":"members[value eq \"` + userID + `\"]"}]}`
		}, 200, func(t *testing.T, _ http.Header, b map[string]any) {
			if _, has := b["members"]; has {
				t.Errorf("members after remove %v", b["members"])
			}
		}},
		{"entra: add a member", http.MethodPatch, func() string { return "/scim/v2/Groups/" + groupID }, func() string {
			return `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations":[{"op":"Add","path":"members","value":[{"value":"` + userID + `"}]}]}`
		}, 200, func(t *testing.T, _ http.Header, b map[string]any) {
			if m, _ := b["members"].([]any); len(m) != 1 {
				t.Errorf("members after add %v", b["members"])
			}
		}},
		{"entra: rename the group", http.MethodPatch, func() string { return "/scim/v2/Groups/" + groupID }, func() string {
			return `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations":[{"op":"Replace","value":{"id":"` + groupID + `","displayName":"Eng"}}]}`
		}, 200, func(t *testing.T, _ http.Header, b map[string]any) {
			if b["displayName"] != "Eng" {
				t.Errorf("renamed to %v", b["displayName"])
			}
		}},
		{"groups of a member", http.MethodGet,
			func() string { return filter("/scim/v2/Groups", `members[value eq "`+userID+`"]`) }, nil, 200,
			func(t *testing.T, _ http.Header, b map[string]any) {
				if b["totalResults"] != float64(1) {
					t.Errorf("found %v", b["totalResults"])
				}
			}},
		{"a bad patch op", http.MethodPatch, func() string { return "/scim/v2/Groups/" + groupID }, func() string {
			return `{"Operations":[{"op":"move","path":"members"}]}`
		}, 400, nil},
		{"bulk is not supported", http.MethodPost, func() string { return "/scim/v2/Bulk" }, func() string { return `{}` }, 404, nil},
		{"delete group", http.MethodDelete, func() string { return "/scim/v2/Groups/" + groupID }, nil, 204, nil},
		{"delete user", http.MethodDelete, func() string { return "/scim/v2/Users/" + userID }, nil, 204, nil},
		{"read a deleted user", http.MethodGet, func() string { return "/scim/v2/Users/" + userID }, nil, 404, nil},
	}
	for _, st := range steps {
		body := ""
		if st.body != nil {
			body = st.body()
		}
		status, h, b := s.call(t, st.method, st.path(), s.token, body)
		if status != st.status {
			t.Fatalf("%s: status %d, want %d: %v", st.name, status, st.status, b)
		}
		if st.check != nil {
			t.Run(st.name, func(t *testing.T) { st.check(t, h, b) })
		}
	}

	entries, _, err := s.conn.Service.ListAudit(s.ctx, core.AuditFilter{Vias: []core.ViaKind{core.ViaSCIM},
		Page: core.Page{Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no change was audited on the scim channel")
	}
	for _, e := range entries {
		if e.Via.TokenName != core.SCIMTokenName {
			t.Errorf("%s via %+v, want the token's name", e.Action, e.Via)
		}
	}
}

func TestSCIMRefusesOtherCredentialsAndAnswersNotFoundWhenOff(t *testing.T) {
	s := serveSCIM(t)
	wildcard, err := s.conn.Service.CreateToken(s.ctx, core.CreateTokenInput{Name: "everything",
		Scopes: []core.Scope{core.ScopeAll}})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		path   string
		token  string
		status int
	}{
		{"no credential", "/scim/v2/Users", "", http.StatusUnauthorized},
		{"a bad token", "/scim/v2/Users", "tix_nonsense", http.StatusUnauthorized},
		{"a token without the scope", "/scim/v2/Users", wildcard.Token, http.StatusForbidden},
		{"the provisioning token on the API", wire.RouteWhoAmI, s.token, http.StatusForbidden},
		{"the provisioning token on the API's directory", wire.RouteActors, s.token, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _, _ := s.call(t, http.MethodGet, tc.path, tc.token, "")
			if status != tc.status {
				t.Fatalf("status %d, want %d", status, tc.status)
			}
		})
	}

	if _, err := s.conn.Service.DisableSCIM(s.ctx); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/scim/v2/Users", "/scim/v2/ServiceProviderConfig", "/scim/v2/Groups"} {
		status, h, b := s.call(t, http.MethodGet, path, s.token, "")
		if status != http.StatusNotFound || h.Get(wire.HeaderContentType) != wire.ContentSCIM {
			t.Errorf("%s while off: %d %q %v, want a SCIM 404", path, status, h.Get(wire.HeaderContentType), b)
		}
	}
	status, _, _ := s.call(t, http.MethodGet, "/scim/v2/Users", wildcard.Token, "")
	if status != http.StatusForbidden {
		t.Errorf("a token without the scope while off = %d, want 403 before 404", status)
	}
}
