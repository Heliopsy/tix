// SPDX-License-Identifier: AGPL-3.0-or-later

package sso_test

import (
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sso"
	"github.com/heliopsy/tix/internal/sso/ssotest"
)

const samlACS = "https://tix.example/login/sso/saml/okta/acs"

var samlNow = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func samlProvider(idp *ssotest.SAML) core.SSOProvider {
	return core.SSOProvider{Key: "okta", Kind: core.SSOKindSAML, DisplayName: "Okta", IdPEntityID: idp.EntityID,
		IdPSSOURL: idp.SSOURL, IdPCertificates: []string{core.NormalizeCertificate(idp.CertPEM())}}
}

// samlStart begins a sign-in and answers the service provider and the
// request the fake IdP received.
func samlStart(t *testing.T, idp *ssotest.SAML, p core.SSOProvider) (*sso.SAML, ssotest.AuthnRequest) {
	t.Helper()
	sp, err := sso.NewSAML(p, samlACS, samlNow)
	if err != nil {
		t.Fatalf("NewSAML: %v", err)
	}
	addr, id, err := sp.AuthnRequest("relay-state")
	if err != nil {
		t.Fatalf("AuthnRequest: %v", err)
	}
	req := ssotest.ReadRequest(t, addr)
	if req.ID != id || req.ACS != samlACS || req.RelayState != "relay-state" {
		t.Fatalf("request = %+v, want id %s at %s with the relay state", req, id, samlACS)
	}
	if req.Issuer != "https://tix.example/login/sso/saml/okta/metadata" {
		t.Fatalf("request issuer = %q, want the metadata address", req.Issuer)
	}
	if !strings.HasPrefix(addr, "https://idp.example/saml/sso?") || !strings.Contains(addr, "tenant=acme") {
		t.Fatalf("auth url = %q, want the IdP's address with its own query kept", addr)
	}
	return sp, req
}

func ada() ssotest.Assertion {
	return ssotest.Assertion{NameID: "00u1ada", Attributes: map[string][]string{
		"email": {"Ada@Example.com"}, "displayName": {"Ada Lovelace"}, "uid": {"ada"},
		"groups": {"tix-admins", "staff"}}}
}

func TestSAMLVerifiesASignedAssertionAndReadsTheIdentity(t *testing.T) {
	for _, sign := range []ssotest.Signing{{Assertion: true}, {Response: true}, {Response: true, Assertion: true}} {
		idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
		sp, req := samlStart(t, idp, samlProvider(idp))
		got, err := sp.Verify(idp.Response(t, req, ada(), sign), req.ID)
		if err != nil {
			t.Fatalf("Verify (%+v): %v", sign, err)
		}
		want := sso.Identity{Subject: "00u1ada", Email: "ada@example.com", EmailVerified: true, Name: "Ada Lovelace",
			Username: "ada", Groups: []string{"tix-admins", "staff"}}
		if got.Identity.Subject != want.Subject || got.Identity.Email != want.Email || !got.Identity.EmailVerified ||
			got.Identity.Name != want.Name || got.Identity.Username != want.Username ||
			strings.Join(got.Identity.Groups, ",") != "tix-admins,staff" {
			t.Errorf("identity (%+v) = %+v, want %+v", sign, got.Identity, want)
		}
		if got.ID == "" || !got.ExpiresAt.Equal(samlNow.Add(5*time.Minute)) {
			t.Errorf("assertion id %q expires %v, want an id kept until %v", got.ID, got.ExpiresAt, samlNow.Add(5*time.Minute))
		}
	}
}

