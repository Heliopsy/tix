# Tasks

## 1. The fallbacks

- [x] 1.1 `cmd/serve.go`: `serveOptions.fromConfig` copies `server.listen` when `--listen` is untyped
- [x] 1.2 `cmd/serve.go`: `runServe` applies it once the configuration is resolved
- [x] 1.3 `cmd/root.go`: `globals.credential` reads `server.token` off the resolved configuration, and
      `overrides` presents it

## 2. The guard

- [x] 2.1 `cmd/config_shadow_test.go`: every key classified as shadowed by a flag or read by a named consumer
- [x] 2.2 Each shadowed key falls back to the file layer, read off the options struct the command hands on
- [x] 2.3 Each shadowed key still obeys a typed flag
- [x] 2.4 `tix serve` answers a request on the address a configuration file names
- [x] 2.5 `tix ssh` accepts a connection on the address a configuration file names
- [x] 2.6 `server.token` from a configuration file authenticates against a real server

## 3. Documentation

- [x] 3.1 `docs/configuration.md`: what `server.listen` and `server.token` do and how the flags layer over them
- [x] 3.2 `docs/deployment.md`: the container example configured by environment rather than a hardcoded address
