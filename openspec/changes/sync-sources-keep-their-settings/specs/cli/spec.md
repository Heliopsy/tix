## ADDED Requirements

### Requirement: Source settings from the command line

`tix sync source add` SHALL accept `--url`, `--project`, `--query`, `--file`, `--page-size` and `--mapping`,
store what is given on the source, and `tix sync source ls` SHALL show the stored settings and never a
credential.

#### Scenario: Adding a jira source with its url

- **WHEN** `tix sync source add jira platform --url https://jira.example --project PLAT` runs
- **THEN** `tix sync source ls -o json` shows the url and project on that source

### Requirement: The indexed flag is a deprecated no-op

`tix field put` SHALL accept `--indexed` without effect, hidden from help, and print a deprecation notice.

#### Scenario: An old script

- **WHEN** `tix field put rv severity --type string --indexed` runs
- **THEN** the field is saved and stderr says the flag is deprecated
