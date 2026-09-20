## What changed

## Why

## Spec

- [ ] Behaviour change is reflected in `openspec/` (or this PR changes no behaviour)
- [ ] `openspec validate tix-v1 --strict` passes
- [ ] Relevant tasks ticked in `tasks.md`

## Checks

- [ ] `just check` passes locally
- [ ] New behaviour has tests in the same change
- [ ] PR title is a conventional commit (release-please reads it)

## Architecture rules

- [ ] No Cobra imports under `internal/`
- [ ] No business logic or database access in `cmd/`
- [ ] Every query goes through the tenant-scoped builder
- [ ] Mutations write rows, audit entry, and event in one transaction
- [ ] List queries are keyset-paginated, no `OFFSET`
- [ ] New `Service` methods have a capability registry entry with CLI, HTTP and Web bindings

## Notes for reviewers
