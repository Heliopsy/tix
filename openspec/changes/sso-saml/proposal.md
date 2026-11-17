# Single sign-on with SAML 2.0

## Why

OIDC and GitHub sign-in shipped with `saml` reserved as a provider kind and refused. Plenty of directories a team
already runs (Entra ID, Okta, Google Workspace, ADFS, Keycloak and Authentik realms set up for other SAML apps)
are configured as SAML first, and some organisations allow nothing else. The identity model was built so SAML
would add a protocol and not a second account model.

## What Changes

- Kind `saml` is accepted. A SAML provider holds the IdP's entity id, its HTTP-Redirect sign-in URL and one or
  more signing certificates, read from the IdP's metadata by URL or as XML, or given one by one. Optional: an SP
  entity id override, a NameID format to request, and IdP-initiated sign-in, off by default.
- Service provider initiated sign-in: an AuthnRequest over HTTP-Redirect, the response posted to an assertion
  consumer service per provider at `/login/sso/saml/KEY/acs`, and SP metadata per provider at
  `/login/sso/saml/KEY/metadata` and `/api/v1/auth/sso/providers/ID/saml-metadata`.
- The response is refused unless the assertion is signed by a configured certificate (or sits inside a signed
  response), names the provider's IdP as issuer, is addressed to this SP (audience, recipient, destination),
  is inside its validity window, answers the request this browser started (InResponseTo), and has not been
  used before. One assertion per response; a transient NameID is refused.
- The assertion becomes the same `Identity` the OIDC adapter produces: the NameID is the subject, and the
  email, name, username and groups attributes are named per provider (the claim settings), with defaults that
  cover the common IdPs. `trust_mfa` reads the AuthnContextClassRef against the provider's `mfa_values`.
- Everything after the protocol is the existing sign-in: domain allowlist, linking, just-in-time creation,
  role mapping, the global admin refusal, the second factor, the session and the password policy.
- Used assertion ids are kept in `sso_assertions` until they expire, so a captured response cannot be posted
  twice, on any replica.
- `tix tenant sso add --kind saml` with `--idp-metadata-url`, `--idp-metadata-file`, `--idp-entity-id`,
  `--idp-sso-url`, `--idp-cert`, `--sp-entity-id`, `--name-id-format`, `--allow-idp-initiated`, the same on
  `edit`, and `tix tenant sso metadata KEY`. The Single sign-on screen gains the SAML settings and shows the ACS
  and metadata addresses.
- Docs: docs/sso.md set-up for Entra ID, Okta, Keycloak, Authentik and Google Workspace; README, ROADMAP, skill.

Not in this change: signed AuthnRequests, encrypted assertions, single logout, SAML sign-in from `tix login
--sso` (SAML is a browser protocol), and periodic metadata refresh (saving the provider again re-reads it).
