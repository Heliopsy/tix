## ADDED Requirements

### Requirement: Task text renders as sanitised Markdown

The task screen SHALL render a task's description and each comment as CommonMark with the GitHub
tables, task lists, autolinks, strikethrough and fenced code. Raw HTML SHALL be shown escaped as the
text that was typed. A link SHALL be rendered only when its address is absolute http or https with a
host, or mailto, and SHALL carry `rel="noopener noreferrer nofollow"`; any other address SHALL render as
its label text. An image SHALL NOT be loaded: it SHALL render as a link to its address labelled by its
alt text, or as the alt text alone when the address is not allowed or the image is inside a link. The
rendering SHALL carry no `style` attribute, no event handler attribute and no script.

#### Scenario: Hostile text renders inert

- **GIVEN** a description containing `<script>`, an `onerror` attribute, a `javascript:` link and a
  `data:` image
- **WHEN** the task screen is drawn
- **THEN** the HTML is shown as escaped text, neither address is a link, and no element carries an
  event handler or a style

#### Scenario: An image is a link

- **GIVEN** a description containing `![chart](https://example.com/c.png)`
- **WHEN** the task screen is drawn
- **THEN** it shows a link to `https://example.com/c.png` labelled `chart` and no `<img>`

### Requirement: References and handles link to their pages

A task reference in task text whose project key belongs to a project the reader can see SHALL link to
that task's screen, and an `@handle` this tenant holds SHALL link to the task list filtered to that
assignee. Text inside code or an existing link SHALL NOT be linked.

#### Scenario: A known reference links and an unknown one does not

- **GIVEN** a project `infra` and a description reading `see infra-1 and covid-19`
- **WHEN** the task screen is drawn
- **THEN** `infra-1` links to `/tasks/infra-1` and `covid-19` is plain text

### Requirement: The editor offers a Preview

The description editor and the comment composer SHALL stay plain textareas. With scripting, each SHALL
offer a Preview toggle that renders the typed text through the same renderer without saving it, with
checklist items drawn inert. The preview route SHALL require the CSRF token.

#### Scenario: Preview writes nothing

- **WHEN** a writer previews a description
- **THEN** the rendered text is shown and the task's version is unchanged

### Requirement: A reader with task write ticks a checklist item in place

Each GitHub task-list item of a description SHALL render, for a reader with `task:write`, as a control
exposed as a checkbox with its checked state and named by the item's text, inside a form that carries
the CSRF token and the version the screen was drawn at. Activating it SHALL post the item's number and
the state asked for, and SHALL work without scripting. The service SHALL rewrite only that item's
marker. A post against a version the task has moved past SHALL be refused with the conflict refusal and
change nothing. A reader without `task:write` SHALL see disabled checkboxes and no form, and comments'
items SHALL always be disabled.

#### Scenario: A tick lands on its item

- **GIVEN** a description with three items, the second open
- **WHEN** a writer ticks the second
- **THEN** only the second item's marker changes, and the screen shows 2 of 3 done

#### Scenario: A tick on a changed description is refused

- **GIVEN** a screen drawn at version 4 while the task is at version 5
- **WHEN** the writer ticks an item
- **THEN** the conflict refusal is shown and the description is unchanged

### Requirement: Rows and cards show how much of a checklist is done

The task list row, the board card, the lane card and the project board card of a task whose description
has checklist items SHALL show a badge reading done/total, and the board cards SHALL also show a progress
element for the same values. A task without items SHALL show neither.

#### Scenario: A card counts its checklist

- **GIVEN** a task whose description has one of three items ticked
- **WHEN** the board is drawn
- **THEN** its card shows `1/3` and a progress element at 1 of 3
