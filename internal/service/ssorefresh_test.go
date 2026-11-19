// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/sso/ssotest"
)

// metadataServer serves a fake IdP's metadata, or what a test makes it
// answer instead.
type metadataServer struct {
	*httptest.Server
	mu     sync.Mutex
	status int
	body   string
	idp    *ssotest.SAML
}

func newMetadataServer(t *testing.T, idp *ssotest.SAML) *metadataServer {
	t.Helper()
	m := &metadataServer{idp: idp, status: http.StatusOK}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.status != http.StatusOK {
			w.WriteHeader(m.status)
			return
		}
		body := m.body
		if body == "" {
			body = m.idp.Metadata()
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(m.Close)
	return m
}

func (m *metadataServer) answer(status int, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status, m.body = status, body
}

type refreshFixture struct {
	samlFixture
	md *metadataServer
}

// newRefreshFixture is a SAML provider read from a metadata address, on a
// service that refreshes every twelve hours.
func newRefreshFixture(t *testing.T, edit func(*core.CreateSSOProviderInput)) refreshFixture {
	t.Helper()
	l, clk, scope, admin := newLocalWith(t, WithTOTPKey(totpTestKey), WithEgress(egress.Permissive()),
		WithPublicURL("https://tix.example"), WithSAMLMetadataRefresh(12*time.Hour))
	idp := ssotest.NewSAML(t, clk.Now)
	md := newMetadataServer(t, idp)
	in := core.CreateSSOProviderInput{Key: "okta", Kind: core.SSOKindSAML, DisplayName: "Okta",
		IdPMetadataURL: md.URL + "/md", AutoCreate: true}
	if edit != nil {
		edit(&in)
	}
	if _, err := l.CreateSSOProvider(authContext(admin), in); err != nil {
		t.Fatalf("CreateSSOProvider: %v", err)
	}
	return refreshFixture{samlFixture: samlFixture{l: l, clk: clk, scope: scope, admin: admin, idp: idp}, md: md}
}

func (f refreshFixture) refresh(t *testing.T) (*core.SSOProvider, error) {
	t.Helper()
	return f.l.RefreshSAMLMetadata(authContext(f.admin), "okta")
}

func (f refreshFixture) provider(t *testing.T) *core.SSOProvider {
	t.Helper()
	p, err := f.l.GetSSOProvider(authContext(f.admin), "okta")
	if err != nil {
		t.Fatalf("GetSSOProvider: %v", err)
	}
	return p
}

func (f refreshFixture) refreshAudits(t *testing.T) int {
	t.Helper()
	entries, _, err := f.l.ListAudit(authContext(f.admin), core.AuditFilter{Page: core.Page{Limit: 200}})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	n := 0
	for _, e := range entries {
		if e.Action == auditSSOProviderRefresh {
			n++
		}
	}
	return n
}

func fingerprints(certs []string) []string {
	out := make([]string, 0, len(certs))
	for _, c := range certs {
		out = append(out, core.CertificateFingerprint(c))
	}
	slices.Sort(out)
	return out
}

func sameSet(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if g, w := fingerprints(got), fingerprints(want); !slices.Equal(g, w) {
		t.Fatalf("%s = %v, want %v", what, g, w)
	}
}

func TestSAMLRefreshRollsCertificatesOverWithAnOverlap(t *testing.T) {
	f := newRefreshFixture(t, nil)
	old := f.idp.CertPEM()
	created := f.provider(t)
	if created.MetadataRefresh == nil || created.MetadataRefresh.Next == nil ||
		!created.MetadataRefresh.Next.Equal(f.clk.Now().Add(12*time.Hour)) {
		t.Fatalf("created refresh state = %+v, want read now and next in 12h", created.MetadataRefresh)
	}

	f.idp.Rotate(t)
	f.clk.Advance(12 * time.Hour)
	p, err := f.refresh(t)
	if err != nil {
		t.Fatalf("refresh after rotation: %v", err)
	}
	sameSet(t, "certificates after rotation", p.IdPCertificates, old, f.idp.CertPEM())
	if p.MetadataRefresh.Result != core.SAMLRefreshUpdated || p.MetadataRefresh.Added != 1 || len(p.IdPRetiring) != 0 {
		t.Fatalf("rotation refresh = %+v, retiring %+v; want one added and nothing retiring", p.MetadataRefresh, p.IdPRetiring)
	}
	if _, err := f.signIn(t, samlPerson("00u1", "ada@example.com")); err != nil {
		t.Fatalf("sign-in signed by the new key: %v", err)
	}

	f.idp.Retire()
	f.clk.Advance(time.Hour)
	p, err = f.refresh(t)
	if err != nil {
		t.Fatalf("refresh after retirement: %v", err)
	}
	sameSet(t, "certificates in the overlap", p.IdPCertificates, old, f.idp.CertPEM())
	if len(p.IdPRetiring) != 1 || p.IdPRetiring[0].Fingerprint != core.CertificateFingerprint(old) ||
		!p.IdPRetiring[0].Until.Equal(f.clk.Now().Add(core.SAMLCertificateOverlap)) ||
		p.MetadataRefresh.Result != core.SAMLRefreshUnchanged {
		t.Fatalf("retirement = %+v, %+v; want the old certificate retiring for the overlap", p.IdPRetiring, p.MetadataRefresh)
	}

	f.clk.Advance(core.SAMLCertificateOverlap - time.Minute)
	if p, _ = f.refresh(t); len(p.IdPCertificates) != 2 {
		t.Fatalf("certificates just before the overlap ends = %d, want both", len(p.IdPCertificates))
	}
	f.clk.Advance(time.Minute)
	p, err = f.refresh(t)
	if err != nil {
		t.Fatalf("refresh after the overlap: %v", err)
	}
	sameSet(t, "certificates after the overlap", p.IdPCertificates, f.idp.CertPEM())
	if p.MetadataRefresh.Removed != 1 || len(p.IdPRetiring) != 0 || p.MetadataRefresh.Result != core.SAMLRefreshUpdated {
		t.Fatalf("drop = %+v, retiring %+v; want the old certificate removed", p.MetadataRefresh, p.IdPRetiring)
	}
	if n := f.refreshAudits(t); n != 2 {
		t.Errorf("refresh audit entries = %d, want 2: the added and the dropped certificate", n)
	}
}

func TestSAMLRefreshFailureKeepsTheCertificates(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		kind   core.Kind
	}{
		{"unreachable", http.StatusBadGateway, "", core.KindUpstream},
		{"not metadata", http.StatusOK, "<html>maintenance</html>", core.KindInvalid},
		{"no signing certificate", http.StatusOK, `<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" ` +
			`entityID="https://idp.example/saml"><md:IDPSSODescriptor><md:SingleSignOnService ` +
			`Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example/sso"/>` +
			`</md:IDPSSODescriptor></md:EntityDescriptor>`, core.KindInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRefreshFixture(t, nil)
			before := f.provider(t)
			f.md.answer(tc.status, tc.body)
			f.clk.Advance(12 * time.Hour)
			if _, err := f.refresh(t); !core.IsKind(err, tc.kind) {
				t.Fatalf("refresh = %v, want %s", err, tc.kind)
			}
			p := f.provider(t)
			sameSet(t, "certificates after a failed read", p.IdPCertificates, before.IdPCertificates...)
			if p.IdPSSOURL != before.IdPSSOURL || p.MetadataRefresh.Result != core.SAMLRefreshFailed ||
				p.MetadataRefresh.Error == "" || !p.MetadataRefresh.Next.Equal(f.clk.Now().Add(core.SAMLRefreshRetry)) {
				t.Fatalf("after a failed read = %+v, want the settings kept, the failure recorded, a retry in an hour",
					p.MetadataRefresh)
			}
			if !p.UpdatedAt.Equal(before.UpdatedAt) || f.refreshAudits(t) != 0 {
				t.Errorf("a failed read changed updated_at or wrote an audit entry")
			}
			if _, err := f.signIn(t, samlPerson("00u1", "ada@example.com")); err != nil {
				t.Errorf("sign-in with the kept certificate after a failed read: %v", err)
			}
		})
	}
}

