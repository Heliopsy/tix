# Tasks

## 1. Spec

- [ ] 1.1 Proposal, design, delta spec and tasks for the embedding library

## 2. Shared composition

- [ ] 2.1 `internal/serving.Build`, the server composition `tix serve` performed in `cmd/serve.go`
- [ ] 2.2 `cmd/serve.go` builds its server through `serving.Build`
- [ ] 2.3 `connect.(*Conn).InTenant`, the local owner of another tenant on the same store

## 3. Go package

- [ ] 3.1 `embedded.Start` with the loopback check, init-if-missing and readiness on return
- [ ] 3.2 `URL`, `Stop` (graceful, idempotent), `Tenants`, `CreateTenant`, `Token`
- [ ] 3.3 Tests: existing and fresh files, missing file, two instances, tenants, token against `whoami`, stop,
  non-loopback refused

## 4. C ABI

- [ ] 4.1 `embedded/capi`: handle table, JSON in and out, error objects, recover
- [ ] 4.2 `embedded/libtix`: the seven exports over `capi`
- [ ] 4.3 Tests: JSON round trips and error paths in Go, and a C harness against the built shared library
- [ ] 4.4 Architecture test: the `tix` binary reaches no cgo package of this module

## 5. Build, notices and CI

- [ ] 5.1 `just lib`, `lib-linux`, `lib-android`, `lib-ios`, `lib-macos`, `lib-windows`, `lib-test`
- [ ] 5.2 Android libraries linked for 16 KB pages and checked with `llvm-readelf`
- [ ] 5.3 `internal/notices` and `just lib-notices`, writing `NOTICES.json` beside the library
- [ ] 5.4 CI job building and testing the library on Linux and macOS, cross-building Android, iOS and Windows

## 6. Docs

- [ ] 6.1 `docs/embedding.md`, linked from `docs/README.md`; the recipes in `docs/development.md`
