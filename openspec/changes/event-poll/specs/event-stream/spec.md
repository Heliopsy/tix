## ADDED Requirements

### Requirement: Events can be followed over plain HTTP

The server SHALL serve `GET /api/v1/events/poll`, answering `200` with `events`, the committed events after `since_seq` that match the request, in sequence order and in the shape the WebSocket delivers, and `next_seq`, the cursor to pass back. `since_seq` SHALL be exclusive. `limit` SHALL default to 200 and SHALL be at most 1000.

#### Scenario: The cursor is exclusive

- **WHEN** a client polls with since_seq 4 and events 3, 4, 5 and 6 match
- **THEN** it receives events 5 and 6, in that order

#### Scenario: A limit larger than the maximum

- **WHEN** a client polls with a limit of 1001
- **THEN** the server answers invalid

### Requirement: The poll cursor never moves backwards and skips nothing

`next_seq` SHALL be the highest sequence number the request examined, which is the last event delivered when the page is full, and SHALL equal `since_seq` when nothing follows it. A `since_seq` absent or zero SHALL start at the newest committed event and deliver none of the backlog, as on the WebSocket.

#### Scenario: Nothing new

- **WHEN** a client polls with since_seq 9 and no event follows 9
- **THEN** it receives no events and next_seq 9

#### Scenario: Starting without a cursor

- **WHEN** a client polls without since_seq in a tenant whose newest event is 12
- **THEN** it receives no events and next_seq 12

### Requirement: A poll applies the WebSocket's credential and filter rules

A poll SHALL require the event subscribe scope, SHALL leave out every event the same credential would not receive over the WebSocket, SHALL narrow a project-scoped credential to its project, SHALL answer forbidden when such a credential names another project, and SHALL answer invalid for an event type outside the vocabulary.

#### Scenario: A project-scoped token names another project

- **WHEN** a token scoped to project A polls with project_ids naming project B
- **THEN** the server answers forbidden

#### Scenario: An administrative event without its scope

- **WHEN** a credential holding only the event subscribe scope polls past a user.created event
- **THEN** that event is not among the events it receives

### Requirement: An expired poll cursor is reported like an expired subscription cursor

A poll whose `since_seq` is older than the oldest retained event SHALL be refused with the same error code and message the WebSocket sends for that cursor, and SHALL deliver no events.

#### Scenario: Pruned history

- **WHEN** the oldest retained event is 4000 and a client polls with since_seq 12
- **THEN** the server answers invalid with "cursor 12 is no longer available; the oldest retained event is 4000"

### Requirement: A poll can wait for the next event

A poll MAY carry `wait`, from 0 to 25 seconds, default 0. When no matching event is available the server SHALL hold the request until one commits or the wait elapses, then answer `200`, with empty `events` on timeout. It SHALL wake on the store's commit signal rather than re-reading on a timer. A larger wait SHALL be invalid.

#### Scenario: An event arrives during the wait

- **WHEN** a client polls with wait 25s and a matching event commits after 2 seconds
- **THEN** the server answers with that event without waiting out the rest

#### Scenario: Nothing arrives

- **WHEN** a client polls with wait 25s and no matching event commits
- **THEN** after 25 seconds the server answers with no events and next_seq unchanged

#### Scenario: Too long a wait

- **WHEN** a client polls with wait 26s
- **THEN** the server answers invalid

### Requirement: Switching between the poll and the WebSocket loses nothing

A client SHALL be able to subscribe over the WebSocket with the `next_seq` of its last poll, or poll with the `seq` of the last event it received over the WebSocket, and receive every later matching event exactly once.

#### Scenario: From the poll to the WebSocket

- **WHEN** a client polls, then subscribes with the returned next_seq and the same filter
- **THEN** it receives every matching event committed after that poll, and none it already had

### Requirement: The command line can poll once

`tix events` SHALL run one poll, taking `--since`, `--type`, `--project`, `--limit` and `--wait`, and SHALL make `next_seq` available to a script.

#### Scenario: Resuming from a script

- **WHEN** a script runs `tix events --since 41 -o json`
- **THEN** it prints an object holding the events after 41 and next_seq

### Requirement: The MCP tool polls without waiting

The `event.poll` tool SHALL be offered to MCP clients without the `wait` parameter, so a model can catch up on events since a cursor without holding a tool call open.

#### Scenario: Listing tools

- **WHEN** an MCP client lists the tools
- **THEN** the event poll tool is present and takes no wait parameter
