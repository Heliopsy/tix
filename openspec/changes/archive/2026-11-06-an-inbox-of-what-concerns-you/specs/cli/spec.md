## ADDED Requirements

### Requirement: tix inbox

The CLI SHALL offer `tix inbox ls` (with `--unread`, `--limit` and `--cursor`), `tix inbox read ID`,
`tix inbox unread ID`, `tix inbox read-all`, `tix inbox count` and `tix inbox prefs` (with `--mute`
and `--unmute` to change the muted kinds), each with the table, JSON and YAML output every command
has.

#### Scenario: Listing unread items

- **WHEN** a user runs `tix inbox ls --unread`
- **THEN** each unread item is printed with its task, actor, kind and age

#### Scenario: Muting a kind

- **WHEN** a user runs `tix inbox prefs --mute comment`
- **THEN** `comment` is listed as muted and comments on watched tasks no longer reach them

### Requirement: tix inbox channel

The CLI SHALL offer `tix inbox channel ls`, `add NAME URL` (with `--format`, `--kinds`, `--off`),
`edit ID` (with `--name`, `--url`, `--format`, `--kinds`, `--active`), `rm ID`, `test ID`,
`deliveries ID` and `redeliver DELIVERY`.

#### Scenario: Testing a channel turned off

- **WHEN** a user runs `tix inbox channel test` on a channel turned off
- **THEN** the command exits 6
