## ADDED Requirements

### Requirement: Checklist filter terms

The filter language SHALL accept `has:checklist`, selecting tasks whose description carries a checklist
marker (`- [ ] `, `* [ ] `, `+ [ ] ` or a ticked form), and `-has:checklist` for the rest;
`checklist:open`, selecting those carrying an unticked marker, and `-checklist:open` for the rest; and
`checklist:done`, selecting those carrying a marker and no unticked one. `checklist:done` SHALL NOT be
negatable. Both storage engines and the in-memory matcher SHALL select the same tasks for each term.

#### Scenario: Open and done are told apart

- **GIVEN** one task with an open item, one with every item ticked, and one with no checklist
- **WHEN** the listing is filtered by `checklist:open`, then by `checklist:done`
- **THEN** the first lists only the task with an open item, and the second only the one with every
  item ticked
