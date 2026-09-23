# tix — task management for humans and AI agents
#
# `just check` runs the fast pre-push gate set.
# `just ci`    runs the entire CI suite locally in containers, matching GitHub.

module  := "github.com/heliopsy/tix"
binary  := "tix"
version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
commit  := `git rev-parse --short HEAD 2>/dev/null || echo "none"`
date    := `date -u +%Y-%m-%dT%H:%M:%SZ`
ldflags := "-s -w -X " + module + "/internal/version.Version=" + version + " -X " + module + "/internal/version.Commit=" + commit + " -X " + module + "/internal/version.Date=" + date

# Container engine: podman by default, override with CONTAINER_ENGINE=docker
engine   := env("CONTAINER_ENGINE", "podman")
ci_image := "localhost/tix-ci:latest"
pg_name  := "tix-test-pg"
pg_dsn   := "postgres://tix:tix@127.0.0.1:55432/tix?sslmode=disable"

# Coverage floors. Two of them, because a run without PostgreSQL proves less.
#
# cov_min applies when both engines ran: that is what CI does, and the only
# number worth defending. cov_min_partial applies when the PostgreSQL suite
# skipped, which costs about seven points, so working without a database stays
# possible without the gate becoming either a lie or a wall. Both floors sit
# about three points under what the suite measures today. Every run prints
# which floor applied and how much headroom is left, so a slow decline shows up
# well before it turns CI red.
#
# There is no excluded package. internal/tui used to be excluded as untestable
# View() rendering; it is now covered like everything else, so the exclusion
# only hid a covered package from the total. Its current number comes from the
# coverage profile like any other package, so no figure is frozen here.
cov_min         := "85"
cov_min_partial := "78"

# Where a test run leaves its machine-readable record of what the environment
# did not allow. It is a file rather than the test output because `go test`
# prints nothing at all from a package whose tests pass, so a skip that stays
# on stdout is a skip nobody sees.
env_log := "coverage-env.log"

default:
    @just --list

# ---------------------------------------------------------------- dev loop

build:
    mkdir -p bin
    go build -ldflags '{{ldflags}}' -o bin/{{binary}} .

build-all:
    mkdir -p bin
    GOOS=linux  GOARCH=amd64 go build -ldflags '{{ldflags}}' -o bin/{{binary}}-linux-amd64 .
    GOOS=linux  GOARCH=arm64 go build -ldflags '{{ldflags}}' -o bin/{{binary}}-linux-arm64 .
    GOOS=darwin GOARCH=amd64 go build -ldflags '{{ldflags}}' -o bin/{{binary}}-darwin-amd64 .
    GOOS=darwin GOARCH=arm64 go build -ldflags '{{ldflags}}' -o bin/{{binary}}-darwin-arm64 .

install:
    go install -ldflags '{{ldflags}}' .

# Install to ~/.local/bin, which is already on PATH.
install-local: build
    mkdir -p ~/.local/bin
    install -m 0755 bin/{{binary}} ~/.local/bin/{{binary}}
    @echo "installed $(~/.local/bin/{{binary}} version)"

run *ARGS:
    go run -ldflags '{{ldflags}}' . {{ARGS}}

clean:
    rm -rf bin/ dist/ coverage.out coverage.filtered.out coverage.html {{env_log}}

test:
    #!/usr/bin/env bash
    set -euo pipefail
    export TIX_TEST_ENV_LOG=$(mktemp); trap 'rm -f "$TIX_TEST_ENV_LOG"' EXIT
    set +e
    go test ./... -race -shuffle=on
    rc=$?
    set -e
    just _env-summary "$TIX_TEST_ENV_LOG"
    exit $rc

# Skip container-backed and long-running tests.
test-short:
    #!/usr/bin/env bash
    set -euo pipefail
    export TIX_TEST_ENV_LOG=$(mktemp); trap 'rm -f "$TIX_TEST_ENV_LOG"' EXIT
    set +e
    go test ./... -short -race -shuffle=on
    rc=$?
    set -e
    just _env-summary "$TIX_TEST_ENV_LOG"
    exit $rc

