// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sso/ssotest"
)

// The single sign-on rows sit in their own file and join the table here, so
// the provider operations are held to the same equivalence as every other.
func init() {
	serviceScenarios = append(serviceScenarios, svcScenario{
		name: "single sign-on providers and a sign-in that comes back with the wrong state",
		covers: []string{"CreateSSOProvider", "GetSSOProvider", "UpdateSSOProvider", "ListSSOProviders",
			"SignInOptions", "BeginSSOLogin", "CompleteSSOLogin", "ExchangeSSOCode", "DeleteSSOProvider"},
		run: func(t *testing.T, tg target, _ *matrixHarness) (sig, error) {
			idp := ssotest.NewOIDC(t, time.Now)
			created, err := tg.svc.CreateSSOProvider(tg.ctx, core.CreateSSOProviderInput{Key: "kc", Kind: core.SSOKindOIDC,
				DisplayName: "Keycloak", Issuer: idp.Issuer(), ClientID: idp.ClientID, ClientSecret: idp.ClientSecret})
			if err != nil {
				return nil, err
			}
			shown, err := tg.svc.GetSSOProvider(tg.ctx, "kc")
			mustf(t, tg, err, "showing the provider")
			name := "KC"
			updated, err := tg.svc.UpdateSSOProvider(tg.ctx, "kc", core.UpdateSSOProviderInput{DisplayName: &name})
			mustf(t, tg, err, "renaming the provider")
			listed, err := tg.svc.ListSSOProviders(tg.ctx)
			mustf(t, tg, err, "listing providers")
			options, err := tg.svc.SignInOptions(tg.ctx)
			mustf(t, tg, err, "reading the sign-in options")
			begin, err := tg.svc.BeginSSOLogin(tg.ctx, core.BeginSSOInput{Provider: "kc",
				RedirectURI: "http://127.0.0.1:4100/callback"})
			mustf(t, tg, err, "beginning a sign-in")
			_, completeErr := tg.svc.CompleteSSOLogin(tg.ctx, core.CompleteSSOInput{Flow: begin.Flow,
				Code: idp.Grant(begin.AuthURL), State: "forged"})
			_, exchangeErr := tg.svc.ExchangeSSOCode(tg.ctx, core.SSOExchangeInput{Code: "never-handed",
				CodeVerifier: "some-verifier-0123456789-0123456789-0123456789"})
			_, malformedErr := tg.svc.ExchangeSSOCode(tg.ctx, core.SSOExchangeInput{Code: "never-handed"})
			if err := tg.svc.DeleteSSOProvider(tg.ctx, "kc"); err != nil {
				return nil, err
			}
			return sig{
				"key":         created.Key,
				"secret_set":  shown.SecretSet,
				"renamed":     updated.DisplayName,
				"listed":      len(listed),
				"offered":     len(options.Providers),
				"pkce":        strings.Contains(begin.AuthURL, "code_challenge_method=S256"),
				"complete":    string(core.KindOf(completeErr)),
				"exchange":    string(core.KindOf(exchangeErr)),
				"malformed":   string(core.KindOf(malformedErr)),
				"app_sign_in": options.AppSignIn,
			}, nil
		},
	})
	serviceScenarios = append(serviceScenarios, svcScenario{
		name:   "a saml provider, its metadata, a response posted with the wrong relay state and a refresh with no metadata url",
		covers: []string{"SAMLMetadata", "RefreshSAMLMetadata"},
		run: func(t *testing.T, tg target, _ *matrixHarness) (sig, error) {
			idp := ssotest.NewSAML(t, time.Now)
			created, err := tg.svc.CreateSSOProvider(tg.ctx, core.CreateSSOProviderInput{Key: "okta",
				Kind: core.SSOKindSAML, DisplayName: "Okta", IdPMetadataXML: idp.Metadata()})
			if err != nil {
				return nil, err
			}
			md, err := tg.svc.SAMLMetadata(tg.ctx, core.SAMLMetadataInput{Provider: "okta", BaseURL: "https://tix.example"})
			mustf(t, tg, err, "reading the metadata")
			begin, err := tg.svc.BeginSSOLogin(tg.ctx, core.BeginSSOInput{Provider: "okta",
				RedirectURI: "https://tix.example" + core.SSOCallbackPath})
			mustf(t, tg, err, "beginning a saml sign-in")
			_, completeErr := tg.svc.CompleteSSOLogin(tg.ctx, core.CompleteSSOInput{Flow: begin.Flow, State: "forged",
				SAMLResponse: "PHJlc3BvbnNlLz4=", Provider: "okta"})
			_, refreshErr := tg.svc.RefreshSAMLMetadata(tg.ctx, "okta")
			if err := tg.svc.DeleteSSOProvider(tg.ctx, "okta"); err != nil {
				return nil, err
			}
			return sig{
				"idp":      created.IdPEntityID == idp.EntityID,
				"certs":    len(created.IdPCertificates),
				"metadata": strings.HasSuffix(md.EntityID, core.SAMLMetadataPath("okta")),
				"acs":      strings.Contains(md.XML, core.SAMLACSPath("okta")),
				"kind":     string(begin.Kind),
				"request":  strings.Contains(begin.AuthURL, "SAMLRequest="),
				"complete": string(core.KindOf(completeErr)),
				"refresh":  string(core.KindOf(refreshErr)),
			}, nil
		},
	})
}
