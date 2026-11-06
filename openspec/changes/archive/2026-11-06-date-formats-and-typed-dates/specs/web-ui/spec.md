## ADDED Requirements

### Requirement: A parsed field shows what it takes before it is typed in

The due date, repeat and quick-add fields SHALL carry an (i) tip listing a few values to type, drawn from
the parser's own lists, so the syntax is visible before a refusal. A due date example SHALL be written in
the reader's date format. Every example shown SHALL be accepted by the parser that reads the field.

#### Scenario: Due examples in the reader's format

- **GIVEN** a reader whose date format is Dotted
- **WHEN** they open the due date tip on Add with details
- **THEN** it lists `tomorrow`, `fri`, `+3d`, `+2w` and `20.11.2026`

#### Scenario: Repeat examples

- **WHEN** a reader opens the repeat tip on the task screen
- **THEN** it lists rules such as `daily`, `every 2 weeks` and `every month on day 15`, each one a rule the parser accepts

### Requirement: A due date field takes words and the reader's own dates

The due date fields on Add with details and the task screen SHALL be text fields that take a word such as
`tomorrow` or `fri`, an offset such as `+3d`, `YYYY-MM-DD`, or a date in the reader's date format, and SHALL
show a saved due date in that format, so saving the field unchanged keeps the day.

#### Scenario: The placeholder shows examples

- **GIVEN** a reader whose date format is Dotted
- **WHEN** the due date field is empty
- **THEN** its placeholder reads `tomorrow, +3d or 20.11.2026`

#### Scenario: A dotted date round-trips

- **GIVEN** a reader whose date format is Dotted
- **WHEN** they save `06.11.2026` in the task screen's Due field
- **THEN** the task falls due on 6 November 2026 and the field shows `06.11.2026`

### Requirement: A due date field keeps a calendar

Each typed due date field SHALL carry a calendar button with an accessible label that opens the browser's
date picker and writes the picked day into the field in the reader's date format, or YYYY-MM-DD for a
format that writes the month as a word, so the server reads it back as the same day. Without scripting the
button SHALL NOT be drawn.

#### Scenario: Picking a day under Dotted

- **GIVEN** a reader whose date format is Dotted
- **WHEN** they pick 6 November 2026 from the calendar beside the due date field
- **THEN** the field reads `06.11.2026` and saving it sets that day
