# Make every configuration key do what it says

## Why

The stated precedence is flags > environment > `.env` > config file > defaults, applied uniformly to every
key, and `tix config show --sources` prints the effective value of each key with the layer it came from. Two
keys did not take part in that.

`server.listen` was resolved, reported by `tix config show --sources`, and then thrown away: `tix serve`
passed its `--listen` flag value, whose declared default is the same `127.0.0.1:8080` as the configuration
default. Every test that passed the flag passed. A deployment that set `TIX_SERVER_LISTEN` got a server on
loopback and no explanation, which is worst in a container, where the environment is the easiest layer to
reach and the command line the hardest. `docs/deployment.md` already claimed "`--listen` also reads from
`server.listen` / `TIX_SERVER_LISTEN`".

`server.token` had no reader at all. The credential handed to the client came from `--token` and `TIX_TOKEN`
only, so `TIX_SERVER_TOKEN`, a `server.token` in a configuration file and the token a named context carries
authenticated nothing: `tix ctx add work --server ... --token ...` wrote a token that was never presented.

## What Changes

- **`tix serve` lays the resolved configuration under its `--listen` flag**, the way `tix ssh` already does
  for every `ssh.` key, so a flag nobody typed leaves the configured value alone.
- **The credential is read from the resolved configuration**, which already carries `--token` and `TIX_TOKEN`
  as its top layer, so `server.token` and a context's token reach the wire in the documented order.
- **A guard over the whole class.** Every key config defines is classified as shadowed by a flag or not, the
  shadowed ones are asserted to fall back to the file layer and to still obey a typed flag, and two
  end-to-end tests assert `tix serve` and `tix ssh` bind the address the configuration file names.

## Impact

- The four `--ssh-*` flags of `tix serve` deliberately keep reading no key. `--ssh-listen` alone turns that
  listener on, so no lower layer may open a port in a process that was not asked for one.
- `--insecure-no-tls` has no key, so a container binding a non-loopback address still passes that one flag.
