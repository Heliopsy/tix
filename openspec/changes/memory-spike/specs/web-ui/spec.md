## ADDED Requirements

### Requirement: Sidebar badges do not read descriptions

The sidebar's overdue badge, drawn on every screen and fetched by the live refresh, SHALL count the reader's overdue tasks without reading their descriptions, so its cost does not grow with the length of those descriptions.

#### Scenario: Long descriptions

- **WHEN** a reader with a hundred overdue tasks of 40 KB descriptions each fetches the sidebar badges
- **THEN** the badge reads 99+ and the request allocates no more than a few MiB
