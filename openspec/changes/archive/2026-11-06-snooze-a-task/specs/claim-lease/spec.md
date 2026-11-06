## ADDED Requirements

### Requirement: A snoozed task is not claimable

Claiming the next task SHALL skip every task whose wake time is after the present. Claiming a
snoozed task by reference SHALL fail as a precondition naming its wake time. A task snoozed while
claimed SHALL keep its lease.

#### Scenario: The queue skips a snoozed task

- **GIVEN** the only open task in OPS is snoozed
- **WHEN** an agent claims the next task in OPS
- **THEN** no task is available

#### Scenario: Claiming a snoozed task by reference

- **GIVEN** OPS-3 is snoozed until tomorrow
- **WHEN** an agent claims OPS-3
- **THEN** the claim fails as a precondition

#### Scenario: A woken task is claimable

- **GIVEN** OPS-3 was snoozed until 10:00
- **WHEN** an agent claims the next task at 10:01
- **THEN** it receives OPS-3
