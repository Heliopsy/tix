## ADDED Requirements

### Requirement: The position control counts the page it is on

The position control a keyset listing renders SHALL state which page of the walk the reader is on,
counting from one, and that number SHALL keep counting for as long as the walk continues. It SHALL
NOT stall at the bound of any memory the control keeps in its own links.

The number SHALL be derived from what the control's own links carry. Where that memory is bounded and
has discarded the earliest of it, the number SHALL be carried alongside rather than recomputed from
what is left.

A number arriving from the address bar SHALL NOT be believed below what the link's own history
already proves, SHALL be bounded, and SHALL reach no query. The worst a value a reader typed can cost
is a wrong label on the control.

A link short of that bound SHALL carry no page number of its own, since its history already counts
it.

#### Scenario: Walking past the bound of the control's memory

- **GIVEN** a listing with more pages than the control's cursor memory holds
- **WHEN** a reader follows the control forward past that bound
- **THEN** each page states a number one higher than the page before it

#### Scenario: Walking back past the bound

- **WHEN** the reader follows the control back from a page past that bound
- **THEN** each page states a number one lower than the page before it, for every page the control
  can still reach

#### Scenario: A number below what the history proves is ignored

- **GIVEN** a page whose link carries a history of forty positions
- **WHEN** the address bar names a page number lower than that history accounts for, or a value that
  is not a number at all
- **THEN** the control states the number the history itself counts

#### Scenario: An early page's link carries no page number

- **WHEN** a reader follows the control forward or back from a page within the bound of its memory
- **THEN** the resulting address carries no page number parameter
