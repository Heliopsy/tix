## ADDED Requirements

### Requirement: Single serve process

The serve command SHALL start the HTTP API, the WebSocket event endpoint, and the web UI from one process listening on one address. No additional process SHALL be required to make any of the three available.

#### Scenario: All surfaces available

- **WHEN** the serve command is started
- **THEN** the API namespace, the WebSocket endpoint, and the web UI all respond on the configured listen address

#### Scenario: Startup failure is reported

- **WHEN** the listen address cannot be bound
- **THEN** the command exits with a non-zero status and an error naming the address and the reason

### Requirement: Loopback default bind

The server SHALL bind a loopback address by default when no listen address is configured.

#### Scenario: No configuration

- **WHEN** the serve command is started with no listen address configured
- **THEN** the server listens on a loopback address only and is not reachable from other hosts

#### Scenario: Explicit loopback

- **WHEN** a loopback listen address is configured without TLS
- **THEN** the server starts normally

### Requirement: Non-loopback bind safety

The server SHALL refuse to bind a non-loopback address unless TLS is configured or an explicit insecure opt-out is supplied. The refusal SHALL name the missing condition and SHALL NOT be overridable by configuration silently.

#### Scenario: Public bind without TLS

- **WHEN** a non-loopback listen address is configured with neither TLS nor the insecure opt-out
- **THEN** the server refuses to start and exits with a non-zero status explaining that TLS or the opt-out is required

#### Scenario: Public bind with TLS

- **WHEN** a non-loopback listen address is configured together with a certificate and key
- **THEN** the server starts and serves over TLS

#### Scenario: Explicit opt-out

- **WHEN** a non-loopback listen address is configured with the explicit insecure opt-out
- **THEN** the server starts over plain HTTP and logs a warning that traffic is unencrypted

### Requirement: TLS configuration

The server SHALL optionally serve TLS using a certificate and key supplied as files, and SHALL support presenting a different certificate per domain. Automatic certificate issuance SHALL NOT be provided in this version.

#### Scenario: Single certificate

- **WHEN** a certificate and key path are configured
- **THEN** the server serves HTTPS using that certificate

#### Scenario: Per-domain certificate

- **WHEN** certificates are configured for two domains and a client connects requesting one of them
- **THEN** the server presents the certificate configured for that domain

#### Scenario: Unreadable certificate

- **WHEN** a configured certificate or key file is missing or unparseable
- **THEN** the server refuses to start and reports which file failed

#### Scenario: No automatic issuance

- **WHEN** an operator configures automatic certificate issuance
- **THEN** the configuration is rejected as unsupported in this version

### Requirement: Graceful shutdown

On receiving a termination signal the server SHALL stop accepting new connections, allow in-flight requests to finish, close WebSocket connections with a normal closure, and exit within a configurable shutdown timeout.

#### Scenario: In-flight request completes

- **WHEN** a termination signal arrives while a request is being served
- **THEN** that request completes and its response is delivered before the process exits

#### Scenario: WebSocket closed cleanly

- **WHEN** a termination signal arrives while WebSocket connections are open
- **THEN** each connection receives a normal closure rather than an abrupt reset

#### Scenario: Timeout reached

- **WHEN** work remains in flight after the shutdown timeout elapses
- **THEN** the server terminates remaining connections and exits with a message stating the timeout was reached

#### Scenario: No new connections

- **WHEN** shutdown has begun
- **THEN** new connection attempts are refused

### Requirement: Background workers

The server SHALL run background workers for lease sweeping, webhook dispatch, and retention pruning. Each worker SHALL be individually configurable and individually disableable.

#### Scenario: Lease sweeper

- **WHEN** the lease sweeper is enabled and a lease passes its expiry
- **THEN** the expiry is materialized and a lease expired event is emitted

#### Scenario: Worker disabled

- **WHEN** the retention pruner is disabled
- **THEN** no pruning occurs and the other workers continue to run

#### Scenario: Worker failure isolation

- **WHEN** one background worker encounters an error
- **THEN** the error is logged, the other workers continue, and the HTTP surface keeps serving

#### Scenario: Workers stop on shutdown

- **WHEN** the server shuts down
- **THEN** each background worker stops before the process exits

### Requirement: Structured logging

The server SHALL emit structured logs at a configurable level.

#### Scenario: Level filtering

- **WHEN** the log level is set to a level above debug
- **THEN** debug records are not emitted and records at or above the configured level are

#### Scenario: Structured fields

- **WHEN** a log record is emitted
- **THEN** it carries machine-parseable key and value fields rather than only free text

### Requirement: Request logging without secrets

The server SHALL log each request with its method, path, status, duration, and resolved tenant. Logs SHALL NOT contain password material, session tokens, API token values, webhook signing secrets, or Authorization header contents.

#### Scenario: Request record

- **WHEN** a request completes
- **THEN** a log record exists carrying the method, path, status, duration, and tenant

#### Scenario: Credential redaction

- **WHEN** a request carrying an Authorization header or a session cookie is logged
- **THEN** no part of the credential value appears in the log

#### Scenario: Body not logged

- **WHEN** a request body containing a password field is processed
- **THEN** the password value does not appear in any log record

### Requirement: Request timeout and body limit

The server SHALL enforce a configurable per-request timeout and a configurable maximum request body size.

#### Scenario: Slow handler

- **WHEN** a request exceeds the configured request timeout
- **THEN** the server terminates the request and responds with a timeout error in the standard error envelope

#### Scenario: Oversized body

- **WHEN** a request body exceeds the configured maximum size
- **THEN** the server rejects the request without reading the body in full

### Requirement: Readiness gating

The server SHALL NOT report ready until database migrations are current and the database is reachable. It SHALL report ready only once both conditions hold.

#### Scenario: Migrations pending at startup

- **WHEN** the server starts against a database whose schema is behind the binary
- **THEN** readiness reports not ready until migrations have been applied

#### Scenario: Database becomes unreachable

- **WHEN** the database becomes unreachable while the server is running
- **THEN** readiness reports not ready while liveness continues to report alive

#### Scenario: Ready after migration

- **WHEN** migrations complete and the database is reachable
- **THEN** readiness reports ready

### Requirement: Listen address and base URL configuration

The server SHALL accept configuration for its listen address and for the externally visible base URL used when constructing absolute links.

#### Scenario: Links use base URL

- **WHEN** a base URL is configured and the server constructs an absolute link
- **THEN** the link uses the configured base URL rather than the listen address

#### Scenario: Base URL defaults

- **WHEN** no base URL is configured
- **THEN** absolute links are derived from the request scheme and Host

### Requirement: Trusted proxy handling

The server SHALL determine the client IP from forwarding headers only when the immediate peer is a configured trusted proxy, and SHALL otherwise use the peer address.

#### Scenario: Untrusted peer sends forwarding headers

- **WHEN** a request arrives from a peer that is not a configured trusted proxy but carries a forwarded-for header
- **THEN** the client IP is taken from the peer address and the header is ignored

#### Scenario: Trusted proxy

- **WHEN** a request arrives from a configured trusted proxy carrying a forwarded-for header
- **THEN** the client IP is taken from that header

#### Scenario: No trusted proxies configured

- **WHEN** no trusted proxies are configured
- **THEN** forwarding headers are ignored for every request
