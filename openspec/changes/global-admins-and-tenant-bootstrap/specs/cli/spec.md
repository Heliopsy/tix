## ADDED Requirements

### Requirement: Global admin commands

The CLI SHALL offer `tix global-admin grant EMAIL`, `revoke EMAIL` and `ls`, and `tix tenant user ls`,
`create`, `edit` and `rm`, each naming the tenant first.

#### Scenario: Revoking the last one

- **WHEN** `tix global-admin revoke` names the only global admin
- **THEN** it exits 6

### Requirement: Tenant create names its first admins

`tix tenant create KEY [NAME]` SHALL take the name as an argument or `--name`, not both, and a repeatable
`--admin EMAIL`. Generated passwords SHALL be printed once: as a table after a warning on stderr, or in
full in `-o json`.

#### Scenario: Two admins

- **WHEN** `tix tenant create test --name Test --admin a@x.test --admin b@x.test` runs
- **THEN** stdout carries both addresses with their passwords and stderr says they are shown once
