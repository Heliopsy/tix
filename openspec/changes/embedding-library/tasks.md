# Tasks

## 1. Spec

- [x] 1.1 Proposal, design, delta spec and tasks for the embedding library

## 2. Shared composition

- [x] 2.1 `internal/serving.Build`, the server composition `tix serve` performed in `cmd/serve.go`
- [x] 2.2 `cmd/serve.go` builds its server through `serving.Build`
- [x] 2.3 `connect.(*Conn).InTenant`, the local owner of another tenant on the same store

## 3. Go package

- [x] 3.1 `embedded.Start` with the loopback check, init-if-missing and readiness on return
- [x] 3.2 `URL`, `Stop` (graceful, idempotent), `Tenants`, `CreateTenant`, `Token`
- [x] 3.3 Tests: existing and fresh files, missing file, two instances, tenants, token against `whoami`, stop,
  non-loopback refused

## 4. C ABI

- [x] 4.1 `embedded/capi`: handle table, JSON in and out, error objects, recover
- [x] 4.2 `embedded/libtix`: the seven exports over `capi`
- [x] 4.3 Tests: JSON round trips and error paths in Go, and a C harness against the built shared library
- [x] 4.4 Architecture test: the `tix` binary reaches no cgo package of this module

## 5. Build, notices and CI

- [x] 5.1 `just lib`, `lib-linux`, `lib-android`, `lib-ios`, `lib-macos`, `lib-windows`, `lib-test`
- [x] 5.2 Android libraries linked for 16 KB pages and checked with `llvm-readelf`
- [x] 5.3 `internal/notices` and `just lib-notices`, writing `NOTICES.json` beside the library
- [x] 5.4 CI job building and testing the library on Linux and macOS, cross-building Android, iOS and Windows

## 6. Docs

- [x] 6.1 `docs/embedding.md`, linked from `docs/README.md`; the recipes in `docs/development.md`
