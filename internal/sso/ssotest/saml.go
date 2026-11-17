// SPDX-License-Identifier: AGPL-3.0-or-later

package ssotest

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"html"
	"io"
	"maps"
	"math/big"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// SAML namespaces and the values a fake IdP writes.
// #nosec G101 -- SAML URNs, not credentials.
const (
	nsAssertion = "urn:oasis:names:tc:SAML:2.0:assertion"
	nsProtocol  = "urn:oasis:names:tc:SAML:2.0:protocol"
	bearer      = "urn:oasis:names:tc:SAML:2.0:cm:bearer"
	success     = "urn:oasis:names:tc:SAML:2.0:status:Success"
	// NameIDPersistent and NameIDTransient are the formats a test names.
	NameIDPersistent = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	NameIDTransient  = "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
)

// SAML is a fake SAML identity provider: a generated RSA key and a
// self-signed certificate it signs responses with, and another key that no
// configuration trusts.
type SAML struct {
	// EntityID is the IdP's entity id; SSOURL its sign-in address.
	EntityID string
	SSOURL   string

	key   *rsa.PrivateKey
	cert  *x509.Certificate
	other *rsa.PrivateKey
	ocert *x509.Certificate
	now   func() time.Time
	seq   int
}

// NewSAML makes a fake IdP whose certificates are valid around now.
func NewSAML(t testing.TB, now func() time.Time) *SAML {
	t.Helper()
	s := &SAML{EntityID: "https://idp.example/saml", SSOURL: "https://idp.example/saml/sso?tenant=acme", now: now}
	s.key, s.cert = newSigner(t, now(), "fake idp")
	s.other, s.ocert = newSigner(t, now(), "somebody else")
	return s
}

func newSigner(t testing.TB, now time.Time, name string) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(now.UnixNano()), Subject: pkix.Name{CommonName: name},
		NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(365 * 24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating a certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parsing the certificate: %v", err)
	}
	return key, cert
}

// CertPEM is the certificate the IdP signs with, as PEM.
func (s *SAML) CertPEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.cert.Raw}))
}

// OtherCertPEM is a certificate the IdP does not sign with.
func (s *SAML) OtherCertPEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.ocert.Raw}))
}

// Metadata is the IdP's metadata document.
func (s *SAML) Metadata() string {
	cert := base64.StdEncoding.EncodeToString(s.cert.Raw)
	return fmt.Sprintf(`<?xml version="1.0"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s">
  <md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:KeyDescriptor use="signing"><ds:KeyInfo><ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor>
    <md:KeyDescriptor use="encryption"><ds:KeyInfo><ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://idp.example/saml/post"/>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="%s"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`, s.EntityID, cert, base64.StdEncoding.EncodeToString(s.ocert.Raw), html.EscapeString(s.SSOURL))
}

// AuthnRequest is what a sign-in address carried to the IdP.
type AuthnRequest struct {
	ID         string
	ACS        string
	Issuer     string
	RelayState string
}

