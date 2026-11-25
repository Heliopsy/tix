# Design

## Where SAML meets the existing sign-in

SAML is a third protocol behind the same two service methods and the same identity. Nothing after the protocol
changes: `signInIdentity` receives an `sso.Identity` and applies the domain allowlist, linking, just-in-time
creation, role mapping, the global admin refusal, the second factor and the session exactly as it does for OIDC
and GitHub.

```text
browser                       tix                                   IdP
  | GET /login/sso/KEY          |                                     |
  |---------------------------->| BeginSSOLogin                       |
  |   303 IdP?SAMLRequest=..&RelayState=STATE, cookie tix_sso_flow    |
  |------------------------------------------------------------------>|
  |                         (person signs in at the IdP)              |
  |   auto-submitted form: POST /login/sso/saml/KEY/acs               |
  |<------------------------------------------------------------------|
  |---------------------------->| CompleteSSOLogin{Flow: cookie,      |
  |                             |   State: RelayState, SAMLResponse}  |
  |                             | verify, consume assertion id,       |
  |                             | signInIdentity                      |
  |   303 next, session cookie  |                                     |
```

`BeginSSOLogin` and `CompleteSSOLogin` branch on the provider's kind rather than gaining SAML twins: the
inputs grow a `saml_response` (and, for IdP-initiated sign-in only, the provider key and the address the
response was posted to), and the begin answer says which kind it started. The `Adapter` interface stays the
OIDC and GitHub redirect-and-code shape; SAML has its own type in `internal/sso`, because an AuthnRequest and a
posted assertion do not fit `AuthURL` and `Exchange` without pretending.

## The flow state and RelayState

The flow is the existing sealed `auth.SSOFlow`: the state is the RelayState, the nonce is the AuthnRequest's
ID, and the redirect URI is the assertion consumer service (ACS) address. It travels in the same HttpOnly
`tix_sso_flow` cookie as an OIDC flow, scoped to `/login/sso`.

The RelayState carries only the 43 character state, not the sealed flow. SAML bindings say RelayState should
not exceed 80 bytes and ADFS and some Shibboleth set-ups enforce it; a sealed flow is several hundred. The
cookie is needed anyway: it is what binds the posted response to the browser that started the sign-in, so a
response captured from one person cannot be replayed into another browser, and so a login CSRF (posting the
attacker's own assertion into a victim's browser) finds no flow.

The ACS is a cross-site POST, which a `SameSite=Lax` cookie does not survive. For a SAML provider the flow
cookie is set `SameSite=None; Secure` when the request came over TLS. Over plain HTTP it stays Lax, which
works only when the IdP is on the same site (a Keycloak on localhost beside tix); docs/sso.md says so. SAML in
production needs HTTPS regardless, because the IdP posts the assertion to that address.

The ACS route is exempt from the CSRF token check, as it must be: the IdP's form cannot carry tix's token. Its
protection is the flow cookie matched to the RelayState, the signed InResponseTo matched to the flow, and the
assertion id consumed once.

## Library: gosaml2

`github.com/russellhaering/gosaml2` with `goxmldsig` and `etree`, over `crewjam/saml`.

- gosaml2 takes its clock per service provider (`Clock *dsig.Clock`), so verification runs on the service's
  `clock.Clock` and `clock.FakeClock` drives every time check in tests. crewjam/saml reads time from package
  variables (`saml.TimeNow`, `saml.Clock`), which a server serving several tenants, or parallel tests, would
  have to mutate.
- It is the smaller dependency: the protocol and XML signatures only. crewjam/saml's main package is usable on
  its own, but brings a session middleware, JWT cookies and an IdP implementation tix has no use for.
- After a signature verifies it continues on the element goxmldsig returns, the canonicalised, verified copy,
  never on the document it was handed. An assertion beside a signed one is refused, as is an unsigned assertion
  in an unsigned response. That is the defence against XML signature wrapping, and the tests below attack it.
- It bounds decompressed size and XML token count, and `mattermost/xml-roundtrip-validator` refuses documents
  whose meaning changes on a round trip through encoding/xml.
- v0.12.0 and goxmldsig v1.6.1 were released in August 2026 and are maintained; govulncheck reports no
  vulnerability reachable from tix with them (see the PR).

gosaml2 leaves parts of the checking to the caller, and tix does them:

- `ValidateEncodedResponse` checks the signature, issuer, status, destination, the bearer subject confirmation,
  its recipient and its NotOnOrAfter. The assertion's `Conditions` (NotBefore, NotOnOrAfter, audience) are
  only reported as warnings by a separate call, so tix checks them itself and refuses rather than warns. An
  assertion with no audience restriction is refused.
