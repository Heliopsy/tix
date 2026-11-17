// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/sso/ssotest"
)

const samlACSURL = "https://tix.example/login/sso/saml/okta/acs"

type samlFixture struct {
	l     *Local
	clk   *clock.Fake
	scope core.TenantScope
	admin *core.Actor
	idp   *ssotest.SAML
}

func newSAMLFixture(t *testing.T, edit func(*core.CreateSSOProviderInput)) samlFixture {
	t.Helper()
	l, clk, scope, admin := newLocalWith(t, WithTOTPKey(totpTestKey), WithEgress(egress.Permissive()),
		WithPublicURL("https://tix.example"))
	idp := ssotest.NewSAML(t, clk.Now)
	in := core.CreateSSOProviderInput{Key: "okta", Kind: core.SSOKindSAML, DisplayName: "Okta",
		IdPMetadataXML: idp.Metadata(), AutoCreate: true}
	if edit != nil {
		edit(&in)
	}
	if _, err := l.CreateSSOProvider(authContext(admin), in); err != nil {
		t.Fatalf("CreateSSOProvider: %v", err)
	}
	return samlFixture{l: l, clk: clk, scope: scope, admin: admin, idp: idp}
}

// begin starts a browser sign-in and answers the sealed flow and the request
// the IdP received.
func (f samlFixture) begin(t *testing.T) (*core.SSOBegin, ssotest.AuthnRequest) {
	t.Helper()
	begin, err := f.l.BeginSSOLogin(loginContext(f.scope), core.BeginSSOInput{Provider: "okta", RedirectURI: ssoCallback,
		Next: "/tasks"})
	if err != nil {
		t.Fatalf("BeginSSOLogin: %v", err)
	}
	req := ssotest.ReadRequest(t, begin.AuthURL)
	if begin.Kind != core.SSOKindSAML || req.RelayState != begin.State || req.ACS != samlACSURL {
		t.Fatalf("begin = %+v, request %+v; want a saml flow posting to %s", begin, req, samlACSURL)
	}
	return begin, req
}

func (f samlFixture) post(begin *core.SSOBegin, state, response string) (*core.LoginResult, error) {
	return f.l.CompleteSSOLogin(loginContext(f.scope), core.CompleteSSOInput{Flow: begin.Flow, State: state,
		SAMLResponse: response, Provider: "okta", RedirectURI: samlACSURL})
}

func (f samlFixture) signIn(t *testing.T, a ssotest.Assertion) (*core.LoginResult, error) {
	t.Helper()
	begin, req := f.begin(t)
	return f.post(begin, req.RelayState, f.idp.Response(t, req, a, ssotest.SignAssertion))
}

func (f samlFixture) denials(t *testing.T, reason string) int {
	t.Helper()
	entries, _, err := f.l.ListAudit(authContext(f.admin), core.AuditFilter{Page: core.Page{Limit: 200}})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	n := 0
	for _, e := range entries {
		if e.Action == auditSSODenied && strings.Contains(string(e.After), `"reason":"`+reason+`"`) {
			n++
		}
	}
	return n
}

func samlPerson(nameID, email string) ssotest.Assertion {
	return ssotest.Assertion{NameID: nameID, Attributes: map[string][]string{"email": {email},
		"displayName": {"Ada Lovelace"}, "uid": {"ada"}, "groups": {"tix-admins"}}}
}

func TestSAMLSignInCreatesAPersonThenFindsThemByNameID(t *testing.T) {
	f := newSAMLFixture(t, func(in *core.CreateSSOProviderInput) {
		in.RoleMapping = map[string]core.Role{"tix-admins": core.RoleAdmin}
	})
	first, err := f.signIn(t, samlPerson("00u1", "Ada@Example.com"))
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}
	if first.Session == nil || first.Next != "/tasks" {
		t.Fatalf("first sign-in = %+v, want a session returning to /tasks", first)
	}
	second, err := f.signIn(t, samlPerson("00u1", "renamed@example.com"))
	if err != nil {
		t.Fatalf("second sign-in: %v", err)
	}
	if second.Session.ActorID != first.Session.ActorID {
		t.Errorf("second sign-in is actor %s, want %s: the NameID names the same person", second.Session.ActorID,
			first.Session.ActorID)
	}
	user, err := f.l.GetUser(authContext(f.admin), first.Session.ActorID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Email != "ada@example.com" || roleOf(t, ssoFixture{l: f.l, scope: f.scope, admin: f.admin}, user.ID) != core.RoleAdmin {
		t.Errorf("created user = %+v, want ada@example.com mapped to admin", user)
	}
	sessions, _, err := f.l.ListSessions(authContext(&core.Actor{ID: first.Session.ActorID, TenantID: f.scope.TenantID,
		Kind: core.ActorUser, Role: core.RoleAdmin}), core.Page{Limit: 10})
	if err != nil || len(sessions) == 0 || sessions[0].Method != "sso:okta" {
		t.Errorf("sessions = %+v, %v; want method sso:okta", sessions, err)
	}
}

