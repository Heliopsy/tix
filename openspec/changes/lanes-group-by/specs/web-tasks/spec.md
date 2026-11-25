## ADDED Requirements

### Requirement: Lanes can be grouped by project, assignee, priority, tag or due

The lanes view SHALL offer a Group lanes by control, drawn only when the lanes are, whose choices are
Project, Assignee, Priority, Tag and Due. Status SHALL NOT be offered, since lanes by status are the
board. Every grouping SHALL draw the same selection the project lanes draw: one filter, one page, and
finished tasks ticked in their lanes.

#### Scenario: Lanes by assignee

- **GIVEN** a page holding tasks assigned to two people and one unassigned task
- **WHEN** the reader draws the lanes grouped by assignee
- **THEN** each person has a lane holding exactly their tasks and an Unassigned lane holds the third

#### Scenario: Lanes by priority

- **GIVEN** a page holding a high and a low priority task
- **WHEN** the reader draws the lanes grouped by priority
- **THEN** a high lane is drawn before a low lane, each holding its task, and no other lane is drawn

#### Scenario: Lanes by due

- **GIVEN** a page holding an overdue task and one with no deadline
- **WHEN** the reader draws the lanes grouped by due
- **THEN** an Overdue lane and a No date lane are drawn and no other

### Requirement: The lanes grouping lives in the address

The grouping SHALL be the `lanes` query parameter. The view switch, the filter form, Clear, the pager,
the live refresh and a card action's return SHALL keep it. The control SHALL be a GET form that works
without scripting.

#### Scenario: Switching views keeps the grouping

- **GIVEN** the lanes grouped by priority with a filter
- **WHEN** the reader switches to the list and back to the lanes
- **THEN** the lanes are still grouped by priority and the filter is unchanged

#### Scenario: Choosing a grouping without scripting

- **WHEN** a reader without scripting picks Tag in the control and presses Apply
- **THEN** the screen is drawn grouped by tag with the filter it had

### Requirement: Each screen has its own default grouping

On the task list the grouping SHALL default to the one this browser last chose there, and to Project
when it chose none. On a project page it SHALL default to Assignee, SHALL NOT offer Project, and SHALL
neither read nor write the remembered choice.

#### Scenario: A project page opens its lanes by assignee

- **WHEN** a reader opens a project page's lanes with no `lanes` parameter
- **THEN** the lanes are grouped by assignee and the control does not offer Project

#### Scenario: The task list remembers the last grouping

- **GIVEN** a reader who chose Priority on the task list
- **WHEN** they open the task list's lanes with no `lanes` parameter
- **THEN** the lanes are grouped by priority

### Requirement: Lanes other than projects follow the empty-lane rule

A person, priority or tag SHALL get a lane when a card on the page lands in it, or when an
`assignee:`, `priority:` or `tag:` term names it, in which case an empty lane says so. A due bucket
SHALL get a lane only when a card lands in it.

#### Scenario: A person named by the filter has a lane

- **GIVEN** a filter `assignee:lena` matching no task
- **WHEN** the lanes are grouped by assignee
- **THEN** a lane for lena is drawn and says it holds no tasks

### Requirement: A task with several tags is drawn once

In lanes grouped by tag a card SHALL be drawn exactly once, in the leftmost drawn lane among its tags,
after the reader's arrangement, with tags the filter names ordered first. The lanes SHALL say this
rule, and a task with no tag SHALL be in a No tag lane.

#### Scenario: A card with two tags

- **GIVEN** a task tagged `api` and `ops`
- **WHEN** the lanes are grouped by tag with no arrangement
- **THEN** the card is in the api lane only

#### Scenario: A starred tag lane takes its cards

- **GIVEN** the same task and a reader who starred the ops lane
- **WHEN** the lanes are grouped by tag
- **THEN** the card is in the ops lane only

### Requirement: Lanes are arranged per grouping

The arrangement of lanes SHALL be kept per grouping, so hiding, starring, collapsing or moving a lane
grouped by one thing SHALL NOT change the lanes grouped by another.

#### Scenario: Hiding an assignee lane leaves the project lanes alone

- **GIVEN** a reader who hid lena's lane in the lanes grouped by assignee
- **WHEN** they draw the lanes grouped by project
- **THEN** every project lane is drawn
