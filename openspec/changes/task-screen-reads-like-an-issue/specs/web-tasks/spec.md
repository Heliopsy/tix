## ADDED Requirements

### Requirement: The task screen is a header, a main column and a properties panel

The task screen SHALL draw a header holding the task's reference, its title, the reader's star and
one actions menu; a main column holding, in order, the description, the subtasks, the
dependencies, the artifacts, the comments and the history; and a properties panel. On a viewport
at least 1024 pixels wide the properties panel SHALL sit to the right of the main column. Below
that width the properties panel SHALL come directly under the header and the record's facts SHALL
come last. The dialog a listing opens a task in SHALL draw the same screen.

#### Scenario: Wide screen

- **WHEN** a reader opens a task on a 1440 pixel wide screen
- **THEN** the description and comments are in the left column and the properties in the right

#### Scenario: Phone

- **WHEN** a reader opens a task on a 390 pixel wide screen
- **THEN** the properties follow the title and precede the description

### Requirement: Every property is one row of the properties panel

The properties panel SHALL hold one row per property, each a label and a value: status, priority,
snooze state when snoozed, claim state when claimed or once claimed, assignee, due date, project,
tags, repeat rule and each custom field the project defines, followed by the record's facts:
creator, created, updated, started and finished when set, version and identifier. For a reader who
may write, the status and priority SHALL open their menus, and the assignee, due date, tags, repeat
rule and each custom field SHALL be changed from a form in their own row that names only that
property. A reader who may not write SHALL see the same rows with no form in them. No property
SHALL be edited anywhere else on the screen.

#### Scenario: Changing the assignee in place

- **WHEN** a writer types a handle into the assignee row and saves
- **THEN** the task is assigned to that actor and no other property changes

#### Scenario: An enum field

- **WHEN** a project defines an enum field
- **THEN** its row offers the field's options

#### Scenario: A reader who may not write

- **WHEN** a reader without write access opens a task
- **THEN** the properties panel holds no form

### Requirement: One actions menu holds every action on the task as a whole

The header SHALL open exactly one menu, and that menu SHALL hold Copy link, Copy the task id, and
for a writer Copy to, Save as template and Snooze (or Unsnooze for a snoozed task), Reclaim the
lease where the reader may reclaim a live lease, and Delete with its permanent and subtask options
where the reader may delete. None of these SHALL be drawn anywhere else on the screen.

#### Scenario: The menu

- **WHEN** a writer opens the actions menu
- **THEN** it offers Copy link, Copy the task id, Copy to, Save as template, Snooze and Delete

#### Scenario: No second copy

- **WHEN** the task screen is drawn
- **THEN** Copy to, Save as template and Delete each appear once, in the menu

### Requirement: The description edits in place

The description SHALL be drawn as rendered text with its checklist tickable. A writer SHALL be
offered an Edit switch that replaces the rendered text with the editor in place and reads Cancel
while open, and a task with no description SHALL offer Add a description in its place. The switch
SHALL work without a script. There SHALL be no separate edit form repeating the description or the
other properties.

#### Scenario: Editing

- **WHEN** a writer presses Edit over a description
- **THEN** the text gives way to the editor holding it, with Save and a Cancel switch

#### Scenario: Empty

- **WHEN** a writer opens a task with no description
- **THEN** the screen offers Add a description and draws no empty editor

### Requirement: The browser sets and clears a due date

The task form SHALL accept `due_at` as a date. A form naming it with a date SHALL set the due date
to that day; a form naming it blank SHALL clear it; a form that does not name it SHALL leave the
due date unchanged.

#### Scenario: Set, keep, clear

- **WHEN** a writer saves the due date row as 2099-03-04, then saves another property, then saves the due date row empty
- **THEN** the task is due 2099-03-04, still due then, and finally has no due date

### Requirement: A shortcut reaches a field behind a disclosure

A keyboard shortcut whose target is inside a closed disclosure SHALL open the disclosure before
focusing the target.

#### Scenario: Edit the title

- **WHEN** a reader presses the edit-title shortcut on a task screen
- **THEN** the Rename disclosure opens and the title field has focus

## MODIFIED Requirements

### Requirement: The browser sets and shows a repeat rule

The task screen's Repeats row and the add-task form SHALL offer a Repeat field taking the rule
syntax, with a hint naming an example. Saving the Repeats row with the field emptied SHALL remove
the rule; a form that does not carry the field SHALL leave the rule unchanged, and no other form on
the task screen SHALL carry it. A rule the service refuses SHALL be reported on the form rather
than dropped. The task screen SHALL state the rule and, once the task has spawned, link its
successor.

A listing row and a board card for a task carrying a rule SHALL show a small repeat marker whose
accessible name states the rule. Completing a recurring task from the browser SHALL create the next
occurrence exactly as on every other surface.

#### Scenario: Setting a rule from the task screen

- **WHEN** a reader enters `every week` in the Repeats row and saves
- **THEN** the task carries `every week` and the screen states it

#### Scenario: Marker on a row

- **WHEN** the listing shows a task carrying `every month`
- **THEN** its row shows the repeat marker naming `every month`, and a row of a task with no rule shows none

#### Scenario: Refused rule

- **WHEN** a reader saves the Repeat field as `every fortnight`
- **THEN** the form reports the refusal and the task is unchanged

### Requirement: The task screen states the star, the parent and the next occurrence

The task screen's star control SHALL state in words, through its accessible name and pressed
state, whether the reader has starred the task. A subtask's screen SHALL link the task it belongs
to. A repeating task's screen SHALL state its rule and, when the next occurrence exists, link it
with its due date; when it does not exist yet, SHALL state the date finishing the task now would
give it, computed by the same rule completion applies; and when the rule has run out, SHALL say
that this is the last occurrence. When the task has started or finished, the screen SHALL say when.

#### Scenario: A weekly task not yet finished

- **GIVEN** an unfinished task due on a Monday that repeats `every week`
- **WHEN** its screen is drawn
- **THEN** it states the rule and that finishing now makes the next one due the following Monday

#### Scenario: A finished occurrence

- **GIVEN** a repeating task that has been finished and created its next occurrence
- **WHEN** its screen is drawn
- **THEN** it links the next occurrence with that occurrence's due date

#### Scenario: The last occurrence

- **GIVEN** a task whose rule has one occurrence left
- **WHEN** its screen is drawn
- **THEN** it says this is the last occurrence

#### Scenario: Starred and a subtask

- **GIVEN** a starred subtask
- **WHEN** its screen is drawn
- **THEN** its star control is pressed and named Unstar, and the screen links its parent
