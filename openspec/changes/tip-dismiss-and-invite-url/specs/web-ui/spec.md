## ADDED Requirements

### Requirement: An open info tip closes when the reader moves on

With scripting, an open info tip SHALL close when the reader presses or taps outside it and its circle,
presses Escape, or opens another tip, so at most one is open. Escape SHALL return focus to the tip's
circle. A press inside the open note SHALL leave it open. The circle SHALL expose whether the tip is open.

#### Scenario: A tap elsewhere closes the tip

- **GIVEN** a reader opened the tip beside the Domains heading
- **WHEN** they tap the page outside the tip
- **THEN** the tip is closed and its circle is announced as collapsed

#### Scenario: Escape closes the tip and returns focus

- **GIVEN** an open tip
- **WHEN** the reader presses Escape
- **THEN** the tip closes, focus is on its circle, and the page does not go back

#### Scenario: One tip at a time

- **GIVEN** an open tip
- **WHEN** the reader opens another tip
- **THEN** the first tip is closed

#### Scenario: A press inside the note keeps it open

- **WHEN** the reader presses inside an open tip's note
- **THEN** the tip stays open
