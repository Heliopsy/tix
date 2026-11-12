## MODIFIED Requirements

### Requirement: Templates in the browser

The task screen's actions SHALL offer Save as template, a form taking the name, the scope and
whether subtasks are kept. A Templates screen, a main sidebar entry, SHALL list the templates a reader
can see with their scope and use count, and let a reader rename, edit the fields of, use and
delete each. The add box menu and the Add with details form SHALL offer New from template,
which takes a template, a title, a project for a tenant-wide template and a due offset.

#### Scenario: Save from the task screen

- **WHEN** a reader opens OPS-3, picks Save as template and saves it as "release"
- **THEN** the task screen returns with a confirmation and the Templates screen lists "release"

#### Scenario: New from template

- **WHEN** a reader picks New from template in the add box menu, chooses "release" and submits
- **THEN** the new task opens, holding the template's title and subtasks

### Requirement: One actions menu holds every action on the task as a whole

The header SHALL open exactly one actions menu, beside the Share menu that holds Copy link. It SHALL
hold Copy reference, and for a writer Copy to, Save as template and Snooze (or Unsnooze for a snoozed
task), Reclaim the lease where the reader may reclaim a live lease, and Delete with its permanent and
subtask options where the reader may delete. None of these SHALL be drawn anywhere else on the screen.

#### Scenario: The menu

- **WHEN** a writer opens the actions menu
- **THEN** it offers Copy reference, Copy to, Save as template, Snooze and Delete

#### Scenario: No second copy

- **WHEN** the task screen is drawn
- **THEN** Copy to, Save as template and Delete each appear once, in the menu

## ADDED Requirements

### Requirement: Copy reference copies the reference as shown

The actions menu's Copy reference SHALL copy the task's reference as the screen displays it, not its
id. The Id property row SHALL keep its own Copy button, labelled for screen readers "Copy the task id",
which copies the id.

#### Scenario: Copying from the menu

- **GIVEN** task infra-5
- **WHEN** a reader picks Copy reference in its actions menu
- **THEN** INFRA-5 is copied, and the menu offers no way to copy the id

### Requirement: The add box shows the quick-add hint line

The one-line add box's placeholder SHALL be `Fix login !high #auth @raj due:fri +web ^web-3`, the hint
line the tix app shows too, with only "Fix login" translated. Its accessible name SHALL stay "New task"
and its (i) tip SHALL keep listing the syntax.

#### Scenario: A Bulgarian reader

- **GIVEN** a reader whose language is Bulgarian
- **WHEN** the task listing is drawn
- **THEN** the placeholder is the translated words followed by `!high #auth @raj due:fri +web ^web-3`

### Requirement: The task screen says a task is waiting on its dependencies

A task with an unfinished dependency SHALL be marked "waiting on another task" beside its status on the
task screen, never "blocked", which names a workflow state category.

#### Scenario: A task waiting on another

- **GIVEN** INFRA-5 depends on INFRA-4, which is not finished
- **WHEN** a reader opens INFRA-5
- **THEN** its status reads waiting on another task, and the word blocked is not drawn for it
