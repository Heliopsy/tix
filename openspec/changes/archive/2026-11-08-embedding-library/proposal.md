# Run tix inside another process

## Why

tix-mobile, the Flutter client, wants to work on a local `.db` file with no server anywhere: on a phone, on a
laptop with nothing installed, offline. iOS and Android do not let an app start a second process, so shipping
the `tix` binary beside the app and running `tix serve` is not an option there. The app has to load tix as a
library and run it in its own process. On desktop the same library saves the app from finding, versioning
and supervising a binary.

The app already speaks the HTTP API, so the shortest path is to run exactly what `tix serve` runs, on a
loopback port, inside the app, and hand the app a token for it.

## What Changes

- A public Go package, `github.com/heliopsy/tix/embedded`, that starts tix on a database file: it opens or
  creates the file, starts the HTTP API and the same background workers `tix serve` runs on a loopback
  address, and returns once the address accepts connections. It lists and creates tenants and mints API
  tokens for the local owner of a tenant, so the app needs no password for its own file. Several instances
  on different files can run in one process. A non-loopback listen address is refused.
- The composition `tix serve` performs moves from `cmd/` into `internal/serving`, so the command and the
  library build the server through one function and cannot drift apart.
- A C ABI over the package for `dart:ffi`: seven exported functions, JSON in and JSON out, addressed by an
  integer handle, that never let a Go panic cross into the caller. It is a separate `main` package built
  with `-buildmode=c-shared` (Linux, Android, macOS, Windows) or `-buildmode=c-archive` (iOS); it is the only
  cgo in the repository and the `tix` binary never links it.
- `just` recipes to build the library for the host and for Android, iOS, macOS and Windows, with Android
  libraries linked for 16 KB pages, and one that writes the licence notices of exactly the modules the
  library links.
- A CI job that builds the library and drives it through its C ABI on Linux and macOS, cross-builds it for
  Android (checking the page alignment), iOS and Windows.

## Capabilities

### New Capabilities

- `embedding`: running tix in-process through the Go package and the C ABI, and building the library.

### Modified Capabilities

None. `tix serve` behaves exactly as before; only where its server is assembled moves.

## Impact

- `embedded/`, `embedded/capi/`, `embedded/libtix/`: new.
- `internal/serving`: new, the server composition shared by `tix serve` and the library.
- `internal/connect`: a connection can stand as the local owner of another tenant on the same store.
- `internal/notices`: new, the generator of the library's licence notices.
- `cmd/serve.go`: calls `internal/serving`.
- `justfile`, `.github/workflows/ci.yaml`: library recipes and the library job.
- `docs/embedding.md`, `docs/README.md`, `docs/development.md`: the library.
