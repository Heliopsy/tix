## ADDED Requirements

### Requirement: Text is cut by the cells it draws, escape sequences included

The terminal interface's truncation primitive SHALL measure a string by the cells it draws and SHALL
cut it to a given number of cells, marking the cut with an ellipsis.

A string that already carries ANSI escape sequences SHALL be measured as though those sequences were
absent, because they draw nothing, and the sequences SHALL be carried through the cut rather than
counted against the budget or broken in half. A styled string that fits SHALL be returned whole.

Where the budget is one cell, which has no room for both a character and the mark, the character
SHALL be kept and the mark dropped.

#### Scenario: A styled string that fits is untouched

- **GIVEN** a string whose visible text is five cells wide, wrapped in a colour sequence
- **WHEN** it is cut to six cells
- **THEN** its visible text is unchanged and its styling is intact

#### Scenario: A styled string that does not fit keeps its visible text

- **GIVEN** a string whose visible text is eleven cells wide, wrapped in a colour sequence
- **WHEN** it is cut to six cells
- **THEN** it draws six cells
- **AND** the five cells before the ellipsis are the first five characters of the visible text, not
  part of an escape sequence

#### Scenario: One cell keeps the character

- **WHEN** a string, styled or plain, is cut to one cell
- **THEN** it draws the first character of its visible text and no ellipsis