func TestSAMLReadsConfiguredAttributesAndTheSecondFactor(t *testing.T) {
	idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
	p := samlProvider(idp)
	p.EmailClaim, p.GroupsClaim, p.NameClaim = "work_mail", "teams", "full"
	p.TrustMFA, p.MFAValues = true, []string{"http://schemas.microsoft.com/claims/multipleauthn"}
	sp, req := samlStart(t, idp, p)
	a := ssotest.Assertion{NameID: "n1", Attributes: map[string][]string{"email": {"wrong@example.com"},
		"work_mail": {"ada@example.com"}, "teams": {"t1"}, "full": {"Ada"},
		"http://schemas.microsoft.com/claims/authnmethodsreferences": {"http://schemas.microsoft.com/claims/multipleauthn"}}}
	got, err := sp.Verify(idp.Response(t, req, a, ssotest.SignAssertion), req.ID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Identity.Email != "ada@example.com" || got.Identity.Name != "Ada" || len(got.Identity.Groups) != 1 ||
		!got.Identity.MFA {
		t.Errorf("identity = %+v, want the configured attributes and the second factor", got.Identity)
	}
	sp, req = samlStart(t, idp, p)
	a = ssotest.Assertion{NameID: "n1", AuthnContext: "urn:oasis:names:tc:SAML:2.0:ac:classes:Password"}
	if got, err = sp.Verify(idp.Response(t, req, a, ssotest.SignAssertion), req.ID); err != nil || got.Identity.MFA {
		t.Errorf("password only = %+v, %v; want no second factor", got.Identity, err)
	}
}

func TestSAMLTakesTheEmailFromAnEmailNameID(t *testing.T) {
	idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
	sp, req := samlStart(t, idp, samlProvider(idp))
	a := ssotest.Assertion{NameID: "Grace@Example.com", NameIDFormat: core.NameIDEmail,
		Attributes: map[string][]string{"givenName": {"Grace"}, "sn": {"Hopper"}}}
	got, err := sp.Verify(idp.Response(t, req, a, ssotest.SignAssertion), req.ID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Identity.Email != "grace@example.com" || got.Identity.Name != "Grace Hopper" {
		t.Errorf("identity = %+v, want the email from the NameID and the name from its parts", got.Identity)
	}
}

func TestSAMLRefusesAResponseThatFailsAnyCheck(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(*ssotest.Assertion)
		sign    ssotest.Signing
		request string
		want    string
	}{
		{"unsigned", nil, ssotest.Signing{}, "", "not signed"},
		{"bad signature", nil, ssotest.Signing{Assertion: true, ForeignKey: true}, "", "signature"},
		{"bad response signature", nil, ssotest.Signing{Response: true, ForeignKey: true}, "", "signature"},
		{"wrong audience", func(a *ssotest.Assertion) { a.Audience = "https://other.example" }, ssotest.SignAssertion, "", "another audience"},
		{"no audience", func(a *ssotest.Assertion) { a.NoAudience = true }, ssotest.SignAssertion, "", "no audience"},
		{"wrong recipient", func(a *ssotest.Assertion) { a.Recipient = "https://evil.example/acs" }, ssotest.SignAssertion, "", "addressed to"},
		{"wrong issuer", func(a *ssotest.Assertion) { a.Issuer = "https://evil.example" }, ssotest.SignAssertion, "", "issued by"},
		{"expired", func(a *ssotest.Assertion) {
			a.NotBefore, a.NotOnOrAfter = samlNow.Add(-time.Hour), samlNow.Add(-time.Minute)
		}, ssotest.SignAssertion, "", "expired"},
		{"conditions expired", func(a *ssotest.Assertion) {
			a.NotBefore, a.NotOnOrAfter, a.ConfirmFor = samlNow.Add(-time.Hour), samlNow.Add(-time.Minute), samlNow.Add(time.Minute)
		}, ssotest.SignAssertion, "", "expired"},
		{"two assertions", func(a *ssotest.Assertion) { a.Twice = true }, ssotest.Signing{Response: true}, "", "2 assertions"},
		{"not yet valid", func(a *ssotest.Assertion) { a.NotBefore = samlNow.Add(5 * time.Minute) }, ssotest.SignAssertion, "", "not valid yet"},
		{"other request", func(a *ssotest.Assertion) { a.InResponseTo = "_someone-elses" }, ssotest.SignAssertion, "", "InResponseTo"},
		{"unsolicited", func(a *ssotest.Assertion) { a.InResponseTo = "-" }, ssotest.SignAssertion, "", "InResponseTo"},
		{"answers a request when none was made", nil, ssotest.SignAssertion, "-", "did not make"},
		{"transient", func(a *ssotest.Assertion) { a.NameIDFormat = ssotest.NameIDTransient }, ssotest.SignAssertion, "", "transient"},
		{"no subject", func(a *ssotest.Assertion) { a.NameID = "" }, ssotest.SignAssertion, "", "subject"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
			sp, req := samlStart(t, idp, samlProvider(idp))
			a := ada()
			if tc.edit != nil {
				tc.edit(&a)
			}
			requestID := req.ID
			if tc.request == "-" {
				requestID = ""
			}
			_, err := sp.Verify(idp.Response(t, req, a, tc.sign), requestID)
			if !core.IsKind(err, core.KindUnauthenticated) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Verify = %v; want unauthenticated naming %q", err, tc.want)
			}
		})
	}
}

