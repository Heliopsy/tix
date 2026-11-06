## ADDED Requirements

### Requirement: Every entry records its access channel

Every audit entry SHALL record an access channel with a kind of web, api, cli-local, cli-remote, ssh, mcp, sync, import or system, set by the entry point the request arrived through and carried with it. An entry with no channel in its context SHALL record unknown.

#### Scenario: A browser change is web

- **WHEN** a signed-in browser creates a task
- **THEN** the task.create entry records the web channel

#### Scenario: An SSH session is ssh

- **WHEN** a terminal session over SSH claims a task
- **THEN** the task.claim entry records the ssh channel with the key fingerprint and the client address

### Requirement: The credential decides the channel

A request with a session cookie SHALL record web. A request with a bearer token SHALL record api, which an X-Tix-Client header of cli or mcp MAY narrow to cli-remote or mcp. No header SHALL make a request web, ssh, system, cli-local, sync or import.

#### Scenario: A header cannot claim SSH

- **WHEN** a token request sends X-Tix-Client: ssh
- **THEN** its entry records api

#### Scenario: A session stays web

- **WHEN** a browser session calls the API with X-Tix-Client: cli
- **THEN** its entry records web on that session

### Requirement: The channel carries what identified the caller

An entry SHALL record the token's ID and name for a token request, an opaque session reference for a browser session, the SSH key fingerprint for an SSH session, and for HTTP the client address as the trusted proxy policy resolves it and the user agent cut to 160 bytes.

#### Scenario: A token is named

- **WHEN** the ci-agent token creates a project
- **THEN** the entry records the token's ID and the name ci-agent

### Requirement: No secret reaches the audit or event log

The token secret, the session cookie and the stored session hash SHALL NOT appear in any audit entry or event row.

#### Scenario: Secrets are absent

- **WHEN** a token and a browser session each make a change
- **THEN** no audit or event row contains the token secret or the cookie value

### Requirement: Derived work keeps who started it

The changes of a sync run SHALL record sync, and of an import import, with the channel that started them as origin. Background work in the server SHALL record system with the job name: lease-sweep, snooze-wake, prune, housekeeping or sandbox-seed.

#### Scenario: A lease expiry names the sweeper

- **WHEN** the server's sweeper expires a lease
- **THEN** the task.lease_expire entry records system with job lease-sweep

#### Scenario: A sync started from the browser

- **WHEN** a sync run is started from a web session
- **THEN** its entries record sync with origin web

### Requirement: Host details of a local command are opt-in

A command run against a database directly SHALL record cli-local, and SHALL record the host name and operating-system user only when cli.record_host is on.

#### Scenario: Off by default

- **WHEN** a local command creates a project with cli.record_host unset
- **THEN** the entry records cli-local with no host or OS user

### Requirement: Entries from before the channel read as unknown

An entry written before the channel was recorded SHALL read as unknown, and a filter on unknown SHALL select it.

#### Scenario: An old entry

- **WHEN** an entry written before migration 0033 is listed
- **THEN** its channel kind is unknown

### Requirement: Audit listings filter by channel and token

Audit listings SHALL accept channels and token names to select by, each repeatable as alternatives, and SHALL refuse a channel that does not exist as invalid.

#### Scenario: By token

- **WHEN** the audit log is listed for the token ci-agent
- **THEN** only entries made with that token are returned

#### Scenario: A made-up channel

- **WHEN** the audit log is listed for the channel telepathy
- **THEN** the request fails as invalid naming the channels that exist
