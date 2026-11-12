## ADDED Requirements

### Requirement: The projects screen counts each project's open tasks

Each row on `/projects` SHALL show how many of the project's tasks are open: in a state the workflow
does not mark terminal, not trashed and not snoozed. A project with none SHALL read "no open tasks". The
counts SHALL be read with one tallied listing for the screen, not one read per project.

#### Scenario: Open and finished tasks

- **WHEN** project infra holds three open tasks and one cancelled task, and project web holds none
- **THEN** the infra row reads "3 open tasks" and the web row reads "no open tasks"