func TestSAMLAcceptsSmallClockSkewOnNotBefore(t *testing.T) {
	idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
	sp, req := samlStart(t, idp, samlProvider(idp))
	a := ada()
	a.NotBefore = samlNow.Add(time.Minute)
	if _, err := sp.Verify(idp.Response(t, req, a, ssotest.SignAssertion), req.ID); err != nil {
		t.Fatalf("Verify with the IdP a minute ahead: %v", err)
	}
}

func TestSAMLAcceptsAnUnsolicitedResponseOnlyWhenAskedTo(t *testing.T) {
	idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
	sp, req := samlStart(t, idp, samlProvider(idp))
	a := ada()
	a.InResponseTo = "-"
	if _, err := sp.Verify(idp.Response(t, req, a, ssotest.SignAssertion), ""); err != nil {
		t.Fatalf("unsolicited Verify: %v", err)
	}
}

// TestSAMLRefusesSignatureWrapping attacks the signature the ways XML
// signature wrapping does: an unsigned assertion beside the signed one, the
// signed response buried under an unsigned one, a signed assertion copied and
// edited under its original signature, and a tampered NameID.
func TestSAMLRefusesSignatureWrapping(t *testing.T) {
	evil := func(doc *etree.Document, name string) *etree.Element {
		el := doc.Root().FindElement("./Assertion").Copy()
		for _, s := range el.FindElements("./Signature") {
			el.RemoveChild(s)
		}
		el.CreateAttr("ID", "_evil")
		el.FindElement("./Subject/NameID").SetText(name)
		return el
	}
	cases := []struct {
		name string
		sign ssotest.Signing
		wrap func(t *testing.T, doc *etree.Document)
	}{
		{"unsigned assertion before the signed one", ssotest.SignAssertion, func(t *testing.T, doc *etree.Document) {
			root := doc.Root()
			root.InsertChildAt(root.FindElement("./Assertion").Index(), evil(doc, "admin"))
		}},
		{"unsigned assertion after the signed one", ssotest.SignAssertion, func(t *testing.T, doc *etree.Document) {
			doc.Root().AddChild(evil(doc, "admin"))
		}},
		{"unsigned assertion beside a signed response", ssotest.Signing{Response: true}, func(t *testing.T, doc *etree.Document) {
			doc.Root().AddChild(evil(doc, "admin"))
		}},
		{"signed response buried in an unsigned one", ssotest.Signing{Response: true}, func(t *testing.T, doc *etree.Document) {
			e := evil(doc, "admin")
			original := doc.Root().Copy()
			outer := doc.Root()
			for _, c := range outer.ChildElements() {
				if c.Tag == "Assertion" || c.Tag == "Signature" {
					outer.RemoveChild(c)
				}
			}
			ext := outer.CreateElement("samlp:Extensions")
			ext.AddChild(original)
			outer.AddChild(e)
		}},
		{"signed assertion copied under its signature", ssotest.SignAssertion, func(t *testing.T, doc *etree.Document) {
			copied := doc.Root().FindElement("./Assertion").Copy()
			copied.FindElement("./Subject/NameID").SetText("admin")
			doc.Root().InsertChildAt(doc.Root().FindElement("./Assertion").Index(), copied)
		}},
		{"NameID edited after signing", ssotest.SignAssertion, func(t *testing.T, doc *etree.Document) {
			doc.Root().FindElement("./Assertion/Subject/NameID").SetText("admin")
		}},
		{"response NameID edited after signing", ssotest.Signing{Response: true}, func(t *testing.T, doc *etree.Document) {
			doc.Root().FindElement("./Assertion/Subject/NameID").SetText("admin")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
			sp, req := samlStart(t, idp, samlProvider(idp))
			doc := idp.ResponseXML(t, req, ada(), tc.sign)
			tc.wrap(t, doc)
			got, err := sp.Verify(ssotest.Encode(t, doc), req.ID)
			if err == nil {
				t.Fatalf("Verify accepted a wrapped response naming %q", got.Identity.Subject)
			}
			if !core.IsKind(err, core.KindUnauthenticated) {
				t.Fatalf("Verify = %v, want unauthenticated", err)
			}
			t.Logf("refused: %v", err)
		})
	}
}

