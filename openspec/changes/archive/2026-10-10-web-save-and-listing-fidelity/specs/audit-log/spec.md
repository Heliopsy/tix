## ADDED Requirements

### Requirement: An audit listing accepts a free-text term

The audit listing SHALL accept a free-text term that selects the entries whose own record holds it: the
action, the kind of record touched, the surface it arrived from, or either snapshot. The match SHALL be a
weak, case-insensitive substring match, and a wildcard character in the term SHALL select that character
rather than act as a wildcard.

The term SHALL be answered where the structured terms are answered, so that a page returned for it carries
as many entries as the page size allows and the cursor returned with it names the position after the last
entry on that page.

Walking the listing from its first page to its last SHALL return every entry the term selects, exactly
once.

The term SHALL be reachable over the HTTP API alongside the structured terms.

#### Scenario: Every match is reachable by walking the listing

- **GIVEN** more entries matching a term than one page holds
- **WHEN** the listing is walked from its first page to its last
- **THEN** every matching entry is returned
- **AND** no entry is returned twice

#### Scenario: The term reads the whole record

- **GIVEN** entries whose only occurrence of a term is in their snapshot, and others whose only occurrence
  is in their action
- **WHEN** the listing is filtered by that term
- **THEN** both are returned

#### Scenario: A wildcard is a character

- **GIVEN** entries holding no percent sign
- **WHEN** the listing is filtered by a percent sign
- **THEN** no entries are returned

### Requirement: The activity screen searches the whole log

The browser's activity screen SHALL answer its free-text box over the tenant's whole audit history rather
than over a bounded window of recent entries.

An empty result SHALL say that nothing in the tenant's history matches the filter. It SHALL NOT qualify
the result with a count of records examined, because no window is being examined.

#### Scenario: An empty search does not claim a window

- **WHEN** the reader searches for a term no entry holds
- **THEN** the screen says nothing matches anywhere in the tenant's history
- **AND** it names no count of records searched
