// SPDX-License-Identifier: AGPL-3.0-or-later

package sso

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"

	"github.com/heliopsy/tix/internal/core"
)

// SAMLClockSkew is how far in the future an assertion's NotBefore may be, for
// an IdP whose clock runs ahead. Expiry is not stretched.
const SAMLClockSkew = 2 * time.Minute

// samlMaxResponse bounds a posted response, decompressed.
const samlMaxResponse = 1 << 20

// Attribute names read when a provider names none, in order. They cover what
// Entra ID, ADFS, Okta, Google Workspace, Keycloak and Authentik send by
// default or by their documented mappings, and the LDAP OIDs.
var (
	samlEmailAttributes = []string{"email", "mail", "emailaddress", "Email",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"urn:oid:0.9.2342.19200300.100.1.3"}
	samlNameAttributes = []string{"name", "displayName", "displayname", "cn",
		"http://schemas.microsoft.com/identity/claims/displayname",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		"urn:oid:2.16.840.1.113730.3.1.241", "urn:oid:2.5.4.3"}
	samlUsernameAttributes = []string{"username", "preferred_username", "uid", "login",
		"http://schemas.goauthentik.io/2021/02/saml/username",
		"urn:oid:0.9.2342.19200300.100.1.1"}
	samlGroupsAttributes = []string{"groups", "memberOf", "Role", "role",
		"http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
		"http://schemas.xmlsoap.org/claims/Group",
		"urn:oid:1.3.6.1.4.1.5923.1.5.1.1"}
	samlGivenAttributes   = []string{"givenName", "firstName", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname", "urn:oid:2.5.4.42"}
	samlSurnameAttributes = []string{"sn", "surname", "lastName", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname", "urn:oid:2.5.4.4"}
)

// samlMethodsAttribute is where Entra ID lists the methods a person signed in
// with, http://schemas.microsoft.com/claims/multipleauthn among them.
const samlMethodsAttribute = "http://schemas.microsoft.com/claims/authnmethodsreferences"

// Origin is the scheme and host of an absolute address.
func Origin(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// SAMLEntityID is tix's entity id for a provider whose assertion consumer
// service is acsURL: the provider's override, or the metadata address on the
// same origin.
func SAMLEntityID(p core.SSOProvider, acsURL string) string {
	if p.SPEntityID != "" {
		return p.SPEntityID
	}
	return Origin(acsURL) + core.SAMLMetadataPath(p.Key)
}

// SAML is one SAML provider seen from one assertion consumer service address
// at one instant. It is built per sign-in, so its clock is the service's.
type SAML struct {
	cfg      core.SSOProvider
	acsURL   string
	entityID string
	now      time.Time
	sp       *saml2.SAMLServiceProvider
}

// NewSAML builds the service provider for p, answering at acsURL, with the
// time read from now.
func NewSAML(p core.SSOProvider, acsURL string, now time.Time) (*SAML, error) {
	certs, err := core.ParseCertificates(p.IdPCertificates)
	if err != nil {
		return nil, core.Precondition("the signing certificates of provider %q do not parse; store them again", p.Key)
	}
	if len(certs) == 0 {
		return nil, core.Precondition("provider %q has no IdP signing certificate", p.Key)
	}
	entityID := SAMLEntityID(p, acsURL)
	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:      p.IdPSSOURL,
		IdentityProviderSSOBinding:  saml2.BindingHttpRedirect,
		IdentityProviderIssuer:      p.IdPEntityID,
		AssertionConsumerServiceURL: acsURL,
		ServiceProviderIssuer:       entityID,
		AudienceURI:                 entityID,
		IDPCertificateStore:         &dsig.MemoryX509CertificateStore{Roots: certs},
		NameIdFormat:                p.NameIDFormat,
		AllowMissingAttributes:      true,
		Clock:                       dsig.NewFakeClockAt(now),
		MaximumDecompressedBodySize: samlMaxResponse,
	}
	return &SAML{cfg: p, acsURL: acsURL, entityID: entityID, now: now, sp: sp}, nil
}

// EntityID is tix's entity id for this provider.
func (s *SAML) EntityID() string { return s.entityID }

// AuthnRequest answers the IdP address that starts a sign-in, carrying an
// unsigned AuthnRequest over HTTP-Redirect and relayState, and the request's
// ID, which the response must answer.
func (s *SAML) AuthnRequest(relayState string) (string, string, error) {
	doc, err := s.sp.BuildAuthRequestDocumentNoSig()
	if err != nil {
		return "", "", core.Internal("building the saml authentication request").Wrap(err)
	}
	id := doc.Root().SelectAttrValue("ID", "")
	if id == "" {
		return "", "", core.Internal("the saml authentication request has no id")
	}
	addr, err := s.sp.BuildAuthURLRedirect(relayState, doc)
	if err != nil {
		return "", "", core.Internal("encoding the saml authentication request").Wrap(err)
	}
	return addr, id, nil
}

// Assertion is a verified assertion: who it names, its id and until when
// that id has to be remembered to refuse a replay.
type Assertion struct {
	Identity  Identity
	ID        string
	ExpiresAt time.Time
}

// Verify checks a posted SAMLResponse and reads the identity it asserts.
// requestID is the AuthnRequest the response must answer, or empty for an
// IdP-initiated response, which must then answer none.
func (s *SAML) Verify(encoded, requestID string) (Assertion, error) {
	encoded = strings.Join(strings.Fields(encoded), "")
	if encoded == "" {
		return Assertion{}, Refused("the identity provider posted no SAML response")
	}
	resp, err := s.sp.ValidateEncodedResponse(encoded)
	if err != nil {
		return Assertion{}, samlError(err)
	}
	if len(resp.Assertions) != 1 {
		return Assertion{}, Refused("the SAML response carries %d assertions; exactly one is accepted",
			len(resp.Assertions))
	}
	a := resp.Assertions[0]
	if err := s.checkAnswers(resp, &a, requestID); err != nil {
		return Assertion{}, err
	}
	expires, err := s.checkConditions(&a)
	if err != nil {
		return Assertion{}, err
	}
	if strings.TrimSpace(a.ID) == "" {
		return Assertion{}, Refused("the SAML assertion has no ID, so it cannot be held to one use")
	}
	ident, err := s.identity(&a)
	if err != nil {
		return Assertion{}, err
	}
	return Assertion{Identity: ident, ID: a.ID, ExpiresAt: expires}, nil
}

// checkAnswers holds the response to the request: the subject confirmation's
// InResponseTo, which the assertion's signature covers, and the response's
// own when it has one.
func (s *SAML) checkAnswers(resp *types.Response, a *types.Assertion, requestID string) error {
	got := ""
	if a.Subject != nil && a.Subject.SubjectConfirmation != nil && a.Subject.SubjectConfirmation.SubjectConfirmationData != nil {
		got = a.Subject.SubjectConfirmation.SubjectConfirmationData.InResponseTo
	}
	if requestID == "" {
		if got != "" || resp.InResponseTo != "" {
			return Refused("the SAML response answers a request this sign-in did not make; start again")
		}
		return nil
	}
	if got != requestID || (resp.InResponseTo != "" && resp.InResponseTo != requestID) {
		return Refused("the SAML response does not answer the request this browser started (InResponseTo); " +
			"start again")
	}
	return nil
}

// checkConditions refuses an assertion outside its validity window or for
// another audience, and answers the latest instant it is valid at.
func (s *SAML) checkConditions(a *types.Assertion) (time.Time, error) {
	c := a.Conditions
	if c == nil || c.NotOnOrAfter == "" {
		return time.Time{}, Refused("the SAML assertion has no NotOnOrAfter condition, so it never expires")
	}
	notAfter, err := time.Parse(time.RFC3339, c.NotOnOrAfter)
	if err != nil {
		return time.Time{}, Refused("the SAML assertion's NotOnOrAfter %q is not a time", c.NotOnOrAfter)
	}
	if !s.now.Before(notAfter) {
		return time.Time{}, Refused("the SAML assertion has expired; start again, and check both clocks")
	}
	if c.NotBefore != "" {
		notBefore, err := time.Parse(time.RFC3339, c.NotBefore)
		if err != nil {
			return time.Time{}, Refused("the SAML assertion's NotBefore %q is not a time", c.NotBefore)
		}
		if s.now.Add(SAMLClockSkew).Before(notBefore) {
			return time.Time{}, Refused("the SAML assertion is not valid yet; check the identity provider's clock")
		}
	}
	if len(c.AudienceRestrictions) == 0 {
		return time.Time{}, Refused("the SAML assertion names no audience; the identity provider must restrict it to %s",
			s.entityID)
	}
	for _, r := range c.AudienceRestrictions {
		if !slices.ContainsFunc(r.Audiences, func(v types.Audience) bool { return strings.TrimSpace(v.Value) == s.entityID }) {
			return time.Time{}, Refused("the SAML assertion is for another audience; set the identity provider's "+
				"audience (SP entity id) to %s", s.entityID)
		}
	}
	expires := notAfter
	if sc := a.Subject.SubjectConfirmation.SubjectConfirmationData; sc != nil {
		if t, err := time.Parse(time.RFC3339, sc.NotOnOrAfter); err == nil && t.After(expires) {
			expires = t
		}
	}
	return expires, nil
}

// identity reads the NameID and the configured attributes.
func (s *SAML) identity(a *types.Assertion) (Identity, error) {
	if a.Subject == nil || a.Subject.NameID == nil || strings.TrimSpace(a.Subject.NameID.Value) == "" {
		return Identity{}, Refused("the SAML assertion names no subject (NameID)")
	}
	if a.Subject.NameID.Format == core.NameIDTransient {
		return Identity{}, Refused("the identity provider sent a transient NameID, which changes at every sign-in; " +
			"set its NameID format to persistent or email address")
	}
	attrs := samlAttributes(a)
	pick := func(configured string, defaults []string) []string {
		if configured != "" {
			return attrs.values(configured)
		}
		for _, name := range defaults {
			if v := attrs.values(name); len(v) > 0 {
				return v
			}
		}
		return nil
	}
	first := func(v []string) string {
		if len(v) == 0 {
			return ""
		}
		return strings.TrimSpace(v[0])
	}
	nameID := strings.TrimSpace(a.Subject.NameID.Value)
	out := Identity{
		Subject:  nameID,
		Email:    strings.ToLower(first(pick(s.cfg.EmailClaim, samlEmailAttributes))),
		Name:     first(pick(s.cfg.NameClaim, samlNameAttributes)),
		Username: first(pick(s.cfg.UsernameClaim, samlUsernameAttributes)),
		Groups:   pick(s.cfg.GroupsClaim, samlGroupsAttributes),
	}
	if out.Email == "" && s.cfg.EmailClaim == "" && a.Subject.NameID.Format == core.NameIDEmail {
		out.Email = strings.ToLower(nameID)
	}
	if out.Name == "" && s.cfg.NameClaim == "" {
		out.Name = strings.TrimSpace(first(pick("", samlGivenAttributes)) + " " + first(pick("", samlSurnameAttributes)))
	}
	out.EmailVerified = out.Email != ""
	if len(s.cfg.MFAValues) > 0 {
		methods := attrs.values(samlMethodsAttribute)
		if a.AuthnStatement != nil && a.AuthnStatement.AuthnContext != nil &&
			a.AuthnStatement.AuthnContext.AuthnContextClassRef != nil {
			methods = append(methods, strings.TrimSpace(a.AuthnStatement.AuthnContext.AuthnContextClassRef.Value))
		}
		out.MFA = anyOf(methods, s.cfg.MFAValues)
	}
	return out, nil
}

// attributeSet is an assertion's attributes, findable by Name or
// FriendlyName.
type attributeSet []types.Attribute

func samlAttributes(a *types.Assertion) attributeSet {
	if a.AttributeStatement == nil {
		return nil
	}
	return attributeSet(a.AttributeStatement.Attributes)
}

func (s attributeSet) values(name string) []string {
	for _, attr := range s {
		if attr.Name != name && attr.FriendlyName != name {
			continue
		}
		var out []string
		for _, v := range attr.Values {
			if t := strings.TrimSpace(v.Value); t != "" {
				out = append(out, t)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// samlError names the check a response failed without echoing the document.
func samlError(err error) error {
	var invalid saml2.ErrInvalidValue
	var missing saml2.ErrMissingElement
	switch {
	case errors.Is(err, dsig.ErrMissingSignature):
		return Refused("the SAML response is not signed; the identity provider must sign the assertion or the response")
	case errors.As(err, &invalid):
		switch invalid.Key {
		case saml2.IssuerTag:
			return Refused("the SAML response was issued by %q, not the provider's IdP entity id", truncate(invalid.Actual))
		case saml2.DestinationAttr, saml2.RecipientAttr:
			return Refused("the SAML response is addressed to %q, not this provider's assertion consumer service %s",
				truncate(invalid.Actual), invalid.Expected)
		case saml2.StatusCodeTag:
			return Refused("the identity provider refused the sign-in (%s)", truncate(invalid.Actual))
		case saml2.NotOnOrAfterAttr:
			return Refused("the SAML assertion has expired; start again, and check both clocks")
		}
		return Refused("the SAML response has an unexpected %s", invalid.Key)
	case errors.As(err, &missing):
		return Refused("the SAML response has no %s", missing.Tag)
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Could not verify certificate"), strings.Contains(msg, "Signature could not be verified"),
		strings.Contains(msg, "signature"), strings.Contains(msg, "Signature"), strings.Contains(msg, "digest"),
		strings.Contains(msg, "Digest"):
		return Refused("the SAML response's signature does not verify against the provider's IdP certificates")
	case strings.Contains(msg, "Cert is not valid"):
		return Refused("the IdP certificate that signed the SAML response is expired or not yet valid")
	case strings.Contains(msg, "unexpected parent"), strings.Contains(msg, "signed and unsigned"):
		return Refused("the SAML response mixes signed and unsigned assertions")
	}
	return Refused("the SAML response does not parse as a signed response: %s", truncate(msg))
}

func truncate(s string) string {
	if len(s) > 120 {
		return s[:120]
	}
	return s
}

// SPMetadata is the service provider metadata an IdP imports for this
// provider: the entity id, the assertion consumer service, and that
// assertions must be signed.
func (s *SAML) SPMetadata() string {
	nameID := ""
	if s.cfg.NameIDFormat != "" {
		nameID = "\n    <md:NameIDFormat>" + xmlEscape(s.cfg.NameIDFormat) + "</md:NameIDFormat>"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s">
  <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">%s
    <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="%s" index="0" isDefault="true"/>
  </md:SPSSODescriptor>
</md:EntityDescriptor>
`, xmlEscape(s.entityID), nameID, xmlEscape(s.acsURL))
}

func xmlEscape(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(v)
}
