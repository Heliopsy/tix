## ADDED Requirements

### Requirement: A project deletion reports how many tasks it deleted

The result of `tix project rm KEY --delete-tasks` SHALL carry `deleted`, the number of tasks removed
permanently, trashed ones included, and `trashed`, the number that were in the trash. With `--dry-run` it
SHALL carry `would_delete` with the number the deletion would remove and `deleted` SHALL be zero.

#### Scenario: Deleting a project with its tasks

- **GIVEN** a project holding two live tasks and one trashed task
- **WHEN** `tix project rm KEY --delete-tasks -o json` runs
- **THEN** the result reads `"deleted": 3` and `"trashed": 1`

#### Scenario: Planning the deletion

- **GIVEN** the same project
- **WHEN** `tix project rm KEY --delete-tasks --dry-run -o json` runs
- **THEN** the result reads `"would_delete": 3`, `"deleted": 0` and `"dry_run": true`, and the tasks remain

### Requirement: User listings leave out removed people unless asked

`tix user ls` and `tix tenant user ls KEY` SHALL leave out people removed from the tenant. With
`--include-removed` they SHALL list them too, marked `removed` in JSON; the `user ls` table SHALL add
`(removed)` to the name and the `tenant user ls` table SHALL show the role `removed`.

#### Scenario: A revoked membership

- **GIVEN** a person whose membership of the tenant was revoked
- **WHEN** `tix user ls` runs
- **THEN** the person is not listed

#### Scenario: Asking for removed people

- **WHEN** `tix user ls --include-removed -o json` runs
- **THEN** the person is listed with `"removed": true`
