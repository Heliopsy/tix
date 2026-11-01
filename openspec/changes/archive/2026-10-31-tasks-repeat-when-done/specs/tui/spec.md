## ADDED Requirements

### Requirement: The terminal interface marks recurring tasks

A board card for a task carrying a recurrence rule SHALL show a repeat marker, and the task detail view
SHALL state the rule in its canonical spelling. The terminal interface does not edit due dates, and SHALL
NOT edit the rule either; it is set from the command line, the browser or the API.

#### Scenario: Card marker

- **WHEN** the board shows a task carrying `every week`
- **THEN** its card carries the repeat marker and a card for a task with no rule does not

#### Scenario: Detail

- **WHEN** the detail view is opened on a task carrying `every 2 weeks on mon,thu`
- **THEN** it states `every 2 weeks on mon,thu`
