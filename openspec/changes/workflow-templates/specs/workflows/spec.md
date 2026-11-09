## ADDED Requirements

### Requirement: Built-in workflow templates

tix SHALL ship twelve built-in workflow templates, each with a key starting `tix-`, a display name without
that prefix, a one-line purpose, a definition that passes the workflow validator and the custom fields a
project using it usually wants. Listing them SHALL return each one's key, name, purpose, a states summary,
definition and fields, purpose being the first thing every list or picker shows.

#### Scenario: Listing the templates

- **WHEN** a reader holding `workflow:read` lists the workflow templates
- **THEN** twelve are returned, starting with `tix-kanban`, each carrying a non-empty key, name and purpose

#### Scenario: Every template is a valid workflow

- **WHEN** each template's definition is validated
- **THEN** it passes, and every non-final state has a route to a final one

### Requirement: Applying a template creates a workflow

Applying a template SHALL create a workflow from its definition under the template's key, or under the key
the caller names, writing the workflow, its audit entry and its event in one transaction. A taken key SHALL be
refused as a conflict unless the caller asks to skip it, rename to the first free `KEY-N`, or replace the
existing definition. Replacing the builtin workflow SHALL be refused.

#### Scenario: Applying under the template's key

- **WHEN** an admin applies `tix-dev-review`
- **THEN** a workflow `tix-dev-review` named Development with review exists and a project can be created on it

#### Scenario: A taken key

- **GIVEN** a workflow `tix-kanban` already exists
- **WHEN** the template is applied again with no policy
- **THEN** the call fails as a conflict, and with rename it creates `tix-kanban-2`

### Requirement: A project can be created from a template

Creating a project with a workflow template SHALL run the project on the workflow under the template's key,
creating it first when the tenant does not hold it, and SHALL add the template's custom fields to the
project, in the same transaction. Naming both a workflow and a template SHALL be refused as invalid.

#### Scenario: Fields come with the project

- **WHEN** a project is created with workflow template `tix-sales`
- **THEN** it runs on the `tix-sales` workflow and carries the fields company, deal_value and expected_close

#### Scenario: The workflow is reused

- **GIVEN** a project already created from `tix-sales`
- **WHEN** a second project is created from it
- **THEN** both run on the same workflow

### Requirement: The template key prefix is reserved

Defining a new workflow by hand whose key starts with `tix-` SHALL be refused as invalid on the key field. A
workflow a template created SHALL still be redefinable by its key, and bundle or snapshot imports SHALL NOT be
refused for the prefix.

#### Scenario: A hand-made tix- workflow

- **WHEN** a caller puts a new workflow `tix-mine`
- **THEN** the call fails as invalid naming the key, and nothing is written

### Requirement: Approval edges name the task:approve scope

A `task:approve` scope SHALL exist, carried by no role but admin. The `tix-agent-review` and `tix-approval`
templates SHALL require it to approve and to reject, so the worker doing the work cannot sign it off.

#### Scenario: A member cannot approve agent work

- **GIVEN** a task in `needs_review` on `tix-agent-review`
- **WHEN** a member without `task:approve` moves it to `approved`
- **THEN** the move is refused as forbidden, and an actor holding `task:approve` can make it

### Requirement: Template rules are enforced like any workflow's

A workflow made from a template SHALL enforce its edges' comment and scope requirements and its lease reverts
exactly as a hand-written one does.

#### Scenario: Sending work back from review

- **GIVEN** a task in `in_review` on `tix-dev-review`
- **WHEN** it is moved to `in_progress` without a comment
- **THEN** the move is refused as invalid, and with a comment it succeeds

#### Scenario: Losing a deal

- **WHEN** a deal on `tix-sales` is moved to `lost` without a comment
- **THEN** the move is refused as invalid
