// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/heliopsy/tix/internal/sso/ssotest"
)

// addSAMLProvider creates provider okta against a fake SAML IdP through the
// Single sign-on screen, from the IdP's pasted metadata.
func addSAMLProvider(t *testing.T, f *fixture) *ssotest.SAML {
	t.Helper()
	idp := ssotest.NewSAML(t, f.clock.Now)
	resp := f.as("alice").post("/admin/sso", url.Values{
		"key": {"okta"}, "kind": {"saml"}, "display_name": {"Okta"}, "idp_metadata_xml": {idp.Metadata()},
		"default_role": {"member"}, "enabled": {"1"}, "auto_create": {"1"},
	})
	wantStatus(t, resp, http.StatusSeeOther)
	return idp
}

func TestSSOScreenShowsASAMLProvidersAddresses(t *testing.T) {
	f := newFixture(t)
	idp := addSAMLProvider(t, f)
	page := f.as("alice").page("/admin/sso")
	row := between(t, page, `<span class="recrow-title">Okta`, `</li>`)
	for _, want := range []string{idp.EntityID, `class="sso-acs">` + f.server.URL + "/login/sso/saml/okta/acs",
		`class="sso-entity">` + f.server.URL + "/login/sso/saml/okta/metadata", "1 signing certificates"} {
		if !strings.Contains(row, want) {
			t.Errorf("the provider's row does not show %q:\n%s", want, row)
		}
	}
	md := f.as("").get("/login/sso/saml/okta/metadata")
	wantStatus(t, md, http.StatusOK)
	body := readAll(t, md)
	if md.Header.Get("Content-Type") != "application/samlmetadata+xml" ||
		!strings.Contains(body, `Location="`+f.server.URL+`/login/sso/saml/okta/acs"`) {
		t.Errorf("metadata = %s %s", md.Header.Get("Content-Type"), body)
	}
	missing := f.as("").get("/login/sso/saml/nobody/metadata")
	_ = missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Errorf("metadata of no provider = %d, want 404", missing.StatusCode)
	}
}

func TestSignInWithASAMLProviderInTheBrowser(t *testing.T) {
	f := newFixture(t)
	idp := addSAMLProvider(t, f)
	visitor := f.as("")
	start := visitor.get("/login/sso/okta?next=%2Ftasks")
	wantStatus(t, start, http.StatusSeeOther)
	req := ssotest.ReadRequest(t, start.Header.Get("Location"))
	if req.ACS != f.server.URL+"/login/sso/saml/okta/acs" || req.RelayState == "" {
		t.Fatalf("request = %+v, want the provider's ACS and a RelayState", req)
	}
	response := idp.Response(t, req, ssotest.Assertion{NameID: "00u9",
		Attributes: map[string][]string{"email": {"newbie@acme.test"}}}, ssotest.SignAssertion)
	form := url.Values{"SAMLResponse": {response}, "RelayState": {req.RelayState}}

	done := visitor.postRaw("/login/sso/saml/okta/acs", form)
	wantStatus(t, done, http.StatusSeeOther)
	if got := done.Header.Get("Location"); got != "/tasks" {
		t.Errorf("signed in and sent to %q, want /tasks", got)
	}
	if visitor.cookie("tix_session") == "" {
		t.Error("the assertion consumer service set no session cookie")
	}

	elsewhere := f.as("").postRaw("/login/sso/saml/okta/acs", form)
	_ = elsewhere.Body.Close()
	if elsewhere.StatusCode != http.StatusUnauthorized {
		t.Errorf("the response posted from a browser that did not start the sign-in = %d, want 401",
			elsewhere.StatusCode)
	}
}

func TestSSOScreenEditsASAMLProvider(t *testing.T) {
	f := newFixture(t)
	idp := addSAMLProvider(t, f)
	admin := f.as("alice")
	page := admin.page("/admin/sso")
	form := between(t, page, `action="/admin/sso/update">`, `</form>`)
	id := between(t, form, `name="id" value="`, `"`)
	certs := html.UnescapeString(between(t, form, `name="idp_certificates" rows="3" class="mono">`, `</textarea>`))
	if !strings.Contains(certs, "BEGIN CERTIFICATE") {
		t.Fatalf("the edit dialog does not draw the certificates:\n%s", form)
	}
	saved := admin.post("/admin/sso/update", url.Values{"id": {id}, "kind": {"saml"}, "display_name": {"Okta SSO"},
		"default_role": {"member"}, "enabled": {"1"}, "idp_entity_id": {idp.EntityID}, "idp_sso_url": {idp.SSOURL},
		"idp_certificates": {certs + "\n" + idp.OtherCertPEM()}, "allow_idp_initiated": {"1"}})
	wantStatus(t, saved, http.StatusSeeOther)
	row := between(t, admin.page("/admin/sso"), `<span class="recrow-title">Okta SSO`, `</li>`)
	if !strings.Contains(row, "2 signing certificates") || !strings.Contains(row, "accepts IdP-initiated sign-in") {
		t.Errorf("the edit did not land:\n%s", row)
	}
	refused := admin.post("/admin/sso/update", url.Values{"id": {id}, "kind": {"saml"}, "display_name": {"Okta"},
		"default_role": {"member"}, "idp_entity_id": {idp.EntityID}, "idp_sso_url": {idp.SSOURL},
		"idp_certificates": {"not a certificate"}})
	if refused.StatusCode != http.StatusBadRequest {
		t.Errorf("a bad certificate = %d, want 400", refused.StatusCode)
	}
	_ = refused.Body.Close()
}

func TestSSOScreenRefreshesASAMLProvidersMetadata(t *testing.T) {
	f := newFixture(t)
	idp := ssotest.NewSAML(t, f.clock.Now)
	var down atomic.Bool
	md := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if down.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(idp.Metadata()))
	}))
	t.Cleanup(md.Close)
	admin := f.as("alice")
	wantStatus(t, admin.post("/admin/sso", url.Values{"key": {"okta"}, "kind": {"saml"}, "display_name": {"Okta"},
		"idp_metadata_url": {md.URL + "/md"}, "default_role": {"member"}, "enabled": {"1"}}), http.StatusSeeOther)
	page := admin.page("/admin/sso")
	row := between(t, page, `<span class="recrow-title">Okta`, `</li>`)
	refresh := between(t, row, `action="/admin/sso/refresh"`, `</form>`)
	id := between(t, refresh, `name="id" value="`, `"`)
	if !strings.Contains(row, "IdP metadata read, certificates updated") {
		t.Fatalf("the row does not say the metadata was read on save:\n%s", row)
	}

	idp.Rotate(t)
	wantStatus(t, admin.post("/admin/sso/refresh", url.Values{"id": {id}}), http.StatusSeeOther)
	row = between(t, admin.page("/admin/sso"), `<span class="recrow-title">Okta`, `</li>`)
	if !strings.Contains(row, "2 signing certificates") || !strings.Contains(row, "IdP metadata read, certificates updated") {
		t.Errorf("after a refresh over a rotation the row reads:\n%s", row)
	}

	down.Store(true)
	wantStatus(t, admin.post("/admin/sso/refresh", url.Values{"id": {id}}), http.StatusSeeOther)
	row = between(t, admin.page("/admin/sso"), `<span class="recrow-title">Okta`, `</li>`)
	failed := between(t, row, `class="sso-refresh-failed">`, `</span></span>`)
	if !strings.Contains(failed, "IdP metadata read failed") || !strings.Contains(failed, "502") ||
		!strings.Contains(row, "2 signing certificates") {
		t.Errorf("after a failed refresh the row reads:\n%s", row)
	}
}
