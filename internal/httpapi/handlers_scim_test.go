// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestSCIMAdministrationRoutes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	var s core.SCIMSettings
	resp := f.call(http.MethodGet, wire.RouteSCIM, nil)
	mustStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &s)
	if s.Enabled || !strings.HasSuffix(s.URL, core.SCIMPath) {
		t.Fatalf("settings before enabling %+v, want off with an absolute endpoint", s)
	}

	resp = f.call(http.MethodPost, wire.RouteSCIMEnable, nil)
	mustStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &s)
	if !s.Enabled || s.Token == "" {
		t.Fatalf("enable answered %+v", s)
	}
	resp = f.call(http.MethodPost, wire.RouteSCIMToken, nil)
	mustStatus(t, resp, http.StatusOK)
	var rotated core.SCIMSettings
	decodeBody(t, resp, &rotated)
	if rotated.Token == "" || rotated.Token == s.Token {
		t.Errorf("rotation answered %+v", rotated)
	}

	resp = f.call(http.MethodPut, wire.RouteSCIMGroups, core.SCIMGroupRoleInput{Group: "Eng Team", Role: core.RoleMember})
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	resp = f.call(http.MethodPut, wire.RouteSCIMGroups, core.SCIMGroupRoleInput{Group: "x", Role: "owner"})
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()
	resp = f.call(http.MethodDelete, "/api/v1/scim/groups/Eng%20Team", nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	resp = f.call(http.MethodDelete, "/api/v1/scim/groups/Eng%20Team", nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()

	resp = f.call(http.MethodPost, wire.RouteSCIMDisable, nil)
	mustStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &s)
	if s.Enabled || len(s.Tokens) != 1 {
		t.Errorf("disable answered %+v", s)
	}
	resp = f.call(http.MethodPut, wire.RouteSCIMGroups, "not an object")
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()
}
