# Tasks

## The manifest

- [ ] `internal/plugin/manifest.go`: the entry, its JSON form, and atomic read and write
- [ ] `internal/plugin/manifest.go`: a manifest that will not parse refuses dispatch without rewriting
- [ ] `internal/plugin/dir.go`: resolve the plugin directory under the tix data directory

## Resolution and safety

- [ ] `internal/plugin/resolve.go`: name to entry, from the manifest only, never from a directory listing
- [ ] `internal/plugin/verify.go`: digest check against the recorded digest
- [ ] `internal/plugin/verify.go`: ownership and group/other write-bit checks on the directory and the file
- [ ] `internal/plugin/env.go`: build the child environment, stripping every tix credential variable
- [ ] `internal/plugin/env.go`: mode, target, tenant, format, version, invoked name and the tix binary path
- [ ] `internal/plugin/exec.go`: run the child on the caller's streams and return its exit status

## Installation

- [ ] `cmd/plugin.go`: `tix plugin install`, from a local path, with `--as` and `--grant`
- [ ] `cmd/plugin.go`: the trust statement and the confirmation, and `--yes` to assume it
- [ ] `cmd/plugin.go`: mint the scoped token on a grant, record its scopes, revoke it on remove
- [ ] `cmd/plugin.go`: warn when the name is already a built-in

## Management

- [ ] `cmd/plugin.go`: `tix plugin list`, marking unreachable entries, with `-o json`
- [ ] `cmd/plugin.go`: `tix plugin info`, with `-o json`
- [ ] `cmd/plugin.go`: `tix plugin remove`, deleting the executable and the entry

## Dispatch

- [ ] `cmd/root.go`: dispatch an unknown verb to a plugin, after the built-in tree has declined
- [ ] `cmd/root.go`: propagate the child's exit status unchanged
- [ ] `cmd/complete.go`: offer plugin names as first-word candidates, from the manifest

## The two guards that already exist

- [ ] `cmd/docs.go`: filter plugin rows out of the command table by an explicit marker
- [ ] `cmd/docs_readme_test.go`: the generated table is unchanged with a plugin installed
- [ ] `internal/capability/parity_test.go`: parity is unchanged with a plugin installed

## Guards

- [ ] `internal/plugin/resolve_test.go`: a `tix-*` executable on `$PATH` is not reachable
- [ ] `internal/plugin/resolve_test.go`: a file in the plugin directory with no entry is not reachable
- [ ] `internal/plugin/verify_test.go`: a modified executable is refused, naming the digest
- [ ] `internal/plugin/verify_test.go`: a group- or world-writable executable or directory is refused
- [ ] `internal/plugin/env_test.go`: the caller's token is absent from the child environment
- [ ] `internal/plugin/env_test.go`: a granted token is present and carries only the recorded scopes
- [ ] `internal/plugin/env_test.go`: local and remote targets each hand over the right pair
- [ ] `internal/plugin/exec_test.go`: every exit status of a child becomes tix's exit status
- [ ] `cmd/plugin_test.go`: a plugin named after a built-in never runs
- [ ] `cmd/plugin_test.go`: declining the confirmation writes nothing
- [ ] `cmd/complete_test.go`: completion offers plugin names and never executes one

## Documentation

- [ ] `docs/plugins.md`: writing one, installing one, the environment contract, the exit-code contract
- [ ] `docs/plugins.md`: what a plugin is trusted with, and what a grant costs
- [ ] `docs/README.md`: link the new page
- [ ] `docs/commands.md`: regenerate the block with `tix docs --table`
- [ ] `docs/scripting.md`: say that a plugin's exit status is passed through unchanged
- [ ] `ROADMAP.md` and `CHANGELOG.md` entries