# One block for the whole run naming every optional capability it could not
# use. Each test binary prints one marked line per gap; this collects them, so
# seven scattered skips become one thing a person actually reads.
_env-summary LOG:
    #!/usr/bin/env bash
    set -euo pipefail
    gaps=$(grep -h '^tix-test-env: SKIPPED | ' "{{LOG}}" 2>/dev/null | sed 's/^tix-test-env: SKIPPED | //' | sort -u || true)
    echo ""
    if [ -z "$gaps" ]; then
      echo "test environment: complete. Every optional capability was available."
      exit 0
    fi
    echo "======================================================================"
    echo "  PARTIAL RUN: this green does not cover everything"
    echo "======================================================================"
    while IFS='|' read -r name why how; do
      [ -n "$name" ] || continue
      printf '  %s: %s\n' "$(echo $name)" "$(echo $why)"
      printf '      enable with: %s\n' "$(echo $how)"
    done <<< "$gaps"
    echo "======================================================================"

# Coverage mirrors CI, which runs a postgres service, so the engine's tests are
# included rather than skipped. Falls back to sqlite-only when nothing is up.
cover:
    #!/usr/bin/env bash
    set -euo pipefail
    # An already-exported DSN wins: CI sets one and starts no container here.
    if [ -z "${TIX_TEST_POSTGRES_DSN:-}" ] && {{engine}} exec {{pg_name}} pg_isready -U tix -q 2>/dev/null; then
      export TIX_TEST_POSTGRES_DSN='{{pg_dsn}}'
    fi
    if [ -n "${TIX_TEST_POSTGRES_DSN:-}" ]; then
      echo "postgres: included"
    else
      echo "postgres: NOT available, its tests will skip (run: just pg-up)"
    fi
    rm -f {{env_log}}
    # Absolute: a test binary runs with its own package directory as cwd.
    export TIX_TEST_ENV_LOG="$PWD/{{env_log}}"
    set +e
    go test ./... -race -shuffle=on -coverprofile=coverage.out -covermode=atomic
    rc=$?
    set -e
    [ $rc -eq 0 ] || exit $rc
    go tool cover -func=coverage.out | tail -1
    go tool cover -html=coverage.out -o coverage.html
    echo "report: coverage.html"
    just _env-summary {{env_log}}

# List the packages nearest the bottom too, so a decline is visible while it is
# still small rather than on the day it turns CI red.
# Enforce the coverage floor and name the headroom left above it.
cover-check: cover
    #!/usr/bin/env bash
    set -euo pipefail
    total=$(go tool cover -func=coverage.out | awk '/^total:/ {print $3}' | tr -d '%')
    if grep -q '^tix-test-env: SKIPPED | postgres |' {{env_log}} 2>/dev/null; then
      floor='{{cov_min_partial}}'
      echo "coverage: ${total}% over SQLite alone; the postgres suite skipped"
      echo "the full floor of {{cov_min}}% needs both engines: just pg-up && just cover-check"
    else
      floor='{{cov_min}}'
      echo "coverage: ${total}% over both engines"
    fi
    echo "lowest packages:"
    awk 'NR>1 {split($1,a,":"); path=a[1]; sub(/\/[^\/]+$/,"",path);
         total[path]+=$2; if ($3>0) hit[path]+=$2}
         END {for (p in total) printf "  %5.1f%%  %s\n", 100*hit[p]/total[p], p}' coverage.out \
      | sort -n | head -5
    headroom=$(awk "BEGIN {printf \"%.1f\", ${total} - ${floor}}")
    echo "floor: ${floor}%, headroom: ${headroom} points"
    awk "BEGIN {exit !(${total} < ${floor})}" && { echo "FAIL: below the ${floor}% floor"; exit 1; } || true
    awk "BEGIN {exit !(${headroom} < 1.5)}" && echo "WARNING: under 1.5 points of headroom, raise coverage before it turns CI red" || true

# Smoke: build the real binary and drive it as a subprocess. It is a sanity
# pass over the shipped artifact, not a second test suite, so it covers the
# zero-configuration path, what the listings actually render, serve, and the
# SSH surface through a real client. Every bug that reached a person recently
# was found by running a binary rather than by a test.
#
# -count=1 because a cached pass proves nothing about a binary just rebuilt.
smoke:
    #!/usr/bin/env bash
    set -euo pipefail
    export TIX_TEST_ENV_LOG=$(mktemp); trap 'rm -f "$TIX_TEST_ENV_LOG"' EXIT
    set +e
    go test -tags smoke ./internal/smoke/ -count=1 -timeout 10m -v
    rc=$?
    set -e
    just _env-summary "$TIX_TEST_ENV_LOG"
    exit $rc

bench:
    go test ./internal/bench/... -bench=. -benchmem -run '^$'

# ---------------------------------------------------------------- gates (host)

fmt:
    gofmt -s -w .

fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    files=$(gofmt -s -l .)
    if [ -n "$files" ]; then echo "not gofmt'd:"; echo "$files"; exit 1; fi

vet:
    go vet ./...

tidy-check:
    go mod tidy
    git diff --exit-code go.mod go.sum

# ---------------------------------------------------------------- gates

# Run a gate tool, in the pinned toolbox image when that is possible.
#
# Two environments have to agree on tool versions and they get there by
# different routes. A developer has a container engine, so the toolbox image is
# the surer answer: it pins every linter and scanner at once and needs nothing
# installed on the host. The CI runner has no engine at all, so it falls back to
# whatever .tool-versions has put on PATH.
#
# The fallback is announced rather than silent. Running a different set of tools
# than you think you are running is how a gate stops meaning anything, and a
# quiet fallback is exactly the shape of the four silent skips this repository
# has already been bitten by.
#
# TIX_TOOL_MODE forces the choice: "container" fails rather than falling back,
# "direct" skips the engine entirely.
tool +ARGS:
    #!/usr/bin/env bash
    set -euo pipefail
    mode="${TIX_TOOL_MODE:-auto}"
    if [ "$mode" != "direct" ] && command -v {{engine}} >/dev/null 2>&1 && {{engine}} info >/dev/null 2>&1; then
      just _ensure-ci-image
      # -buildvcs=false: the repository is bind-mounted, so git inside the
      # container sees an ownership mismatch and the VCS stamp fails.
      exec {{engine}} run --rm -v "$PWD":/src:z -w /src -e GOFLAGS=-buildvcs=false {{ci_image}} {{ARGS}}
    fi
    if [ "$mode" = "container" ]; then
      echo "TIX_TOOL_MODE=container but {{engine}} is unavailable" >&2
      exit 1
    fi
    if [ "$mode" != "direct" ]; then
      echo "note: no container engine, running {{ARGS}} from PATH at whatever version is installed" >&2
    fi
    exec {{ARGS}}

lint: (tool "golangci-lint" "run" "./...")
actionlint: (tool "actionlint")
hadolint: (tool "hadolint" "Containerfile" "Containerfile.ci")
yamllint: (tool "yamllint" ".")
# Two passes: specs are prose and exempt from line length, everything else is not.
mdlint: (tool "markdownlint" "--ignore" "node_modules" "--ignore" "openspec" ".") \
        (tool "markdownlint" "--config" ".markdownlint-specs.yaml" "openspec")
# A glob, not a directory: node resolves a directory argument as a module. The
# CI image already carries node for markdownlint and openspec, so a developer
# without node installed still gets the gate through the container.
# The web assets' JavaScript suite, on node's own runner: no npm, no build step.
jstest: (tool "node" "--test" "--test-reporter=tap" "internal/web/jstest/*.test.mjs")
sec: (tool "gosec" "./...")
vuln: (tool "govulncheck" "./...")
trivy: (tool "trivy" "fs" "--severity" "CRITICAL,HIGH" "--exit-code" "1" "--ignorefile" ".trivyignore" "--scanners" "vuln,secret" ".")
trivy-sarif: (tool "trivy" "fs" "--format" "sarif" "--output" "trivy.sarif" "--severity" "CRITICAL,HIGH" "--ignorefile" ".trivyignore" ".")
# OPENSPEC_TELEMETRY=0 because the toolbox image and the CI job both set it, and
# the PATH fallback did not. A contributor with no container engine was sending
# anonymous usage data to a third party by following the documented workflow,
# without being told. It is disabled everywhere now, not only where somebody
# happened to remember.
spec:
    OPENSPEC_TELEMETRY=0 just tool openspec validate tix-v1 --strict

