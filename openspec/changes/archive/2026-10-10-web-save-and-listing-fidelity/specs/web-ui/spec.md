## ADDED Requirements

### Requirement: An emptied form field clears the value it holds

A browser form field that a reader empties and saves SHALL clear the stored value. A field the submission
does not name at all SHALL leave the stored value unchanged.

The distinction SHALL be drawn on whether the submission carries the field's key, not on whether the value
it carries is empty, so that a submission naming some of a record's fields changes only the fields it
names.

This SHALL hold for a task's custom fields and for an account's display name, which are the two values the
browser could previously set and never remove.

Where clearing a value would leave a required field empty, the save SHALL be refused with a validation
error and the stored value SHALL be left as it was. A refused save SHALL NOT report success.

#### Scenario: Emptying a custom field clears it

- **GIVEN** a task whose custom field holds a value
- **WHEN** the reader empties that field's input and saves
- **THEN** the field is shown empty on the task afterwards

#### Scenario: A save naming one field leaves the others alone

- **GIVEN** a task carrying values in two custom fields
- **WHEN** a submission names only the first of them
- **THEN** the first holds the submitted value and the second is unchanged

#### Scenario: A save carrying no custom field inputs changes none of them

- **GIVEN** a task whose custom field holds a value
- **WHEN** the reader saves the task's title from a form carrying no custom field input
- **THEN** the custom field still holds its value

#### Scenario: Emptying a required field is refused

- **GIVEN** a task whose required custom field holds a value
- **WHEN** the reader empties that field's input and saves
- **THEN** the save is refused as invalid
- **AND** the stored value is unchanged

#### Scenario: Emptying a display name removes it

- **GIVEN** an account with a display name
- **WHEN** an administrator empties the name input and saves
- **THEN** the account has no display name afterwards

#### Scenario: A save naming no display name leaves it alone

- **GIVEN** an account with a display name
- **WHEN** a submission changes the account's role and names no display name
- **THEN** the account keeps its display name

### Requirement: Putting a project away removes that project and nothing else

The task listing's visibility control SHALL remove from the listing exactly the projects the reader has
put away, whatever the number of projects in the tenant.

The listing SHALL be narrowed by excluding the projects put away, rather than by naming the projects to
show, so that the rows shown do not depend on any listing of projects having been complete.

The control SHALL offer a box for every project in the tenant, so that a project can be put away and
brought back whatever its position in the project listing.

#### Scenario: Hiding one project of many keeps the rest

- **GIVEN** a tenant with more projects than one page of the project listing returns
- **AND** a task in a project past that first page
- **WHEN** the reader puts a single other project away
- **THEN** that task is still listed

#### Scenario: The control names every project

- **GIVEN** a tenant with more projects than one page of the project listing returns
- **WHEN** the task listing is rendered
- **THEN** the visibility control offers a box for every project in the tenant

### Requirement: A browser screen reporting a whole-tenant figure counts the whole tenant

Where a browser screen presents a count as the number of records of a kind in the tenant, that count SHALL
be taken over all of them rather than over one page of a listing.

Where the count cannot be established, the screen SHALL say so in place of the figure rather than show a
number that is short.

A browser screen that resolves a record by identifier out of a listing SHALL find any record in the
tenant, not only one on the listing's first page.

#### Scenario: The tenant diagram counts every project

- **GIVEN** a tenant with more projects than one page of the project listing returns
- **WHEN** the tenant diagram is rendered
- **THEN** its project row shows the number of projects in the tenant

#### Scenario: A task in a distant project can be finished

- **GIVEN** a task in a project past the first page of the project listing
- **WHEN** the reader ticks it done
- **THEN** the task is finished

### Requirement: The delivery log offers its next page

The webhook delivery log SHALL render the position control every keyset listing on this surface renders,
offering the following page whenever one exists.

#### Scenario: An older delivery is reachable

- **GIVEN** more deliveries than one page of the log shows
- **WHEN** the log is rendered
- **THEN** it offers a link to the next page
- **AND** that page shows deliveries the first page did not
