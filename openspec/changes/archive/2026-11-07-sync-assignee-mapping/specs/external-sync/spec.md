## ADDED Requirements

### Requirement: Imported assignees match tix actors

A mapping SHALL resolve the source value of its `assignee` target through `users.map`, an explicit table from
the source value to a tix handle or email, then through the `users.match` strategies in order, `email`,
`handle` and `name` by default. `match: []` SHALL turn the strategies off. Lookups SHALL stay in the tenant.

#### Scenario: Matched by an explicit entry

- **WHEN** `users.map` maps `J. Smith` to handle `john` and a record's assignee is `J. Smith`
- **THEN** the imported task is assigned to `john`

#### Scenario: Strategies turned off

- **WHEN** `users.match` is `[]` and a record's assignee is the handle of a tix actor not named in `users.map`
- **THEN** the task is left unassigned and the value is reported unmatched

### Requirement: Automatic assignee match strategies

The `email` strategy SHALL match a user of the tenant whose email equals the value, or the value at
`users.email_field`, ignoring case. The `handle` strategy SHALL match an actor handle ignoring case. The
`name` strategy SHALL match a display name ignoring case only when exactly one actor carries it.

#### Scenario: Matched by email from another field

- **WHEN** the assignee value is `Jane Doe`, `email_field` holds `JANE@corp.test`, and a tix user has email `jane@corp.test`
- **THEN** the task is assigned to that user's actor

#### Scenario: An ambiguous name does not match

- **WHEN** two actors share the display name `Alex` and the assignee value is `alex`
- **THEN** the name strategy matches neither and the value is reported unmatched

### Requirement: Unmatched assignees are left unassigned and reported

Under `users.unknown: unassigned`, the default, an assignee nothing matches SHALL leave the task unassigned
and SHALL keep the value in the custom field `<prefix>assignee`, `<prefix>` being the mapping's unmapped
prefix. The result's `unmatched_users` SHALL list each such value once, sorted, and one warning SHALL list them.

#### Scenario: An unknown person

- **WHEN** a record's assignee is `Nobody Known` and no actor matches
- **THEN** the task has no assignee, `ext_assignee` holds `Nobody Known`, and `unmatched_users` is `["Nobody Known"]`

### Requirement: Placeholder actors for unmatched assignees

Under `users.unknown: placeholder` an unmatched assignee SHALL be assigned to a placeholder actor with no
account, membership or scopes, named by the value. A later run SHALL reuse it, and an actor that holds an
account SHALL never be taken as a placeholder. The value SHALL still be listed in `unmatched_users`.

#### Scenario: The assignment is carried and reused

- **WHEN** two runs import records assigned to the unknown `Jane Roe` under the placeholder policy
- **THEN** both tasks are assigned to one placeholder actor that cannot sign in, and the second run creates no actor

### Requirement: Dry runs resolve assignees

A dry run SHALL resolve assignees exactly as the real run would, write nothing, create no placeholder, and
report the same `unmatched_users` and summary warning, worded as what would happen.

#### Scenario: Unmatched users before the import

- **WHEN** a dry run meets an assignee nothing matches under the placeholder policy
- **THEN** `unmatched_users` lists it, the warning says it would get a placeholder, and no actor or task is created

### Requirement: The users section is validated

A mapping SHALL be refused before any record is read when `users.match` names a strategy other than `email`,
`handle` or `name`, repeats one, when `users.unknown` is not `unassigned` or `placeholder`, or when a
`users.map` entry has an empty side. The error SHALL name the entry.

#### Scenario: An unknown strategy

- **WHEN** a mapping sets `users.match: [email, phone]`
- **THEN** the run is refused as invalid naming `users.match` and `phone`, and nothing is written

### Requirement: A parent given as a link matches its imported record

When a record's parent value finds no imported record and is an absolute path or an http(s) url, the import
SHALL look the parent up again by the last segment of its path, so `/api/v3/work_packages/42` links to the
record imported with identity `42`.

#### Scenario: OpenProject parent link

- **WHEN** work package 42 was imported and work package 43 names `_links.parent.href` `/api/v3/work_packages/42`
- **THEN** the task imported from 43 is a child of the task imported from 42

### Requirement: The OpenProject adapter ships a default mapping

The OpenProject adapter SHALL ship a default mapping covering subject, description, the stock statuses onto
the built-in workflow, priorities, type, assignee with its email, due and start dates, parent and project,
which SHALL validate once its `project` names a tix project and SHALL NOT preserve unnamed HAL links.

#### Scenario: The default mapping imports work packages

- **WHEN** the default mapping, with `project` filled in, drives an import of OpenProject work packages
- **THEN** titles, statuses, priorities, assignees and parents arrive without any other mapping edits
