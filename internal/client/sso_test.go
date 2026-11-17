// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestSSOCallsIssueTheExpectedRequests(t *testing.T) {
	ctx := context.Background()
	answer := func(body string) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(wire.HeaderContentType, wire.ContentJSON)
			_, _ = io.WriteString(w, body)
		}
	}
	provider := `{"id":"p1","key":"kc","kind":"oidc"}`
	for _, tc := range []struct {
		name, method, path, body string
		call                     func(*Client) error
	}{
		{"list", http.MethodGet, wire.RouteSSOProviders, `{"items":[` + provider + `]}`,
			func(c *Client) error { _, err := c.ListSSOProviders(ctx); return err }},
		{"get", http.MethodGet, wire.APIPrefix + "/sso/providers/kc", provider,
			func(c *Client) error { _, err := c.GetSSOProvider(ctx, "kc"); return err }},
		{"create", http.MethodPost, wire.RouteSSOProviders, provider,
			func(c *Client) error {
				_, err := c.CreateSSOProvider(ctx, core.CreateSSOProviderInput{Key: "kc"})
				return err
			}},
		{"update", http.MethodPatch, wire.APIPrefix + "/sso/providers/kc", provider,
			func(c *Client) error {
				_, err := c.UpdateSSOProvider(ctx, "kc", core.UpdateSSOProviderInput{})
				return err
			}},
		{"options", http.MethodGet, wire.RouteSignInOptions, `{"password_login":"allowed","providers":[]}`,
			func(c *Client) error { _, err := c.SignInOptions(ctx); return err }},
		{"begin", http.MethodPost, wire.RouteSSOBegin, `{"auth_url":"https://x","flow":"f","state":"s"}`,
			func(c *Client) error { _, err := c.BeginSSOLogin(ctx, core.BeginSSOInput{Provider: "kc"}); return err }},
		{"complete", http.MethodPost, wire.RouteSSOComplete, `{"session":{"token":"t"}}`,
			func(c *Client) error { _, err := c.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: "f"}); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, got := newClient(t, answer(tc.body))
			if err := tc.call(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if got.method != tc.method || got.path != tc.path {
				t.Errorf("request = %s %s, want %s %s", got.method, got.path, tc.method, tc.path)
			}
		})
	}
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	if err := c.DeleteSSOProvider(ctx, "kc"); err != nil || got.method != http.MethodDelete {
		t.Errorf("delete = %v, %s", err, got.method)
	}
}

func TestSAMLMetadataReadsTheDocument(t *testing.T) {
	doc := `<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://tix.example/m">` +
		`<md:SPSSODescriptor><md:AssertionConsumerService Location="https://tix.example/acs"/></md:SPSSODescriptor>` +
		`</md:EntityDescriptor>`
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(wire.HeaderContentType, wire.ContentSAMLMetadata)
		_, _ = io.WriteString(w, doc)
	})
	md, err := c.SAMLMetadata(context.Background(), core.SAMLMetadataInput{Provider: "okta"})
	if err != nil {
		t.Fatalf("SAMLMetadata: %v", err)
	}
	if got.method != http.MethodGet || got.path != wire.APIPrefix+"/auth/sso/providers/okta/saml-metadata" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if md.XML != doc || md.EntityID != "https://tix.example/m" || md.ACSURL != "https://tix.example/acs" {
		t.Errorf("metadata = %+v", md)
	}
	c, _ = newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(wire.HeaderContentType, wire.ContentJSON)
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":"not_found","message":"no saml provider"}}`)
	})
	if _, err := c.SAMLMetadata(context.Background(), core.SAMLMetadataInput{Provider: "x"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("missing provider = %v, want not found", err)
	}
}
