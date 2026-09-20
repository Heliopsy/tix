# tix — task management for humans and AI agents
#
# `just check` runs the fast pre-push gate set.
# `just ci`    runs the entire CI suite locally in containers, matching GitHub.

module  := "github.com/thereisnotime/tix"
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

# Packages excluded from the coverage threshold.
# internal/tui is bubbletea View() rendering: string-producing, low-value to unit test.
# Its logic lives in pure functions that ARE covered; see AGENTS.md.
cov_exclude := "internal/tui"

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
    rm -rf bin/ dist/ coverage.out coverage.html

test:
    go test ./... -race -shuffle=on

# Skip container-backed and long-running tests.
test-short:
    go test ./... -short -race -shuffle=on

cover:
    go test ./... -race -shuffle=on -coverprofile=coverage.out -covermode=atomic
    @go tool cover -func=coverage.out | tail -1
    go tool cover -html=coverage.out -o coverage.html
    @echo "report: coverage.html"

# Enforce the coverage threshold, excluding {{cov_exclude}}.
cover-check: cover
    #!/usr/bin/env bash
    set -euo pipefail
    grep -v '{{cov_exclude}}' coverage.out > coverage.filtered.out
    total=$(go tool cover -func=coverage.filtered.out | grep '^total:' | awk '{print $3}' | tr -d '%')
    echo "coverage (excluding {{cov_exclude}}): ${total}%"
    awk "BEGIN {exit !(${total} < 80)}" && { echo "below the 80% threshold"; exit 1; } || true

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

# ---------------------------------------------------------------- gates (containerized)

# Run an arbitrary command inside the pinned CI toolbox image.
tool +ARGS: _ensure-ci-image
    # -buildvcs=false: the repository is bind-mounted, so git inside the
    # container sees an ownership mismatch and the VCS stamp fails.
    {{engine}} run --rm -v "$PWD":/src:z -w /src -e GOFLAGS=-buildvcs=false {{ci_image}} {{ARGS}}

lint: (tool "golangci-lint" "run" "./...")
actionlint: (tool "actionlint")
hadolint: (tool "hadolint" "Containerfile" "Containerfile.ci")
yamllint: (tool "yamllint" ".")
# Two passes: specs are prose and exempt from line length, everything else is not.
mdlint: (tool "markdownlint" "--ignore" "node_modules" "--ignore" "openspec" ".") \
        (tool "markdownlint" "--config" ".markdownlint-specs.yaml" "openspec")
sec: (tool "gosec" "./...")
vuln: (tool "govulncheck" "./...")
trivy: (tool "trivy" "fs" "--severity" "CRITICAL,HIGH" "--exit-code" "1" "--ignorefile" ".trivyignore" "--scanners" "vuln,secret" ".")
trivy-sarif: (tool "trivy" "fs" "--format" "sarif" "--output" "trivy.sarif" "--severity" "CRITICAL,HIGH" "--ignorefile" ".trivyignore" ".")
spec: (tool "openspec" "validate" "tix-v1" "--strict")

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

# Cross-tenant isolation suite.
leak:
    go test ./... -run 'TestTenantIsolation|TestLeak' -count=1

# ---------------------------------------------------------------- postgres

pg-up:
    #!/usr/bin/env bash
    set -euo pipefail
    if {{engine}} inspect {{pg_name}} >/dev/null 2>&1; then
      echo "{{pg_name}} already exists"; exit 0
    fi
    {{engine}} run -d --name {{pg_name}} \
      -e POSTGRES_USER=tix -e POSTGRES_PASSWORD=tix -e POSTGRES_DB=tix \
      -p 55432:5432 docker.io/library/postgres:18-alpine
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
    TIX_TEST_POSTGRES_DSN='{{pg_dsn}}' go test ./... -race -shuffle=on

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
check: fmt-check vet tidy-check lint mdlint yamllint actionlint test spec docs-check

# The entire CI suite, locally, in containers. Matches what GitHub runs.
ci: tidy-check fmt-check vet lint build test-postgres cover-check sec vuln trivy actionlint hadolint yamllint mdlint spec docs-check release-dry
    @echo ""
    @echo "all CI gates passed"

# Run a single named CI job in isolation, e.g. `just ci-job sec`.
ci-job JOB:
    just {{JOB}}
