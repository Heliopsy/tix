## ADDED Requirements

### Requirement: An at sign names the assignee

The filter language SHALL read a word `@HANDLE` as `assignee:HANDLE`, and `-@HANDLE` as
`-assignee:HANDLE`, on every transport. A quoted `"@HANDLE"` SHALL stay free text, and a bare `@`
SHALL be free text.

#### Scenario: At sign and assignee agree

- **WHEN** a caller lists tasks with the filter `@raj tag:ops`
- **THEN** they get the same tasks as with `assignee:raj tag:ops`

#### Scenario: Negated at sign

- **WHEN** a caller lists tasks with the filter `-@raj`
- **THEN** no task assigned to raj is listed
