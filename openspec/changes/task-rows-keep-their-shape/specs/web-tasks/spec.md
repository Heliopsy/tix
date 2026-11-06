## ADDED Requirements

### Requirement: Task rows line their attributes up in columns

From a viewport 640px wide, every row of a task listing SHALL draw each shown attribute (status,
priority, tags, assignee, project, flags, reference) in a cell of the same width at the same
horizontal position as every other row, whatever the title, tags or reference of the row hold.

#### Scenario: A longer reference moves nothing

- **GIVEN** a listing at 1440px holding VUL-9 and VUL-10
- **WHEN** the list is drawn
- **THEN** the status, priority, assignee, project and reference of both rows start at the same x

### Requirement: A task row stays bounded whatever its content

A task row SHALL clamp its title to two lines with an ellipsis, SHALL cut a tag, handle or flag
too long for its cell with an ellipsis rather than wrapping it, SHALL never be wider than its
listing, and SHALL never render the task's description or any Markdown block in the row.

#### Scenario: A hostile task keeps the row's shape

- **GIVEN** a task with a 140 character title, two 60 character tags and a Markdown description
  holding a heading, a table, fenced code and an unbroken 150 character URL
- **WHEN** the list is drawn at phone, tablet and laptop widths
- **THEN** its row is no wider than the list, at most twice the median row height, and holds no
  paragraph, list, heading, table or code block
