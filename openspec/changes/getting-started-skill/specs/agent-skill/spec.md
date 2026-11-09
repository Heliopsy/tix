## ADDED Requirements

### Requirement: A getting-started skill asks how the person wants to use tix first

The repository SHALL ship a getting-started skill beside the main skill. It SHALL direct an agent to ask, before
explaining or installing anything, which interface the person wants (desktop app, browser, Android app, CLI, TUI,
SSH or an AI agent) and whether they already have a server and an account or want to run tix locally or
self-host.

#### Scenario: An agent helps a person new to tix

- **WHEN** an agent loads the getting-started skill for a person who has not used tix
- **THEN** its first step is the two questions, before the concepts and the setup

### Requirement: The getting-started skill explains the concepts with one example

The getting-started skill SHALL explain instance, tenant, project, task, workflow, credentials and roles in plain
language, using one example that runs through every concept.

#### Scenario: A person asks what a tenant is

- **WHEN** a reader reads the concepts step
- **THEN** a tenant is described as a separate workspace on an instance that one person can belong to several of

### Requirement: The getting-started skill keeps a newcomer off a real store

The getting-started skill SHALL tell the agent not to write to a real store without asking, to try things on a
scratch database named with `--db`, and never to print a password, token, invite link or recovery code.

#### Scenario: An agent demonstrates tix locally

- **WHEN** an agent follows the skill's local walkthrough
- **THEN** every command it shows names a scratch database

### Requirement: The getting-started skill is guarded like the main skill

Every command and flag the getting-started skill names SHALL be checked against the real command tree, it SHALL
carry exactly one `verified-against` marker naming an ancestor of HEAD, and it SHALL NOT link a relative file it
does not ship.

#### Scenario: The getting-started skill names a flag that does not exist

- **WHEN** the skill passes a flag no command registers
- **THEN** the guard fails and names the skill file

### Requirement: Newcomers are pointed at the getting-started skill

The README and the documentation index SHALL point a person new to tix, or an agent helping one, at the
getting-started skill before the rest of their agent guidance.

#### Scenario: A reader opens the docs index

- **WHEN** a reader opens docs/README.md
- **THEN** a pointer to the getting-started skill comes before the guides
