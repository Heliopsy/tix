## ADDED Requirements

### Requirement: Row and task screen actions answer in place

With JavaScript, starring, moving or changing the priority of a task from a list row SHALL replace
that row only, and starring, moving, re-prioritising, tagging or commenting on the task screen SHALL
replace the page content without the sidebar. The scroll position SHALL be kept and focus SHALL
return to the control that was used. The chosen state SHALL be drawn before the server answers, and
a refused or failed action SHALL put the previous state back and say why. Without JavaScript the same
forms SHALL post and redirect.

#### Scenario: Starring a row halfway down the list

- **GIVEN** the task list scrolled so the twentieth row is in view
- **WHEN** the reader stars that row
- **THEN** the star fills at once, only that row is replaced, the list stays where it was, and the
  star keeps focus

#### Scenario: A move the server refuses

- **WHEN** a move chosen from a row's status menu is refused
- **THEN** the row shows its previous status and a toast says why

#### Scenario: No JavaScript

- **GIVEN** a browser with scripting off
- **WHEN** the reader stars a row
- **THEN** the form posts and the listing is drawn again on that row

### Requirement: A toast offers to undo what it reports

A toast reporting a move SHALL offer Undo when the task's workflow has a route back to the state it
left; one reporting a priority change SHALL offer the previous priority back; one reporting a soft
delete SHALL offer to restore the task. Undo SHALL be a form that works without JavaScript, and the
undo SHALL NOT be carried into any address the screen builds from the request.

#### Scenario: Moving a task by mistake

- **WHEN** the reader moves a task from todo to doing
- **THEN** the toast offers Undo, and pressing it moves the task back to todo

#### Scenario: A move with no way back

- **GIVEN** a workflow in which done has no transition to review, directly or through other states
- **WHEN** a task is moved from review to done
- **THEN** the toast offers no Undo

#### Scenario: A permanent delete

- **WHEN** a task is deleted permanently
- **THEN** the toast offers no Undo

### Requirement: Live refreshes never take the screen from the reader

A live refresh SHALL wait while an action in the refreshed region is on its way or a menu in it is
open, and when it lands it SHALL keep the scroll position of scrolled boxes inside the region, keep
focus on the same control, and leave the new links and forms working as boosted ones.

#### Scenario: A refresh after the reader scrolled the list

- **GIVEN** the task list scrolled down
- **WHEN** another reader changes a task and the list is refreshed
- **THEN** the list stays scrolled where it was and a title clicked afterwards opens without a full
  page load

### Requirement: Moving between screens feels immediate

Links into tasks and listings SHALL start on the primary mouse button's press. Boosted navigations
SHALL cross-fade where the browser supports view transitions, and SHALL NOT for a reader who prefers
reduced motion. Back to the task list SHALL restore the list's scroll position. The task filter SHALL
apply as the reader types, after a pause, and when a select beside it changes, and the Apply button
SHALL remain. The list, board and lanes switch SHALL swap the page without reloading its assets.

#### Scenario: Back to the list

- **GIVEN** the task list scrolled to the fortieth row
- **WHEN** the reader opens a task and presses Back
- **THEN** the list is shown scrolled to where it was

#### Scenario: Reduced motion

- **GIVEN** a reader whose system prefers reduced motion
- **WHEN** they open a task from the list
- **THEN** the screen is swapped with no cross-fade

### Requirement: The keyboard shortcuts can be found

The sidebar SHALL carry a control that opens the keyboard shortcuts overlay, and "?" SHALL open the
overlay on every screen, including after any number of boosted navigations.

#### Scenario: After navigating

- **WHEN** the reader opens a task from the list and presses "?"
- **THEN** the shortcuts overlay opens
