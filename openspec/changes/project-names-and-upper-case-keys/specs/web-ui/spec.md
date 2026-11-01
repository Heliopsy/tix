## ADDED Requirements

### Requirement: Project pickers name projects in the reader's chosen style

Every web control a project is chosen in (the add box, Add with details, bulk Copy to, a heading
menu's New task in, Copy to, New from template, a template's project, the move target of a project
delete and the statistics filter) SHALL label each project in the style held in the
`tix_project_label` cookie: `name-key` draws `Name (KEY)`, `key-name` draws `KEY · Name`, `name`
draws `Name` and `key` draws `KEY`. No cookie, or a value naming no style, SHALL draw `Name (KEY)`.
The task screen's Project row SHALL name its project in the same style, linked by the key. A project with no name SHALL be labelled by its key. A project with a text icon SHALL have it before
the label. Each option's value SHALL be the stored project key whatever the style. A name longer than
24 characters SHALL be shortened with an ellipsis, and the key SHALL never be shortened. On a screen
at most 639px wide, the add box and a heading's add field SHALL list each project by its key alone.

#### Scenario: Default style

- **GIVEN** a project `infra` named Infrastructure and no `tix_project_label` cookie
- **WHEN** the reader opens the task screen
- **THEN** the add box offers `<option value="infra">Infrastructure (INFRA)</option>`

#### Scenario: Key first

- **GIVEN** the cookie holds `key-name`
- **WHEN** the reader opens Copy to on a task
- **THEN** the project option reads `INFRA · Infrastructure` and its value is `infra`

#### Scenario: A phone

- **GIVEN** a screen 390px wide
- **WHEN** the reader opens the task screen
- **THEN** the add box's project picker reads `INFRA`, whole

#### Scenario: A long name

- **GIVEN** a project named with more than 24 characters
- **WHEN** a picker lists it
- **THEN** the name ends in an ellipsis and the whole key follows it

### Requirement: The project label style is chosen on Settings

The Settings screen SHALL offer the project label style in its Listings section as a select with an
info tip, posting to `/projectlabel`. Posting a known non-default style SHALL store it in the
`tix_project_label` cookie for a year; posting the default or an unknown value SHALL clear the
cookie. The preference SHALL be listed in `web.Preferences` and documented in the web UI cookie
table.

#### Scenario: Choosing a style

- **WHEN** the reader posts `label=key-name` to `/projectlabel`
- **THEN** the cookie holds `key-name` and Settings shows it selected

#### Scenario: An unknown value

- **WHEN** the reader posts `label=bogus`
- **THEN** no `tix_project_label` cookie is left

### Requirement: Keys and references are drawn upper case in the browser

Every project key and task reference the web interface shows a reader (rows, cards, lanes, the task
screen and its title, breadcrumbs and relations, toasts, aria labels and titles, activity, inbox,
templates, the projects screen, the command palette, Recently viewed and its folded rail names, and the
question asked before a permanent delete) SHALL be drawn upper case, as
`INFRA` and `INFRA-8`. Links, element ids, form values and data attributes SHALL keep the stored
lower-case value. Addresses SHALL answer in any case: `/tasks/INFRA-8` and `/projects/INFRA` SHALL
show the same task and project as their lower-case forms without a redirect, and a filter
`project:INFRA` SHALL select the tasks of `infra`.

#### Scenario: Task screen

- **WHEN** the reader opens `/tasks/infra-1`
- **THEN** the page title and the heading read `INFRA-1` and the project link reads `INFRA` over `/projects/infra`

#### Scenario: Upper-case address

- **WHEN** the reader opens `/tasks/INFRA-1`
- **THEN** the task is shown with status 200
