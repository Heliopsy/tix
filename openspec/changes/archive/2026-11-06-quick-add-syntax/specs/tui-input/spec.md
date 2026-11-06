## ADDED Requirements

### Requirement: The new-task prompt reads quick-add syntax

The terminal interface's new-task prompt SHALL read its line through the quick-add parser,
resolving relative dates in the local time zone, with the open project as the default. It
SHALL draw a row under the field naming what the line will set. A line the parser refuses
SHALL keep the prompt open with the message, and a refusal from the service SHALL reopen the
prompt with the line as typed.

#### Scenario: Preview row

- **WHEN** the reader types `Fix login !high #auth` into the new-task prompt
- **THEN** a row under the field names priority high and tag `auth`

#### Scenario: Service refusal keeps the line

- **WHEN** the reader submits `Fix login @nobody` and no actor has that handle
- **THEN** the new-task prompt is open again holding `Fix login @nobody` and the message names the handle
