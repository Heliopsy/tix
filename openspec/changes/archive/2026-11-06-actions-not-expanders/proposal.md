# Actions are buttons and dialogs, not expanders

## Why

Most screens ended in a `<details>` fold: Issue a token, Create a user, Map a hostname, Enrol a key,
Register an endpoint, Add a member and Add a channel were a "+" and a word under the list, often a
screen's scroll away from the heading of the thing they add to. The project board stacked four more under
its columns (Project settings, Claims, Housekeeping, Danger zone, with Clean up now folded inside
Housekeeping), and explanations were folded too. The owner asked for them gone more than once: they read
as broken sections at the bottom of the page rather than as controls.

## What Changes

- A create action is a primary button with a plus icon in the page header, or on the right of its
  section's heading, and an empty list carries the same button. It opens the form in a modal dialog:
  focus stays inside, Escape and Cancel close it, and a refused submission comes back with the dialog
  open, its message inside and what was typed kept. Without scripting the button is a link to
  `?dialog=ID`, which draws the same form in place. The POST routes are unchanged.
- A project's settings move off the board to `GET /projects/{key}/settings`, reached by a Settings button
  in the board's header: Project, Claims, Housekeeping and the Danger zone, last, as always-drawn headed
  sections. Saving any of them returns there.
- A destructive action is a red button that opens a confirm dialog holding the only form that takes the
  step: revoking a token, deleting a user, deleting a template, removing a notification channel, a
  project's Clean up now and archiving a project. Delete project leads to its existing confirmation screen.
- Explanations that were folded are drawn as text (transfer, bundles, sync runs). The sign-in help and the
  key scheme comparison open in a dialog from a button.
- Users' Manage is a button opening a dialog; Save this filter opens a dialog from the filter menu.
- A template test fails when a template holds a `<details>` that is not one of the listed toggles.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: create actions, destructive actions, project settings and folded explanations.

## Impact

`internal/web` (templates, `actiondlg.go`, handlers' refusals, `assets/forms.js`, `assets/app.css`),
`internal/i18n`, `docs/web-ui.md`, `AGENTS.md`. One new browser route; no service, HTTP API or CLI change.
