## ADDED Requirements

### Requirement: The browser sets and explains lease length

The task screen SHALL show a Lease row a writer can edit, and under it how long a claim would last and which
level decided it. Each project's board SHALL carry a Claims panel setting the project's lease length and longest
lease, blank to inherit.

#### Scenario: Saving a project's lease length

- **WHEN** a writer saves 2h as the lease length in a project's Claims panel
- **THEN** the project's lease default is 2h, and a task in it with no lease of its own says claims last 2h, from the project