func TestSAMLRefusesAReplayedResponse(t *testing.T) {
	f := newSAMLFixture(t, nil)
	begin, req := f.begin(t)
	response := f.idp.Response(t, req, samlPerson("00u1", "ada@example.com"), ssotest.SignAssertion)
	if _, err := f.post(begin, req.RelayState, response); err != nil {
		t.Fatalf("first post: %v", err)
	}
	again, err := f.post(begin, req.RelayState, response)
	if !core.IsKind(err, core.KindUnauthenticated) || !strings.Contains(err.Error(), "already been used") {
		t.Fatalf("replay = %+v, %v; want refused as already used", again, err)
	}
	if f.denials(t, ssoDeniedReplay) != 1 {
		t.Errorf("sso.denied for the replay = %d, want 1", f.denials(t, ssoDeniedReplay))
	}
	f.clk.Advance(10 * time.Minute)
	begin2, req2 := f.begin(t)
	if _, err := f.post(begin2, req2.RelayState, response); err == nil {
		t.Error("an old response posted into a new flow was accepted")
	}
}

func TestSAMLRefusesAResponseNotBelongingToTheFlow(t *testing.T) {
	f := newSAMLFixture(t, nil)
	begin, req := f.begin(t)
	other, otherReq := f.begin(t)
	response := f.idp.Response(t, req, samlPerson("00u1", "ada@example.com"), ssotest.SignAssertion)
	if _, err := f.post(begin, "forged", response); !core.IsKind(err, core.KindUnauthenticated) ||
		!strings.Contains(err.Error(), "RelayState") {
		t.Errorf("forged RelayState = %v, want refused naming it", err)
	}
	if _, err := f.post(other, otherReq.RelayState, response); !core.IsKind(err, core.KindUnauthenticated) ||
		!strings.Contains(err.Error(), "InResponseTo") {
		t.Errorf("response to another request = %v, want refused naming InResponseTo", err)
	}
	if f.denials(t, ssoDeniedToken) != 1 {
		t.Errorf("sso.denied for the bad response = %d, want 1", f.denials(t, ssoDeniedToken))
	}
	f.clk.Advance(time.Hour)
	if _, err := f.post(begin, req.RelayState, response); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("expired flow = %v, want refused", err)
	}
}

func TestSAMLRefusesABadSignatureWithoutSigningAnybodyIn(t *testing.T) {
	f := newSAMLFixture(t, nil)
	begin, req := f.begin(t)
	response := f.idp.Response(t, req, samlPerson("00u1", "ada@example.com"),
		ssotest.Signing{Assertion: true, ForeignKey: true})
	result, err := f.post(begin, req.RelayState, response)
	if !core.IsKind(err, core.KindUnauthenticated) || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("bad signature = %+v, %v; want refused", result, err)
	}
	account, err := f.l.accountByEmail(loginContext(f.scope), "ada@example.com")
	if err != nil || account != nil {
		t.Errorf("after a refused response the account is %+v, %v; want none", account, err)
	}
}

func TestSAMLIdPInitiatedIsOffUntilAllowed(t *testing.T) {
	f := newSAMLFixture(t, nil)
	_, req := f.begin(t)
	a := samlPerson("00u1", "ada@example.com")
	a.InResponseTo = "-"
	unsolicited := func() (*core.LoginResult, error) {
		return f.l.CompleteSSOLogin(loginContext(f.scope), core.CompleteSSOInput{Provider: "okta",
			RedirectURI: samlACSURL, SAMLResponse: f.idp.Response(t, req, a, ssotest.SignAssertion)})
	}
	if _, err := unsolicited(); !core.IsKind(err, core.KindUnauthenticated) || !strings.Contains(err.Error(), "IdP-initiated") {
		t.Fatalf("unsolicited = %v, want refused naming the setting", err)
	}
	if f.denials(t, ssoDeniedUnsolicited) != 1 {
		t.Errorf("sso.denied unsolicited = %d, want 1", f.denials(t, ssoDeniedUnsolicited))
	}
	on := true
	if _, err := f.l.UpdateSSOProvider(authContext(f.admin), "okta", core.UpdateSSOProviderInput{AllowIdPInitiated: &on}); err != nil {
		t.Fatalf("allowing IdP-initiated sign-in: %v", err)
	}
	result, err := unsolicited()
	if err != nil || result.Session == nil {
		t.Fatalf("allowed unsolicited = %+v, %v; want a session", result, err)
	}
	if _, err := f.l.CompleteSSOLogin(loginContext(f.scope), core.CompleteSSOInput{Provider: "okta",
		RedirectURI: "https://tix.example/elsewhere", SAMLResponse: "x"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("unsolicited to another address = %v, want invalid", err)
	}
}

func TestSAMLBeginRefusesTheCommandLine(t *testing.T) {
	f := newSAMLFixture(t, nil)
	_, err := f.l.BeginSSOLogin(loginContext(f.scope), core.BeginSSOInput{Provider: "okta",
		RedirectURI: "http://127.0.0.1:43123/callback"})
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "browser only") {
		t.Errorf("loopback begin = %v, want refused as browser only", err)
	}
	_, err = f.l.BeginSSOLogin(loginContext(f.scope), core.BeginSSOInput{Provider: "okta",
		RedirectURI: "https://tix.example/somewhere"})
	if core.FieldOf(err) != "redirect_uri" {
		t.Errorf("odd redirect = %v, want refused naming redirect_uri", err)
	}
}

