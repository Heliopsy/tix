## MODIFIED Requirements

### Requirement: Authenticator chain

The system SHALL resolve credentials through an ordered chain of authenticators so that additional identity providers can be added without changing callers. External identity providers SHALL be configured per tenant, never as a server authentication mode, and an external identity SHALL be recorded in `user_identities` rather than on the user record.

#### Scenario: Chain order

- **WHEN** a request carries both a bearer token and a session cookie
- **THEN** exactly one authenticator resolves the actor according to a deterministic, documented order

#### Scenario: No provider configured

- **WHEN** an operator sets `auth.mode` to `oidc`
- **THEN** the configuration is refused with a message naming `tix tenant sso add`, and the server does not start

#### Scenario: External identity columns present

- **WHEN** a person signs in through a tenant's provider for the first time
- **THEN** a `user_identities` row records the tenant, provider, subject and user, and the user record carries no provider or subject

## ADDED Requirements

### Requirement: A tenant configures its own sign-in providers

A tenant SHALL hold any number of sign-in providers, each with a key unique in the tenant, a kind of `oidc` or `oauth_github`, a display name and an enabled flag. Kind `saml` SHALL be refused as not implemented. Creating, changing, listing and deleting providers SHALL require the `sso:admin` scope, which the admin role carries.

#### Scenario: Member refused

- **WHEN** a member lists the tenant's providers
- **THEN** the call is forbidden

#### Scenario: SAML reserved

- **WHEN** an admin creates a provider of kind `saml`
- **THEN** it is refused as not implemented and nothing is stored

### Requirement: A provider's client secret is sealed and never returned

A provider's client secret SHALL be sealed with the server's sealing key, bound to the provider, and SHALL NOT appear in any response, audit entry, event or listing after it is stored; only whether one is set and, for a long secret, its last four characters SHALL be shown. Storing a secret without a sealing key SHALL be refused.

#### Scenario: Secret not echoed

- **WHEN** an admin creates a provider with a client secret and then shows it
- **THEN** neither answer contains the secret

### Requirement: An OIDC sign-in is checked end to end

An OIDC sign-in SHALL use the authorization code flow with PKCE, a random state and a random nonce. The ID token SHALL be refused unless its signature verifies against the issuer's published keys, its issuer and audience match the provider, it has not expired, and its nonce matches the flow. A callback whose state does not match the flow SHALL be refused.

#### Scenario: Wrong audience

- **WHEN** the provider returns an ID token issued for another client
- **THEN** the sign-in is refused and no session is minted

#### Scenario: Forged state

- **WHEN** the callback carries a state other than the one the flow began with
- **THEN** the sign-in is refused before the code is exchanged

### Requirement: GitHub sign-in reads a verified email and checks organisations

A GitHub sign-in SHALL take the subject from the account's numeric id and the email from the email API. An email SHALL count as verified only when GitHub reports it primary and verified. When allowed organisations are set, the person SHALL be an active member of one of them or be refused.

#### Scenario: Outside the organisation

- **WHEN** a provider allows organisation `acme` and the person belongs to none
- **THEN** the sign-in is refused naming the organisation requirement

### Requirement: An external identity links by subject, then verified email

A sign-in SHALL use the user linked to the provider and subject. Otherwise it SHALL link a verified email to the account with that email when it is a member of the tenant. Otherwise, with `auto_create`, it SHALL create an account with that email when none exists anywhere. Otherwise it SHALL be refused with a message saying what to do.

#### Scenario: Linked by email

- **WHEN** a member of the tenant signs in through a provider for the first time with a verified email
- **THEN** the identity is linked to their account and the next sign-in finds it by subject

#### Scenario: Unverified email

- **WHEN** the provider does not mark the email verified and no identity is linked
- **THEN** the sign-in is refused saying the email was not verified

#### Scenario: Not invited

- **WHEN** an unknown email signs in through a provider without `auto_create`
- **THEN** the sign-in is refused and says to ask an administrator for an invitation

### Requirement: Just-in-time creation never adopts another tenant's account

Just-in-time creation SHALL NOT add an account that exists in other tenants to this one; such a sign-in SHALL be refused and say an invitation is needed. A created account SHALL have no password and SHALL receive the mapped role or the provider's default role.

#### Scenario: Account elsewhere

- **WHEN** the email belongs to an account that is a member of another tenant only
- **THEN** the sign-in is refused and no membership is created

### Requirement: Allowed domains apply to every SSO sign-in

When a provider names allowed email domains, every sign-in through it, linked identities included, SHALL be refused unless the email's domain is one of them.

#### Scenario: Other domain

- **WHEN** a provider allows `example.com` and the email is `a@other.org`
- **THEN** the sign-in is refused

### Requirement: Groups map to tenant roles

When a provider maps groups to roles and the person's groups match, the highest matched role SHALL be applied at every sign-in. When none match, a created person SHALL get the default role and an existing member SHALL keep theirs.

#### Scenario: Group grants admin

- **WHEN** the mapping says `tix-admins` is admin and a member signs in with that group
- **THEN** their role in the tenant becomes admin

### Requirement: An SSO sign-in is confined to its tenant

An account that is a global admin SHALL be refused SSO sign-in. A session minted by SSO SHALL NOT switch to another tenant.

#### Scenario: Global admin

- **WHEN** a global admin's email arrives through a provider
- **THEN** the sign-in is refused and names password or SSH key sign-in

#### Scenario: Switch refused

- **WHEN** a session minted by SSO asks to switch tenant
- **THEN** the switch is refused

### Requirement: SSO sessions use the session machinery

A session minted by SSO SHALL follow the tenant's lifetime, idle timeout and keep me signed in policy and SHALL record its method as `sso:` followed by the provider key.

#### Scenario: Method recorded

- **WHEN** a person signs in through provider `kc`
- **THEN** their session list shows the method `sso:kc`

### Requirement: The second factor applies after SSO

The tenant's second factor requirement and a person's own enrolment SHALL apply after an SSO sign-in as after a password. A provider marked `trust_mfa` SHALL skip it only when the ID token's `amr` contains, or its `acr` equals, one of the provider's MFA values; `trust_mfa` SHALL be refused on GitHub providers.

#### Scenario: Code asked after SSO

- **WHEN** the tenant requires a code in the browser and a person signs in through a provider without `trust_mfa`
- **THEN** they are asked for a code before a session exists

#### Scenario: Trusted factor

- **WHEN** the provider has `trust_mfa` and the token's `amr` contains `mfa`
- **THEN** the session is minted without asking for a code

### Requirement: A command-line sign-in through a provider

`tix login --sso [KEY]` SHALL start the flow with a loopback redirect on 127.0.0.1, open the browser, and print the address when no browser opens, then complete the sign-in and end in the same session output as `tix login`. The service SHALL accept only a loopback redirect or the browser callback as a redirect URI.

#### Scenario: Loopback completes

- **WHEN** the browser returns the code to the loopback listener
- **THEN** the command prints a session like a password sign-in

#### Scenario: Foreign redirect

- **WHEN** a flow is begun with a redirect URI on another host and path
- **THEN** it is refused
