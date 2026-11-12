## ADDED Requirements

### Requirement: Activity filters by a time window and an action

The Activity filter SHALL offer From and To date-and-time controls, read in the reader's time zone, and an
action box matching an entry's action exactly, carried in the query string as from, to and action, each shown
back as a removable chip and kept by paging and the live refresh. To SHALL include the minute it names, or the
whole day when it names only a date.

#### Scenario: Narrowing to a morning

- **WHEN** the reader sets From to 09:00 and To to 12:00 today
- **THEN** only entries recorded from 09:00 up to and including 12:00 in their zone are listed, with a chip for each bound

#### Scenario: Narrowing to an action

- **WHEN** the reader types task.create in the action box
- **THEN** only entries whose action is task.create are listed and an action task.create chip clears it

### Requirement: An unreadable Activity time window is refused

A from or to value that is not a date or a date and time, or a window whose To is before its From, SHALL be
refused rather than ignored.

#### Scenario: A reversed window

- **WHEN** the address asks for from=2026-01-02 and to=2026-01-01
- **THEN** the screen answers with a refusal instead of the whole log
