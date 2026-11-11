## ADDED Requirements

### Requirement: An import or sync names the token it ran with

An Activity row, a task History row and the raw view SHALL name the API token an import or sync was started
with, as via import (token NAME) or via sync (token NAME), using the name the token had when the change was
made.

#### Scenario: A sync started with a token

- **WHEN** a task's History shows a change a sync made after the ci-runner token started it
- **THEN** its Channel column reads via sync (token ci-runner) and its hover says it started from api

#### Scenario: The token was deleted since

- **WHEN** the token that started an import has been deleted
- **THEN** the row still reads via import (token NAME) with the name it had then

### Requirement: Sync runs names the token a run was started with

Each row of the Sync runs table SHALL show, beside the actor that ran it, the name of the API token the run was
started with, and only the actor when no token started it.

#### Scenario: An agent runs a sync with a token

- **WHEN** deploy-bot starts a sync with the ci-runner token
- **THEN** that run's Ran as cell reads deploy-bot via token ci-runner

#### Scenario: A run from the browser

- **WHEN** a person starts a sync from the web screen
- **THEN** that run's Ran as cell shows only the person
