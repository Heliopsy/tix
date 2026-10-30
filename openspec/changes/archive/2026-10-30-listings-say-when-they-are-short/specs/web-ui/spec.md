## ADDED Requirements

### Requirement: A screen never presents part of a listing as the whole of it

Where the browser interface renders a figure or a control built from a keyset-paginated listing, it
SHALL either read that listing to its end or state, on the screen, that what it shows is not the
whole set.

A statement of incompleteness SHALL appear with the control or figure it qualifies, and SHALL be
written in the register the rest of the interface uses for such notes.

A figure whose listing stopped before the end SHALL NOT be rendered as a number, since the number
would be a floor presented as a total; the screen SHALL print in its place that the figure is
incomplete, distinguishably from a listing that failed to be read at all.

#### Scenario: A listing that reached the end says nothing

- **GIVEN** a tenant whose projects and actors are both fewer than the bounded walks cover
- **WHEN** a signed-in reader opens the task screen, the settings screen, the statistics screen, the
  token screen or the tenant screen
- **THEN** no control on those screens claims to be short
- **AND** every diagram figure whose walk completed is rendered as a number

#### Scenario: A walk that stopped at its bound is said on the screen

- **GIVEN** a tenant with more projects than the project walk covers
- **WHEN** a reader opens the task screen or the settings screen
- **THEN** the project visibility control states that the list it offers is not the whole set

#### Scenario: A figure whose walk stopped prints no total

- **GIVEN** a tenant with more projects than the project walk covers
- **WHEN** a reader opens the tenant screen
- **THEN** the Projects row of the diagram prints that the count is incomplete, in the figure's place

#### Scenario: One caveat for one control rendered twice

- **GIVEN** the project visibility control, which the task screen and the settings screen both render
- **WHEN** the walk behind its choices stops before the end
- **THEN** both screens state it, from the control's own state rather than from either screen's

### Requirement: The tenant diagram's token figure means what the token screen shows

The count on the tenant diagram's API tokens row SHALL be the number of tokens the token screen lists
for the same reader, and SHALL be produced from the same listing that screen renders.

#### Scenario: An administrator's figure is the tenant's

- **GIVEN** a reader holding `tenant:admin` in a tenant where another actor holds a token
- **WHEN** they read the API tokens figure on the tenant screen and then open the token screen
- **THEN** the figure equals the number of rows the token screen lists
- **AND** the figure is not the count of the reader's own tokens alone

#### Scenario: A reader who cannot list tokens sees no figure

- **GIVEN** a reader who may read the tenant screen but may not list API tokens
- **WHEN** they read the API tokens row
- **THEN** the row states that the figure is unavailable rather than printing one

### Requirement: The token listing reaches every actor of the tenant

The token screen shown to a reader holding `tenant:admin` SHALL list the tokens of every actor the
actor directory holds, walked through the listing's cursor rather than read as a single page.

Where the walk reaches its bound before the end of the directory, the screen SHALL state that the
rows are not every token of the tenant.

#### Scenario: A token held by an actor past the first directory page

- **GIVEN** a tenant with more actors than one listing page holds, and a token belonging to an actor
  past that page
- **WHEN** a reader holding `tenant:admin` opens the token screen
- **THEN** that token is listed, with a revocation control of its own

#### Scenario: A directory longer than the walk

- **GIVEN** a tenant whose actor directory is longer than the bounded walk covers
- **WHEN** a reader holding `tenant:admin` opens the token screen
- **THEN** the screen states that the rows are not every token of the tenant

### Requirement: The statistics project picker offers every project

The project filter on the statistics screen SHALL offer every project of the tenant, read through the
listing's cursor rather than as a single page.

A failure to read the project listing SHALL leave the screen without a picker rather than without
figures, and the screen SHALL state that the picker does not list every project whenever it is
incomplete, whether because the read failed or because the walk stopped at its bound.

#### Scenario: Filtering by a project past the first listing page

- **GIVEN** a tenant with more projects than one default listing page holds
- **WHEN** a reader opens the statistics screen
- **THEN** the project picker offers every project of the tenant
- **AND** choosing one past the first page comes back as the selected option

#### Scenario: A picker that could not be read in full

- **GIVEN** a tenant whose project listing cannot be read to its end
- **WHEN** a reader opens the statistics screen
- **THEN** the figures are rendered
- **AND** the screen states that the picker does not list every project of the tenant
