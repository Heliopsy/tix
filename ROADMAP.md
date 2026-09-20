# Roadmap

v1 scope is the `tix-v1` OpenSpec change. This file records what comes after, and
the seam in v1 that makes each one additive rather than a rewrite.

## v2

### Bidirectional sync with Jira and OpenProject

v1 imports one way and refreshes idempotently. v2 pushes changes back.

**Seam in v1:** `external_refs` already records `system`, `external_id`, `external_url`,
`external_version`, and `last_synced_at` for every imported entity, and `sync_sources`
holds a per-source cursor. `external_version` is populated in v1 but read only for change
detection; it becomes the conflict-detection field. The `Importer` interface is shaped so
a `Syncer` adding `Push` extends it.

**Still to design:** conflict resolution policy when both sides changed, change-origin
tracking so a pushed change does not echo back as an inbound change, inbound webhook
receivers for Jira and OpenProject, and per-field sync direction.

### SSO / OIDC

**Seam in v1:** `users.sso_subject` and `users.sso_provider` exist and are never read.
`internal/auth` defines an `Authenticator` interface with a local implementation, and the
middleware holds a chain. Adding a provider is a new implementation plus a config block,
with no schema migration and no signature change.

### MCP server mode

**Seam in v1:** `core.Service` is transport-neutral and complete. An MCP server is a third
adapter alongside `internal/httpapi` and `cmd/`, mapping tools onto Service methods.
Nothing in v1 needs to change to add it.

### ACME / automatic certificates

**Seam in v1:** `tenant_domains.cert_mode` already distinguishes how a domain gets its
certificate. v1 supports supplied certificate files; ACME becomes another mode.

### Full-text search on SQLite

**Seam in v1:** search is behind a store method with a PostgreSQL `tsvector` implementation
and a SQLite `LIKE` implementation. FTS5 becomes a third implementation of the same method.
The asymmetry is documented in `docs/scaling.md`.

### Desktop application

A thin wrapper around the browser interface rather than a second client: the same
server-rendered screens in a native window, so there is one interface to build and one to
test. A wrapper keeps the multiplatform cost near zero compared with a native rewrite, and
the CLI already covers the scripted path.

**Seam in v1:** the browser interface is server rendered with no JavaScript build step, and
`tix serve` binds a local address by default. A desktop build starts the server on a
loopback port and points a webview at it, so it needs no new product surface. The open
question is bundling: a webview per platform is small but ties the build to each platform's
toolchain, while shipping a browser engine is large but uniform.

### Tests on arm64 as well as amd64

The release matrix already cross-compiles linux, darwin, windows and android for amd64 and
arm64, but every test runs on amd64 only, so an architecture-specific fault would ship. The
parts most exposed are the pure Go SQLite driver, anything touching unaligned access or
atomics, and the time handling that lease expiry depends on.

**Seam in v1:** `CGO_ENABLED=0` everywhere and a `just ci` that runs the whole gate set in a
container, so the same recipes run on an arm64 runner without change. What is missing is the
runner and a matrix axis in the workflow.

## Considered and deliberately not planned

- Time tracking, billing, sprint and velocity reporting. Different product.
- Real-time collaborative text editing on task bodies. Large cost, narrow benefit.
- A hosted multi-customer SaaS control plane. Multi-tenancy supports it; operating it is out of scope.