// ReadRequest decodes the AuthnRequest of an HTTP-Redirect sign-in address.
func ReadRequest(t testing.TB, authURL string) AuthnRequest {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parsing %q: %v", authURL, err)
	}
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("SAMLRequest"))
	if err != nil {
		t.Fatalf("decoding SAMLRequest: %v", err)
	}
	inflated, err := io.ReadAll(flate.NewReader(bytes.NewReader(raw)))
	if err != nil {
		t.Fatalf("inflating SAMLRequest: %v", err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(inflated); err != nil {
		t.Fatalf("reading SAMLRequest: %v", err)
	}
	root := doc.Root()
	out := AuthnRequest{ID: root.SelectAttrValue("ID", ""), ACS: root.SelectAttrValue("AssertionConsumerServiceURL", ""),
		RelayState: u.Query().Get("RelayState")}
	if iss := root.SelectElement("Issuer"); iss != nil {
		out.Issuer = iss.Text()
	}
	return out
}

// Assertion is what the next response asserts. Zero values take the
// defaults a well-behaved IdP would send for the request.
type Assertion struct {
	ID           string
	NameID       string
	NameIDFormat string
	Issuer       string
	Audience     string
	NoAudience   bool
	Recipient    string
	InResponseTo string
	NotBefore    time.Time
	NotOnOrAfter time.Time
	// ConfirmFor replaces the subject confirmation's NotOnOrAfter.
	ConfirmFor time.Time
	// Twice puts a second assertion in the response, a copy with its own ID.
	Twice        bool
	AuthnContext string
	Attributes   map[string][]string
}

// Signing says what the response's signatures are.
type Signing struct {
	// Response and Assertion sign the response, the assertion, or both;
	// neither leaves it unsigned.
	Response  bool
	Assertion bool
	// ForeignKey signs with the key no provider trusts.
	ForeignKey bool
}

// SignAssertion is the most common IdP default: the assertion signed.
var SignAssertion = Signing{Assertion: true}

// ResponseXML builds a response to a request, signed as asked.
func (s *SAML) ResponseXML(t testing.TB, req AuthnRequest, a Assertion, sign Signing) *etree.Document {
	t.Helper()
	now := s.now().UTC()
	s.seq++
	if a.ID == "" {
		a.ID = fmt.Sprintf("_assertion-%d-%d", now.UnixNano(), s.seq)
	}
	if a.Issuer == "" {
		a.Issuer = s.EntityID
	}
	if a.Audience == "" {
		a.Audience = req.Issuer
	}
	if a.Recipient == "" {
		a.Recipient = req.ACS
	}
	if a.InResponseTo == "" {
		a.InResponseTo = req.ID
	}
	if a.NotBefore.IsZero() {
		a.NotBefore = now.Add(-30 * time.Second)
	}
	if a.NotOnOrAfter.IsZero() {
		a.NotOnOrAfter = now.Add(5 * time.Minute)
	}
	if a.NameIDFormat == "" {
		a.NameIDFormat = NameIDPersistent
	}
	stamp := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }

	assertion := etree.NewElement("saml:Assertion")
	assertion.CreateAttr("xmlns:saml", nsAssertion)
	assertion.CreateAttr("ID", a.ID)
	assertion.CreateAttr("Version", "2.0")
	assertion.CreateAttr("IssueInstant", stamp(now))
	assertion.CreateElement("saml:Issuer").SetText(a.Issuer)
	subject := assertion.CreateElement("saml:Subject")
	nameID := subject.CreateElement("saml:NameID")
	nameID.CreateAttr("Format", a.NameIDFormat)
	nameID.SetText(a.NameID)
	conf := subject.CreateElement("saml:SubjectConfirmation")
	conf.CreateAttr("Method", bearer)
	data := conf.CreateElement("saml:SubjectConfirmationData")
	if a.InResponseTo != "-" {
		data.CreateAttr("InResponseTo", a.InResponseTo)
	}
	confirmFor := a.NotOnOrAfter
	if !a.ConfirmFor.IsZero() {
		confirmFor = a.ConfirmFor
	}
	data.CreateAttr("NotOnOrAfter", stamp(confirmFor))
	data.CreateAttr("Recipient", a.Recipient)
	conditions := assertion.CreateElement("saml:Conditions")
	conditions.CreateAttr("NotBefore", stamp(a.NotBefore))
	conditions.CreateAttr("NotOnOrAfter", stamp(a.NotOnOrAfter))
	if !a.NoAudience {
		conditions.CreateElement("saml:AudienceRestriction").CreateElement("saml:Audience").SetText(a.Audience)
	}
	authn := assertion.CreateElement("saml:AuthnStatement")
	authn.CreateAttr("AuthnInstant", stamp(now))
	authn.CreateAttr("SessionIndex", "_session")
	ctx := a.AuthnContext
	if ctx == "" {
		ctx = "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport"
	}
	authn.CreateElement("saml:AuthnContext").CreateElement("saml:AuthnContextClassRef").SetText(ctx)
	if len(a.Attributes) > 0 {
		stmt := assertion.CreateElement("saml:AttributeStatement")
		for _, name := range slices.Sorted(maps.Keys(a.Attributes)) {
			attr := stmt.CreateElement("saml:Attribute")
			attr.CreateAttr("Name", name)
			for _, v := range a.Attributes[name] {
				attr.CreateElement("saml:AttributeValue").SetText(v)
			}
		}
	}
	if sign.Assertion {
		assertion = s.sign(t, assertion, sign.ForeignKey)
	}

	resp := etree.NewElement("samlp:Response")
	resp.CreateAttr("xmlns:samlp", nsProtocol)
	resp.CreateAttr("xmlns:saml", nsAssertion)
	resp.CreateAttr("ID", "_response-"+a.ID)
	resp.CreateAttr("Version", "2.0")
	resp.CreateAttr("IssueInstant", stamp(now))
	resp.CreateAttr("Destination", a.Recipient)
	if a.InResponseTo != "-" {
		resp.CreateAttr("InResponseTo", a.InResponseTo)
	}
	resp.CreateElement("saml:Issuer").SetText(a.Issuer)
	resp.CreateElement("samlp:Status").CreateElement("samlp:StatusCode").CreateAttr("Value", success)
	resp.AddChild(assertion)
	if a.Twice {
		second := assertion.Copy()
		second.CreateAttr("ID", a.ID+"-2")
		resp.AddChild(second)
	}
	if sign.Response {
		resp = s.sign(t, resp, sign.ForeignKey)
	}
	doc := etree.NewDocument()
	doc.SetRoot(resp)
	return doc
}

// Response builds a response as ResponseXML does and encodes it as the
// HTTP-POST binding posts it.
func (s *SAML) Response(t testing.TB, req AuthnRequest, a Assertion, sign Signing) string {
	t.Helper()
	return Encode(t, s.ResponseXML(t, req, a, sign))
}

// Encode renders a response document as the base64 SAMLResponse form value.
func Encode(t testing.TB, doc *etree.Document) string {
	t.Helper()
	raw, err := doc.WriteToBytes()
	if err != nil {
		t.Fatalf("writing the response: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func (s *SAML) sign(t testing.TB, el *etree.Element, foreign bool) *etree.Element {
	t.Helper()
	key, cert := s.key, s.cert
	if foreign {
		key, cert = s.other, s.ocert
	}
	ctx, err := dsig.NewSigningContext(key, [][]byte{cert.Raw})
	if err != nil {
		t.Fatalf("signing context: %v", err)
	}
	ctx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	signed, err := ctx.SignEnveloped(el)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	return signed
}
