## ADDED Requirements

### Requirement: A history row copies its audit entry as JSON

Every activity row and task History row SHALL carry a Copy JSON button beside Raw that copies the row's audit
entry, as the audit API lists it, as indented JSON without opening the dialog, and briefly reads Copied. A row
of several steps SHALL copy them as an array. The button SHALL be drawn only with scripting.

#### Scenario: Copy JSON on an activity row

- **WHEN** a reader presses Copy JSON on an activity row for a retitled task
- **THEN** the clipboard holds that audit entry as indented JSON with its before and after snapshots
- **AND** the document decodes to the same value the audit API lists for that entry

### Requirement: Each step of a raw entry copies alone

In the raw entry dialog of a row with several steps, each step SHALL have a Copy JSON button copying that one
entry, and Copy all as JSON SHALL copy what the row's Copy JSON copies.

#### Scenario: A two-step move

- **WHEN** a task is completed in two transitions and the move's raw entry is opened
- **THEN** each step has a Copy JSON button copying its own entry