func TestSAMLRefreshLeavesPinnedSettings(t *testing.T) {
	f := newRefreshFixture(t, func(in *core.CreateSSOProviderInput) {
		in.IdPSSOURL = "https://idp.example/pinned"
	})
	if p := f.provider(t); !slices.Equal(p.IdPPinned, []string{core.PinIdPSSOURL}) {
		t.Fatalf("pins = %v, want the sign-in URL", p.IdPPinned)
	}
	f.idp.SSOURL = "https://idp.example/moved"
	f.idp.Rotate(t)
	p, err := f.refresh(t)
	if err != nil || p.IdPSSOURL != "https://idp.example/pinned" || len(p.IdPCertificates) != 2 {
		t.Fatalf("refresh = %+v, %v; want the pinned URL kept and the new certificate added", p, err)
	}

	pinned := []string{f.idp.OtherCertPEM()}
	if _, err := f.l.UpdateSSOProvider(authContext(f.admin), "okta",
		core.UpdateSSOProviderInput{IdPCertificates: &pinned}); err != nil {
		t.Fatalf("pinning certificates: %v", err)
	}
	f.idp.Rotate(t)
	if p, err = f.refresh(t); err != nil {
		t.Fatalf("refresh with pinned certificates: %v", err)
	}
	sameSet(t, "pinned certificates", p.IdPCertificates, pinned...)

	same := p.IdPSSOURL
	if p, err = f.l.UpdateSSOProvider(authContext(f.admin), "okta", core.UpdateSSOProviderInput{IdPSSOURL: &same}); err != nil ||
		!p.Pinned(core.PinIdPSSOURL) || !p.Pinned(core.PinIdPCertificates) {
		t.Fatalf("saving a pinned value unchanged = %v, %v; want it still pinned", p, err)
	}
	empty := []string{}
	url := f.md.URL + "/md"
	if p, err = f.l.UpdateSSOProvider(authContext(f.admin), "okta", core.UpdateSSOProviderInput{IdPCertificates: &empty,
		IdPMetadataURL: &url}); err != nil || p.Pinned(core.PinIdPCertificates) {
		t.Fatalf("clearing the certificates with a metadata read = %v, %v; want them unpinned", p, err)
	}
	if len(p.IdPCertificates) != 3 || !slices.Contains(fingerprints(p.IdPCertificates), core.CertificateFingerprint(f.idp.CertPEM())) {
		t.Fatalf("certificates read on save = %d, want the three the metadata publishes", len(p.IdPCertificates))
	}
}

