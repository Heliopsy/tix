# Tasks

## Groundwork

- [x] Migration: `sso_assertions` for replay protection; store methods on both engines
- [x] `core`: SAML settings on `SSOProvider` and its inputs, validation, ACS and metadata paths, `SAMLMetadata`
- [x] `internal/sso`: SAML service provider on gosaml2, IdP metadata parsing, SP metadata, a fake IdP in `ssotest`

## Service

- [x] Accept kind `saml`; read IdP metadata by URL or XML on create and edit
- [x] `BeginSSOLogin` builds an AuthnRequest for a SAML provider; `CompleteSSOLogin` takes a SAML response
- [x] Assertion checks, replay protection, IdP-initiated sign-in behind a setting, then `signInIdentity`
- [x] `SAMLMetadata`

## Surfaces

- [x] Registry entry, HTTP route, client, `just openapi`
- [x] `tix tenant sso add/edit` SAML flags, `tix tenant sso metadata`
- [x] Web: ACS and metadata routes, SAML settings on the Single sign-on screen
- [x] i18n: bg, de, fr

## Tests

- [x] Signed by a generated IdP key end to end; negative tests for a bad signature, wrong audience, expired,
      replayed, unsigned and wrapped responses

## Docs

- [ ] docs/sso.md with Entra ID, Okta, Keycloak, Authentik and Google Workspace; README, ROADMAP, skill,
      docs/commands.md
