// SPDX-License-Identifier: AGPL-3.0-or-later

package sso

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// samlMaxMetadata bounds an IdP metadata document. Entra's federation
// metadata, the largest common one, is well under it.
const samlMaxMetadata = 2 << 20

// IdPMetadata is what tix reads from a SAML IdP's metadata.
type IdPMetadata struct {
	EntityID     string
	SSOURL       string
	Certificates []string
}

type mdEntities struct {
	XMLName  xml.Name   `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntitiesDescriptor"`
	Entities []mdEntity `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
}

type mdEntity struct {
	XMLName  xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	EntityID string   `xml:"entityID,attr"`
	IdP      *struct {
		Keys []struct {
			Use   string   `xml:"use,attr"`
			Certs []string `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo>X509Data>X509Certificate"`
		} `xml:"urn:oasis:names:tc:SAML:2.0:metadata KeyDescriptor"`
		SSO []struct {
			Binding  string `xml:"Binding,attr"`
			Location string `xml:"Location,attr"`
		} `xml:"urn:oasis:names:tc:SAML:2.0:metadata SingleSignOnService"`
	} `xml:"urn:oasis:names:tc:SAML:2.0:metadata IDPSSODescriptor"`
}

// ParseIdPMetadata reads an IdP's entity id, HTTP-Redirect sign-in address
// and signing certificates from its metadata: one EntityDescriptor, or an
// EntitiesDescriptor holding the IdP named by entityID, or holding one IdP.
func ParseIdPMetadata(raw []byte, entityID string) (IdPMetadata, error) {
	var entities []mdEntity
	var one mdEntity
	if err := xml.Unmarshal(raw, &one); err == nil {
		entities = []mdEntity{one}
	} else {
		var all mdEntities
		if err := xml.Unmarshal(raw, &all); err != nil {
			return IdPMetadata{}, metadataInvalid("the IdP metadata is not a SAML EntityDescriptor or EntitiesDescriptor")
		}
		entities = all.Entities
	}
	var idps []mdEntity
	for _, e := range entities {
		if e.IdP != nil && (entityID == "" || e.EntityID == entityID) {
			idps = append(idps, e)
		}
	}
	switch {
	case len(idps) == 0 && entityID != "":
		return IdPMetadata{}, metadataInvalid("the IdP metadata describes no identity provider %q", entityID)
	case len(idps) == 0:
		return IdPMetadata{}, metadataInvalid("the IdP metadata describes no identity provider (IDPSSODescriptor)")
	case len(idps) > 1:
		return IdPMetadata{}, metadataInvalid("the IdP metadata describes %d identity providers; name one with "+
			"the IdP entity id", len(idps))
	}
	e := idps[0]
	out := IdPMetadata{EntityID: strings.TrimSpace(e.EntityID)}
	for _, s := range e.IdP.SSO {
		if s.Binding == bindingRedirect {
			out.SSOURL = strings.TrimSpace(s.Location)
			break
		}
	}
	if out.SSOURL == "" {
		return IdPMetadata{}, metadataInvalid("the IdP metadata offers no HTTP-Redirect SingleSignOnService, " +
			"the binding tix sends its request over")
	}
	for _, k := range e.IdP.Keys {
		if k.Use != "" && k.Use != "signing" {
			continue
		}
		for _, c := range k.Certs {
			if pem := core.NormalizeCertificate(c); pem != "" {
				out.Certificates = append(out.Certificates, pem)
			}
		}
	}
	if len(out.Certificates) == 0 {
		return IdPMetadata{}, metadataInvalid("the IdP metadata names no signing certificate")
	}
	return out, nil
}

const bindingRedirect = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"

func metadataInvalid(format string, args ...any) error {
	return core.OnField(core.Invalid(format, args...), "idp_metadata_url")
}

// FetchIdPMetadata reads an IdP's metadata from its address, through the
// client's egress policy, and parses it.
func (c *Client) FetchIdPMetadata(ctx context.Context, addr, entityID string) (IdPMetadata, error) {
	target, err := c.policy.VetURL(ctx, addr)
	if err != nil {
		return IdPMetadata{}, metadataInvalid("the IdP metadata URL %q is not one this server may read: it must be "+
			"https, carry no user name, and not resolve to a private or local address unless server.egress_allow "+
			"names it", addr)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return IdPMetadata{}, metadataInvalid("the IdP metadata URL %q is not an address", addr)
	}
	req.Header.Set("Accept", "application/samlmetadata+xml, application/xml, text/xml")
	resp, err := c.http.Do(req)
	if err != nil {
		return IdPMetadata{}, core.Upstream("reading the IdP metadata at %s failed", addr).Wrap(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return IdPMetadata{}, core.Upstream("reading the IdP metadata at %s answered %s", addr, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, samlMaxMetadata+1))
	if err != nil {
		return IdPMetadata{}, core.Upstream("reading the IdP metadata at %s failed", addr).Wrap(err)
	}
	if len(raw) > samlMaxMetadata {
		return IdPMetadata{}, metadataInvalid("the IdP metadata at %s is larger than 2 MiB", addr)
	}
	return ParseIdPMetadata(raw, entityID)
}
