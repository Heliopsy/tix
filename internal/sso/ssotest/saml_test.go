// SPDX-License-Identifier: AGPL-3.0-or-later

package ssotest

import (
	"bytes"
	"compress/flate"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// TestTheFakeSAMLIdPSignsWhatItSaysItSigns pins the fake IdP: a fake whose
// signatures did not verify, or whose foreign key verified, would make every
// refusal test built on it meaningless.
func TestTheFakeSAMLIdPSignsWhatItSaysItSigns(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	idp := NewSAML(t, func() time.Time { return now })
	trusted := certOf(t, idp.CertPEM())
	if certOf(t, idp.OtherCertPEM()).Equal(trusted) {
		t.Fatal("the foreign certificate is the trusted one")
	}
	if !strings.Contains(idp.Metadata(), base64.StdEncoding.EncodeToString(trusted.Raw)) {
		t.Error("the metadata does not carry the signing certificate")
	}
	req := ReadRequest(t, authURL(t, `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" `+
		`xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_r1" AssertionConsumerServiceURL="https://sp/acs">`+
		`<saml:Issuer>https://sp/md</saml:Issuer></samlp:AuthnRequest>`))
	if req.ID != "_r1" || req.ACS != "https://sp/acs" || req.Issuer != "https://sp/md" || req.RelayState != "rs" {
		t.Fatalf("request = %+v", req)
	}
	validate := func(el *etree.Element) error {
		ctx := dsig.NewDefaultValidationContext(&dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{trusted}})
		ctx.Clock = dsig.NewFakeClockAt(now)
		_, err := ctx.Validate(el)
		return err
	}
	a := Assertion{NameID: "n1", ConfirmFor: now.Add(time.Minute), Twice: true,
		Attributes: map[string][]string{"b": {"2"}, "a": {"1"}}}
	parsed := func(doc *etree.Document) *etree.Element {
		raw, err := base64.StdEncoding.DecodeString(Encode(t, doc))
		if err != nil {
			t.Fatal(err)
		}
		back := etree.NewDocument()
		if err := back.ReadFromBytes(raw); err != nil {
			t.Fatal(err)
		}
		return back.Root()
	}
	signed := idp.ResponseXML(t, req, a, Signing{Response: true})
	if err := validate(parsed(signed)); err != nil {
		t.Errorf("the response signature does not verify: %v", err)
	}
	if len(signed.Root().FindElements("./Assertion")) != 2 {
		t.Error("Twice did not add a second assertion")
	}
	if validate(parsed(idp.ResponseXML(t, req, a, Signing{Response: true, ForeignKey: true}))) == nil {
		t.Error("a response signed with the foreign key verified")
	}
	if idp.ResponseXML(t, req, a, Signing{Assertion: true}).Root().FindElement("./Assertion/Signature") == nil {
		t.Error("the assertion carries no signature")
	}
	if raw, err := base64.StdEncoding.DecodeString(idp.Response(t, req, a, Signing{})); err != nil ||
		!bytes.Contains(raw, []byte(`InResponseTo="_r1"`)) || bytes.Contains(raw, []byte("Signature")) {
		t.Errorf("an unsigned response = %s, %v", raw, err)
	}
}

func certOf(t *testing.T, text string) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode([]byte(text))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func authURL(t *testing.T, request string) string {
	t.Helper()
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	_, _ = w.Write([]byte(request))
	_ = w.Close()
	q := url.Values{"SAMLRequest": {base64.StdEncoding.EncodeToString(buf.Bytes())}, "RelayState": {"rs"}}
	return "https://idp.example/sso?" + q.Encode()
}
