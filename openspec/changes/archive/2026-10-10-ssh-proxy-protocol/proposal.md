# The SSH listener learns the real client address behind an L4 proxy

## Why

The HTTP surface resolves the client address through a trusted-proxy policy. The SSH surface does not
resolve it at all: `internal/sshd` reads `sess.RemoteAddr()` and `ctx.RemoteAddr()` directly in every
place that names a source.

| Reader | Uses | Consequence behind a proxy |
| --- | --- | --- |
| `sshd.go:174` rate limiter | `sourceOf(ctx.RemoteAddr())` | one bucket for the whole internet |
| `session.go:131` recorded `Remote` | `sourceOf(sess.RemoteAddr())` | every session attributed to the proxy |
| `session.go`, `keepalive.go` logs | `sourceOf(...RemoteAddr())` | no log distinguishes two users |

That is correct for a directly exposed listener and wrong behind anything that proxies TCP. HAProxy, an
AWS NLB, an nginx `stream` block and most cloud L4 balancers are ordinary ways to expose SSH, and each of
them replaces the peer address with its own.

The rate limiter is the part that fails unsafely rather than merely unhelpfully. `allow(source)` is a
per-source allowance; collapsing every client onto one source throttles all users collectively while
handing a single attacker the allowance intended for the entire population.

There are no headers in an SSH stream, so the HTTP mechanism does not carry over. The standard answer is
the PROXY protocol, which an L4 proxy prepends to the connection before any application bytes.

## What changes

- A new `internal/proxyproto` package parses PROXY protocol v1 and v2 headers.
- The SSH listener is wrapped so that, **only when the immediate peer is a configured trusted proxy**, the
  header is read and the connection reports the client address it names.
- A new `ssh.trusted_proxies` configuration key, an IP and CIDR list with the same shape and the same
  default-deny meaning as `server.trusted_proxies`.
- Every existing reader of `RemoteAddr()` in `internal/sshd` is unchanged: they read the wrapped
  connection's address and get the right answer without knowing why.

## What does not change

- With no `ssh.trusted_proxies` configured, nothing is parsed and behaviour is byte-for-byte what it is
  today. This is the default.
- `server.trusted_proxies` is not reused for SSH and does not imply it. The HTTP reverse proxy and the L4
  SSH proxy need not be the same host, and a security-relevant list should not be inherited by a surface
  the operator was not thinking about when they wrote it.
