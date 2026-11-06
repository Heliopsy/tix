# Tasks

## 1. Mapping

- [x] 1.1 `users` section: `map`, `match`, `email_field`, `unknown`, validated in `Mapping.check`
- [x] 1.2 `Mapped.AssigneeEmail` read from `email_field`, and an exported custom field prefix

## 2. Service

- [x] 2.1 Per-run user directory and resolver with the explicit map and the three strategies
- [x] 2.2 `unassigned` keeps the value under the mapping prefix; `placeholder` creates or reuses an actor
- [x] 2.3 Dry run resolves assignees read-only and never creates a placeholder
- [x] 2.4 `SyncResult.UnmatchedUsers` and one summary warning
- [x] 2.5 A parent link falls back to its last path segment

## 3. OpenProject

- [x] 3.1 Embedded default mapping and `DefaultMapping()`
- [x] 3.2 Tests: the default parses once `project` is filled, and drives a two-page import end to end

## 4. Surfaces and docs

- [x] 4.1 Web: the source dialog starts from the OpenProject mapping, and a run result names the people with no tix account
- [x] 4.2 `docs/migrating.md`: assignees, the `users` section and an OpenProject walkthrough
- [x] 4.3 Skill: assignees carry across through sync
