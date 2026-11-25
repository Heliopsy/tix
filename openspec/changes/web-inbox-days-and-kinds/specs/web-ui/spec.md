## ADDED Requirements

### Requirement: The inbox groups items by day

The inbox page SHALL draw its items under Today, Yesterday and Earlier, days counted in the reader's
timezone, leaving out a heading with no items.

#### Scenario: Just after midnight

- **GIVEN** a reader in UTC+3 at 01:00 with items from 00:30 and from 23:30 the evening before
- **WHEN** they open the inbox
- **THEN** the first is under Today and the second under Yesterday

### Requirement: The inbox filters by kind

The inbox bar SHALL carry one chip per kind, each a link adding its kind to the kinds kept or taking it
away, kept across All, Unread and the pager. With no chip on, every kind SHALL be listed.

#### Scenario: Other kinds only

- **GIVEN** a reader whose only item is a mention
- **WHEN** they open `/inbox?kinds=assigned,comment`
- **THEN** the list says nothing of these kinds is there

### Requirement: The inbox summary card

The inbox page SHALL open with a card counting the reader's unfinished assigned tasks that are overdue and
those due later today, the tenant's open tasks and the tasks finished in the last seven days, each linking
to the screen that lists them. A count the service will not give SHALL be left out, never drawn as zero.

#### Scenario: One late task

- **GIVEN** a reader with one unfinished assigned task due three days ago
- **WHEN** they open the inbox
- **THEN** the card's Overdue reads 1 and links to My work

### Requirement: Stop a kind from an inbox item

Each inbox row SHALL carry Stop for its kind while the reader receives that kind, which mutes the kind as
the Settings switch does and answers with a toast naming it. Items already in the inbox SHALL stay.

#### Scenario: Stopping mentions

- **WHEN** a reader presses Stop on a mention
- **THEN** mentions are muted, the toast says mentions no longer reach the inbox, and the mention stays
