# Design

## Reuse: the policy already exists

`auth.NewProxyPolicy([]string)` and `(*ProxyPolicy).Trusts(netip.Addr)` are transport-agnostic; only
`ClientIP`, `Scheme` and `PeerIP` take an `*http.Request`. The SSH path uses the first two and adds no
second notion of "trusted". `internal/config/validate.go` already validates a proxy list and names the
offending key, so `ssh.trusted_proxies` is validated by the same call.

This is what keeps the change small. The new code is the wire format and the listener wrapper, not the
security policy.

## Where the header is read, and why not in Accept

The obvious placement is wrong. Reading the header inside `Accept()` serializes the listener behind one
client's first packet: a peer that connects and sends nothing stalls every other pending connection until
it times out. Accept must stay a syscall.

So `Accept` returns immediately with a wrapper, and the header is consumed on the wrapper's first `Read`.
That is the shape `go-proxyproto` uses and it is right for the general case, but it does not survive this
codebase unmodified, because of the next section.

## The ordering constraint that decides the design

`sshd.go:174` calls `s.limiter.allow(sourceOf(ctx.RemoteAddr()))` during the connection callback, which
runs **before** the SSH transport exchanges any application bytes. A wrapper that parsed lazily on first
`Read` would therefore be asked for `RemoteAddr()` before it had parsed anything, and would answer with
the proxy's address: the rate limiter, the one reader that fails unsafely, would be the one reader still
getting the wrong answer.

The parse is therefore eager, performed in a goroutine per connection between accept and hand-off, with a
deadline. `RemoteAddr()` blocks until the parse completes or the deadline expires. Every existing caller
gets the resolved address with no change, including the callback that runs first.

## Untrusted peers are not parsed, and this is the security boundary

An unconditional parser is a vulnerability, not a feature: any client could prepend a PROXY header and
claim any source address, defeating the rate limiter it was added to fix and forging the recorded
`Remote`. The wrapper consults `Trusts(peer)` using the **transport** peer address, which cannot be
forged, and only then reads a header.

For an untrusted peer nothing is read and nothing is consumed from the stream, so an SSH client that
connects directly to a listener that also serves a proxy is unaffected.

## Decisions a reviewer may want to argue with

1. **v1 and v2 both, not v1 only.** AWS NLB emits v2 exclusively. Shipping v1 alone would silently fail
   for a common deployment, and silently here means "falls back to the proxy address", which is the exact
   bug being fixed. The two are distinguishable by their first 12 bytes with no ambiguity.
2. **A trusted peer that sends no valid header is refused, not accepted.** The alternative is accepting it
   with the proxy's address, which reintroduces the bug for whichever connection was malformed and makes a
   misconfigured proxy look like it is working. Refusing makes it loud.
3. **A v2 `PROXY` command whose family names no IP keeps the transport address too.** `AF_UNSPEC` and
   `AF_UNIX` carry no client address the receiver could prefer, which is what the protocol says to do with
   them, so they are treated as the proxy speaking for itself rather than as a malformed header.
4. **`LOCAL` (v2 command 0x0) keeps the transport address.** That is what the protocol says it means: the
   proxy is speaking for itself, such as a health check, not relaying a client.
5. **`UNKNOWN` (v1) likewise keeps the transport address**, because the proxy has said it does not know.
6. **Hand-rolled rather than `go-proxyproto`.** Consistent with the repository's stated refusal of
   dependencies for small, stable formats, and the reason is stronger than usual here: this parser sits on
   a security boundary and is short enough to read in full. The format is frozen and has not changed since
   2014.
7. **A deadline is mandatory.** A trusted proxy that opens a connection and stalls would otherwise hold a
   slot indefinitely. The deadline is the SSH listener's existing budget for a connection that has not
   opened a session, `Options.IdleTimeout`, rather than a new knob. `ssh.Server.HandshakeTimeout` is not set
   by this codebase, so that is the only existing budget there is to reuse.
