## ADDED Requirements

### Requirement: The palette creates a task from a template

The palette SHALL offer New task from each template the reader can read, read from
`GET /palette/templates` and matched on its name and on the title it gives, once something is typed.
Choosing one SHALL post the form the Use link on Templates opens. A tenant-wide template's task SHALL go
in the project on screen; with no project on screen the palette SHALL open that form instead.

#### Scenario: A tenant-wide template on a project's board

- **GIVEN** a tenant-wide template named Release and a reader on the board of project infra
- **WHEN** they type "release" in the palette and choose New task from Release
- **THEN** a task is created from the template in infra

#### Scenario: A tenant-wide template with no project on screen

- **GIVEN** a reader on Tasks
- **WHEN** they choose New task from a tenant-wide template
- **THEN** the New from template form opens with that template chosen

### Requirement: The palette switches tenant through the tenant switcher

The palette SHALL offer Switch to each tenant the header's tenant switcher has a Switch button for, once
something is typed, and choosing one SHALL send that button's form. A tenant the switcher offers no
Switch button for SHALL NOT be offered.

#### Scenario: Switching from the palette

- **GIVEN** a reader who belongs to tenants home and test on this host
- **WHEN** they type "test" and choose Switch to Test
- **THEN** the session moves to test as the switcher's Switch button does

### Requirement: The palette picks a colour scheme by name

The palette SHALL offer each colour scheme by name, System, Light, Dark, OLED, Low contrast and
System, OLED when dark, under Colour scheme once something is typed, and choosing one SHALL send the
sidebar's colour scheme switch with that scheme.

#### Scenario: Choosing OLED

- **WHEN** the reader types "oled" and chooses OLED
- **THEN** the colour scheme cookie records `oled`

### Requirement: The palette acts on the task on screen

On a task screen, its own page or the task dialog, the palette SHALL offer under Task what the screen
lets the reader do to the task: change the status, the priority or the due date, assign, add a tag or a
comment, star or unstar, snooze or unsnooze, copy the reference, hand to an agent, rename, write the
description, copy to, move to, save as template and reclaim the lease. Each SHALL be found by words for
it, such as "star" or "assign".

#### Scenario: Starring from the palette

- **GIVEN** a reader on a task they have not starred
- **WHEN** they type "star" and press Enter
- **THEN** the task is starred, as pressing its star does

### Requirement: A task action drives a control the screen draws

Each task action SHALL drive a control the task screen draws and SHALL be offered only when that control
is drawn, so a reader who may not change the task is offered nothing that would. A screen with no task on
it SHALL offer no task action.

#### Scenario: A viewer

- **GIVEN** a reader who may only read the task
- **WHEN** they open the palette on it
- **THEN** it offers Copy reference and no action that changes the task

#### Scenario: A listing

- **WHEN** the reader opens the palette on Tasks
- **THEN** it offers no task action
