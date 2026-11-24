# Single sign-on

A tenant can let its people sign in with an account they already have: an OpenID Connect (OIDC) identity
provider such as Keycloak, Authentik, Microsoft Entra ID, Google or Okta, GitHub, or a SAML 2.0 identity
provider such as Entra ID, Okta, Keycloak, Authentik or Google Workspace. Providers are configured per tenant,
by the tenant's own administrators. Each enabled provider is a **Sign in with NAME** button on the web sign-in
screen. OIDC and GitHub providers are also a choice for `tix login --sso`; SAML is a browser protocol and signs
in on the web only. Every kind, SAML included, also signs in a native app such as tix mobile, through the system
browser: see [Signing in from the app](#signing-in-from-the-app). SAML has its own set-up, in
[SAML providers](#saml-providers) below; everything from [Linking rules](#linking-rules) on applies to every kind.

## Before you start

- **The server needs its sealing key.** A provider's client secret is sealed with `auth.totp_key`
  (`TIX_AUTH_TOTP_KEY`), the same key that seals second factors, and the sign-in itself is carried between its
  two steps sealed with a key derived from it. Without it a provider cannot be added. See
  [second-factor.md](second-factor.md).
- **The server needs to know where it is.** The address registered with the provider is
  `PUBLIC_URL/login/sso/callback`, for example `https://tix.example.com/login/sso/callback`. Set
  `server.public_url` so `tix tenant sso ls` and the Single sign-on screen print it. A tenant served on its own
  domain registers that domain instead: `https://acme.example.com/login/sso/callback`.
- **The server has to reach the provider.** Discovery, the key set, the token endpoint and the GitHub API are
  requested under the same outbound policy as webhooks. A provider on a private address, a Keycloak inside the
  company network for instance, needs that host in `server.egress_allow`. See
  [deployment.md](deployment.md#outbound-requests-to-tenant-urls).
- **Managing providers needs `sso:admin`.** The admin role holds it; a token needs it named.
- **SAML needs HTTPS.** The IdP posts the assertion to tix from its own site, and the cookie that ties the
  response to the browser that asked for it survives that cross-site POST only when it is `Secure`. Over plain
  HTTP a SAML sign-in works only with an IdP on the same site, such as a Keycloak on `localhost` beside tix.

## Adding a provider

```sh
printf %s "$CLIENT_SECRET" | tix tenant sso add kc --kind oidc --name Keycloak \
  --issuer https://id.example.com/realms/acme --client-id tix --client-secret -
tix tenant sso ls
tix tenant sso show kc
tix tenant sso edit kc --role-map tix-admins=admin --auto-create
tix tenant sso rm kc
```

The first argument is the provider's key: lowercase letters, digits and dashes. It appears in
`/login/sso/KEY`, in `tix login --sso=KEY` and in a session's sign-in method, `sso:KEY`.

The client secret is read from standard input with `--client-secret -`, sealed, and never shown again: every
listing says only whether one is set and, for a secret of 16 characters or more, its last four. To replace it,
`tix tenant sso edit KEY --client-secret -`. An OIDC provider registered as a public client may have no secret;
PKCE is used either way. A GitHub provider always needs one. A SAML provider has no client id or secret.

| Setting | Flag | Default |
| --- | --- | --- |
| Button name | `--name` | the key |
| Issuer (OIDC) | `--issuer` | required; its `/.well-known/openid-configuration` is read |
| Client id | `--client-id` | required |
| Scopes (OIDC) | `--scope`, repeatable | `openid`, `profile`, `email` |
| Claims (OIDC) | `--email-claim`, `--name-claim`, `--username-claim`, `--groups-claim` | `email`, `name`, `preferred_username`, `groups` |
| Allowed email domains | `--allowed-domain`, repeatable | any |
| Groups to roles | `--role-map GROUP=ROLE`, repeatable | none |
| Create unknown people | `--auto-create` | off |
| Role of a created account | `--default-role` | `member` |
| Trust the provider's second factor (OIDC) | `--trust-mfa`, with `--mfa-value` | off; `mfa` |
| Allowed GitHub organisations | `--allowed-org`, repeatable | any |
| GitHub Enterprise Server | `--web-url` and `--api-url` | github.com |
| IdP metadata (SAML) | `--idp-metadata-url` or `--idp-metadata-file` | none; fills the three below |
| IdP entity id, sign-in URL, certificates (SAML) | `--idp-entity-id`, `--idp-sso-url`, `--idp-cert FILE` | from the metadata |
| SP entity id (SAML) | `--sp-entity-id` | `PUBLIC_URL/login/sso/saml/KEY/metadata` |
| NameID format to request (SAML) | `--name-id-format` | the IdP's choice |
| IdP-initiated sign-in (SAML) | `--allow-idp-initiated` | off |
| Switched on | `--disabled` on add, `--enabled=false` on edit | on |

The same operations are `/api/v1/sso/providers` in the [HTTP API](api.md) and the Single sign-on screen in the
browser.

### The single sign-on screen

Under **Manage**, **Single sign-on** lists the tenant's providers with what each does, names the redirect URL to
register (for a SAML provider, its ACS URL, SP entity id and metadata address), and holds the password sign-in
setting. **Add a provider** opens the form; each provider has an
**Edit** dialog, where a blank secret keeps the stored one, and a red **Delete** that asks first.

## Setting up each provider

Every OIDC provider needs the same three things: a confidential client (or a public one with PKCE) whose
redirect URI is the callback above, the issuer URL, and the client id and secret. Copy the issuer exactly, with
or without its trailing slash as the provider's discovery document states it, because the ID token's `iss` has
to match it character for character.

### Keycloak

1. In the realm, **Clients**, **Create client**: type OpenID Connect, a client id such as `tix`.
2. Turn **Client authentication** on and keep **Standard flow**. Valid redirect URIs:
   `https://tix.example.com/login/sso/callback`, and `http://127.0.0.1/*` for `tix login --sso`.
3. Copy the secret from **Credentials**.
4. For groups, add a **Group Membership** mapper to the client's dedicated scope with token claim name `groups`
   and **Full group path** off.

```sh
printf %s "$SECRET" | tix tenant sso add kc --kind oidc --name Keycloak \
  --issuer https://id.example.com/realms/acme --client-id tix --client-secret - \
  --role-map tix-admins=admin
```

Keycloak sets `email_verified` from the user's profile, so an address an administrator has not marked verified
is not used for linking or creation.

### Authentik

1. **Applications**, **Providers**, **Create**, OAuth2/OpenID Provider, client type confidential, redirect URIs
   the callback and `http://127.0.0.1:.*/callback` as a regular expression for the command line.
2. Create an application using it. The issuer is `https://auth.example.com/application/o/SLUG/`, with the
   trailing slash.

```sh
printf %s "$SECRET" | tix tenant sso add authentik --kind oidc --name Authentik \
  --issuer https://auth.example.com/application/o/tix/ --client-id "$CLIENT_ID" --client-secret -
```

Authentik puts groups in `groups` with its default scope mappings.

### Microsoft Entra ID

1. **App registrations**, **New registration**, platform Web, redirect URI the callback. Add
   `http://localhost` under **Mobile and desktop applications** for the command line.
2. **Certificates & secrets**, a new client secret.
3. The issuer is `https://login.microsoftonline.com/TENANT_ID/v2.0`.
4. Entra ID does not send `email_verified`, so nobody is linked or created by email. Add the optional claim
   `xms_edov` (email domain owner verified) under **Token configuration** and map it with
   `--email-claim email`, or invite people and keep `--allowed-domain` to your own domains. Groups arrive as
   object ids in `groups` once **Token configuration** adds the groups claim; map those ids in `--role-map`.

```sh
printf %s "$SECRET" | tix tenant sso add entra --kind oidc --name Microsoft \
  --issuer https://login.microsoftonline.com/TENANT_ID/v2.0 --client-id "$CLIENT_ID" --client-secret - \
  --allowed-domain example.com
```

### Google

1. Google Cloud console, **APIs & Services**, **Credentials**, OAuth client id of type Web application, with the
   callback as an authorised redirect URI. For the command line, a second client of type Desktop app accepts
   any loopback port.
2. The issuer is `https://accounts.google.com`.

```sh
printf %s "$SECRET" | tix tenant sso add google --kind oidc --name Google \
  --issuer https://accounts.google.com --client-id "$CLIENT_ID" --client-secret - \
  --allowed-domain example.com --auto-create
```

Google sends `email_verified`. Restrict it with `--allowed-domain` to your Workspace domain, or anyone with a
Google account can be created when `--auto-create` is on.

### Okta

1. **Applications**, **Create App Integration**, OIDC, Web Application, with the callback as sign-in redirect
   URI and `http://127.0.0.1/callback` as well for the command line.
2. The issuer is your authorisation server, `https://ACME.okta.com/oauth2/default`, or the org server
   `https://ACME.okta.com`.
3. For groups, add a groups claim to the authorisation server and request its scope:
   `--scope openid --scope profile --scope email --scope groups`.

### GitHub

1. **Settings**, **Developer settings**, **OAuth Apps**, **New OAuth App**. Authorization callback URL: the
   callback. GitHub also accepts a loopback address on any port for the same app, which `tix login --sso` uses.
2. Generate a client secret.

```sh
printf %s "$SECRET" | tix tenant sso add github --kind oauth_github --name GitHub \
  --client-id "$CLIENT_ID" --client-secret - --allowed-org acme --role-map acme-admins=admin
```

GitHub is OAuth, not OIDC: there is no ID token. tix exchanges the code with PKCE and reads the account
(`/user`), its addresses (`/user/emails`) and, when organisations are named, the account's membership of each
(`/user/memberships/orgs/ORG`, which asks for `read:org`). The subject is the account's numeric id, which does
not change when the login is renamed. Only the primary address counts, and only when GitHub reports it
verified. An organisation that restricts OAuth app access has to approve the app, or its members read as
outside it. For GitHub Enterprise Server set `--web-url https://github.example.com --api-url
https://github.example.com/api/v3`.

`--trust-mfa` is refused on GitHub: it does not tell an app whether the person used a second factor.

## SAML providers

A SAML provider needs three things from the IdP, which its metadata carries: its entity id, its sign-in URL for
the HTTP-Redirect binding, and the certificate it signs with. The IdP needs two things from tix:

| tix | The IdP calls it |
| --- | --- |
| `PUBLIC_URL/login/sso/saml/KEY/acs` | ACS URL, Reply URL, Single sign-on URL, Assertion Consumer Service |
| `PUBLIC_URL/login/sso/saml/KEY/metadata` | Entity ID, Identifier, Audience URI, SP entity id |

KEY is the provider's key, so pick it before you configure the IdP. The metadata address serves tix's SP
metadata without a credential, so an IdP that imports metadata by URL can read both from it; `tix tenant sso
metadata KEY` prints the same document for one that wants a file. Use `--sp-entity-id` only when the IdP needs
another entity id, such as one already registered for an older SP.

```sh
tix tenant sso add okta --kind saml --name Okta \
  --idp-metadata-url https://acme.okta.com/app/exk1abc/sso/saml/metadata --auto-create
tix tenant sso add google --kind saml --name Google --idp-metadata-file GoogleIDPMetadata.xml
tix tenant sso add adfs --kind saml --name ADFS --idp-entity-id http://adfs.example.com/adfs/services/trust \
  --idp-sso-url https://adfs.example.com/adfs/ls/ --idp-cert adfs-signing.pem
tix tenant sso metadata okta > tix-sp.xml
```

The metadata is read when the provider is saved, and again on a schedule while the provider is enabled and
has a metadata URL (see [Certificate rollover](#certificate-rollover)). A provider set up from a file or by hand
is never reread. A flag given beside the metadata wins over it.

**Which attributes are read.** The NameID is the subject. Ask the IdP for a persistent NameID, or an email
address that does not change; a transient one is refused, because it differs at every sign-in. The email, name,
username and groups are read from attributes, matched on their Name or FriendlyName. Name one with
`--email-claim`, `--name-claim`, `--username-claim` or `--groups-claim`, or leave them empty and tix takes the
first of the names IdPs commonly send:

| Field | Names tried |
| --- | --- |
| email | `email`, `mail`, `emailaddress`, `Email`, `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress`, `urn:oid:0.9.2342.19200300.100.1.3`; then an emailAddress NameID |
| name | `name`, `displayName`, `displayname`, `cn`, the Entra and ADFS display name and name claims, `urn:oid:2.16.840.1.113730.3.1.241`, `urn:oid:2.5.4.3`; then given name and surname |
| username | `username`, `preferred_username`, `uid`, `login`, Authentik's `.../saml/username`, `urn:oid:0.9.2342.19200300.100.1.1` |
| groups | `groups`, `memberOf`, `Role`, `role`, the Entra groups claim, `http://schemas.xmlsoap.org/claims/Group`, `urn:oid:1.3.6.1.4.1.5923.1.5.1.1` |

**The second factor.** `--trust-mfa` on a SAML provider needs `--mfa-value`: the AuthnContextClassRef values,
or Entra ID's `authnmethodsreferences` values, that mean a second factor was used. There is no default, because
IdPs do not agree on one. For Entra ID that is `http://schemas.microsoft.com/claims/multipleauthn`.

**Signed AuthnRequests are not sent.** tix has no SP key, so an IdP that insists on signed requests (Keycloak's
**Client signature required**, ADFS with request signing on) has to have that turned off. Assertions must be
signed, and encrypted assertions are not supported.

**IdP-initiated sign-in** (a tile in the IdP's app portal) is off unless `--allow-idp-initiated` is set. A
response nobody asked for is not tied to any browser's sign-in, so anyone who obtains one for their own account
can sign a victim's browser into it. Leave it off unless people need the tile; the button on the sign-in screen
does the same job.

### Certificate rollover

Entra ID, Okta and Google Workspace rotate the certificate they sign with. They publish the next one in their
metadata first, switch to it, and later stop publishing the old one. `tix serve` reads the metadata of every
enabled SAML provider with a metadata URL every `auth.saml_metadata_refresh` (12 hours by default, `0` for
never) from the URL stored on the provider, and merges what it finds:

- A signing certificate the metadata names that the provider lacks is added, so a response signed with it is
  accepted as soon as the IdP switches.
- A certificate the metadata stopped naming keeps verifying for seven days, so a response signed just before
  the switch, or a slow IdP, still signs in. After that it is dropped.
- The sign-in URL follows the metadata too. The entity id picks the IdP out of the metadata, so changing it is
  an edit.
- A value set with `--idp-entity-id`, `--idp-sso-url` or `--idp-cert`, or typed into the edit dialog, is
  pinned and a refresh leaves it alone. Give the flag as `""`, or clear the box, to hand it back to the
  metadata.
- A read that fails, or metadata that is not valid or names no signing certificate, changes nothing: sign-in
  carries on with the certificates the provider has, the failure is logged and recorded on the provider, and
  the read is tried again after an hour.

A refresh that changes the certificates or the sign-in URL writes an `sso.provider.refresh` audit entry and an
`sso.provider.updated` event. Every read records its time, outcome and any error on the provider, which `tix
tenant sso show KEY`, `GET /api/v1/sso/providers/KEY` (`metadata_refresh`, `idp_certificates_retiring`,
`idp_pinned`) and the Single sign-on screen show. To read it now, run `tix tenant sso refresh KEY`, press
**Refresh metadata** on the provider's row, or `POST /api/v1/sso/providers/KEY/refresh`; each needs `sso:admin`.

```sh
tix tenant sso refresh okta
tix tenant sso show okta -o json | jq .metadata_refresh
```

With several replicas, the time of the last read is kept on the provider, so a provider one process read is not
due on another and no lock is taken. Setting `auth.saml_metadata_refresh: 0` on all but the process that runs the
other background workers keeps it to one reader.

### Entra ID (SAML)

1. **Enterprise applications**, **New application**, **Create your own application**, the non-gallery option.
2. **Single sign-on**, **SAML**. Under **Basic SAML Configuration** set **Identifier (Entity ID)** to the
   metadata address and **Reply URL** to the ACS URL.
3. Under **Attributes & Claims**, the defaults send `emailaddress`, `name`, `givenname` and `surname`, which tix
   reads. Set **Unique User Identifier** to `user.objectid` with format persistent, so the subject survives a
   change of UPN. For roles, **Add a group claim**; groups arrive as object ids, which `--role-map` maps.
4. Assign users or groups under **Users and groups**.
5. Copy **App Federation Metadata Url** from **SAML Certificates**.

```sh
tix tenant sso add entra --kind saml --name Microsoft \
  --idp-metadata-url "https://login.microsoftonline.com/TENANT_ID/federationmetadata/2007-06/federationmetadata.xml?appid=APP_ID" \
  --allowed-domain example.com --role-map 1b2c3d4e-0000-0000-0000-000000000000=admin
```

### Okta (SAML)

1. **Applications**, **Create App Integration**, **SAML 2.0**.
2. **Single sign-on URL**: the ACS URL, with **Use this for Recipient URL and Destination URL** ticked.
   **Audience URI (SP Entity ID)**: the metadata address. **Name ID format**: Persistent or EmailAddress.
3. **Attribute Statements**: `email` from `user.email`, `name` from `user.displayName`. **Group Attribute
   Statements**: `groups`, with a filter such as starts with `tix-`.
4. On the **Sign On** tab, copy the **Metadata URL**.

```sh
tix tenant sso add okta --kind saml --name Okta \
  --idp-metadata-url https://acme.okta.com/app/exk1abc/sso/saml/metadata --role-map tix-admins=admin
```

### Keycloak (SAML)

1. In the realm, **Clients**, **Import client**, and give it the metadata address's document
   (`tix tenant sso metadata KEY`), or **Create client** of type SAML with the metadata address as **Client ID**.
2. Under **Settings**, **Name ID format** persistent or email, with **Force name ID format** on. Under
   **Signature and Encryption**, keep **Sign documents** or **Sign assertions** on. Under **Keys**, turn
   **Client signature required** off, and leave **Encrypt assertions** off.
3. If the client was created by hand, set **Assertion Consumer Service POST Binding URL** under **Advanced** to
   the ACS URL.
4. Under **Client scopes**, the dedicated scope, add mappers: **User Property** `email` as SAML attribute
   `email`, and **Group list** named `groups` with **Full group path** off.
5. The IdP metadata is `https://id.example.com/realms/acme/protocol/saml/descriptor`.

```sh
tix tenant sso add kc-saml --kind saml --name Keycloak \
  --idp-metadata-url https://id.example.com/realms/acme/protocol/saml/descriptor --role-map tix-admins=admin
```

### Authentik (SAML)

1. **Applications**, **Providers**, **Create**, **SAML Provider**. **ACS URL**: the ACS URL. **Audience**: the
   metadata address. **Service Provider Binding**: Post.
2. Choose a **Signing Certificate**; without one Authentik sends unsigned assertions, which tix refuses.
3. The default property mappings send email, name, username and groups under the names tix reads. Pick a
   **NameID Property Mapping** that does not change for a person, such as the email or the user's UUID.
4. Create an application using the provider, then copy the metadata URL from the provider's page (its
   **Download** button links it), or download the file.

```sh
tix tenant sso add authentik-saml --kind saml --name Authentik \
  --idp-metadata-url "https://auth.example.com/api/v3/providers/saml/3/metadata/?download"
```

### Google Workspace (SAML)

1. Admin console, **Apps**, **Web and mobile apps**, **Add app**, **Add custom SAML app**.
2. On the IdP details step, **Download Metadata**. Google publishes no metadata URL, so tix reads the file.
3. **Service provider details**: **ACS URL** the ACS URL, **Entity ID** the metadata address, **Name ID format**
   EMAIL with **Primary email**.
4. **Attribute mapping**: **Primary email** to `email`, and under **Group membership** the groups to send as
   `groups`.
5. Turn the app on for the organisational units that should sign in.

```sh
tix tenant sso add google --kind saml --name Google --idp-metadata-file GoogleIDPMetadata.xml \
  --allowed-domain example.com --auto-create
```

## How a sign-in works

The browser opens `/login/sso/KEY`. tix seals the sign-in's state, nonce, PKCE verifier, redirect URI,
Keep me signed in and the screen to return to into an HttpOnly cookie scoped to `/login/sso`, and sends the
browser to the provider. The provider sends it back to `/login/sso/callback` with a code. tix checks the state
against the cookie, exchanges the code with the verifier, and for OIDC verifies the ID token: the signature
against the issuer's published keys (cached and refreshed when a token names a key it has not seen), the
issuer, the audience (the client id), the expiry and the nonce. A sign-in has ten minutes to come back.

Nothing is stored until the sign-in completes, so an abandoned one leaves nothing behind, and any replica with
the same `auth.totp_key` completes a sign-in another began.

**SAML** starts at the same address. tix sends the browser to the IdP's sign-in URL with an AuthnRequest and a
RelayState that is the flow's state, and keeps the request's ID in the sealed cookie. The IdP posts the
response to the ACS. tix checks the RelayState against the cookie, then the response: the signature against
the provider's certificates (on the assertion, or on the response around it), the issuer, the destination and
recipient, the audience (the SP entity id), NotBefore (two minutes of clock skew allowed) and NotOnOrAfter,
that the signed InResponseTo is the request in the cookie, and that there is exactly one assertion. The
assertion's ID is recorded until it expires and a second post of it is refused, on every replica. The ACS takes
the IdP's form without tix's CSRF token, since another site cannot carry it; the cookie, the RelayState and the
signed InResponseTo are what tie the post to the browser.

## Linking rules

In order:

1. **Subject.** An identity already linked to this provider's subject signs in its account.
2. **Verified email.** Otherwise, an address the provider vouches for (OIDC `email_verified`, a GitHub
   primary and verified address, or the email a SAML IdP asserts) links to the account with that address, if
   it is a member of this tenant.
   The link is recorded in `user_identities`, so the next sign-in matches on the subject even if the address
   changes.
3. **Just-in-time creation.** Otherwise, with `--auto-create`, a new account is created for an address no
   account has anywhere, with the role the groups map to or `--default-role`. It has no password.
4. **Refusal.** Otherwise the sign-in is refused, saying why: no email, an unverified email, no account here
   (ask for an invitation), or an account that belongs only to other tenants.

People a provider created over SCIM ([scim.md](scim.md)) are members with a verified address, so their first
sign-in links them by rule 2, and a person SCIM deactivated is refused here as disabled.

An account that exists only in other tenants is never adopted. A tenant reaches an existing account only by
invitation; once the person accepts it, signing in through the provider links them by rule 2.

Allowed email domains apply to every sign-in, linked identities included, so moving someone's address off the
domain at the provider stops them signing in here.

**Groups to roles.** When the person's groups (the groups claim, or for GitHub the named organisations they
belong to) match the mapping, the highest mapped role is applied at every sign-in, so removing someone from
a group at the provider demotes them at their next sign-in. When nothing matches, an existing member keeps
their role.

Deleting a provider deletes its links and keeps the accounts.

## Sessions and the second factor

A provider sign-in ends in the same session a password sign-in does: the tenant's lifetime, idle timeout and
Keep me signed in all apply. The session's sign-in method is `sso:KEY`, which the sessions list and the
`user.login` audit entry show.

The tenant's second factor requirement still applies after a provider sign-in, as does a person's own
enrolment: the code is asked for before the session exists. `--trust-mfa` skips it when the ID token says the
person passed a second factor at the provider: its `amr` contains, or its `acr` equals, one of `--mfa-value`
(default `mfa`, RFC 8176). Set the values your provider actually sends, for example an Entra ID `acr` or a
Keycloak level of assurance, and check them in the provider's token preview first.

## Command line sign-in

This is the terminal's flow, with a loopback redirect on the same machine. A phone cannot listen on a loopback
address, and SAML cannot use one at all, so an app signs in as described in
[Signing in from the app](#signing-in-from-the-app) instead.

SAML providers are not offered here: the IdP posts its answer to a browser. Sign in on the web, then create a
token for the machine on the **Connect** screen. `--sso` alone skips SAML providers when it picks one.

```sh
tix --server https://tix.example.com login --sso
tix --server https://tix.example.com login --sso=github --no-browser
```

`--sso` alone uses the tenant's only provider and refuses, naming them, when there are several. tix listens on
`127.0.0.1` on a free port, opens the provider in the browser and prints the address too; on a machine with no
browser, open it elsewhere on the same machine, since the provider sends the browser back to the loopback
address. The result is the same session `tix login` prints, and `--tenant` and `--context` work as for a
password. Register the loopback redirect with the provider as described above.

## Signing in from the app

A native app, such as tix mobile, signs in through any enabled provider, OIDC, GitHub or SAML, without a change
at the identity provider. The app opens the ordinary web sign-in in the system browser, the browser runs the
web flow to its end, and the server hands the result to the app instead of setting a browser session. The
redirect URI the provider knows stays `PUBLIC_URL/login/sso/callback`, or the SAML ACS, so nothing new is
registered there.

1. The app reads `GET /api/v1/auth/sso/options`. It offers provider buttons only when `app_sign_in` is true,
   which it is while `auth.sso_app_redirects` lists at least one address
   ([configuration.md](configuration.md#signing-in-from-an-app)).
2. It makes a PKCE verifier and opens
   `/login/sso/KEY?app_redirect=URI&code_challenge=CHALLENGE&code_challenge_method=S256&app_state=STATE` in the
   system browser. `app_redirect` has to be exactly one of the listed addresses, `code_challenge` is required
   and only `S256` is taken, and `app_state`, which the app gets back unchanged, is at most 256 characters.
   Anything else is refused on the ordinary sign-in error screen. The app's values are sealed into the flow
   cookie with the rest of the sign-in.
3. The browser signs in as on the web: the provider, the [linking rules](#linking-rules) and the tenant's
   second factor, asked in the browser. An app sign-in always counts as Keep me signed in. The second factor is
   asked when the tenant requires it on the web or on the command line, since the app's session is the one its
   password sign-in gets, and when the tenant's second factor policy is required. Someone with no factor yet
   enrols right there in the browser, as `tix login` does, rather than getting a session that can only enrol.
   An enrolment that completes here shows its recovery codes first, with **Open the app** under them: the
   code is minted when that button is pressed, so taking a while to keep the codes does not spend it. The
   button works for 15 minutes; after that it sends the app `error=expired`, and signing in again just asks
   for a code.
4. Where the web would set its session cookie, the browser goes to `URI?code=CODE&state=STATE` instead: a 302
   after the provider's redirect back, and a small page that navigates on its own and links the address after
   a POST (the SAML ACS, the second factor's form), because a browser may refuse to follow a redirect to an
   app's scheme that no tap started. A refusal goes to `URI?error=REASON&state=STATE`, with one of
   `access_denied` (the provider or the person ended it, or the account may not sign in here), `not_linked`
   (no account matches, see the linking rules), `expired` (the sign-in took too long, was started in another
   browser, or was already used), `provider_error` (the provider's answer did not verify) and `server_error`.
   That covers a start refused after `app_redirect` was checked and a second factor step that expired or
   failed. A wrong or rate limited code stays in the browser, so the person can try again.
5. The app posts the code and its verifier to `POST /api/v1/auth/sso/exchange` within two minutes, and gets
   back exactly what `POST /api/v1/auth/login` answers for a completed sign-in. The session's sign-in method is
   `sso:KEY`, and it is audited as a `user.login` like any other. See [api.md](api.md#signing-in-from-an-app).

The code is 32 random bytes. Only its SHA-256 is stored, in `sso_handoffs`, bound to the PKCE challenge, the
account, the tenant and the provider. The exchange deletes the row, so a code is taken once across every
replica, and a wrong verifier spends it too. Every new code and every exchange also deletes the tenant's
expired codes. An unknown, used or expired code, or a wrong verifier, is `401
invalid credentials`, logged as `sso_code_invalid` ([deployment.md](deployment.md)). An account disabled, or a
provider switched off, between the browser and the exchange is refused the same way.

The browser half only finishes in the browser. An app's flow sent to `POST /api/v1/auth/sso/complete`, or its
second factor ticket sent to `POST /api/v1/auth/login`, is a 400 naming `flow` or `ticket`, and no code is
stored.

## Turning passwords off

```sh
tix tenant edit acme --password-login disabled
tix tenant edit acme --password-login allowed
```

Or **Password sign-in** on the Single sign-on screen. While disabled, the password step of a sign-in is refused
for everybody but a global admin, and the sign-in screen shows only the provider buttons. It is refused while
the tenant has no enabled provider, and disabling or deleting the last enabled provider is refused while
passwords are off. Changing it needs `sso:admin`.

API tokens, SSH key sign-in (`tix login --ssh-key`) and the SSH listener keep working. An invitation is still
accepted with a password, which signs the invited person in once.

### Break-glass

A global admin keeps password sign-in on every tenant, whatever its policy. The sign-in screen of a tenant
without passwords links **Global admin password sign-in** (`/login?password=1`), and `tix login --email ...`
works as before. This is the way back in when a provider is down or misconfigured: sign in as a global admin,
fix the provider or set `--password-login allowed`. Keep at least one global admin with a password and a second
factor, and keep its credentials somewhere you can reach without the identity provider.

A global admin cannot sign in through a provider at all: see below.

## Security notes

- **Whoever administers a provider can sign in as anyone it links.** A tenant admin who controls the identity
  provider can assert any email. So an SSO session is confined to its tenant: it cannot switch to another
  tenant, existing accounts of other tenants are never adopted, and an account that is a global admin is refused
  provider sign-in, because its authority reaches every tenant.
- **Email is only trusted verified.** Linking and creation use an address only when the provider says it is
  verified. A provider that does not send `email_verified` links nobody by email; people can still be invited
  and then linked once the provider vouches for their address, or the provider can be configured to send it.
  SAML has no such flag, so the email a SAML IdP asserts counts as verified: the IdP is the tenant's own
  directory, set up by the tenant's admin. Keep `--allowed-domain` on a SAML provider whose directory holds
  addresses it does not own.
- **SAML responses are taken once.** A captured response cannot be posted again, here or at another replica, and
  IdP-initiated sign-in, which nothing ties to a browser, is off unless a provider turns it on.
- **Use `--allowed-domain` with `--auto-create`** on any provider whose accounts anyone can open, such as Google
  or GitHub, or the whole world can create an account.
- **Secrets stay sealed.** The client secret is sealed with the server key and bound to the provider; it never
  reaches a response, an audit entry or an event. Losing `auth.totp_key` means storing every client secret again.
- **The flow is bound to the browser.** State, nonce and PKCE verifier (for SAML, the request id) live in a
  sealed HttpOnly cookie; a callback from another browser, or with another state, is refused before the code is
  exchanged or the response read.
- **App sign-in hands back a code, never a session.** The address the browser goes to has to match an entry of
  `auth.sso_app_redirects` exactly, with no prefix or pattern matching, so a sign-in cannot be sent to another
  app or site, and an `http` or `https` address is refused unless it is listed in full. The code it carries is
  worthless without the verifier only the app that started the sign-in holds (PKCE S256, compared in constant
  time), expires in two minutes and is taken once, even by two exchanges racing on different replicas. No log
  line, audit entry or event carries the code, its hash or the verifier.
- **Audit.** `sso.login`, `sso.link`, `sso.user_created` and `sso.denied` (with the reason) are recorded in the
  tenant's audit log, as are `sso.provider.create`, `sso.provider.update` and `sso.provider.delete`. Their
  events are delivered only to subscribers holding `sso:admin`.

## The data model

`sso_providers` holds each tenant's providers: the settings every kind shares as columns, the protocol's own
settings as JSON, and the sealed secret. `user_identities` links (tenant, provider, subject) to an account, with
the email last reported and the last sign-in. SAML is another kind on the same two tables, with its IdP
settings in the provider's JSON, alongside the pinned settings, the certificates retiring after a rollover and
the outcome of the last metadata read. `sso_assertions` holds the ids of SAML assertions that signed somebody in,
until they expire, and `sso_handoffs` the hashed codes an app sign-in handed out, until they are exchanged or
expire. The v1 columns `users.sso_provider` and `users.sso_subject`, which nothing used, are gone.
