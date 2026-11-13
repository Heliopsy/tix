## ADDED Requirements

### Requirement: Live events screen

The web UI SHALL serve a Live events screen at `/events` that follows the tenant's event stream over the WebSocket and lists events newest first, each row showing its sequence number, type, time, actor and subject, with Show linking to the subject's task or project and Details showing the event as JSON with a Copy JSON button. It SHALL keep at most 500 events and say so.

#### Scenario: An event arrives

- **WHEN** a reader has `/events` open and a task is created
- **THEN** a row for `task.created` appears at the top with the task's reference and a Show link to it

### Requirement: Live events filter

The Live events filter SHALL be a GET form whose state is in the query string: `type` (comma separated, a trailing `*` matches a prefix) and `project` are sent to the server, `actor` is matched in the browser, and `since` replays events after that sequence number before following live. An unknown project or a malformed sequence number SHALL draw the reason and a way back instead of subscribing.

#### Scenario: A narrowed stream is a link

- **WHEN** a reader applies the type `task.*` and the project `infra`
- **THEN** the address carries `type=task.*` and `project=infra` and the subscription names them

#### Scenario: An unknown project

- **WHEN** a reader opens `/events?project=nope`
- **THEN** the screen says no project has that key and links back to `/events`

### Requirement: Live events pause and connection state

The Live events screen SHALL offer Pause and Resume, counting the events that arrive while paused and putting them on top on resume, and Clear the list. It SHALL show whether it is connecting, live or offline, reconnect with backoff, and when the server no longer keeps the replay cursor it SHALL say so and continue from now.

#### Scenario: Pausing

- **WHEN** a reader pauses and three events arrive
- **THEN** the screen reads "Paused, 3 new events." and Resume shows them at the top

### Requirement: Live events entry points

The Live events screen SHALL be listed in the sidebar's Manage group and in the command palette, both only with Advanced screens on.

#### Scenario: Finding the screen

- **WHEN** a reader with Advanced screens on opens the sidebar's Manage group
- **THEN** it lists Live events linking to `/events`
