// SPDX-License-Identifier: AGPL-3.0-or-later

package scim

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestParseFilter(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		kind resource
		want []core.SCIMCondition
		bad  bool
	}{
		{"userName", `userName eq "ann@example.com"`, resourceUser,
			[]core.SCIMCondition{{Attr: core.SCIMAttrUserName, Value: "ann@example.com"}}, false},
		{"names ignore case", `USERNAME EQ "a"`, resourceUser,
			[]core.SCIMCondition{{Attr: core.SCIMAttrUserName, Value: "a"}}, false},
		{"schema prefix", `urn:ietf:params:scim:schemas:core:2.0:User:userName eq "a"`, resourceUser,
			[]core.SCIMCondition{{Attr: core.SCIMAttrUserName, Value: "a"}}, false},
		{"externalId", `externalId eq "00u1"`, resourceUser,
			[]core.SCIMCondition{{Attr: core.SCIMAttrExternalID, Value: "00u1"}}, false},
		{"work email", `emails[type eq "work"].value eq "a@example.com"`, resourceUser,
			[]core.SCIMCondition{{Attr: core.SCIMAttrEmail, Value: "a@example.com"}}, false},
		{"email value", `emails.value eq "a@example.com"`, resourceUser,
			[]core.SCIMCondition{{Attr: core.SCIMAttrEmail, Value: "a@example.com"}}, false},
		{"and", `id eq "1" and userName eq "x"`, resourceUser,
			[]core.SCIMCondition{{Attr: core.SCIMAttrID, Value: "1"}, {Attr: core.SCIMAttrUserName, Value: "x"}}, false},
		{"escaped quote", `userName eq "a\"b"`, resourceUser,
			[]core.SCIMCondition{{Attr: core.SCIMAttrUserName, Value: `a"b`}}, false},
		{"group name", `displayName eq "Eng"`, resourceGroup,
			[]core.SCIMCondition{{Attr: core.SCIMAttrDisplayName, Value: "Eng"}}, false},
		{"group member term", `id eq "g" and members[value eq "U1"]`, resourceGroup,
			[]core.SCIMCondition{{Attr: core.SCIMAttrID, Value: "g"}, {Attr: core.SCIMAttrMember, Value: "U1"}}, false},
		{"group member value", `members.value eq "U1"`, resourceGroup,
			[]core.SCIMCondition{{Attr: core.SCIMAttrMember, Value: "U1"}}, false},
		{"contains", `userName co "a"`, resourceUser, nil, true},
		{"or", `userName eq "a" or userName eq "b"`, resourceUser, nil, true},
		{"present", `title pr`, resourceUser, nil, true},
		{"unknown attribute", `title eq "x"`, resourceUser, nil, true},
		{"group attribute on users", `displayName eq "x"`, resourceUser, nil, true},
		{"home email", `emails[type eq "home"].value eq "x"`, resourceUser, nil, true},
		{"unclosed", `userName eq "a`, resourceUser, nil, true},
		{"dangling and", `userName eq "a" and`, resourceUser, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseFilter(tc.raw, tc.kind)
			if tc.bad {
				if status, typ, _ := classify(err); status != http.StatusBadRequest || typ != TypeInvalidFilter {
					t.Fatalf("parseFilter(%q) = %v, %v; want invalidFilter", tc.raw, got, err)
				}
				return
			}
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("parseFilter(%q) = %v, %v; want %v", tc.raw, got, err, tc.want)
			}
		})
	}
}

func patch(t *testing.T, raw string) PatchRequest {
	t.Helper()
	var p PatchRequest
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestApplyUserPatch(t *testing.T) {
	base := core.SCIMUser{UserName: "ann", Email: "ann@example.com", DisplayName: "Ann", Active: true,
		GivenName: "Ann", FamilyName: "Lee", ExternalID: "x"}
	tests := []struct {
		name string
		ops  string
		want func(u *core.SCIMUser) bool
		bad  bool
	}{
		{"deactivate", `[{"op":"replace","path":"active","value":false}]`,
			func(u *core.SCIMUser) bool { return !u.Active }, false},
		{"entra string boolean", `[{"op":"Replace","path":"active","value":"False"}]`,
			func(u *core.SCIMUser) bool { return !u.Active }, false},
		{"no path", `[{"op":"replace","value":{"active":false,"displayName":"A B"}}]`,
			func(u *core.SCIMUser) bool { return !u.Active && u.DisplayName == "A B" }, false},
		{"name parts", `[{"op":"add","path":"name.givenName","value":"Anne"}]`,
			func(u *core.SCIMUser) bool { return u.GivenName == "Anne" && u.FamilyName == "Lee" }, false},
		{"name object", `[{"op":"replace","path":"name","value":{"familyName":"Park"}}]`,
			func(u *core.SCIMUser) bool { return u.GivenName == "" && u.FamilyName == "Park" }, false},
		{"work email", `[{"op":"replace","path":"emails[type eq \"work\"].value","value":"a@x.com"}]`,
			func(u *core.SCIMUser) bool { return u.Email == "a@x.com" }, false},
		{"emails list", `[{"op":"replace","path":"emails","value":[{"value":"h@x.com","type":"home"},{"value":"p@x.com","primary":true}]}]`,
			func(u *core.SCIMUser) bool { return u.Email == "p@x.com" }, false},
		{"remove externalId", `[{"op":"remove","path":"externalId"}]`,
			func(u *core.SCIMUser) bool { return u.ExternalID == "" }, false},
		{"extension ignored", `[{"op":"add","path":"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:manager","value":"x"}]`,
			func(u *core.SCIMUser) bool { return reflect.DeepEqual(*u, base) }, false},
		{"remove userName", `[{"op":"remove","path":"userName"}]`, nil, true},
		{"remove without a path", `[{"op":"remove"}]`, nil, true},
		{"unknown op", `[{"op":"copy","path":"active"}]`, nil, true},
		{"bad boolean", `[{"op":"replace","path":"active","value":"maybe"}]`, nil, true},
		{"nothing", `[]`, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := base
			err := applyUserPatch(&u, patch(t, `{"Operations":`+tc.ops+`}`))
			if tc.bad {
				if status, _, _ := classify(err); status != http.StatusBadRequest {
					t.Fatalf("got %v, want a 400", err)
				}
				return
			}
			if err != nil || !tc.want(&u) {
				t.Fatalf("got %+v, %v", u, err)
			}
		})
	}
}

