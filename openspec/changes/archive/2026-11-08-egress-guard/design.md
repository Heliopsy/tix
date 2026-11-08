# Design

## One policy, decided at connect time

`egress.Policy` is a value: `AllowPrivate`, a list of allowed prefixes, and a list of allowed host names.
The zero value is strict. Every client built from it gets:

- a `net.Dialer` whose `Control` hook parses the literal `ip:port` about to be connected and refuses an
  address the policy does not permit. This runs after resolution and for every address Happy Eyeballs
  tries, so a name that resolved publicly when it was saved and privately now is caught;
- a transport with `Proxy: nil`, because a proxied request dials the proxy and the hook would judge the
  proxy's address;
- a redirect check that applies the url check to every hop before the caller's own rule (sync's same-origin
  rule; webhooks follow no redirect at all).

`CheckURL` refuses what the text alone settles (scheme, missing host, a literal internal address,
`localhost`). It never resolves: the dialer decides names, so the save-time check is a courtesy.

A host name on the allow list skips the `Control` hook for that dial only, because the hook sees an address
and the operator vouched for the name. The docs say to list only names whose DNS the operator controls.

## Classification

`Blocked` unmaps IPv4-mapped addresses first and then uses the `net/netip` predicates (loopback,
unspecified, private, link-local unicast and multicast, interface-local multicast, multicast) plus a table
of special-purpose prefixes from RFC 6890 and successors. Transition forms that embed an IPv4 address
(IPv4-compatible, IPv4-translated, NAT64, 6to4, Teredo) are refused whole rather than decoded: decoding
invites mistakes and none of them is a sensible webhook or tracker address. Documentation ranges are not
listed; they route nowhere.

## Who is strict

- Sync mirrors `SyncHost{Trusted}`: a service built for `tix serve` uses the operator's policy, and a
  trusted service (a command against the database) uses `egress.Permissive()`. The policy travels to the
  adapter on `SourceConfig.Egress`, so `importerFor` and the test hook keep their signatures.
- Webhooks and channels stay strict on every surface, as before. A direct-database drain still delivers to
  urls tenants chose, so being the operator's process changes nothing for them.

## Errors

The dialer returns `egress.ErrNotAllowed` ("address not allowed") unwrapped from the `net.OpError`, which
would otherwise carry the resolved address. Sync maps it to an invalid error that names the system and the
config key, and is not retried; webhooks keep recording the category `endpoint address is not permitted`.

## Test exception removed

The webhook guard allowed loopback in any binary built by `go test`. That made the strict policy untestable
against an httptest server and is gone: tests that deliver to one now open loopback through the same
allow list an operator would use.
