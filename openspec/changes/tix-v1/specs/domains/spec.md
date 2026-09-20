## ADDED Requirements

### Requirement: Hostname to tenant mapping

The system SHALL allow a tenant to register one or more hostnames in `tenant_domains`. A hostname
SHALL map to at most one tenant across the deployment, and mapping SHALL be case-insensitive.

#### Scenario: Domain registered

- **WHEN** a tenant registers `acme.example.com`
- **THEN** the hostname is stored against that tenant and can be listed by its administrators

#### Scenario: Hostname already claimed

- **WHEN** a second tenant attempts to register a hostname already mapped to another tenant
- **THEN** the registration is rejected with a conflict error

#### Scenario: Case and port ignored

- **WHEN** a request arrives with host `ACME.Example.com:8443`
- **THEN** it resolves to the same tenant as `acme.example.com`

#### Scenario: Domain removed

- **WHEN** a domain is removed from a tenant
- **THEN** later requests for that hostname no longer resolve to that tenant

### Requirement: Resolution precedes authentication

The system SHALL resolve the tenant from the request `Host` header before performing
authentication. Authentication and authorization SHALL then be evaluated against the already
resolved tenant.

#### Scenario: Order of evaluation

- **WHEN** a request arrives with both a Host header and credentials
- **THEN** the tenant is determined from the Host header first and credentials are validated afterwards against that tenant

#### Scenario: Unauthenticated request to a known host

- **WHEN** an unauthenticated request arrives for a mapped hostname
- **THEN** the tenant is resolved and the request is then refused for lack of credentials, without disclosing tenant data

### Requirement: Resolved tenant is pinned for the request

Once resolved, the tenant SHALL be pinned for the entire lifetime of the request. No header,
parameter, body field, or credential SHALL be able to change it.

#### Scenario: Parameter cannot override

- **WHEN** a request to a hostname mapped to tenant A includes a parameter naming tenant B
- **THEN** the request continues to operate on tenant A and the parameter is ignored

#### Scenario: Consistent across the request

- **WHEN** a request performs multiple service calls and emits events
- **THEN** every call and every emitted event is attributed to the pinned tenant

### Requirement: Credential tenant must match the resolved tenant

When a presented credential belongs to a tenant other than the resolved tenant, the request SHALL
be rejected. The system SHALL NOT silently honour the credential's tenant in place of the resolved
one.

#### Scenario: Token from another tenant

- **WHEN** an API token issued for tenant B is presented on a hostname mapped to tenant A
- **THEN** the request is rejected and no data from either tenant is returned

#### Scenario: Session from another tenant

- **WHEN** a session cookie established on tenant B's hostname is sent to tenant A's hostname
- **THEN** the request is treated as unauthenticated for tenant A

#### Scenario: Member of both tenants

- **WHEN** a user who is a member of both tenants authenticates on tenant A's hostname
- **THEN** the request operates on tenant A only, using that user's role in tenant A

### Requirement: Unknown host behaviour is configurable

The system SHALL provide a configuration setting controlling what happens when the request
hostname maps to no tenant. It SHALL support at least falling back to the default tenant and
responding with a not-found status, and SHALL document which is in effect.

#### Scenario: Fallback mode

- **WHEN** the setting selects fallback and a request arrives for an unmapped hostname
- **THEN** the request is resolved to the default tenant and served normally

#### Scenario: Strict mode

- **WHEN** the setting selects not-found and a request arrives for an unmapped hostname
- **THEN** the response is a 404 that does not reveal which tenants exist

#### Scenario: Missing Host header

- **WHEN** a request arrives with no usable Host header
- **THEN** it is handled by the configured unknown-host behaviour rather than defaulting to an arbitrary tenant

#### Scenario: Setting reported

- **WHEN** the diagnostic command runs
- **THEN** it reports the configured unknown-host behaviour

### Requirement: Domain verification state

Each registered domain SHALL carry a verification state of at least `pending` and `verified`,
together with the time of the last verification attempt. The system SHALL provide a verification
challenge that proves control of the hostname.

#### Scenario: New domain is pending

- **WHEN** a domain is registered
- **THEN** its state is pending and a verification challenge value is issued

#### Scenario: Successful verification

- **WHEN** the challenge is satisfied and verification is run
- **THEN** the domain becomes verified and the verification time is recorded

#### Scenario: Failed verification

- **WHEN** the challenge is not satisfied
- **THEN** the domain stays pending, the attempt time is recorded, and the failure reason is reported

#### Scenario: Unverified domains can be refused

