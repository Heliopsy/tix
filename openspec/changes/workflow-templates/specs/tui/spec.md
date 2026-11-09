## ADDED Requirements

### Requirement: The new project form picks a workflow

After the new project prompt the terminal SHALL open a form choosing the workflow when the tenant has more than
one or there are templates to offer, and create the project at once otherwise. While a template is selected the
form SHALL show its purpose first, then its name marked Built-in, its states and its fields, and submitting it
SHALL create the project from the template.

#### Scenario: Choosing a template

- **GIVEN** a tenant with the default workflow and the built-in templates
- **WHEN** the reader enters `deals Deals`, cycles to `tix-sales` and submits
- **THEN** the project deals is created with workflow template `tix-sales`
