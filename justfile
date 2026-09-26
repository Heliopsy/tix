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
# Tagged by the content of the recipe that builds it, so editing Containerfile.ci
# produces a tag that does not exist yet and _ensure-ci-image rebuilds. Under a
# fixed tag it did not: a new tool added to the image was simply never there,
# and the gate that needed it either fell back to whatever was on PATH or failed
# for a reason that pointed nowhere near the actual cause.
ci_tag   := shell('sha256sum Containerfile.ci | cut -c1-12')
ci_image := "localhost/tix-ci:" + ci_tag
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
#
# CHANGELOG.md is exempt from both because release-please writes it. It puts
# two blank lines before each version and a breaking-change note runs to
# whatever length the commit footer was, so the file arrives failing and the
# only way to make it pass is to edit a file that is regenerated on the next
# release. Linting generated output teaches people to ignore the linter.
mdlint: (tool "markdownlint" "--ignore" "node_modules" "--ignore" "openspec" "--ignore" "CHANGELOG.md" ".") \
        (tool "markdownlint" "--config" ".markdownlint-specs.yaml" "openspec")
# A glob, not a directory: node resolves a directory argument as a module. The
# CI image already carries node for markdownlint and openspec, so a developer
# without node installed still gets the gate through the container.
# The web assets' JavaScript suite, on node's own runner: no npm, no build step.
jstest: (tool "node" "--test" "--test-reporter=tap" "internal/web/jstest/*.test.mjs")
sec: (tool "gosec" "./...")
vuln: (tool "govulncheck" "./...")
# Secret scan over the whole history, not just the current tree. A credential
# that was committed and later removed is still published the moment the
# repository is, so the working tree alone proves nothing. --all reaches every
# ref; --full-history is omitted because it only changes anything under path
# filtering, which this does not use.
#
# The tree itself is deliberately not scanned. .env is gitignored and is meant
# to hold real tokens locally, and failing a gate on a file that will never be
# published would train people to ignore the gate. What is committed is what
# matters here.
#
# .gitleaks.toml keeps the default ruleset and adds only allowlist entries, each
# carrying the argument for why the thing it exempts is not a secret.
secrets:
    #!/usr/bin/env bash
    set -euo pipefail
    # A shallow clone has one commit, so the scan would pass in under a second
    # having looked at almost nothing, and report success. Refuse instead: a
    # history gate that cannot see the history is worse than no gate, because
    # it produces a green tick nobody re-examines.
    if [ "$(git rev-parse --is-shallow-repository)" = "true" ]; then
      echo "refusing to scan a shallow clone: this gate reads the whole history." >&2
      echo "in CI, give the checkout step fetch-depth: 0." >&2
      exit 1
    fi
    just tool gitleaks git --log-opts=--all --config=.gitleaks.toml --redact .

