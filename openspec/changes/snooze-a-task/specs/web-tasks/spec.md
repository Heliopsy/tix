## ADDED Requirements

### Requirement: Snooze from the browser

A reader with task write SHALL find Snooze on the task screen, in the lane card's more actions
menu and in the list row's menu, offering Later today, Tomorrow morning, Next week and a picked date
and time, resolved in the reader's time zone. A snoozed task SHALL offer Unsnooze in the same
places. Each choice SHALL be a plain form post that works without scripting, swaps in place under
htmx, and returns to the listing it came from. A reader without task write SHALL NOT be offered
either.

#### Scenario: Snooze from a lane card

- **GIVEN** a reader who may write tasks
- **WHEN** they choose Tomorrow morning in OPS-3's more actions menu
- **THEN** OPS-3 is snoozed until 09:00 tomorrow in their zone and leaves the lanes

#### Scenario: A picked time in the past

- **WHEN** a reader picks a date and time that has passed
- **THEN** the task is not snoozed and the refusal is shown where they asked

### Requirement: A snoozed task carries its wake time

Wherever a snoozed task is drawn, on its own screen and in a listing filtered to show snoozed
tasks, it SHALL carry a badge saying it is snoozed and until when, in the reader's time format.

#### Scenario: The task screen names the wake time

- **GIVEN** OPS-3 is snoozed until Friday 09:00
- **WHEN** a reader opens OPS-3
- **THEN** the screen shows that it is snoozed until Friday 09:00 and offers Unsnooze
