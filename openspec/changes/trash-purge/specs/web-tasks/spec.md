## ADDED Requirements

### Requirement: A trash row offers Delete permanently

A task row in the trash SHALL offer, beside Restore, a red Delete permanently button that opens a confirm dialog naming the task and saying it cannot be undone. Only the dialog's red button SHALL post to the task's purge route, returning to the trash with a "task deleted permanently" message.

#### Scenario: Deleting from the trash for good

- **GIVEN** the trash listing two deleted tasks
- **WHEN** the reader presses Delete permanently on one and confirms in the dialog
- **THEN** the browser returns to the trash and only the other task is listed
