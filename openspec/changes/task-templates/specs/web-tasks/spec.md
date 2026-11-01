## ADDED Requirements

### Requirement: Templates in the browser

The task screen's actions SHALL offer Save as template, a form taking the name, the scope and
whether subtasks are kept. A Templates screen under Configure SHALL list the templates a reader
can see with their scope and use count, and let a reader rename, edit the fields of, use and
delete each. The add box menu and the Add with details form SHALL offer New from template,
which takes a template, a title, a project for a tenant-wide template and a due offset.

#### Scenario: Save from the task screen

- **WHEN** a reader opens OPS-3, picks Save as template and saves it as "release"
- **THEN** the task screen returns with a confirmation and the Templates screen lists "release"

#### Scenario: New from template

- **WHEN** a reader picks New from template in the add box menu, chooses "release" and submits
- **THEN** the new task opens, holding the template's title and subtasks
