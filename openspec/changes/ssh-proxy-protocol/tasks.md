# Tasks

## internal/proxyproto

- [ ] `Header` type carrying the source address and whether the proxy spoke for itself.
- [ ] v1 text parser: `PROXY TCP4|TCP6|UNKNOWN ...\r\n`, bounded by the 107-byte maximum the spec sets.
- [ ] v2 binary parser: 12-byte signature, version and command, family and protocol, address block.
- [ ] Reject: truncated input, a bad signature, an unknown version, a length that disagrees with the
      family, and a v1 line past its maximum.
- [ ] Table tests over both versions, both families, `UNKNOWN`, `LOCAL`, and each rejection above.

## internal/sshd

- [ ] Listener wrapper consulting `auth.ProxyPolicy.Trusts` on the transport peer address.
- [ ] Eager parse off the accept path, with a deadline; `RemoteAddr()` waits for it.
- [ ] Untrusted peer: no read, no consumption, transport address reported.
- [ ] Trusted peer without a valid header: connection closed.
- [ ] Test that a stalled trusted peer delays no other pending connection.
- [ ] Test that the rate limiter buckets by resolved address, driving the real callback.

## Configuration

- [ ] `ssh.trusted_proxies` in `internal/config`, with its `TIX_SSH_TRUSTED_PROXIES` mapping.
- [ ] Validate through the existing `auth.NewProxyPolicy` call, naming the SSH key.
- [ ] Wire through `cmd/ssh.go` and `cmd/serve.go`, which also hosts SSH via `--ssh-listen`.
- [ ] Test that `server.trusted_proxies` alone grants the SSH listener nothing.

## Docs

- [ ] `docs/configuration.md`: the key, the env var, the default-deny meaning.
- [ ] `docs/deployment.md`: a worked HAProxy `send-proxy-v2` example and the matching configuration.
