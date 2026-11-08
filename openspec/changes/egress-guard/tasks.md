# Tasks

- [x] `internal/egress`: classification, policy, dialer `Control` hook, transport, redirect check, client
- [x] Unit tests: IPv4, IPv6, mapped and transition forms; allow list; loopback httptest refused and allowed;
      redirect to a private address
- [x] Webhook guard rebuilt on `egress.Policy`; test-binary loopback exception removed
- [x] Sync HTTP client uses the policy; refusal not retried and names no address
- [x] Service: `WithEgress`, sync scope strict on a shared host and permissive on a trusted one, url
      checked when a source is saved
- [x] `server.egress_allow` and `server.egress_allow_private` keys, validated at load, wired through
      `connect.Dial` and `tix serve`
- [x] Tests per client: sync, webhooks and channels refused on loopback, allowed with the escape hatch, and
      a redirect into the network refused
- [x] Docs: configuration, deployment, migrating, SECURITY.md, skill
