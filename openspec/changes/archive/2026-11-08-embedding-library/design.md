# Design: running tix inside another process

## Context

`tix serve` is the composition of a local connection (`connect.Dial`: open, migrate, seed, local actor), the
browser handler, the HTTP router, and the background workers (lease sweeper, snooze waker, retention pruner,
webhook dispatcher, server registration, event pumps), all assembled in `cmd/serve.go` around
`server.Assemble`. tix-mobile already talks to that HTTP API. It needs the same thing on a phone, where an app
may not start a process, and on desktop, where shipping and supervising a binary is a burden.

## Goals / Non-Goals

**Goals:**

- One function that builds the server, called by both `tix serve` and the library.
- A small Go API (start, URL, stop, tenants, create tenant, token) and a C ABI over it for `dart:ffi`.
- Loopback only, several instances per process, no panic across the C boundary.
- Build recipes for every platform the app ships on, and a CI job that proves the library builds and works.

**Non-Goals:**

- PostgreSQL. The app owns a file; the package takes a path, not a DSN.
- The SSH listener. A phone serving a terminal is not a use case, and the hosted command runner lives in
  `cmd/`, which the library must not import.
- Attaching the libraries to releases (see Release wiring below).
- A second API. The app uses HTTP; the library only adds what HTTP cannot do without a credential.

## Decisions

- **`embedded`, at the module root.** It has to be importable from another module, so it cannot sit under
  `internal/`. `embed` would be the obvious name and is the wrong one: it is the standard library package
  behind `//go:embed`, which this repository uses, so every file importing both would need an alias, and
  `embed.Start` reads like a stdlib call. `embedded` says what the caller gets, tix embedded in their process.
- **`internal/serving` owns the composition.** `serving.Build(conn, cfg, Params)` builds the themes, egress
  policy, browser handler and `server.Assemble` options from an open local connection and the resolved
  configuration. `cmd/serve.go` keeps its flags and the SSH listener (which needs `RunHosted` from `cmd/`) and
  passes the listener in. The library calls the same function with serve's flag defaults, so a worker, an
  option or a handler option added to serve reaches the library without anybody remembering to.
- **The library reads no configuration.** It resolves with `config.Defaulted()`. The environment and files of
  an app process are not a tix operator's configuration, and a `TIX_*` variable or a stray `~/.config/tix`
  on a desktop changing what the app's database does would be a bug nobody could reproduce. The defaults are
  what `tix serve --db FILE` uses on a machine with nothing configured.
- **The connection is the holder of the database.** `connect.Dial` with no token gives the local actor of the
  default tenant, standing as a global admin (`direct`), exactly what `tix tenant create --db` and
  `tix token create --db` run as. Listing and creating tenants go through `core.Service` as that actor, so the
  rules and the audit entries are the service's own.
- **Tokens in another tenant: `Conn.InTenant`.** `tix token create --db FILE --tenant work` dials a second
  connection whose actor is the local actor of `work`. The library already holds the store, so
  `connect.(*Conn).InTenant` resolves the tenant on the same store and returns a connection sharing the
  service, with the local actor of that tenant (created on first use, as `ensureLocalActor` does) and
  nothing to close. No second SQLite handle on a file that already has a writer.
- **Token defaults.** No scopes means `*`, which the holder has; no lifetime means 90 days, the default the
  CLI and the browser form offer; a negative lifetime (`"never"` over the ABI) means no expiry, the explicit
  opt-in `--expires never` is on the CLI.
- **Readiness is the bound listener.** `Start` returns after `server.Listen` and before `Serve` runs its
  accept loop in a goroutine. A connection made after `Listen` returns waits in the kernel backlog until
  `Serve` accepts it, so the URL is usable the moment `Start` returns, with no polling.
- **`Start`'s context bounds the start.** It covers opening and migrating. The instance then lives until
  `Stop`; tying it to a request context the app cancels would stop the server underneath it.
- **Loopback is checked on an IP literal, before anything is opened.** `server.IsLoopbackAddr` accepts
  `localhost`, which a hosts file may point anywhere. The library parses the host as an IP and requires
  `IsLoopback`. Serve's bind guard still applies underneath.
- **Two packages for the C ABI.** `embedded/capi` is ordinary Go: a handle table behind a mutex and one
  function per export taking and returning JSON strings, with `recover` turning a panic into an error object.
  `embedded/libtix` is `package main` with `import "C"` and seven `//export` functions that convert strings
  and call `capi`. The logic is tested and covered under `CGO_ENABLED=0` like everything else; the shim is
  thin enough that the ABI test, which builds the real shared library and calls it from C, covers it.
- **cgo stays out of the binary structurally.** `embedded/libtix` is a `main` package, which nothing can
  import, and every file in it needs cgo, so `CGO_ENABLED=0 go build ./...` skips it. An architecture test
  lists the `tix` main package's dependencies with cgo enabled and fails on any module package with cgo files.
- **Error codes are tix error kinds.** `invalid`, `not_found`, `conflict`, `forbidden`, `internal` and the
  rest, as the HTTP API reports them. Malformed JSON is `invalid`, an unknown handle `not_found`, a panic
  `internal`.
- **Android pages.** Android 15 devices may run 16 KB pages, and Google Play requires native libraries to be
  16 KB aligned. NDK r28 aligns by default; the recipe passes `-Wl,-z,max-page-size=16384` anyway so an older
  NDK produces the same file, and verifies every `LOAD` segment with `llvm-readelf`.
- **Notices are generated, not maintained.** `internal/notices/gen` runs `go list -deps` for the library
  package with the target's `GOOS`, `GOARCH` and `CGO_ENABLED=1`, takes each module's licence file from the
  module cache, identifies it against the licences the dependency tree uses, and writes `NOTICES.json`. An
  unidentified licence fails the recipe instead of shipping an entry that says `unknown`. Standard library
  only, so no tool dependency enters `go.mod`.

## Known limitations

- `internal/connections.Default` is a process-wide registry that names the server it reports for. With two
  instances in one process the last one started names it, so `tix status` against either file may report the
  other's live connection count. Data and serving are unaffected.
- The workers wake on their intervals (lease sweep every minute, heartbeat every 30 seconds, webhook dispatch,
  hourly pruning), as they do under `tix serve`. An app should stop the instance when it is backgrounded;
  iOS suspends it regardless.

## Release wiring (not done here)

GoReleaser builds with `CGO_ENABLED=0` and cannot cross-link C for every target from one runner. A release
job would be a matrix in `release.yaml` after the binaries: an Ubuntu runner building Linux (amd64, arm64),
Android (four ABIs, with the NDK the runner image carries) and Windows (mingw-w64); a macOS runner building the
macOS universal dylib and an iOS xcframework (device and simulator archives joined by `xcodebuild
-create-xcframework`). Each platform is tarred with its header and `NOTICES.json`, checksummed into the
existing `checksums.txt`, signed with the same cosign keyless step, and uploaded to the release. tix-mobile
would then pin a version and verify the checksum instead of building tix itself.

## Risks / Trade-offs

- **Library size.** The library carries the whole server, browser assets included, so it is tens of
  megabytes before compression. Leaving the browser out would save the assets but fork the composition the
  shared constructor exists to keep single; it can be an option later if the size matters on a phone.
- **A second process on the same file.** SQLite allows it, and tix's busy timeout handles the contention, but
  `tix serve` on a desktop and the app on the same synced file is the network-filesystem warning in another
  form. The library leaves the storage location checks in place.
