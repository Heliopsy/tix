## ADDED Requirements

### Requirement: The New project workflow picker leads with each template's purpose

The New project dialog SHALL offer the workflow as a list of radio choices, the tenant's workflows first and
then the built-in templates. Each template choice SHALL show its purpose first, then its name with a Built-in
label, its states and its fields, in the reader's language. Choosing a template SHALL create the project from
it. The form SHALL work without scripting.

#### Scenario: Purpose first

- **WHEN** a reader opens New project
- **THEN** the Sales template's choice draws Track deals from first contact to signed. before Sales or BizDev pipeline and its Built-in label

#### Scenario: Creating from a template without scripting

- **WHEN** the form is posted with workflow `template:tix-sales`
- **THEN** the project is created on `tix-sales` and carries the template's fields

### Requirement: The Workflows screen adds a workflow from a template

The Workflows screen SHALL carry an Add from template button that opens a dialog listing the templates purpose
first, with an optional key and a choice of what to do when the key is taken. Without scripting the button
SHALL be a link that draws the dialog in place, and a refused submission SHALL come back with the dialog open.

#### Scenario: Applying a template from the browser

- **WHEN** a reader submits Add from template with `tix-bugs`
- **THEN** the browser is sent to the new workflow, and submitting it again comes back refused in the dialog
