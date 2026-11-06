## ADDED Requirements

### Requirement: The terminal interface draws keys and references upper case

The terminal interface SHALL draw project keys and task references upper case in its lists, cards,
lanes, headers, palette, dialogs, status lines, activity and inbox, and SHALL accept a project key
in any case, including `tix tui --project INFRA`.

#### Scenario: A card

- **WHEN** the board shows task `infra-3`
- **THEN** its card reads `INFRA-3`

#### Scenario: Opening a board in any case

- **WHEN** a user runs `tix tui --project INFRA`
- **THEN** the board of `infra` opens