func TestSAMLRefreshEditsKeepUnchangedValuesUnpinned(t *testing.T) {
	f := newRefreshFixture(t, nil)
	p := f.provider(t)
	entity, ssoURL, certs := p.IdPEntityID, p.IdPSSOURL, slices.Clone(p.IdPCertificates)
	name := "Okta SSO"
	p, err := f.l.UpdateSSOProvider(authContext(f.admin), "okta", core.UpdateSSOProviderInput{DisplayName: &name,
		IdPEntityID: &entity, IdPSSOURL: &ssoURL, IdPCertificates: &certs})
	if err != nil || len(p.IdPPinned) != 0 {
		t.Fatalf("an edit giving the stored values = %v, %v; want nothing pinned, as the web dialog sends them", p, err)
	}
}

func TestSAMLRefreshRefusesAProviderWithoutAMetadataURL(t *testing.T) {
	f := newSAMLFixture(t, nil)
	if _, err := f.l.RefreshSAMLMetadata(authContext(f.admin), "okta"); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("refresh of a provider read from a pasted document = %v, want a precondition refusal", err)
	}
	if _, err := f.l.RefreshSAMLMetadata(authContext(f.admin), "nope"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("refresh of an unknown provider = %v, want not found", err)
	}
}

func TestSAMLRefreshNeedsSSOAdmin(t *testing.T) {
	f := newRefreshFixture(t, nil)
	cases := []struct {
		name  string
		actor *core.Actor
		ok    bool
	}{
		{"member", &core.Actor{ID: "m1", TenantID: f.scope.TenantID, Kind: core.ActorUser, Role: core.RoleMember}, false},
		{"viewer", &core.Actor{ID: "v1", TenantID: f.scope.TenantID, Kind: core.ActorUser, Role: core.RoleViewer}, false},
		{"admin", f.admin, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.l.RefreshSAMLMetadata(authContext(tc.actor), "okta")
			if tc.ok != (err == nil) || (!tc.ok && !core.IsKind(err, core.KindForbidden)) {
				t.Fatalf("refresh as %s = %v, want allowed %v", tc.name, err, tc.ok)
			}
		})
	}
}

