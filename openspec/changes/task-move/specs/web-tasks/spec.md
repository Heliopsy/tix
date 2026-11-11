## ADDED Requirements

### Requirement: Move to another project in the browser

The task screen's actions menu SHALL offer "Move to…" beside "Copy to…" to a reader who may write tasks. It
SHALL open a form listing every project but the task's own, and moving SHALL open the task under its new
reference with a toast that says when the status was not kept. A refusal SHALL come back in the form.

#### Scenario: Read-only reader

- **WHEN** a viewer opens a task
- **THEN** no Move to… action is drawn

#### Scenario: Old address

- **WHEN** a task moved from INFRA-1 to WEB-1 and a reader opens `/tasks/infra-1`
- **THEN** the answer is a permanent redirect to `/tasks/web-1`

### Requirement: Bulk move to a project in the browser

The bulk selection bar SHALL offer "Move to…" with a project choice that moves every selected task to it and
answers with a toast counting the moved tasks and naming each refusal.

#### Scenario: Bulk move

- **WHEN** a reader selects two tasks and moves them to WEB
- **THEN** both are in WEB and the toast says two tasks were moved
