## ADDED Requirements

### Requirement: Day-first and long date formats

`output.time_format` SHALL accept `long` (21 Sep 2026 14:05), `dotted` (21.09.2026 14:05) and `dayfirst`
(21/09/2026 14:05) beside `iso`, `rfc3339`, `short`, `us` and `relative`. The web settings, the TUI
settings and the configuration file SHALL offer the same list.

#### Scenario: A dotted format renders day first

- **GIVEN** `output.time_format: dotted`
- **WHEN** an instant of 2026-09-21T14:05Z is shown in UTC
- **THEN** it reads `21.09.2026 14:05`

### Requirement: A typed date is read in the format's order

A date a person types SHALL be read in the order their date format writes dates: day first under `dotted`
and `dayfirst`, month first under `us`. `YYYY-MM-DD` and a date with the month as a word SHALL be read
under every format. A date written with digits alone in an order the format does not name SHALL be
refused, never guessed.

#### Scenario: Day first under dotted

- **GIVEN** a reader whose format is `dotted`
- **WHEN** they type `06.11.2026` as a due date
- **THEN** the task falls due on 6 November 2026

#### Scenario: Month first under us

- **GIVEN** a reader whose format is `us`
- **WHEN** they type `06/11/2026` as a due date
- **THEN** the task falls due on 11 June 2026

#### Scenario: No order, no guess

- **GIVEN** a reader whose format is `iso`
- **WHEN** they type `06/11/2026` as a due date
- **THEN** it is refused and no due date is set
