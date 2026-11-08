# A slimmer embedded build

## Why

The embedding library (`embedded/libtix`) links everything `tix serve` composes, because both build their
server through `internal/serving.Build`. An app that embeds tix only talks to the HTTP API on loopback, yet
it ships the browser interface, the PostgreSQL driver, the terminal table renderers and the server-side
integrations. The stripped library is about 31 MB per target, and on Android it has to be stored
uncompressed for 16 KB page alignment, so it adds about 33 MB to every installed APK (tix-mobile's arm64 APK
went from 20.4 MB to 53.4 MB). Measured on linux/amd64: 14.0 MB of machine code, 9.9 MB of line tables
that grow with the code, and the web templates and assets (about 1.9 MB) plus `openapi.yaml` (0.5 MB) as
embedded data.

## What Changes

- A build tag, `tix_embedded`, that the library recipes (`just lib`, `lib-android`, `lib-ios`, `lib-macos`,
  `lib-windows`) pass and the `tix` binary never does.
- Under the tag, `internal/serving.Build` composes the HTTP API, the WebSocket event stream and the
  background workers a local database needs (lease sweeper, outbox, retention), and leaves out:
  - the browser interface: handlers, templates, assets, the Markdown renderer;
  - the PostgreSQL store and its driver, since an embedded database is always a SQLite file;
  - terminal and table rendering used only by CLI output;
  - server-side integrations that make no sense in-process: MCP, external sync, inbound mail channels.
  Each exclusion is decided in design.md against what the API still needs, not by guessing.
- Routes the tagged build does not serve answer the usual not-found envelope, and the capability registry
  knows which operations a tagged build omits, so the contract tests keep passing for both builds.
- CI builds both variants, reports the size of each library, and fails if the tagged build ever links a
  package on the excluded list.
- **Not changed:** the `tix` binary, the C ABI, the HTTP API the embedded build does serve, and the
  `NOTICES.json` contract (it naturally lists fewer modules).

## Expected Effect

An estimate, to be replaced with measurements: roughly a quarter to a third smaller, so about 20 to 24 MB
per target instead of 31 MB. The service layer, the SQLite engine, the Go crypto and HTTP stacks and the
runtime stay, and they are most of the size.

## Capabilities

### Modified Capabilities

- `embedding`: the library is built with the `tix_embedded` tag and serves the API without the browser
  interface and the server-only integrations.

## Impact

- `internal/serving`, `internal/web` wiring, `internal/store` engine selection, `internal/capability`
  (which operations a build omits), the `lib*` recipes, the library CI job, `docs/embedding.md`.
- tix-mobile picks it up by bumping `tix.lock`; its parity guard must treat operations the embedded build
  omits as unavailable on local instances (the browser-only ones already have no app binding).