# Every file in docs/ must be linked from docs/README.md.
docs-check:
    #!/usr/bin/env bash
    set -euo pipefail
    [ -d docs ] || { echo "no docs/ yet, skipping"; exit 0; }
    rc=0
    for f in docs/*.md; do
      base=$(basename "$f")
      [ "$base" = "README.md" ] && continue
      grep -q "$base" docs/README.md || { echo "docs/README.md does not link $base"; rc=1; }
    done
    exit $rc

# Capability registry parity: every operation reachable from CLI, HTTP and web.
parity:
    go test ./internal/capability/... -run TestParity -count=1

# Cross-tenant isolation suite: the scoped builder on both engines, and the
# PostgreSQL row-level security policies. Layer 4 only exists on PostgreSQL, so
# the recipe starts one and passes the DSN rather than skipping it in silence.
# -v is deliberate: a suite that proves isolation has to show what it ran.
leak: pg-up
    TIX_TEST_POSTGRES_DSN='{{pg_dsn}}' go test ./... -count=1 -v \
      -run 'TestTenantIsolation|TestRowLevelSecurity|TestPoliciesExistForEveryScopedTable|TestPruningStaysInsideTheTenant|TestTenantSettingDoesNotLeakOntoTheNextTransaction|TestProjectAppearanceStaysInsideTheTenant'

# ---------------------------------------------------------------- postgres

pg-up:
    #!/usr/bin/env bash
    set -euo pipefail
    # An existing container is not a running one: a stopped {{pg_name}} used to
    # count as success here, and the suite then failed to connect rather than
    # skipping, which reads as broken code instead of a stopped database.
    if {{engine}} inspect {{pg_name}} >/dev/null 2>&1; then
      {{engine}} start {{pg_name}} >/dev/null
    else
      {{engine}} run -d --name {{pg_name}} \
        -e POSTGRES_USER=tix -e POSTGRES_PASSWORD=tix -e POSTGRES_DB=tix \
        -p 55432:5432 docker.io/library/postgres:18-alpine
    fi
    echo "waiting for postgres..."
    for i in $(seq 1 60); do
      {{engine}} exec {{pg_name}} pg_isready -U tix -q && { echo "ready"; exit 0; }
      sleep 1
    done
    echo "postgres did not become ready in time"; exit 1

pg-down:
    -{{engine}} rm -f {{pg_name}}

pg-psql:
    {{engine}} exec -it {{pg_name}} psql -U tix -d tix

# Run the suite against postgres as well as sqlite.
test-postgres: pg-up
    #!/usr/bin/env bash
    set -euo pipefail
    export TIX_TEST_ENV_LOG=$(mktemp); trap 'rm -f "$TIX_TEST_ENV_LOG"' EXIT
    set +e
    TIX_TEST_POSTGRES_DSN='{{pg_dsn}}' go test ./... -race -shuffle=on
    rc=$?
    set -e
    just _env-summary "$TIX_TEST_ENV_LOG"
    exit $rc

# ---------------------------------------------------------------- images

image:
    {{engine}} build -f Containerfile -t localhost/tix:latest --build-arg GO_VERSION=$(go mod edit -json | python3 -c 'import sys,json;print(json.load(sys.stdin)["Go"])') .

image-ci:
    {{engine}} build -f Containerfile.ci -t {{ci_image}} .

_ensure-ci-image:
    #!/usr/bin/env bash
    set -euo pipefail
    {{engine}} image exists {{ci_image}} 2>/dev/null || {{engine}} inspect {{ci_image}} >/dev/null 2>&1 || just image-ci

release-dry: (tool "goreleaser" "build" "--snapshot" "--clean")

# ---------------------------------------------------------------- aggregates

# Fast pre-push gate set. Run this before every commit.
check: fmt-check vet tidy-check lint mdlint yamllint actionlint test smoke jstest spec docs-check

# The entire CI suite, locally, in containers. Matches what GitHub runs.
ci: tidy-check fmt-check vet lint build test-postgres cover-check smoke jstest sec vuln trivy actionlint hadolint yamllint mdlint spec docs-check release-dry
    @echo ""
    @echo "all CI gates passed"

# Run a single named CI job in isolation, e.g. `just ci-job sec`.
ci-job JOB:
    just {{JOB}}

# ---------------------------------------------------------------- bench

# Latency budgets at the default small fixture, including postgres when it is up.
bench-budgets:
    #!/usr/bin/env bash
    set -euo pipefail
    if {{engine}} exec {{pg_name}} pg_isready -U tix -q 2>/dev/null; then
      export TIX_TEST_POSTGRES_DSN='{{pg_dsn}}'
      echo "postgres: included"
    else
      echo "postgres: not running, its run will skip (run: just pg-up)"
    fi
    go test ./internal/bench/ -v -run 'Test' -count=1

# The design target: a million tasks over fifty projects and five tenants.
# Seeding takes minutes; see internal/bench/README.md before running it.
bench-full ENGINE="postgres": pg-up
    TIX_BENCH_SIZE=full TIX_BENCH_ENGINE={{ENGINE}} TIX_TEST_POSTGRES_DSN='{{pg_dsn}}' \
      go test ./internal/bench/ -v -run 'Test' -count=1 -timeout 3h
