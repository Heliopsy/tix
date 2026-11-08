## ADDED Requirements

### Requirement: The terminal interface shares a task

On a selected card and on the task screen, the share key SHALL offer the link, the Markdown link
`[KEY-123 Title](url)` and the key and title. The choice SHALL be copied to the terminal's clipboard
with OSC 52, locally and over SSH, and SHALL also be drawn on screen in full until the next key.

#### Scenario: Copying the Markdown link

- **GIVEN** a task INFRA-12 whose `web_url` is `https://tix.example.com/tasks/infra-12`
- **WHEN** the reader presses the share key and picks Markdown
- **THEN** the terminal is sent OSC 52 with `[INFRA-12 Title](https://tix.example.com/tasks/infra-12)`
  and the same text is drawn on screen

### Requirement: Sharing without a web address

The link SHALL be the task's `web_url`, else the remote context's server URL with the task's path.
With neither, the picker SHALL offer the key and title and the Markdown label without a link, and say
why there is no link.

#### Scenario: A local database

- **GIVEN** a local database and no `server.public_url`
- **WHEN** the reader presses the share key
- **THEN** the picker offers key and title and the Markdown label, and its note names
  `server.public_url`
