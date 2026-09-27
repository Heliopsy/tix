## ADDED Requirements

### Requirement: The number of rows a listing shows is configured per surface

Each surface that renders a listing SHALL take its page size from its own configuration key: `cli.page_size`
for the command line and `web.page_size` for the browser. Both SHALL default to 25 rows, and both SHALL be
resolved through the documented layers, where a flag beats the environment, which beats a `.env` entry, which
beats a configuration file, which beats the built-in default.

#### Scenario: A configuration file decides how many rows a listing prints

- **WHEN** `cli.page_size` names a number in a configuration file and a listing command runs with no `--limit`
- **THEN** the listing prints that many rows, and reports that a further page is available

#### Scenario: The environment decides how many rows a listing prints

- **WHEN** `TIX_CLI_PAGE_SIZE` names a number and a listing command runs with no `--limit`
- **THEN** the listing prints that many rows

#### Scenario: A typed limit still wins

- **WHEN** `cli.page_size` names one number and `--limit` names another
- **THEN** the listing prints the number the flag names

#### Scenario: A limit inside a filter expression still wins

- **WHEN** `cli.page_size` names one number and `--filter` carries a `limit:` term naming another
- **THEN** the listing prints the number the expression names

#### Scenario: The browser shows the configured number of rows

- **WHEN** `web.page_size` names a number and a browser listing is opened
- **THEN** that page carries that many rows

#### Scenario: Nothing configured shows twenty-five rows

- **WHEN** no layer sets a page size and a browser listing is opened
- **THEN** that page carries 25 rows

### Requirement: A page size outside the contract's range is refused

A configured page size below 1 or above the maximum a listing may return SHALL be refused when configuration
is resolved, naming the key and the layer that supplied it. It SHALL NOT be clamped or replaced by a default.

#### Scenario: A page size of zero is refused

- **WHEN** a layer sets `cli.page_size` to `0`
- **THEN** the command fails, names `cli.page_size`, and prints no rows

#### Scenario: A negative page size is refused

- **WHEN** a layer sets `web.page_size` to a negative number
- **THEN** configuration is refused, naming `web.page_size`

#### Scenario: An absurd page size is refused

- **WHEN** a layer sets a page size above the maximum a listing may return
- **THEN** configuration is refused, naming that key

### Requirement: A reader can ask one browser page for a different size

A browser listing SHALL accept a `limit` query parameter naming how many rows that page carries, overriding
the configured size for that request. The listing's own paging controls SHALL carry the requested size, so a
walk keeps the size it was asked for. A `limit` that is not a usable number SHALL leave the configured size
in place rather than failing the screen.

#### Scenario: A request asks for more rows than the configured size

- **WHEN** a browser listing is opened with a `limit` naming a number inside the accepted range
- **THEN** that page carries that many rows

#### Scenario: Paging keeps the requested size

- **WHEN** a reader opens a listing with a `limit` and follows the next-page control
- **THEN** the following page carries the same number of rows

#### Scenario: An unusable requested size is ignored

- **WHEN** a browser listing is opened with a `limit` that is not a number, is zero, is negative, or is above
  the maximum a listing may return
- **THEN** the page renders with the configured size