- **WHEN** the deployment is configured to serve verified domains only and a request arrives for a pending domain
- **THEN** the request is handled as an unknown host

### Requirement: Per-domain TLS certificate configuration

The system SHALL allow a certificate and private key file to be configured per domain. When a
request arrives for a domain with configured files, that certificate SHALL be presented. Automatic
certificate acquisition through ACME SHALL NOT be part of this version.

#### Scenario: Certificate selected by hostname

- **WHEN** a TLS connection requests `acme.example.com` and that domain has configured certificate files
- **THEN** that certificate is presented

#### Scenario: No per-domain certificate

- **WHEN** a domain has no configured certificate
- **THEN** the server's default certificate is presented, or the connection is refused if none is configured

#### Scenario: Invalid certificate material

- **WHEN** a configured certificate file is missing, unreadable, or does not match its key
- **THEN** the server reports the specific domain and file at startup and refuses to start with a broken configuration

#### Scenario: ACME not offered

- **WHEN** an operator looks for automatic certificate issuance
- **THEN** the documentation and configuration state that certificates must be supplied or terminated at a proxy

### Requirement: Per-project vanity paths

Within a tenant, each project SHALL be addressable by a path of the form `/p/<key>/` where `<key>`
is the project key. The path SHALL resolve only within the tenant resolved for the request.

#### Scenario: Project path resolves

- **WHEN** a request for `/p/infra/` arrives on a hostname mapped to a tenant owning project `infra`
- **THEN** that project's view is served

#### Scenario: Same key in another tenant

- **WHEN** two tenants each own a project keyed `infra` and a request for `/p/infra/` arrives on one tenant's hostname
- **THEN** only that tenant's project is served

#### Scenario: Unknown key

- **WHEN** a request for `/p/<key>/` names a project the resolved tenant does not own
- **THEN** the response is a not-found result that does not reveal whether the key exists elsewhere

### Requirement: Proxy-aware host determination

When the deployment is configured to trust a reverse proxy, the system SHALL derive the request
hostname from the configured forwarded header. When no proxy is trusted, forwarded headers SHALL be
ignored.

#### Scenario: Trusted proxy

- **WHEN** the deployment trusts a proxy and a request carries a forwarded host header
- **THEN** the tenant is resolved from the forwarded value

#### Scenario: Untrusted source

- **WHEN** the deployment trusts no proxy and a request carries a forwarded host header
- **THEN** the header is ignored and the connection's Host header is used

#### Scenario: Spoofing attempt

- **WHEN** a client sends a forwarded host header naming another tenant's domain to an untrusted deployment
- **THEN** the tenant does not change

### Requirement: Domain administration is tenant-scoped

Domain registration, removal, verification, and certificate configuration SHALL be performed only
by an actor with administrative rights in the owning tenant. Listing domains SHALL return only the
resolved tenant's domains.

#### Scenario: Non-administrator refused

- **WHEN** an ordinary member attempts to register a domain
- **THEN** the operation is refused with an authorization error

#### Scenario: Listing is scoped

- **WHEN** a tenant administrator lists domains
- **THEN** only that tenant's domains are returned

#### Scenario: Cross-tenant domain modification

- **WHEN** an administrator of tenant A attempts to modify a domain belonging to tenant B
- **THEN** the operation fails as not found

### Requirement: Domain changes take effect without restart

Adding, removing, or verifying a domain SHALL take effect for subsequent requests without
restarting the server. Certificate file changes SHALL be applied on reload or on the next
connection without dropping established connections.

#### Scenario: New domain serves immediately

- **WHEN** a domain is registered and verified while the server is running
- **THEN** the next request for that hostname resolves to its tenant

#### Scenario: Removed domain stops resolving

- **WHEN** a domain is removed while the server is running
- **THEN** subsequent requests for that hostname are handled by the unknown-host behaviour

### Requirement: Domain resolution is observable

Every request SHALL record which tenant it was resolved to and by what means, and resolution
failures SHALL be reported with the offending hostname. Sensitive credential material SHALL NOT be
included in these records.

#### Scenario: Resolution logged

- **WHEN** a request is resolved to a tenant by hostname
- **THEN** the log entry records the hostname, the tenant, and that resolution was by domain

#### Scenario: Fallback logged

- **WHEN** an unmapped hostname falls back to the default tenant
- **THEN** the log entry records that fallback occurred and names the hostname

#### Scenario: Mismatch logged

- **WHEN** a credential's tenant does not match the resolved tenant
- **THEN** the rejection is logged with the hostname and the reason, without the credential value
