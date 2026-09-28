## ADDED Requirements

### Requirement: The registry records the status operation and the one surface it does not reach

The capability registry SHALL hold exactly one operation for the new `Status` service method, bound to the
command line, the HTTP API and the browser.

It SHALL record a terminal gap rather than an exemption, because the terminal interface has no
administration screen at all and the server is one more thing it cannot show, which is a parity defect kept
visible rather than an operation a terminal cannot serve.

The declared scope SHALL be the scope the service actually enforces, which the authority gate proves
against the real service rather than taking on trust.

The recorded terminal gap count SHALL rise by exactly one, so that opening a gap and raising the count
happen in one change and neither can drift from the other. The operation count SHALL rise by exactly one
for the same reason.

The prose in `docs/tui.md` SHALL state the new figures, both the gap count and the operation count, because
a page that names the old one is a page that has quietly stopped being true.

#### Scenario: The operation is declared once

- **WHEN** the registry is asked for the operation bound to `Status`
- **THEN** exactly one operation names that method
- **AND** it declares a command line binding, an HTTP route and a browser screen

#### Scenario: The terminal absence is recorded as a gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** the status operation is among them
- **AND** its reason is marked as a gap rather than as an operation the terminal cannot serve

#### Scenario: The counts move together

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** the gate fails if the gap is closed without lowering the number

#### Scenario: The page states the figures the registry holds

- **WHEN** the terminal documentation's gap section is read
- **THEN** every number it states in digits is one the registry holds
- **AND** the shares it breaks the gaps into still add up to the gap count

### Requirement: The browser screen serves the same report

The browser interface SHALL offer a status screen reading the same operation, so the capability is bound on
the web rather than exempted.

The screen SHALL show the installation figures, the servers with their attached state, and the work counts
for the tenant the session is pinned to.

A server that has stopped heartbeating SHALL be visibly distinguished on the screen rather than omitted.

#### Scenario: The screen is served and names its operation

- **WHEN** the browser routes are enumerated
- **THEN** the status route names the `Status` service method
- **AND** the template it declares is one the build embeds

#### Scenario: A stale server is visible in the browser

- **WHEN** the screen renders a server that stopped heartbeating
- **THEN** that server appears
- **AND** it is marked as not heartbeating
