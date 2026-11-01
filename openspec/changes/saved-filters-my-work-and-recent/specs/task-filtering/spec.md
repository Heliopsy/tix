## ADDED Requirements

### Requirement: me in actor terms

The filter language SHALL accept `me` as the value of `assignee:`, `creator:` and `claimed-by:`, negated
or not, meaning the reader, on every surface that answers a filter.

#### Scenario: The terminal answers assignee:me

- **WHEN** a reader types `assignee:me` in the terminal interface's filter bar
- **THEN** the board shows the tasks assigned to that reader
