## ADDED Requirements

### Requirement: Quick-add syntax

The system SHALL provide one parser, shared by every surface that offers it, that reads a
single line into a task title and the attributes it names: priority (`!highest`, `!high`,
`!normal`, `!low`, `!lowest`, `!1` to `!5`, `priority:X`), tags (`#tag`, `tag:X`), assignee
(`@handle`, `assignee:X`), due date (`due:X`), project (`+key`, `project:key`), parent
(`^ref`, `parent:ref`) and initial status (`status:X`). Recognised tokens SHALL be removed
from the title and the remaining words SHALL form the title in their original order.

#### Scenario: A line with every attribute

- **WHEN** the line `Fix login !high #auth @raj due:2026-11-20 +web ^infra-3 status:doing` is parsed
- **THEN** the title is `Fix login`, the priority is high, the tags are `auth`, the assignee is `raj`, the due date is 2026-11-20, the project is `web`, the parent is `infra-3` and the status is `doing`

#### Scenario: Unicode titles

- **WHEN** the line `Поправи входа #сигурност !high` is parsed
- **THEN** the title is `Поправи входа` and the tag is `сигурност`

### Requirement: Quick-add tokens only count as whole valid words

A sigil SHALL only start a token at the beginning of a whitespace-separated word, and only
when the rest of the word is a valid value for it. A word that does not qualify SHALL stay in
the title unchanged. A keyed form (`due:`, `project:`, `status:`, `priority:`, `tag:`,
`assignee:`, `parent:`) with an invalid value SHALL be refused.

#### Scenario: Look-alikes stay in the title

- **WHEN** the line `Mail a@b.c about C# and #42 see https://x.io/#top +1 !important` is parsed
- **THEN** the title is the whole line and no attribute is set

#### Scenario: A bad keyed value is refused

- **WHEN** the line `Ship it due:someday` is parsed
- **THEN** the parse is refused with a message naming `someday`

### Requirement: Quick-add escapes

A backslash before a token SHALL keep the token as literal title text without the backslash.
Text inside a pair of quotes (`"…"`, `„…“`, `“…”` or `«…»`) SHALL be kept in the title as
written, quotes included. A backslash before anything that is not a token SHALL be left as it
is.

#### Scenario: Backslash

- **WHEN** the line `Tag posts \#news` is parsed
- **THEN** the title is `Tag posts #news` and no tag is set

#### Scenario: Quotes

- **WHEN** the line `Rename "#main @ops" to trunk` is parsed
- **THEN** the title is `Rename "#main @ops" to trunk` and no tag or assignee is set

### Requirement: Quick-add due dates

`due:` SHALL accept `today`, `tomorrow`, a weekday name in full or as three letters (the next
such day after today), `+Nd` and `+Nw` counted from today, and the absolute forms the filter
language accepts. Calendar words SHALL be resolved against a present the caller supplies, in
that present's time zone, and a date without a time SHALL mean midnight UTC of that day.

#### Scenario: Next Friday

- **WHEN** `due:fri` is parsed with a present of Friday 2026-11-06
- **THEN** the due date is 2026-11-13

#### Scenario: Relative days

- **WHEN** `due:+3d` is parsed with a present of 2026-11-01
- **THEN** the due date is 2026-11-04

### Requirement: Quick-add contradictions are refused

A line that names the same single-valued attribute twice with different values, or that
contradicts a value the caller set explicitly alongside it, SHALL be refused. Tags SHALL be
combined. A line with nothing left for the title SHALL be refused.

#### Scenario: Two priorities

- **WHEN** the line `Fix it !high !low` is parsed
- **THEN** the parse is refused naming both values

#### Scenario: Only tokens

- **WHEN** the line `#auth !high` is parsed
- **THEN** the parse is refused because the title is empty
