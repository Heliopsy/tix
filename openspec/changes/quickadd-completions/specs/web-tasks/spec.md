## ADDED Requirements

### Requirement: The quick-add box completes tags, people and projects

While the caret is in a word of the quick-add box that starts with `#`, `@` or `+`, the web UI SHALL
list the tenant's tags, people or projects that the rest of the word matches. A word with no such
prefix, or inside an open quote, SHALL get no list. Arrows SHALL move through the list, Enter or Tab
SHALL take the reached option, Escape SHALL close it, and Enter with no option reached SHALL add the
task as typed.

#### Scenario: A tag completes

- **GIVEN** the tenant has the tag `backend`
- **WHEN** the reader types `Fix login #ba` into the quick-add box
- **THEN** `#backend` is offered, and taking it leaves `Fix login #backend ` in the box

#### Scenario: A bare word is the title

- **WHEN** the reader types `Fix lo`
- **THEN** no list is drawn and Enter adds the task

### Requirement: Completions write what the quick-add parser reads

A taken completion SHALL keep its sigil and be followed by a space. A tag or handle the sigil form
cannot carry SHALL be written as `tag:VALUE` or `assignee:VALUE`, and a value containing whitespace
SHALL NOT be offered.

#### Scenario: A tag the sigil cannot carry

- **GIVEN** the tenant has the tag `c++`
- **WHEN** the reader takes it after typing `#c`
- **THEN** the box holds `tag:c++ `

### Requirement: A project page offers every project to quick add

The quick-add box on a project page SHALL offer every project after `+`, while the filter box there
SHALL go on offering no project.

#### Scenario: Another project from a project page

- **GIVEN** the projects `infra` and `web`
- **WHEN** the reader opens `/projects/infra`
- **THEN** the quick-add suggestion data lists both projects and the filter suggestion data lists none
