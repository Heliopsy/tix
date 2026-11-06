## ADDED Requirements

### Requirement: Bulk actions from the command line

The CLI SHALL offer `tix task bulk ACTION REF...`, where ACTION is one of `move`, `priority`,
`tag-add`, `tag-remove`, `assign`, `star`, `unstar`, `copy`, `delete` and `restore`, taking
`--to`, `--priority`, `--tag`, `--assignee`, `--project` and `--cascade` for the argument, `-`
as a reference to read references from standard input, and sending one request. It SHALL print
one outcome per task and exit non-zero when any task was refused.

#### Scenario: One request for many tasks

- **WHEN** `tix task bulk priority OPS-1 OPS-2 --priority high` is run
- **THEN** both tasks have priority high and the output lists two applied outcomes

### Requirement: Many references wherever one is taken for a change

`tix task edit` and `tix task copy` SHALL accept many references and apply the same change to
each, refusing `--version` and `--title` when more than one is given. `tix tag rm` SHALL take
`REF... TAG`. `tix tag add` SHALL accept a comma separated list of references, or `-` to read
them from standard input, in its reference position.

#### Scenario: Edit many tasks

- **WHEN** `tix task edit OPS-1 OPS-2 --priority low` is run
- **THEN** both tasks have priority low and one outcome per task is printed

#### Scenario: Tag many tasks

- **WHEN** `tix tag add OPS-1,OPS-2 ops` is run
- **THEN** both tasks carry the tag `ops`

### Requirement: Task listing as CSV

`tix task ls` SHALL accept `-o csv` and print a header row and one row per task in the columns
`ref, project, title, status, priority, assignee, tags, due, created, updated`, with the
assignee as a handle when it resolves, escaped by RFC 4180 and with the formula guard. Other
commands SHALL refuse `-o csv` with a usage error.

#### Scenario: CSV listing

- **WHEN** `tix task ls -o csv` is run
- **THEN** the first line is `ref,project,title,status,priority,assignee,tags,due,created,updated`

#### Scenario: CSV elsewhere

- **WHEN** `tix project ls -o csv` is run
- **THEN** the command exits with a usage error naming the formats it accepts
