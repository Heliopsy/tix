# Contributing to tix

## Getting started

```sh
git clone git@github.com:thereisnotime/tix.git
cd tix
go mod download
just build   # outputs bin/tix
just test    # tests with the race detector
```

Requirements: [Go](https://go.dev/dl/) at the version in `go.mod`,
[just](https://just.systems/), [podman](https://podman.io/) (or docker, via
`CONTAINER_ENGINE=docker`), and [pre-commit](https://pre-commit.com/#install).

After cloning, install the git hooks:

```sh
pre-commit install --hook-type commit-msg
```

## Before you push

```sh
just check
```

That runs formatting, vet, tidy, lint, tests, spec validation, and the docs index check.
To reproduce the full CI suite locally, including PostgreSQL and every scanner:

```sh
just ci
```

Linters and scanners run inside a pinned toolbox image, so local results match CI rather
than approximating it. The first run builds that image.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org/), enforced by a commit-msg hook:

```
<type>[optional scope]: <description>

Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
```

Types map to semver: `fix:` is a patch, `feat:` a minor, `feat!:` or `BREAKING CHANGE:` a
major. Release notes and version bumps are generated from them, so the type matters.

Do not add `Co-Authored-By` trailers.

## Specs come first

OpenSpec is the normative contract. A change to behaviour updates `openspec/` before the
code:

```sh
openspec validate tix-v1 --strict
```

Requirements use SHALL and every requirement carries at least one scenario. Tick tasks in
`tasks.md` as soon as the code ships.

## Making changes

1. Branch from `main`.
2. Make the change. New behaviour ships with its test in the same commit.
3. `just check` passes.
4. Open a pull request against `main` and fill in the template.

Pull requests need one approving review and green CI. The PR title must be a conventional
commit, because that is what release-please reads.

## Architecture rules

These are enforced by lint, by tests, or by review. They are listed in full in
[AGENTS.md](AGENTS.md); the ones most often hit:

- `cmd/` holds flags and wiring only. No package under `internal/` imports Cobra.
- `internal/core` imports only the standard library.
- Business rules and authorization live only in `internal/service`.
- Every query is built by the tenant-scoped builder. Raw database calls are a lint error.
- Every mutation writes its rows, audit entry, and event in one transaction.
- List queries are keyset-paginated. `OFFSET` is never used.
- Builds stay `CGO_ENABLED=0`.

## Contributor licence agreement

tix is AGPL-3.0. Contributions are accepted under the [CLA](CLA.md) so the project can be
offered under other terms in future. A bot comments on your first pull request with a
one-click signing link.

## Tests

Tests live next to the code (`*_test.go`), are table-driven, use a real temporary SQLite
database rather than mocks, and use `clock.FakeClock` for anything time-dependent.
Coverage must stay at or above 80%, excluding `internal/tui`.