func TestParseIdPMetadataReadsTheRedirectEndpointAndSigningCertificate(t *testing.T) {
	idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
	md, err := sso.ParseIdPMetadata([]byte(idp.Metadata()), "")
	if err != nil {
		t.Fatalf("ParseIdPMetadata: %v", err)
	}
	if md.EntityID != idp.EntityID || md.SSOURL != idp.SSOURL || len(md.Certificates) != 1 ||
		md.Certificates[0] != core.NormalizeCertificate(idp.CertPEM()) {
		t.Errorf("metadata = %+v, want the entity, the redirect endpoint and only the signing certificate", md)
	}
	wrapped := `<EntitiesDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata">` +
		strings.TrimPrefix(idp.Metadata(), `<?xml version="1.0"?>`) + `</EntitiesDescriptor>`
	if md, err := sso.ParseIdPMetadata([]byte(wrapped), idp.EntityID); err != nil || md.EntityID != idp.EntityID {
		t.Errorf("EntitiesDescriptor = %+v, %v; want the named IdP", md, err)
	}
	for name, raw := range map[string]string{
		"not xml":      "hello",
		"other entity": idp.Metadata(),
		"no redirect":  strings.ReplaceAll(idp.Metadata(), "HTTP-Redirect", "SOAP"),
		"no signing":   strings.ReplaceAll(idp.Metadata(), `use="signing"`, `use="encryption"`),
	} {
		entity := ""
		if name == "other entity" {
			entity = "https://other.example"
		}
		if _, err := sso.ParseIdPMetadata([]byte(raw), entity); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("%s: ParseIdPMetadata = %v, want invalid", name, err)
		}
	}
}

func TestSPMetadataNamesTheEntityAndTheConsumerService(t *testing.T) {
	idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
	p := samlProvider(idp)
	p.NameIDFormat = core.NameIDPersistent
	sp, err := sso.NewSAML(p, samlACS, samlNow)
	if err != nil {
		t.Fatalf("NewSAML: %v", err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromString(sp.SPMetadata()); err != nil {
		t.Fatalf("metadata does not parse: %v", err)
	}
	root := doc.Root()
	acs := root.FindElement("./SPSSODescriptor/AssertionConsumerService")
	if root.SelectAttrValue("entityID", "") != sp.EntityID() || acs == nil ||
		acs.SelectAttrValue("Location", "") != samlACS ||
		root.FindElement("./SPSSODescriptor").SelectAttrValue("WantAssertionsSigned", "") != "true" ||
		root.FindElement("./SPSSODescriptor/NameIDFormat").Text() != core.NameIDPersistent {
		t.Errorf("metadata = %s", sp.SPMetadata())
	}
	p.SPEntityID = "urn:tix:acme"
	if got := sso.SAMLEntityID(p, samlACS); got != "urn:tix:acme" {
		t.Errorf("entity id = %q, want the override", got)
	}
}

func TestNewSAMLRefusesAProviderWithoutUsableCertificates(t *testing.T) {
	idp := ssotest.NewSAML(t, func() time.Time { return samlNow })
	p := samlProvider(idp)
	p.IdPCertificates = []string{"not a certificate"}
	if _, err := sso.NewSAML(p, samlACS, samlNow); !core.IsKind(err, core.KindPrecondition) {
		t.Errorf("NewSAML = %v, want a precondition", err)
	}
	p.IdPCertificates = nil
	if _, err := sso.NewSAML(p, samlACS, samlNow); !core.IsKind(err, core.KindPrecondition) {
		t.Errorf("NewSAML with none = %v, want a precondition", err)
	}
}
