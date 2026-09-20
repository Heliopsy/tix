# Tasks

Work packages are file-exclusive so they can be built in parallel. No two concurrent
packages own the same file. Where several packages contribute to one Go package, each
owns distinct files and the wave-opening package lands the shared skeleton first.

Each package follows per-slice TDD: its types, then its tests (red), then its
implementation. The repository stays buildable between packages.

A package is done when its tests pass, `just check` is green, its tasks here are
ticked, and coverage is at or above 80% (excluding `internal/tui`).

## 1. Wave 0 — Foundation (blocks everything)

- [x] 1.1 WP-01 Repository bootstrap: `go.mod`, `main.go`, `internal/version/`, `.tool-versions`
- [x] 1.2 WP-01 `justfile` with dev, gate, container, and aggregate recipes (`check`, `ci`)
- [x] 1.3 WP-01 `Containerfile` (runtime, Go version from build arg) and `Containerfile.ci` (pinned toolbox)
- [x] 1.4 WP-01 `.golangci.yml` including the forbidigo rule banning raw database calls
- [x] 1.5 WP-01 GitHub workflows: ci, codeql, dependency-review, release, release-please, scorecard, openspec-badge, cla, pr-title
- [x] 1.6 WP-01 `AGENTS.md`, `CLAUDE.md` referencing it, `ROADMAP.md` recording v2 seams
- [x] 1.7 WP-01 `LICENSE` (AGPL-3.0), `CLA.md`, `README.md`, `CONTRIBUTING.md`, `SECURITY.md`
- [x] 1.8 WP-02 `openspec/config.yaml` with project context and design rules
- [x] 1.9 WP-02 `proposal.md` and `design.md`
- [x] 1.10 WP-03 `internal/core`: domain types for every entity, stdlib imports only
- [x] 1.11 WP-03 `internal/core`: `Service` interface covering the complete product surface
- [x] 1.12 WP-03 `internal/core`: `TenantScope`, `Actor`, context carriers and accessors
- [x] 1.13 WP-03 `internal/core`: error taxonomy and its exit-code and HTTP-status mappings
- [x] 1.14 WP-03 `internal/core`: `TaskRef` parsing accepting identifiers and human refs
- [x] 1.15 WP-03 `internal/core`: filter and keyset cursor types
- [x] 1.16 WP-04 `internal/clock`: `Clock` interface, real and fake implementations
- [x] 1.17 WP-04 `internal/id`: sortable identifier generation from `crypto/rand`
- [x] 1.18 WP-05 `internal/store/store.go`: `Store` and `Tx` interfaces
- [x] 1.19 WP-05 `internal/store/sql/builder.go`: tenant-scoped query builder with no unscoped API
- [x] 1.20 WP-05 `internal/store/migrations/0001_init.sql`: full schema
- [ ] 1.21 WP-05 Migration seeding the builtin default workflow
- [ ] 1.22 WP-05 PostgreSQL partitioning and row-level security policies

## 2. Wave 2 — Independent foundations

- [x] 2.1 WP-10 `internal/config`: config types and `TIX_*` key-path mapping
- [x] 2.2 WP-10 `internal/config`: file location resolution and loading
- [x] 2.3 WP-10 `internal/config`: `.env` discovery by upward walk, never overriding the real environment
- [x] 2.4 WP-10 `internal/config`: per-directory context discovery, stopping at the git root
- [x] 2.5 WP-10 `internal/config`: five-layer precedence resolution into one struct
- [x] 2.6 WP-10 `internal/config`: source attribution for `config show --sources`, with secret redaction
- [x] 2.7 WP-11 `internal/output`: `Formatter` interface and table, JSON, YAML implementations
- [x] 2.8 WP-12 `internal/authz`: scope vocabulary and role composition
- [x] 2.9 WP-12 `internal/authz`: `Policy.Can` with tenant membership evaluation
- [x] 2.10 WP-13 `internal/auth`: argon2id hashing and verification
- [x] 2.11 WP-13 `internal/auth`: session mint, verify, expiry, revoke
- [x] 2.12 WP-13 `internal/auth`: API token mint, verify, scope and tenant binding, revoke
- [x] 2.13 WP-13 `internal/auth`: `Authenticator` interface and chain (the SSO seam)
- [x] 2.14 WP-14 `internal/store/sqlite`: driver open, WAL, busy timeout, writer and reader pools
- [x] 2.15 WP-14 `internal/store/sql`: shared scoped queries for every entity
- [x] 2.16 WP-14 `internal/store/sql`: keyset pagination helpers
- [x] 2.17 WP-14 Migration runner, forward-only, one transaction per file
- [x] 2.18 WP-14 Concurrent-writer test proving serialization rather than failure
- [x] 2.19 WP-14 Reflection test asserting every generated statement carries a tenant predicate

## 3. Wave 3 — Service layer