func TestSAMLProviderReadsMetadataByURLAndKeepsExplicitValues(t *testing.T) {
	l, clk, _, admin := newLocalWith(t, WithTOTPKey(totpTestKey), WithEgress(egress.Permissive()),
		WithPublicURL("https://tix.example"))
	idp := ssotest.NewSAML(t, clk.Now)
	fetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches++
		if r.URL.Path != "/md" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(idp.Metadata()))
	}))
	t.Cleanup(srv.Close)
	ctx := authContext(admin)
	p, err := l.CreateSSOProvider(ctx, core.CreateSSOProviderInput{Key: "okta", Kind: core.SSOKindSAML,
		IdPMetadataURL: srv.URL + "/md", IdPSSOURL: "https://idp.example/pinned"})
	if err != nil {
		t.Fatalf("CreateSSOProvider: %v", err)
	}
	if p.IdPEntityID != idp.EntityID || p.IdPSSOURL != "https://idp.example/pinned" || len(p.IdPCertificates) != 1 ||
		p.RedirectURL != samlACSURL || p.MetadataURL != "https://tix.example/login/sso/saml/okta/metadata" || p.SecretSet {
		t.Fatalf("provider = %+v, want the metadata's entity and certificate, the pinned sign-in URL, the addresses", p)
	}
	idp.EntityID = "https://idp.example/rotated"
	url := srv.URL + "/md"
	p, err = l.UpdateSSOProvider(ctx, "okta", core.UpdateSSOProviderInput{IdPMetadataURL: &url})
	if err != nil || p.IdPEntityID != "https://idp.example/rotated" || p.IdPSSOURL != idp.SSOURL || fetches != 2 {
		t.Fatalf("re-read = %+v, %v (%d fetches); want the metadata read again", p, err, fetches)
	}
	missing := srv.URL + "/gone"
	if _, err := l.UpdateSSOProvider(ctx, "okta", core.UpdateSSOProviderInput{IdPMetadataURL: &missing}); !core.IsKind(err, core.KindUpstream) {
		t.Errorf("unreadable metadata = %v, want an upstream failure", err)
	}
	secret := "s3cret"
	if _, err := l.UpdateSSOProvider(ctx, "okta", core.UpdateSSOProviderInput{ClientSecret: &secret}); core.FieldOf(err) != "client_secret" {
		t.Errorf("secret on saml = %v, want refused", err)
	}
	if _, err := l.CreateSSOProvider(ctx, core.CreateSSOProviderInput{Key: "kc", Kind: core.SSOKindOIDC,
		Issuer: "https://id.example", ClientID: "c", IdPMetadataXML: idp.Metadata()}); core.FieldOf(err) != "idp_metadata_xml" {
		t.Errorf("metadata on oidc = %v, want refused", err)
	}
	if _, err := l.CreateSSOProvider(ctx, core.CreateSSOProviderInput{Key: "x", Kind: core.SSOKindSAML,
		IdPMetadataXML: "<nope/>"}); core.FieldOf(err) != "idp_metadata_xml" {
		t.Errorf("bad metadata = %v, want refused naming idp_metadata_xml", err)
	}
}

func TestSAMLMetadataIsPublicAndNamesTheAddresses(t *testing.T) {
	f := newSAMLFixture(t, nil)
	md, err := f.l.SAMLMetadata(loginContext(f.scope), core.SAMLMetadataInput{Provider: "okta",
		BaseURL: "https://tix.example/"})
	if err != nil {
		t.Fatalf("SAMLMetadata: %v", err)
	}
	if md.EntityID != "https://tix.example/login/sso/saml/okta/metadata" || md.ACSURL != samlACSURL ||
		!strings.Contains(md.XML, samlACSURL) {
		t.Errorf("metadata = %+v", md)
	}
	for _, in := range []core.SAMLMetadataInput{{Provider: "nobody", BaseURL: "https://tix.example"},
		{Provider: "", BaseURL: "https://tix.example"}} {
		if _, err := f.l.SAMLMetadata(loginContext(f.scope), in); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("SAMLMetadata(%+v) = %v, want not found", in, err)
		}
	}
	if md, err := f.l.SAMLMetadata(loginContext(f.scope), core.SAMLMetadataInput{Provider: "okta"}); err != nil ||
		md.ACSURL != samlACSURL {
		t.Errorf("no base = %+v, %v; want the public address", md, err)
	}
	if _, err := f.l.SAMLMetadata(loginContext(f.scope), core.SAMLMetadataInput{Provider: "okta",
		BaseURL: "not a url"}); core.FieldOf(err) != "base_url" {
		t.Errorf("bad base = %v, want refused naming base_url", err)
	}
	list, err := f.l.ListSSOProviders(authContext(f.admin))
	if err != nil || len(list) != 1 || list[0].RedirectURL != samlACSURL {
		t.Errorf("listed = %+v, %v; want the ACS as the address to register", list, err)
	}
}
