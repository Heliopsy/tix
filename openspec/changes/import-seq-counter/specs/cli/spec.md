## ADDED Requirements

### Requirement: Doctor reports task numbering behind its tasks

`tix doctor` SHALL include a `task-numbering` check that warns, naming each project with its counter and highest task number, when a project's counter reads below the highest number its tasks hold, and passes otherwise. The status report SHALL list such projects as `work.numbering_behind`.

#### Scenario: A counter behind its tasks

- **GIVEN** a project whose counter reads 0 while a task holds number 1
- **WHEN** the user runs `tix doctor`
- **THEN** the `task-numbering` check warns and names counter 0 and highest task 1