- [x] 3.1 WP-20 `internal/service/local.go`: `Local` struct and `core.Service` assertion
- [x] 3.2 WP-20 `internal/service/tx.go`: transaction helper used by every mutation
- [x] 3.3 WP-20 `internal/service/audit.go`: audit entry capture with secret exclusion
- [x] 3.4 WP-20 `internal/outbox`: in-transaction event append
- [x] 3.5 WP-20 `internal/outbox`: cursor-based `Tailer` with per-engine wake-up
- [x] 3.6 WP-21 `internal/service/tenant.go`: tenant CRUD and membership
- [x] 3.7 WP-21 `internal/service/domain.go`: domain registration and resolution
- [x] 3.8 WP-21 `internal/service/project.go`: project CRUD
- [x] 3.9 WP-21 `internal/service/workflow.go`: workflow definition, validation, transition legality
- [x] 3.10 WP-21 `internal/service/field.go`: field definitions and value validation
- [x] 3.11 WP-21 `internal/service/bootstrap.go`: `EnsureDefaults` creating the default tenant and project
- [x] 3.12 WP-21 Cross-tenant leak suite over every service method
- [x] 3.13 WP-22 `internal/service/task.go`: task CRUD, transitions, soft and hard delete
- [x] 3.14 WP-22 `internal/service/dep.go`: dependencies with cycle rejection and blocked reporting
- [x] 3.15 WP-22 `internal/service/tag.go`: tag attach and detach
- [x] 3.16 WP-22 `internal/service/comment.go`: comment create, edit, soft delete
- [x] 3.17 WP-22 `internal/service/artifact.go`: structured result artifacts
- [x] 3.18 WP-22 Task listing: filtering, sorting, keyset pagination, no OFFSET
- [x] 3.19 WP-23 `internal/lease`: claim compare-and-swap helper
- [x] 3.20 WP-23 `internal/lease`: `ClaimNext` with dependency gating and retry
- [x] 3.21 WP-23 `internal/lease`: renew, release, lease token verification
- [x] 3.22 WP-23 `internal/lease`: `Sweeper` materializing expiry
- [x] 3.23 WP-23 `internal/service/claim.go`: claim operations over the lease helpers
- [x] 3.24 WP-23 Concurrent `ClaimNext` test proving no task is claimed twice
- [x] 3.25 WP-24 `internal/retention`: per-tenant policy evaluation and pruning
- [x] 3.26 WP-24 `internal/retention`: subscriber-cursor safety check
- [x] 3.27 WP-24 `internal/service/prune.go`: prune operation with dry run

## 4. Wave 4 — Transports

- [x] 4.1 WP-30 `internal/httpapi/router.go`: route table over the standard library mux
- [x] 4.2 WP-30 `internal/httpapi/middleware.go`: host-to-tenant resolution before authentication
- [x] 4.3 WP-30 `internal/httpapi/middleware.go`: bearer and cookie authentication
- [x] 4.4 WP-30 `internal/httpapi`: handlers for every resource
- [x] 4.5 WP-30 `internal/httpapi`: error envelope mapping the service taxonomy to status codes
- [x] 4.6 WP-30 `internal/httpapi`: health and readiness endpoints
- [x] 4.7 WP-30 `internal/server`: lifecycle, TLS, graceful shutdown
- [x] 4.8 WP-30 `internal/server`: non-loopback bind guard requiring TLS or explicit opt-out
- [x] 4.9 WP-30 `internal/server`: sweeper, webhook dispatcher, and pruner workers
- [x] 4.10 WP-30 `cmd/serve.go`
- [x] 4.11 WP-31 `internal/httpapi/ws.go`: WebSocket protocol messages
- [x] 4.12 WP-31 `internal/httpapi/hub.go`: per-tenant fan-out and slow-consumer handling
- [x] 4.13 WP-31 Gap-free `since_seq` resume test
- [x] 4.14 WP-32 `internal/client`: remote `core.Service` implementation
- [x] 4.15 WP-32 `internal/client`: error reconstruction preserving codes
- [x] 4.16 WP-32 Transport-equivalence suite running every scenario against both implementations
- [x] 4.17 WP-33 `internal/webhook`: endpoint registry
- [x] 4.18 WP-33 `internal/webhook`: HMAC signing over timestamp and body
- [x] 4.19 WP-33 `internal/webhook`: delivery worker, retry schedule, delivery locking
- [x] 4.20 WP-33 `internal/webhook`: opportunistic inline drain for the command line
- [x] 4.21 WP-34 `internal/connect`: resolution order selecting local or remote
- [x] 4.22 WP-34 `cmd/root.go`: global flags, output selection, exit code mapping
- [x] 4.23 WP-34 `cmd/task*.go`, `cmd/dep.go`, `cmd/tag.go`, `cmd/comment.go`
- [x] 4.24 WP-34 `cmd/project.go`, `cmd/workflow.go`, `cmd/field.go`
- [x] 4.25 WP-34 `cmd/claim.go` including the exec wrapper
- [x] 4.26 WP-34 `cmd/tenant.go`, `cmd/domain.go`, `cmd/user.go`, `cmd/token.go`, `cmd/login.go`
- [x] 4.27 WP-34 `cmd/ctx.go`, `cmd/config.go`, `cmd/doctor.go`, `cmd/prune.go`, `cmd/webhook.go`
- [x] 4.28 WP-34 `cmd/completion.go` with dynamic completion of refs, projects, tags, statuses
- [x] 4.29 WP-34 `cmd/docs.go` generating the command tree as Markdown
- [x] 4.30 WP-34 Golden tests over the command tree covering output shapes and exit codes

