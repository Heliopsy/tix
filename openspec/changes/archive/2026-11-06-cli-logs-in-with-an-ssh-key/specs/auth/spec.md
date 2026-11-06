## ADDED Requirements

### Requirement: A person signs in with an enrolled SSH key

The service SHALL issue a session for a signature, made with an enrolled and unrevoked SSH key of a
person in the request's tenant, over a challenge it issued. The session SHALL be exactly what a password
sign-in issues, with the same lifetime, and the sign-in SHALL write the `user.login` audit entry and the
`session.created` event with `method` set to `ssh-key`. A password sign-in SHALL record `method`
`password`.

#### Scenario: A valid signature yields a session

- **GIVEN** a person with an enrolled ed25519 key and no second factor
- **WHEN** they sign a fresh challenge with that key under the namespace for this server
- **THEN** a session is issued and the audit entry records method `ssh-key`

#### Scenario: A key of another person, or none, is refused

- **WHEN** the signature verifies but the key is not enrolled in this tenant
- **THEN** the sign-in is refused saying the key is not enrolled, with reason `ssh_key_not_enrolled`

#### Scenario: A revoked key is refused

- **WHEN** the key was revoked
- **THEN** the sign-in is refused saying the key was revoked, with reason `ssh_key_revoked`

#### Scenario: A key enrolled against an agent is refused

- **WHEN** the key belongs to an agent actor
- **THEN** no session is issued and the refusal points at API tokens

### Requirement: An SSH sign-in challenge is short-lived, bound and single use

A challenge SHALL be a value signed by the server naming its tenant, its audience (the host the request
was made to, or `local`), its issue time and an expiry two minutes later. A challenge SHALL be refused
after its expiry, on another tenant or host, or when altered.

#### Scenario: An expired challenge is refused

- **WHEN** a challenge is presented more than two minutes after it was issued
- **THEN** the sign-in is refused saying the challenge expired

### Requirement: An SSH sign-in challenge is accepted at most once per key

A challenge SHALL be accepted at most once per key: accepting it SHALL record a last use no earlier
than its issue time, and a challenge issued at or before a key's last use SHALL be refused, decided by
the database so that it holds across replicas.

#### Scenario: A replayed challenge is refused

- **GIVEN** a sign-in that succeeded with a challenge and a signature
- **WHEN** the same challenge and signature are presented again
- **THEN** the sign-in is refused saying the challenge was already used, and no second session exists

### Requirement: An SSH sign-in signature is bound to tix and to the server

The signature SHALL be an OpenSSH `sshsig` signature whose namespace is `tix-login@` followed by the
audience, and whose public key is the presented key. A signature under any other namespace, a signature
made by another key, and a SHA-1 `ssh-rsa` signature SHALL be refused.

#### Scenario: A signature for another namespace is refused

- **WHEN** the nonce is signed under the namespace `git` or `tix-login@other.example`
- **THEN** the sign-in is refused naming the namespace it expected

#### Scenario: A signature by another key is refused

- **WHEN** the signature was made by a key other than the one presented
- **THEN** the sign-in is refused saying the signature does not verify

### Requirement: An SSH sign-in resolves the tenant like a password sign-in

The tenant SHALL be the one the request resolved, as for a password sign-in, and only that tenant's
enrolments SHALL be read. When the request names a tenant and it is not the resolved one, the sign-in
SHALL be refused as invalid, naming both.

#### Scenario: A key enrolled in two tenants signs in to the one the server names

- **GIVEN** one key enrolled for a person in tenants A and B
- **WHEN** they sign in against tenant A without naming a tenant
- **THEN** the session is for tenant A

#### Scenario: Naming another tenant is refused

- **WHEN** they sign in against tenant A naming tenant B
- **THEN** the sign-in is refused naming tenant A

### Requirement: An SSH key does not replace the second factor

An SSH sign-in SHALL be subject to the second factor exactly as a password sign-in on the command line:
a person with a confirmed enrolment, or in a tenant requiring a factor on the command line, SHALL receive
the same challenge, and the session SHALL be issued only when the code step completes, with the audit
entry recording method `ssh-key`.

#### Scenario: An enrolled person is asked for the code

- **GIVEN** a person with a confirmed TOTP enrolment and an enrolled key
- **WHEN** they sign a fresh challenge with the key
- **THEN** the answer is a code challenge and no session, and the code completes the sign-in
