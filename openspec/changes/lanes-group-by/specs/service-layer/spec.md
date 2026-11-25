## ADDED Requirements

### Requirement: Lanes grouped by something other than the project are arrangeable views

The view arrangement operations SHALL accept `lanes:assignee`, `lanes:priority`, `lanes:tag` and
`lanes:due`. An assignee group SHALL be an actor of the tenant, stored by identifier and shown by
handle; a priority group SHALL be a priority name; a tag group SHALL be a tag of the tenant; a due
group SHALL be a deadline bucket. `-` SHALL name the Unassigned and No tag groups.

#### Scenario: An assignee lane is named by handle

- **GIVEN** a caller who starred group `lena` on `lanes:assignee`
- **WHEN** they list the view's preferences
- **THEN** the starred group is named `lena`, and starring it again by lena's actor id changes the same preference

#### Scenario: An unknown priority is refused

- **WHEN** a caller hides group `urgent` on `lanes:priority`
- **THEN** the call fails as invalid and nothing is written
