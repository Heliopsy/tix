## ADDED Requirements

### Requirement: Add another keeps the quick-add box ready

The quick-add form SHALL carry an Add another checkbox, drawn ticked when this browser last added
with it ticked. A quick add with it ticked SHALL return to the listing it came from with the box
empty and focused, even when Open a new task is on. The choice SHALL be posted with the form and
remembered in the `tix_add_another` cookie, so it works without scripting.

#### Scenario: Adding with Add another

- **GIVEN** Open a new task is on
- **WHEN** the reader adds `Fix login` from `/tasks` with Add another ticked
- **THEN** the answer redirects to `/tasks`, not to the task, and that listing alone draws the box empty with `autofocus` and the checkbox ticked

#### Scenario: Turning it off

- **WHEN** the reader adds a task with Add another not ticked
- **THEN** the cookie is cleared and the next listing draws the checkbox unticked and the box without `autofocus`

### Requirement: Quick add from the clipboard

The quick-add row SHALL carry a Quick add from the clipboard button, drawn hidden and revealed only
where the browser can read the clipboard. Pressing it, or `ctrl+shift+v` or `meta+shift+v` on a task
screen outside a text field, SHALL put the clipboard's first non-empty line in the box and focus it
without adding. An empty or unreadable clipboard SHALL show "The clipboard holds no text."

#### Scenario: A multi-line clipboard

- **GIVEN** the clipboard holds a blank line, then `Renew the certificate`, then `more notes`
- **WHEN** the reader presses ctrl+shift+v on `/tasks`
- **THEN** the box holds `Renew the certificate`, has focus, and nothing has been added

#### Scenario: The help overlay lists it

- **WHEN** the reader presses `?`
- **THEN** Quick add from the clipboard is listed under ctrl+shift+v in every scheme
