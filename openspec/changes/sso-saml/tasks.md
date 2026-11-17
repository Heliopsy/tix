# Tasks

## Groundwork

- [ ] Migration: `sso_assertions` for replay protection; store methods on both engines
- [ ] `core`: SAML settings on `SSOProvider` and its inputs, validation, ACS and metadata paths, `SAMLMetadata`
- [ ] `internal/sso`: SAML service provider on gosaml2, IdP metadata parsing, SP metadata, a fake IdP in `ssotest`

## Service

- [ ] Accept kind `saml`; read IdP metadata by URL or XML on create and edit
- [ ] `BeginSSOLogin` builds an AuthnRequest for a SAML provider; `CompleteSSOLogin` takes a SAML response
- [ ] Assertion checks, replay protection, IdP-initiated sign-in behind a setting, then `signInIdentity`
- [ ] `SAMLMetadata`

## Surfaces

- [ ] Registry entry, HTTP route, client, `just openapi`
- [ ] `tix tenant sso add/edit` SAML flags, `tix tenant sso metadata`
- [ ] Web: ACS and metadata routes, SAML settings on the Single sign-on screen
- [ ] i18n: bg, de, fr

## Tests

- [ ] Signed by a generated IdP key end to end; negative tests for a bad signature, wrong audience, expired,
      replayed, unsigned and wrapped responses

## Docs

- [ ] docs/sso.md with Entra ID, Okta, Keycloak, Authentik and Google Workspace; README, ROADMAP, skill,
      docs/commands.md
