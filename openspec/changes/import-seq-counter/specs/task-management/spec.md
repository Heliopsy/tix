## ADDED Requirements

### Requirement: The next task number is above every number held

Reserving a project's next task number SHALL advance its counter past both the counter's own value and the highest number any task of the project holds, trashed ones included, so a counter left behind never reissues a number. The counter SHALL never move backwards.

#### Scenario: A counter behind its tasks

- **GIVEN** a project whose counter reads 0 while a trashed task holds number 868
- **WHEN** a task is created in it
- **THEN** the task is numbered 869

### Requirement: Existing counters are repaired on upgrade

The upgrade SHALL raise every project's counter that reads below the highest number its tasks hold to that number, on both engines and on PostgreSQL under forced row-level security, and SHALL leave a counter already above its tasks unchanged.

#### Scenario: Repairing counters an import left behind

- **GIVEN** a project with a counter of 0 and tasks up to 95, and another with a counter of 50 and tasks up to 2
- **WHEN** the database is upgraded
- **THEN** the first counter reads 95 and the second still reads 50
