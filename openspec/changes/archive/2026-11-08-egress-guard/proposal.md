# Egress guard

## Why

A tenant admin chooses urls the server then requests: webhook endpoints, inbox channels and external sync
sources. Webhooks and channels already refused internal addresses through a guard of their own, but sync
had no such check at all. A Jira or OpenProject source could point at loopback, the cloud metadata
endpoint or a service on the cluster network, and a run imports what it reads as tasks the tenant can see.
On a host shared by several tenants that turns a tenant admin into a prober of the operator's network.

## What Changes

- One egress guard, `internal/egress`, for every request to a tenant-supplied url. It checks the address
  a socket is about to connect to in the dialer's `Control` hook, so DNS rebinding and redirects are
  covered, and also checks each redirect hop and the url's text. Only `http` and `https` are accepted.
- The guard refuses loopback, unspecified, link-local, private and unique-local, carrier-grade NAT,
  multicast, reserved and special-purpose ranges, and the IPv6 forms that carry an IPv4 address
  (IPv4-mapped, IPv4-compatible, IPv4-translated, NAT64, 6to4, Teredo).
- **BREAKING** On `tix serve` a sync source whose url reaches an internal address is refused, when it is
  saved for a literal address or `localhost`, and at connect time otherwise. A command run against the
  database directly stays the operator's own and reaches what the operator's network allows, mirroring
  `SyncHost{Trusted}`.
- New operator keys `server.egress_allow` (addresses, CIDR ranges, host names, `.suffix`es) and
  `server.egress_allow_private` open exceptions for every client. `webhooks.allow_private_targets` keeps
  opening every internal address for webhooks and channels alone.
- Webhook and channel deliveries move onto the shared guard. Their policy gains `0.0.0.0/8`, `240.0.0.0/4`,
  `fec0::/10`, 6to4, Teredo and the other transition forms, and the test-binary exception that let the
  suite reach loopback without configuration is removed.
- Refusals say "address not allowed" and nothing about what a name resolved to.

## Capabilities

### Modified Capabilities

- `external-sync`: a source's url is held to the egress policy on a shared server.
- `webhooks`: endpoints and channels are held to the shared egress policy and its allow list.
- `configuration`: `server.egress_allow` and `server.egress_allow_private`.

## Impact

- New package `internal/egress`; `internal/webhook` guard rebuilt on it; `internal/sync` HTTP client uses
  it; `internal/service`, `internal/connect`, `internal/server`, `cmd/serve.go` wire the policy;
  `internal/config` adds and validates the keys.
- Docs: `docs/configuration.md`, `docs/deployment.md`, `docs/migrating.md`, `SECURITY.md`, the skill.
- Servers whose sync sources reach an internal tracker must allow it in `server.egress_allow`.
