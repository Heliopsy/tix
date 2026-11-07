## ADDED Requirements

### Requirement: Activity and History show the channel

Each Activity row and each task History row SHALL show its access channel as an icon and words such as via SSH, via token ci-agent or via MCP (token scout-runner), with every recorded detail in its hover and its raw view.

#### Scenario: A token's row

- **WHEN** the feed shows a change made with the ci-agent token
- **THEN** the row reads via token ci-agent and its hover names the client address

#### Scenario: A task's history

- **WHEN** a task's History is shown
- **THEN** its Channel column shows each change's channel

### Requirement: Activity filters by channel and token

The Activity filter SHALL offer a channel control and a token box carried in the query string as via and token, each shown back as a removable chip, and each row's channel SHALL link to the feed narrowed to that channel and token.

#### Scenario: Narrowing to SSH

- **WHEN** the reader picks SSH in the channel control
- **THEN** only changes made over SSH are listed and a via SSH chip clears it
