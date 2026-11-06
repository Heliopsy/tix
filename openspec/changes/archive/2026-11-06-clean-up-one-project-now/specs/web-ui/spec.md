## ADDED Requirements

### Requirement: A board cleans up its own project now

A project's Housekeeping panel SHALL offer Clean up now to a tenant admin, which runs housekeeping once for
that project only, after a confirmation that names how many of its tasks would go to the trash and how many
would be deleted for good. It SHALL be disabled, with the reason, while nothing is due or the trash is off.
After the run the board SHALL say how many tasks were moved to the trash and how many deleted for good.

#### Scenario: Only the project is cleaned up

- **GIVEN** a done rule of 1 day and a task done 2 days ago in each of INFRA and OPS
- **WHEN** an admin confirms Clean up now on the INFRA board
- **THEN** the INFRA task is in the trash, the OPS task is not, and the board says 1 was moved to the trash

#### Scenario: A run needs the confirmation

- **WHEN** a post to the project's cleanup run arrives without the confirmation
- **THEN** nothing is moved and the request is refused

#### Scenario: Nothing to do

- **GIVEN** no task is due, or the trash is off
- **WHEN** an admin opens the board's Housekeeping panel
- **THEN** Clean up now is disabled and says why

#### Scenario: A member cannot run it

- **WHEN** a member who is not a tenant admin posts to the project's cleanup run
- **THEN** the request is refused as forbidden and nothing is moved