## 5. Wave 5 — Surfaces

- [x] 5.1 WP-40 `internal/web`: templates, layout, and embedded assets with vendored htmx
- [x] 5.2 WP-40 `internal/web`: board, task list, task detail
- [x] 5.3 WP-40 `internal/web`: workflow and field definition editors
- [x] 5.4 WP-40 `internal/web`: tenant, domain, user, token administration
- [x] 5.5 WP-40 `internal/web`: webhook administration with delivery log and redelivery
- [x] 5.6 WP-40 `internal/web`: import, export, and sync screens
- [x] 5.7 WP-40 `internal/web`: live activity feed over the event stream
- [x] 5.8 WP-40 Forms verified to work with JavaScript disabled
- [x] 5.9 WP-41 `internal/tui`: program, model, and message plumbing
- [x] 5.10 WP-41 `internal/tui`: project picker, board, task detail, filter bar
- [x] 5.11 WP-41 `internal/tui`: live updates preserving selection
- [x] 5.12 WP-41 `cmd/tui.go` and terminal restore on interrupt
- [x] 5.13 WP-42 `internal/store/postgres`: driver, dialect handling, placeholder rewriting
- [x] 5.14 WP-42 `internal/store/postgres`: LISTEN/NOTIFY tailer wake-up
- [x] 5.15 WP-42 `internal/store/postgres`: row-level security wiring per transaction
- [x] 5.16 WP-42 `internal/store/postgres`: tsvector search and partition management
- [x] 5.17 WP-43 `internal/transfer`: deterministic snapshot export excluding secrets
- [x] 5.18 WP-43 `internal/transfer`: import with merge and replace modes and identifier remapping
- [x] 5.19 WP-43 `cmd/export.go`, `cmd/import.go`
- [x] 5.20 WP-44 `internal/sync`: `Importer` interface and mapping file format
- [x] 5.21 WP-44 `internal/sync/generic`: CSV and JSON adapter
- [x] 5.22 WP-44 `internal/sync/jira`: Jira adapter with pagination and backoff
- [x] 5.23 WP-44 `internal/sync/openproject`: OpenProject adapter
- [x] 5.24 WP-44 External reference recording making re-import idempotent
- [x] 5.25 WP-44 Per-source cursors advancing only on success
- [x] 5.26 WP-44 `cmd/sync.go` with dry run reporting creates, updates, and skips
- [x] 5.27 WP-44 Adapter tests against recorded HTTP fixtures

## 6. Wave 6 — Component sharing

- [x] 6.1 WP-45 `internal/bundle`: bundle schema, versioning, deterministic encoding
- [x] 6.2 WP-45 Export selection per component kind, excluding work items and secrets
- [x] 6.3 WP-45 Validation of a whole bundle before any write
- [x] 6.4 WP-45 Collision policy: skip, rename, replace, with no silent overwrite
- [x] 6.5 WP-45 Preview reporting the plan without writing or auditing
- [x] 6.6 WP-45 Atomic import, audited per component, attributed to the bundle
- [x] 6.7 WP-45 `core.Service` methods and authorization for both directions
- [x] 6.8 WP-45 CLI `tix bundle export|import` streaming stdout and stdin
- [x] 6.9 WP-45 HTTP routes and the web UI screens, keeping capability parity

## 7. Wave 7 — Closing

- [x] 7.1 WP-50 `internal/capability`: operation registry covering every Service method
- [x] 7.2 WP-50 Parity test: every Service method appears in the registry
- [x] 7.3 WP-50 Parity test: every operation has CLI, HTTP, and Web bindings, or a justified exemption
- [x] 7.4 WP-50 Parity test: every binding resolves against the real mux, command tree, and templates
- [x] 7.5 WP-51 `docs/`: agents, workflows, configuration, tenancy, api, scripting, deployment, scaling, migrating, shell-completion
- [x] 7.6 WP-51 `docs/README.md` index and the link check
- [x] 7.7 WP-51 README command table generated by `tix docs`
- [x] 7.8 WP-52 `internal/bench`: seeded fixture and benchmarks with p95 budgets
- [x] 7.9 WP-52 Cross-transport integration test: direct-database write observed over WebSocket and webhook
- [ ] 7.10 WP-52 Coverage at or above 80% and the full gate set green
- [ ] 7.11 WP-52 Repository made public and v0.1.0 tagged
