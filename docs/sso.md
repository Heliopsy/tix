# Single sign-on

A tenant can let its people sign in with an account they already have: an OpenID Connect (OIDC) identity
provider such as Keycloak, Authentik, Microsoft Entra ID, Google or Okta, or GitHub. Providers are configured
per tenant, by the tenant's own administrators. Each enabled provider is a **Sign in with NAME** button on the
web sign-in screen and a choice for `tix login --sso`.

SAML is not implemented yet. Its kind, `saml`, is reserved and refused.

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
PKCE is used either way. A GitHub provider always needs one.

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
| Switched on | `--disabled` on add, `--enabled=false` on edit | on |

The same operations are `/api/v1/sso/providers` in the [HTTP API](api.md) and the Single sign-on screen in the
browser.

### The single sign-on screen

Under **Manage**, **Single sign-on** lists the tenant's providers with what each does, names the redirect URL to
register, and holds the password sign-in setting. **Add a provider** opens the form; each provider has an
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

## How a sign-in works

The browser opens `/login/sso/KEY`. tix seals the sign-in's state, nonce, PKCE verifier, redirect URI,
Keep me signed in and the screen to return to into an HttpOnly cookie scoped to `/login/sso`, and sends the
browser to the provider. The provider sends it back to `/login/sso/callback` with a code. tix checks the state
against the cookie, exchanges the code with the verifier, and for OIDC verifies the ID token: the signature
against the issuer's published keys (cached and refreshed when a token names a key it has not seen), the
issuer, the audience (the client id), the expiry and the nonce. A sign-in has ten minutes to come back.

Nothing is stored until the sign-in completes, so an abandoned one leaves nothing behind, and any replica with
the same `auth.totp_key` completes a sign-in another began.

## Linking rules

In order:

1. **Subject.** An identity already linked to this provider's subject signs in its account.
2. **Verified email.** Otherwise, an address the provider vouches for (OIDC `email_verified`, or a GitHub
   primary and verified address) links to the account with that address, if it is a member of this tenant.
   The link is recorded in `user_identities`, so the next sign-in matches on the subject even if the address
   changes.
3. **Just-in-time creation.** Otherwise, with `--auto-create`, a new account is created for an address no
   account has anywhere, with the role the groups map to or `--default-role`. It has no password.
4. **Refusal.** Otherwise the sign-in is refused, saying why: no email, an unverified email, no account here
   (ask for an invitation), or an account that belongs only to other tenants.

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

```sh
tix --server https://tix.example.com login --sso
tix --server https://tix.example.com login --sso=github --no-browser
```

`--sso` alone uses the tenant's only provider and refuses, naming them, when there are several. tix listens on
`127.0.0.1` on a free port, opens the provider in the browser and prints the address too; on a machine with no
browser, open it elsewhere on the same machine, since the provider sends the browser back to the loopback
address. The result is the same session `tix login` prints, and `--tenant` and `--context` work as for a
password. Register the loopback redirect with the provider as described above.

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
- **Use `--allowed-domain` with `--auto-create`** on any provider whose accounts anyone can open, such as Google
  or GitHub, or the whole world can create an account.
- **Secrets stay sealed.** The client secret is sealed with the server key and bound to the provider; it never
  reaches a response, an audit entry or an event. Losing `auth.totp_key` means storing every client secret again.
- **The flow is bound to the browser.** State, nonce and PKCE verifier live in a sealed HttpOnly cookie; a
  callback from another browser, or with another state, is refused before the code is exchanged.
- **Audit.** `sso.login`, `sso.link`, `sso.user_created` and `sso.denied` (with the reason) are recorded in the
  tenant's audit log, as are `sso.provider.create`, `sso.provider.update` and `sso.provider.delete`. Their
  events are delivered only to subscribers holding `sso:admin`.

## The data model

`sso_providers` holds each tenant's providers: the settings every kind shares as columns, the protocol's own
settings as JSON, and the sealed secret. `user_identities` links (tenant, provider, subject) to an account, with
the email last reported and the last sign-in. SAML will be another kind on the same two tables. The v1 columns
`users.sso_provider` and `users.sso_subject`, which nothing used, are gone.
