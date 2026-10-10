## ADDED Requirements

### Requirement: The listener believes a forwarded client address only from a trusted proxy

The listener SHALL read a PROXY protocol header from a connection only when the transport peer address is
matched by `ssh.trusted_proxies`. For any other peer the listener SHALL consume nothing from the stream
and SHALL report the transport peer address as the client address.

The transport peer address SHALL be the address the operating system reports, which a client cannot
choose. Trust SHALL NOT be derived from anything the connection carries.

#### Scenario: An untrusted peer sends a PROXY header

- **WHEN** a client whose address is not listed connects and prepends a well-formed PROXY header naming
  another address
- **THEN** the header is not parsed, the reported client address is the client's own transport address,
  and the bytes it sent are delivered to the SSH transport as ordinary input

#### Scenario: No trusted proxies are configured

- **WHEN** `ssh.trusted_proxies` is unset or empty and any client connects
- **THEN** no header is read and every reported client address is the transport peer address

#### Scenario: A trusted proxy speaks for a client

- **WHEN** a peer matched by `ssh.trusted_proxies` connects and sends a PROXY header naming a client
- **THEN** the reported client address is the one the header names

### Requirement: The resolved address is what every reader sees

The listener SHALL resolve the client address before any component reads it, including the connection
callback that applies the per-source rate limit.

The rate limiter, the recorded session `Remote` and every log line naming a source SHALL report the
resolved address.

#### Scenario: The rate limiter counts clients, not the proxy

- **WHEN** two clients reach the listener through one trusted proxy and each opens connections
- **THEN** each is limited against its own allowance, and neither consumes the other's

#### Scenario: The session record names the client

- **WHEN** a session is established through a trusted proxy
- **THEN** the recorded `Remote` is the client's address and not the proxy's

### Requirement: Both PROXY protocol versions are accepted

The listener SHALL accept version 1 and version 2 headers, distinguishing them by their leading bytes.

#### Scenario: A version 1 header

- **WHEN** a trusted proxy sends a v1 text header naming a TCP4 or TCP6 client
- **THEN** the named address is the reported client address

#### Scenario: A version 2 header

- **WHEN** a trusted proxy sends a v2 binary header with the PROXY command and an AF_INET or AF_INET6
  address block
- **THEN** the named address is the reported client address

#### Scenario: The proxy speaks for itself

- **WHEN** a trusted proxy sends a v1 `UNKNOWN` header, a v2 `LOCAL` command, or a v2 `PROXY` command whose
  address family names no IP, such as `AF_UNSPEC` or `AF_UNIX`
- **THEN** the reported client address is the proxy's own transport address

### Requirement: A trusted proxy that sends no valid header is refused

The listener SHALL close a connection from a trusted proxy that does not begin with a valid PROXY header,
and SHALL NOT fall back to the transport peer address.

The listener SHALL apply a deadline to reading the header and SHALL close a connection that does not
complete one within it.

#### Scenario: A malformed header from a trusted proxy

- **WHEN** a peer matched by `ssh.trusted_proxies` sends bytes that are not a valid PROXY header
- **THEN** the connection is closed and no session begins

#### Scenario: A trusted proxy stalls

- **WHEN** a peer matched by `ssh.trusted_proxies` connects and sends nothing
- **THEN** the connection is closed once the deadline passes, and no other pending connection is delayed
  by it

### Requirement: The trusted proxy list is configured for SSH in its own right

The configuration SHALL accept `ssh.trusted_proxies`, a list of IP addresses and CIDR blocks, settable by
`TIX_SSH_TRUSTED_PROXIES` like every other key.

`server.trusted_proxies` SHALL NOT be consulted by the SSH listener.

A malformed entry SHALL be refused at configuration load, naming `ssh.trusted_proxies`.

#### Scenario: The HTTP list does not grant SSH trust

- **WHEN** `server.trusted_proxies` names an address and `ssh.trusted_proxies` is empty
- **THEN** that address is not trusted by the SSH listener

#### Scenario: A malformed entry

- **WHEN** `ssh.trusted_proxies` contains a value that is neither an IP address nor a CIDR block
- **THEN** startup fails, naming `ssh.trusted_proxies` and the offending value