func TestApplyGroupPatch(t *testing.T) {
	ids := func(g core.SCIMGroup) string {
		var out []string
		for _, m := range g.Members {
			out = append(out, m.ID)
		}
		return strings.Join(out, ",")
	}
	base := core.SCIMGroup{DisplayName: "Eng", Members: []core.SCIMRef{{ID: "A"}, {ID: "B"}}}
	tests := []struct {
		name    string
		ops     string
		members string
		display string
		bad     bool
	}{
		{"add", `[{"op":"add","path":"members","value":[{"value":"C"},{"value":"A"}]}]`, "A,B,C", "Eng", false},
		{"add one object", `[{"op":"Add","path":"members","value":{"value":"C"}}]`, "A,B,C", "Eng", false},
		{"remove by filter keeps the id's case", `[{"op":"remove","path":"members[value eq \"B\"]"}]`, "A", "Eng", false},
		{"remove by value", `[{"op":"remove","path":"members","value":[{"value":"A"}]}]`, "B", "Eng", false},
		{"remove all", `[{"op":"remove","path":"members"}]`, "", "Eng", false},
		{"replace", `[{"op":"replace","path":"members","value":[{"value":"Z"}]}]`, "Z", "Eng", false},
		{"rename without a path", `[{"op":"replace","value":{"id":"g","displayName":"Ops"}}]`, "A,B", "Ops", false},
		{"rename", `[{"op":"replace","path":"displayName","value":"Ops"}]`, "A,B", "Ops", false},
		{"remove the name", `[{"op":"remove","path":"displayName"}]`, "", "", true},
		{"add by filter", `[{"op":"add","path":"members[value eq \"B\"]"}]`, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := base
			g.Members = slices.Clone(base.Members)
			err := applyGroupPatch(&g, patch(t, `{"Operations":`+tc.ops+`}`))
			if tc.bad {
				if status, _, _ := classify(err); status != http.StatusBadRequest {
					t.Fatalf("got %v, want a 400", err)
				}
				return
			}
			if err != nil || ids(g) != tc.members || g.DisplayName != tc.display {
				t.Fatalf("got %q %q, %v; want %q %q", ids(g), g.DisplayName, err, tc.members, tc.display)
			}
		})
	}
}

func TestClassifyMapsKindsToSCIMStatuses(t *testing.T) {
	tests := []struct {
		err    error
		status int
		typ    string
	}{
		{core.Invalid("x"), http.StatusBadRequest, TypeInvalidValue},
		{core.NotFound("x"), http.StatusNotFound, ""},
		{core.Conflict("x"), http.StatusConflict, TypeUniqueness},
		{core.Forbidden("x"), http.StatusForbidden, ""},
		{core.Unauthenticated("x"), http.StatusUnauthorized, ""},
		{errors.New("boom"), http.StatusInternalServerError, ""},
	}
	for _, tc := range tests {
		status, typ, _ := classify(tc.err)
		if status != tc.status || typ != tc.typ {
			t.Errorf("classify(%v) = %d %q, want %d %q", tc.err, status, typ, tc.status, tc.typ)
		}
	}
}

func TestHandlerRefusesWithoutAProvisioningToken(t *testing.T) {
	h := New(nil, nil)
	tests := []struct {
		name   string
		actor  *core.Actor
		status int
	}{
		{"no actor", nil, http.StatusUnauthorized},
		{"a session", &core.Actor{ID: "a", Role: core.RoleAdmin}, http.StatusForbidden},
		{"a wildcard token", &core.Actor{ID: "a", TokenID: "t", Scopes: []core.Scope{core.ScopeAll}}, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, core.SCIMPath+"/Users", nil)
			if tc.actor != nil {
				r = r.WithContext(core.WithActor(r.Context(), tc.actor))
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), SchemaError) {
				t.Fatalf("status %d body %s, want %d with the error schema", w.Code, w.Body, tc.status)
			}
		})
	}
}

func TestQueryReadsPaging(t *testing.T) {
	tests := []struct {
		raw          string
		start, count int
		bad          bool
	}{
		{"", 1, DefaultCount, false},
		{"startIndex=0&count=-5", 1, 0, false},
		{"startIndex=3&count=1000", 3, MaxResults, false},
		{"count=x", 0, 0, true},
		{"startIndex=x", 0, 0, true},
	}
	for _, tc := range tests {
		r := httptest.NewRequest(http.MethodGet, core.SCIMPath+"/Users?"+tc.raw, nil)
		q, err := query(r, resourceUser)
		if tc.bad != (err != nil) || !tc.bad && (q.StartIndex != tc.start || q.Count != tc.count) {
			t.Errorf("query(%q) = %+v, %v", tc.raw, q, err)
		}
	}
}
