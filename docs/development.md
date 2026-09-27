# Development

[CONTRIBUTING.md](../CONTRIBUTING.md) covers the process: what to run before pushing, commit format,
the specs-first rule and the CLA. This page is the reference for the recipes themselves.

```sh
just build     # bin/tix
just test      # race detector, shuffled
just check     # fast pre-push gate set
just ci        # the entire CI suite locally, in containers
```

Requires Go and [just](https://just.systems/). Linters, scanners and PostgreSQL run in containers via
podman, so nothing needs installing on the host, and every CI job invokes the same `just` recipe so
local and CI results cannot drift apart.

| Recipe | Does |
| -------- | ------ |
| `just build` / `just build-all` | Build for the host, or the full release matrix |
| `just test` / `just cover` | Tests with the race detector; coverage report |
| `just lint` `just sec` `just vuln` `just trivy` | Individual gates, in the pinned toolbox image |
| `just spec` | `openspec validate --strict` |
| `just pg-up` / `just test-postgres` | PostgreSQL in a container, and the suite against it |
| `just cover-check` | The coverage floor, the headroom above it, and what the run skipped |
| `just ci` | Everything CI runs, locally |

[testing.md](testing.md) covers the suite itself: what a partial run announces, the coverage floors,
and mutation testing with `just mutate`.