trivy: (tool "trivy" "fs" "--severity" "CRITICAL,HIGH" "--exit-code" "1" "--ignorefile" ".trivyignore" "--scanners" "vuln,secret" ".")
trivy-sarif: (tool "trivy" "fs" "--format" "sarif" "--output" "trivy.sarif" "--severity" "CRITICAL,HIGH" "--ignorefile" ".trivyignore" ".")
# OPENSPEC_TELEMETRY=0 because the toolbox image and the CI job both set it, and
# the PATH fallback did not. A contributor with no container engine was sending
# anonymous usage data to a third party by following the documented workflow,
# without being told. It is disabled everywhere now, not only where somebody
# happened to remember.
spec:
    #!/usr/bin/env bash
    set -euo pipefail
    # Every change under openspec/changes/, not just tix-v1. The gate named one
    # change by hand, so v0-5-0-polish collected a release worth of deltas that
    # nothing validated: the recipe was watching the change that was already
    # finished rather than the one being written.
    for dir in openspec/changes/*/; do
        name=$(basename "$dir")
        [ "$name" = "archive" ] && continue
        echo "validating $name"
        OPENSPEC_TELEMETRY=0 just tool openspec validate "$name" --strict
    done

# Every file in docs/ must be linked from docs/README.md, and every screenshot
# from screenshots/README.md. Both indexes have drifted from their own
# directory before: a page nobody links is a page nobody reads, and an image
# nobody lists is one nobody knows to regenerate when it goes stale.
docs-check:
    #!/usr/bin/env bash
    set -euo pipefail
    rc=0
    if [ -d docs ]; then
      for f in docs/*.md; do
        base=$(basename "$f")
        [ "$base" = "README.md" ] && continue
        grep -q "$base" docs/README.md || { echo "docs/README.md does not link $base"; rc=1; }
      done
    fi
    if [ -d screenshots ]; then
      for f in screenshots/*.png; do
        [ -e "$f" ] || continue
        base=$(basename "$f")
        grep -q "$base" screenshots/README.md || { echo "screenshots/README.md does not list $base"; rc=1; }
      done
    fi
    exit $rc

# Capability registry parity: every operation reachable from CLI, HTTP and web.
#
# The whole package, not `-run TestParity`. No test here has ever been named
# that, so the filter matched nothing and the recipe reported "ok ... [no tests
# to run]" and passed. A gate that watches nothing is worse than no gate,
# because its green tick is read as evidence.
#
# The count check is the other half. Running the package would silently go
# vacuous again if the tests were renamed or moved, so the recipe asserts it
# actually ran them.
parity:
    #!/usr/bin/env bash
    set -euo pipefail
    out=$(go test ./internal/capability/... -count=1 -v 2>&1)
    echo "$out" | grep -E '^(ok|FAIL|---)' || true
    ran=$(echo "$out" | grep -c '^=== RUN   Test' || true)
    if [ "$ran" -lt 10 ]; then
      echo "parity ran $ran tests, expected at least 10: the suite has moved or been renamed" >&2
      exit 1
    fi
    echo "parity: $ran tests"

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

# Every image recipe builds and inspects a localhost tag, never the registry
# name. A local tag that already read ghcr.io/... would be one `push` away from
# publishing a dirty working tree over a release.
image_local  := "localhost/tix"
image_latest := image_local + ":latest"
image_amd64  := image_local + ":latest-amd64"
image_arm64  := image_local + ":latest-arm64"
image_list   := image_local + ":multiarch"
# Not 8080. That is the port a developer is most likely to already have
# something on, and the failure then looks like a broken image.
image_port   := "18080"
image_ctr    := "tix-image-verify"
image_vol    := "tix-image-verify-data"
image_tar    := "tix-image.tar"

# One build path for every image this project produces, so the tag a person
# inspects locally and the tag the release workflow pushes cannot be assembled
# from different arguments. An empty PLATFORM means the host's.
_image-build PLATFORM TAG:
    #!/usr/bin/env bash
    set -euo pipefail
    # Read from go.mod rather than written here, so the runtime image's toolchain
    # cannot drift from the one the tests ran under.
    go_version=$(go mod edit -json | python3 -c 'import sys,json;print(json.load(sys.stdin)["Go"])')
    args=(build -f Containerfile
          --build-arg "GO_VERSION=$go_version"
          --build-arg "VERSION={{version}}"
          --build-arg "COMMIT={{commit}}"
          --build-arg "DATE={{date}}"
          -t "{{TAG}}")
    if [ -n "{{PLATFORM}}" ]; then
      args+=(--platform "{{PLATFORM}}")
    fi
    {{engine}} "${args[@]}" .

# The runtime image for this host.
image: (_image-build "" image_latest)

# Both published architectures, and a manifest list over them.
#
# No QEMU and no registry: the builder stage runs on the host architecture and
# Go cross-compiles to TARGETARCH, and the list is assembled from local storage.
# It is assembled here rather than only while publishing because a step that
# first runs during a release is a step nobody has ever run.
image-multiarch: (_image-build "linux/amd64" image_amd64) (_image-build "linux/arm64" image_arm64)
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "{{engine}}" != "podman" ]; then
      echo "image-multiarch assembles the list with 'podman manifest'. docker cannot do" >&2
      echo "that from local images: 'docker manifest create' resolves names in a registry." >&2
      echo "run this with the default engine, or unset CONTAINER_ENGINE." >&2
      exit 1
    fi

    # The architecture of the binary inside each image, read from the ELF header
    # rather than from the image's own metadata. The metadata comes straight from
    # --platform, so it says arm64 whether or not the cross-compile happened: an
    # image built with TARGETARCH lost carries an amd64 binary and still labels
    # itself arm64, and the manifest list below would still look correct.
    probe=$(mktemp -d); trap 'rm -rf "$probe"' EXIT
    for spec in "{{image_amd64}}=62" "{{image_arm64}}=183"; do
      tag=${spec%=*}; want=${spec#*=}
      cid=$(podman create "$tag")
      podman cp "$cid:/usr/local/bin/tix" "$probe/tix"
      podman rm "$cid" >/dev/null
      got=$(python3 -c 'import sys,struct;f=open(sys.argv[1],"rb").read(20);print(struct.unpack_from("<H",f,18)[0])' "$probe/tix")
      if [ "$got" != "$want" ]; then
        echo "$tag holds an ELF e_machine $got binary, expected $want" >&2
        exit 1
      fi
      echo "$tag: e_machine $got"
      rm -f "$probe/tix"
    done

    podman manifest rm {{image_list}} >/dev/null 2>&1 || true
    podman manifest create {{image_list}} >/dev/null
    podman manifest add {{image_list}} containers-storage:{{image_amd64}} >/dev/null
    podman manifest add {{image_list}} containers-storage:{{image_arm64}} >/dev/null
    got=$(podman manifest inspect {{image_list}} | python3 -c \
      'import sys,json;print(" ".join(sorted(m["platform"]["os"]+"/"+m["platform"]["architecture"] for m in json.load(sys.stdin)["manifests"])))')
    if [ "$got" != "linux/amd64 linux/arm64" ]; then
      echo "manifest list holds [$got], expected [linux/amd64 linux/arm64]" >&2
      exit 1
    fi
    echo "manifest list {{image_list}}: $got"

# Run the image and prove it serves, rather than only that it built.
#
# The image published before this gate existed started, logged "listening" and
# reset every connection, because serve defaults to loopback and a loopback bind
# inside a network namespace reaches nobody. A build that succeeds proves
# nothing about that.
image-verify: image
    #!/usr/bin/env bash
    set -euo pipefail
    cleanup() {
      {{engine}} rm -f {{image_ctr}} >/dev/null 2>&1 || true
      {{engine}} volume rm -f {{image_vol}} >/dev/null 2>&1 || true
    }
    trap cleanup EXIT
    cleanup

    base="http://127.0.0.1:{{image_port}}"
    await() {
      for _ in $(seq 1 60); do
        if curl -fsS --max-time 2 "$base/healthz" >/dev/null 2>&1; then return 0; fi
        sleep 1
      done
      echo "the container never answered $base/healthz" >&2
      {{engine}} logs {{image_ctr}} >&2 || true
      return 1
    }

    # A volume, not a host bind mount: rootless podman maps the container's
    # 65532 to a subuid that cannot write a directory owned by the host user,
    # and the failure reads as the image being broken.
    {{engine}} volume create {{image_vol}} >/dev/null
    {{engine}} run -d --name {{image_ctr}} -v {{image_vol}}:/data \
      -p {{image_port}}:8080 {{image_latest}} >/dev/null
    await

    # The version the ldflags stamped, so neither a stale image nor something
    # else already listening on this port can satisfy this check.
    curl -fsS --max-time 5 "$base/healthz" | python3 -c \
      'import json,sys; d=json.load(sys.stdin); want={"status":"ok","version":sys.argv[1]}; bad={k:d.get(k) for k,v in want.items() if d.get(k)!=v}; sys.exit("/healthz reported %r, wanted %r" % (bad, want)) if bad else None' \
      "{{version}}"
    echo "healthz: ok at {{version}}"

    # The named checks, not only the top-level flag: a body that reported ready
    # with a failing database check would pass a looser assertion.
    curl -fsS --max-time 5 "$base/readyz" | python3 -c \
      'import json,sys; d=json.load(sys.stdin); c={x.get("name"):x.get("ok") for x in d.get("checks",[])}; bad=[k for k in ("database","migrations") if c.get(k) is not True]; sys.exit("/readyz reported ready=%r checks=%r, failing %r" % (d.get("ready"), c, bad)) if (d.get("ready") is not True or bad) else None'
    echo "readyz: database and migrations ok"

    # The signed-in-less landing template by name. A 200 carrying an error page
    # would satisfy a check that only looked for an HTML tag.
    # -L because an unauthenticated root redirects to /login, and the template
    # worth asserting is the one at the end of that.
    page=$(curl -fsSL --max-time 5 "$base/")
    if ! printf '%s' "$page" | grep -qF '<title>Sign in &middot; tix</title>'; then
      echo "the web UI did not render its sign-in template" >&2
      printf '%s' "$page" | head -20 >&2
      exit 1
    fi
    # The embedded assets, which are the part of the web UI a container can lose:
    # go:embed puts them in the binary, and nothing else in this gate would
    # notice if they stopped being served.
    css=$(curl -fsS --max-time 5 "$base/assets/app.css" | wc -c)
    if [ "$css" -lt 1000 ]; then
      echo "/assets/app.css served $css bytes, which is not the stylesheet" >&2
      exit 1
    fi
    echo "web: sign-in template and ${css}-byte stylesheet served"

    # The uid of the running server, not the image's declared USER. Reading the
    # image config would pass on an image whose entrypoint had dropped back to
    # root. The tix row specifically: podman's own ps appears in this listing as
    # root, so anything that scanned the whole output would find a 0.
    uid=$({{engine}} top {{image_ctr}} -eo uid,comm | awk '$2 == "tix" {print $1}')
    if [ "$uid" != "65532" ]; then
      echo "the server process runs as uid '${uid}', expected 65532" >&2
      {{engine}} top {{image_ctr}} -eo uid,comm >&2
      exit 1
    fi
    echo "nonroot: the server process runs as uid $uid"

    # A row written through the binary in the image, then read back from a
    # different container over the same volume. The container is destroyed and
    # replaced rather than restarted: a restart keeps the writable layer, so the
    # row would survive it wherever the database sat, and this check would pass
    # on an image whose /data mount did nothing. Unique per run, so a database
    # left behind by an earlier run could not satisfy it either.
    marker="persist-$$-${RANDOM}"
    {{engine}} exec {{image_ctr}} /usr/local/bin/tix task add "$marker" >/dev/null
    {{engine}} rm -f {{image_ctr}} >/dev/null
    {{engine}} run -d --name {{image_ctr}} -v {{image_vol}}:/data \
      -p {{image_port}}:8080 {{image_latest}} >/dev/null
    await
    {{engine}} exec {{image_ctr}} /usr/local/bin/tix task list -o json | python3 -c \
      'import json,sys; t=[x.get("title") for x in json.load(sys.stdin)]; sys.exit("over the replacement container /data holds titles %r, expected exactly one %r" % (t, sys.argv[1])) if t.count(sys.argv[1])!=1 else None' \
      "$marker"
    echo "persistence: $marker survived a replacement container on /data"

# Trivy against the built image, not the source tree.
#
# The filesystem scan reads go.mod and the working tree. This reads the layers,
# which is the only thing a person who pulls the image gets: the base image's
# own packages, the Go modules the shipped binary actually carries, and anything
# a build step baked in.
image-scan: image
    #!/usr/bin/env bash
    set -euo pipefail
    # A docker archive in the repository directory, because `just tool`
    # bind-mounts it and trivy in the toolbox container cannot otherwise reach
    # the engine's own storage.
    trap 'rm -f {{image_tar}}' EXIT
    {{engine}} save --format docker-archive -o {{image_tar}} {{image_latest}}
    just tool trivy image --input {{image_tar}} \
      --severity CRITICAL,HIGH --exit-code 1 \
      --ignorefile .trivyignore --scanners vuln,secret

# The tags one release publishes, printed one per line.
#
# At 0.x a minor bump is allowed to break things, and this project means it:
# release-please is configured bump-minor-pre-major. So the widest tag that can
# carry a compatibility promise is the minor series, and a bare major is
# deliberately absent. A `:0` tag would walk somebody from 0.5.x to 0.6.x across
# exactly the break the version scheme exists to announce, and they would never
# have typed a version to find out.
#
# latest is published because it is what somebody trying the project out types,
# and it is the one tag everybody already expects to move under them.
image-tags VERSION:
    #!/usr/bin/env bash
    set -euo pipefail
    v=$(printf '%s' '{{VERSION}}' | sed 's/^v//')
    case "$v" in
      *[!0-9.]* | *..* | .* | *.) echo "not a release version: '{{VERSION}}'" >&2; exit 1 ;;
    esac
    case "$v" in
      *.*.*) ;;
      *) echo "not a three-part version: '{{VERSION}}'" >&2; exit 1 ;;
    esac
    printf '%s\n%s\nlatest\n' "$v" "${v%.*}"

# The tag policy, asserted rather than described.
#
# A fixed version rather than this build's, so the expectation is written out in
# full and a reader can see what the policy is from the assertion alone.
image-tags-check:
    #!/usr/bin/env bash
    set -euo pipefail
    got=$(just image-tags v1.4.2)
    want=$(printf '1.4.2\n1.4\nlatest')
    if [ "$got" != "$want" ]; then
      echo "image tags for v1.4.2 are:" >&2; printf '%s\n' "$got" >&2
      echo "expected:" >&2; printf '%s\n' "$want" >&2
      exit 1
    fi
    # Named on its own, because this is the tag whose absence is the policy. The
    # equality check above would already catch it, but it would report "the list
    # differs" rather than the thing that actually matters.
    if printf '%s\n' "$got" | grep -qx 1; then
      echo "a bare major tag '1' is published: at 0.x that moves people across breaking changes" >&2
      exit 1
    fi
    # A version that is not a release must not quietly produce tags. `git
    # describe` on an untagged commit yields things like 0.5.0-2-gabc1234, and
    # publishing that plus latest from a working tree is worse than failing.
    for bad in v0.5.0-2-g6fd7de6 0.5 latest dev ""; do
      if just image-tags "$bad" >/dev/null 2>&1; then
        echo "image-tags accepted '$bad', which is not a release version" >&2
        exit 1
      fi
    done
    echo "image tags: $(printf '%s' "$got" | tr '\n' ' ')"

# Push the manifest list under the release tags, and write the digest those tags
# resolve to so the signature and the attestations can name it.
#
# The only recipe here that touches a registry, and part of no gate.
image-push REPO VERSION: image-multiarch
    #!/usr/bin/env bash
    set -euo pipefail
    for tag in $(just image-tags '{{VERSION}}'); do
      podman manifest push --all \
        --digestfile image-digest.txt \
        {{image_list}} "docker://{{REPO}}:$tag"
      echo "pushed {{REPO}}:$tag"
    done
    echo "digest: $(cat image-digest.txt)"

# The container image gate: build it, prove it serves, scan it, prove both
# published architectures build with the right binary inside, and prove the tag
# policy is what it claims.
image-gate: image image-verify image-scan image-multiarch image-tags-check

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
ci: tidy-check fmt-check vet lint build test-postgres cover-check smoke jstest sec vuln secrets trivy actionlint hadolint yamllint mdlint spec docs-check release-dry image-gate
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