func TestSAMLRefreshDueness(t *testing.T) {
	clk := clock.NewFakeAt()
	now := clk.Now()
	at := func(d time.Duration, result string) *core.SAMLMetadataRefresh {
		return &core.SAMLMetadataRefresh{At: now.Add(-d), Result: result}
	}
	saml := core.SSOProvider{Kind: core.SSOKindSAML, Enabled: true, IdPMetadataURL: "https://idp.example/md"}
	with := func(p core.SSOProvider, edit func(*core.SSOProvider)) core.SSOProvider { edit(&p); return p }
	cases := []struct {
		name     string
		p        core.SSOProvider
		interval time.Duration
		due      bool
	}{
		{"never read", saml, 12 * time.Hour, true},
		{"read an hour ago", with(saml, func(p *core.SSOProvider) { p.MetadataRefresh = at(time.Hour, "unchanged") }), 12 * time.Hour, false},
		{"read twelve hours ago", with(saml, func(p *core.SSOProvider) { p.MetadataRefresh = at(12*time.Hour, "updated") }), 12 * time.Hour, true},
		{"failed an hour ago", with(saml, func(p *core.SSOProvider) { p.MetadataRefresh = at(time.Hour, "failed") }), 12 * time.Hour, true},
		{"failed a minute ago", with(saml, func(p *core.SSOProvider) { p.MetadataRefresh = at(time.Minute, "failed") }), 12 * time.Hour, false},
		{"refresh off", saml, 0, false},
		{"disabled", with(saml, func(p *core.SSOProvider) { p.Enabled = false }), 12 * time.Hour, false},
		{"no metadata url", with(saml, func(p *core.SSOProvider) { p.IdPMetadataURL = "" }), 12 * time.Hour, false},
		{"oidc", with(saml, func(p *core.SSOProvider) { p.Kind = core.SSOKindOIDC }), 12 * time.Hour, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.MetadataRefreshDue(now, tc.interval); got != tc.due {
				t.Errorf("due = %v, want %v", got, tc.due)
			}
		})
	}
}

func TestSAMLRefreshScheduleIsOffWithoutAnInterval(t *testing.T) {
	l, clk, _, admin := newLocalWith(t, WithTOTPKey(totpTestKey), WithEgress(egress.Permissive()))
	idp := ssotest.NewSAML(t, clk.Now)
	md := newMetadataServer(t, idp)
	p, err := l.CreateSSOProvider(authContext(admin), core.CreateSSOProviderInput{Key: "okta", Kind: core.SSOKindSAML,
		IdPMetadataURL: md.URL + "/md"})
	if err != nil || p.MetadataRefresh == nil || p.MetadataRefresh.Next != nil {
		t.Fatalf("created = %+v, %v; want read now and no next read", p, err)
	}
	if p, err = l.RefreshSAMLMetadata(authContext(admin), "okta"); err != nil || p.MetadataRefresh.Next != nil {
		t.Fatalf("manual refresh with the schedule off = %+v, %v; want it read and no next read", p, err)
	}
}
