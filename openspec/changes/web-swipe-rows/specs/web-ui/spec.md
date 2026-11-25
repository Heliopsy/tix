## ADDED Requirements

### Requirement: A task list row swipes to complete or snooze on a touch screen

A task list row with a tick SHALL follow a sideways touch or pen swipe over the action it reveals: Complete
towards the end of the line, Snooze until tomorrow morning towards its start. Releasing past 30% of the row's
width SHALL post that action through the existing routes; releasing short of it SHALL change nothing. A mouse
and vertical scrolling SHALL be unaffected.

#### Scenario: Swiping a row to complete it

- **WHEN** the reader swipes an open row towards the end of the line past the threshold on a phone
- **THEN** the task is completed and the toast offers Undo

#### Scenario: Letting go too early

- **WHEN** the reader swipes a row a fifth of its width and lets go
- **THEN** the row slides back and nothing is posted

### Requirement: Only a row the reader may act on swipes

A row SHALL offer Complete by swipe only when it is unfinished and the reader may transition tasks, and Snooze
only when the reader may write them. A deleted row, or a row without a tick, SHALL not swipe.

#### Scenario: A viewer's list

- **WHEN** a reader with read access only opens the list on a phone
- **THEN** no row carries a swipe target

### Requirement: Completing and snoozing from the list can be undone

The toast after a complete from the list SHALL offer Undo moving the task back to the state it left when the
workflow allows it, and the toast after a snooze SHALL offer Undo that wakes it or restores its earlier wake
time.

#### Scenario: Undoing a swiped snooze

- **WHEN** the reader snoozes a task by swiping and presses Undo
- **THEN** the task is no longer snoozed and is back in the listing

### Requirement: A swipe has no slide for a reader who asks for less motion

Under prefers-reduced-motion the row SHALL not animate back or across, and the gesture SHALL still work.

#### Scenario: Reduced motion

- **WHEN** the system asks for reduced motion and the reader swipes past the threshold
- **THEN** the action is posted and the row returns without a transition
