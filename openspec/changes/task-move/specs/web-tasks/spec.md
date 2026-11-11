## ADDED Requirements

### Requirement: Move to another project in the browser

The task screen's menu SHALL offer Move to… beside Copy to… to a reader who may write tasks. It SHALL
open a form choosing among the other projects, and moving SHALL open the task under its new reference
with a toast. A refused move SHALL draw the form again with the message.

#### Scenario: Moving from the task screen

- **WHEN** a reader opens Move to… on OPS-3, picks WEB and submits
- **THEN** the browser shows the task under its WEB reference
