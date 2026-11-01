## ADDED Requirements

### Requirement: A browser chooses whether a task opens as a page or in a dialog

The web interface SHALL offer a per-browser preference, Task opens in, stored in the `tix_task_open`
cookie, whose positions are Page and Dialog. Absent, or any value other than `dialog`, SHALL mean
Page, which SHALL behave exactly as the interface did before the preference existed. The control
SHALL be a switch in the Listings section of Settings that posts without scripting and names the
position in force.

#### Scenario: An untouched browser opens tasks as pages

- **GIVEN** a browser that has never set Task opens in
- **WHEN** the reader opens the task list
- **THEN** the page carries no task dialog
- **AND** the Settings switch for Task opens in reads Page

#### Scenario: Choosing Dialog

- **WHEN** the reader presses the Task opens in switch on Settings
- **THEN** the browser stores `tix_task_open=dialog`
- **AND** the reader is returned to the Listings section with a toast naming the setting
- **AND** the task list carries the task dialog

### Requirement: In Dialog mode a task link opens the task screen over the page

When Task opens in is Dialog, a plain primary click on a link to a task screen, on any screen other
than a task screen, SHALL open that task in a modal dialog over the page instead of navigating. This
SHALL apply to task titles in the list, on board cards and lane cards, to the activity feed, to
search results and to the toast offered after adding a task. The address SHALL change to the task's
address, so that reloading or opening the address elsewhere draws the full task page, and Back SHALL
close the dialog and leave the page under it as it was. Escape, the dialog's Close button and a click
on the backdrop SHALL close it, and focus SHALL return to the element that opened it, or to the same
task's row when that element has been redrawn. A click with a modifier key, a middle click, and any
click in a browser without scripting SHALL open the full task page as before. Under 600px the dialog
SHALL fill the screen.

#### Scenario: Opening and closing from the list

- **GIVEN** Task opens in is Dialog and the reader is on the task list
- **WHEN** they click a task's title
- **THEN** the task screen is drawn in a modal dialog over the list
- **AND** the address is the task's
- **WHEN** they press Escape
- **THEN** the dialog closes, the address is the list's again, and focus is on the title they clicked

#### Scenario: Back closes the dialog

- **GIVEN** a task open in the dialog over a filtered list
- **WHEN** the reader presses the browser's Back
- **THEN** the dialog closes and the filtered list is still on screen, not reloaded

#### Scenario: A deep link is the full page

- **GIVEN** Task opens in is Dialog
- **WHEN** the reader reloads while a task is open in the dialog
- **THEN** the full task page is drawn, with its back link

#### Scenario: A modified click opens a new tab

- **GIVEN** Task opens in is Dialog
- **WHEN** the reader ctrl-clicks a task's title
- **THEN** no dialog opens and the browser opens the full task page as it would for any link

### Requirement: The server draws the task screen alone for the dialog

A request for a task screen carrying the header `X-Tix-Dialog: 1` SHALL be answered with the task
screen's content and its flash, without the layout, the sidebar or the page's live binding, inside
one element marking it as the dialog's answer. Ids the task screen shares with a listing's rows SHALL
be distinct in that answer, so a control in the dialog cannot address an element of the page behind
it. The back link SHALL be a Close button. The same request without the header SHALL be answered
with the full page. Other screens SHALL ignore the header.

#### Scenario: The dialog's answer

- **WHEN** a signed-in reader requests a task screen with `X-Tix-Dialog: 1`
- **THEN** the answer carries the task's title, its edit form and its comments
- **AND** it carries no `<html>`, no sidebar and no live binding
- **AND** it offers Close where the page offers a back link

#### Scenario: Saving from the dialog

- **GIVEN** a task open in the dialog
- **WHEN** the reader saves its edit form, changes its status or priority, stars it, adds a tag or a
  comment
- **THEN** the dialog is redrawn with the result in place
- **AND** the row or card for the task on the page under it shows the change

#### Scenario: Deleting from the dialog

- **GIVEN** a task open in the dialog
- **WHEN** the reader deletes it
- **THEN** the dialog closes and the listing is drawn without the task

### Requirement: A browser chooses whether clicking a title edits it

The web interface SHALL offer a per-browser preference, Clicking a title, stored in the
`tix_title_click` cookie, whose positions are Opens the task and Edits the title. Absent, or any
value other than `edit`, SHALL mean Opens the task. The control SHALL be a switch in the Listings
section of Settings that posts without scripting and names the position in force.

#### Scenario: Choosing Edits the title

- **WHEN** the reader presses the Clicking a title switch on Settings
- **THEN** the browser stores `tix_title_click=edit`
- **AND** every task title in the list, on board cards and on lane cards is marked as editable and
  has an open link beside it

### Requirement: A title can be edited where it is listed

When Clicking a title is Edits the title, a plain click on a task's title in the list, on a board
card or on a lane card SHALL turn it into a text field holding the title. Enter, or leaving the
field, SHALL save it through the task's existing edit address with the version the listing was drawn
with; Escape SHALL put the title back unchanged. An unchanged title SHALL send nothing. A refusal,
including a conflict with a newer version and an empty title, SHALL be shown beside the field with
the field kept open. Enter or F2 on a selected row SHALL edit its title. The open link beside the
title SHALL open the task, as a page or in the dialog as Task opens in says. Without scripting the
title SHALL be a link to the task.

#### Scenario: Renaming from the list

- **GIVEN** Clicking a title is Edits the title
- **WHEN** the reader clicks a title, types a new one and presses Enter
- **THEN** the task's title is the new one and the row shows it
- **AND** the task's description and assignee are unchanged

#### Scenario: Somebody else saved first

- **GIVEN** a listing drawn at a task's version 3, and the task since saved at version 4
- **WHEN** the reader renames it from the listing
- **THEN** the save is refused and the reason is shown beside the field
- **AND** the title is unchanged

### Requirement: A save that does not name the description or assignee leaves them alone

Saving a task through the task screen's edit address SHALL change the description only when the
submission names `body`, and the assignee only when it names `assignee`. A submission naming either
with an empty value SHALL clear it, as before.

#### Scenario: Saving only the title

- **GIVEN** a task with a description and an assignee
- **WHEN** a submission to its edit address names only `version` and `title`
- **THEN** the title changes and the description and assignee do not