- It does not check InResponseTo. tix compares the subject confirmation's InResponseTo (inside the signed
  assertion) with the AuthnRequest id in the flow, and the response's own InResponseTo when present. An
  IdP-initiated response, which answers no request, must carry none.
- It accepts several assertions and reads the first. tix refuses anything but exactly one.

## Clock skew

NotBefore is allowed up to two minutes in the future, for an IdP whose clock runs ahead. Expiry
(Conditions/NotOnOrAfter and the subject confirmation's) is not stretched: IdPs already give assertions a few
minutes.

## Replay protection

```text
sso_assertions
  id            TEXT PK
  tenant_id     TEXT -> tenants
  provider_id   TEXT -> sso_providers  ON DELETE CASCADE
  assertion_id  TEXT
  expires_at    TEXT  the later of the assertion's two NotOnOrAfter
  UNIQUE (tenant_id, provider_id, assertion_id)
```

A verified assertion's id is inserted before `signInIdentity` runs; a duplicate is a conflict and the sign-in
is refused as a replay. Rows past `expires_at` are deleted on the same insert, so the table holds at most the
assertions of the last few minutes. It is a table rather than memory so that a response replayed against
another replica is refused too, as the sealed flow already lets any replica finish a sign-in. The insert is a
bookkeeping write in its own transaction with no audit entry; the sign-in it guards is audited as `sso.login` or
`sso.denied`.

## Settings

Kind `saml` keeps its protocol settings in `sso_providers.config`, as the design of `sso-oidc` intended, so no
column changes:

| field | meaning |
| --- | --- |
| `idp_metadata_url` | where the IdP's metadata was read; saving the provider with it re-reads it |
| `idp_entity_id` | the IdP's entity id, required as the response and assertion issuer |
| `idp_sso_url` | the IdP's HTTP-Redirect SingleSignOnService |
| `idp_certificates` | PEM signing certificates; several during a rollover |
| `sp_entity_id` | override for tix's entity id, by default the metadata address |
| `name_id_format` | the NameID format to ask for; empty lets the IdP choose |
| `allow_idp_initiated` | accept a response that answers no request; off by default |

Metadata given as XML is parsed and not kept. When metadata and an explicit `idp_*` value are both given, the
explicit value wins, so a certificate can be pinned while the URL fills the rest. The certificates are public
and are returned in listings. No client id or secret applies to SAML, and the secret columns stay empty.

The attribute names reuse `email_claim`, `name_claim`, `username_claim` and `groups_claim`. Empty means a list
of the names common IdPs send: `email`, `mail`, the Entra and ADFS `.../claims/emailaddress`, and the
`urn:oid:0.9.2342.19200300.100.1.3` LDAP OID, and the same kind of list for the others. An attribute matches on
its Name or its FriendlyName.

A SAML IdP has no email_verified. The email it asserts is treated as verified: the IdP is the tenant's own
directory, added by the tenant's admin, who could equally configure an OIDC provider claiming
`email_verified`. `allowed_domains` remains the guard against a directory asserting addresses it does not own.

The NameID is the subject. A transient NameID is refused, because it changes at every sign-in and would link
nobody; docs/sso.md asks for persistent or email address.

`trust_mfa` on a SAML provider needs `mfa_values`, compared with the AuthnContextClassRef and with Entra's
`authnmethodsreferences` attribute. There is no default value that means a second factor across IdPs.

## Addresses

The ACS is `/login/sso/saml/KEY/acs` and the metadata `/login/sso/saml/KEY/metadata`, per provider, so the
provider is known from the address before anything in the response is trusted, and an IdP-initiated response
needs no lookup by issuer. Both are built on the request's origin (the public URL when configured), as the OIDC
redirect URL is. The default SP entity id is the metadata address, which is the convention most IdPs expect
and is resolvable.

The metadata is public, as SAML metadata is: Entra, Okta and Keycloak import it by URL. It names the entity id
and the ACS and asks for signed assertions. It carries no key, since tix does not sign requests.

## IdP-initiated sign-in

Off by default. An unsolicited response has no flow, so nothing binds it to a browser and nothing proves this
browser asked for it; enabling it accepts login CSRF in exchange for a tile in the IdP's app portal. When on,
the response must carry no InResponseTo, the replay table still applies, and the session returns to the start
screen.

## What is left out

- Signed AuthnRequests and encrypted assertions both need an SP key pair. The provider's sealed secret slot
  could hold one later. Keycloak signs nothing by default for a client that does not require it, and the docs
  say to switch off "Client signature required".
- Single logout.
- `tix login --sso` through a SAML provider: the IdP posts to a browser, so the command line would need a
  loopback ACS registered at the IdP. It is refused with a pointer to the browser.
- Scheduled metadata refresh; certificate rollover is a re-save, or a second certificate added ahead of it.
