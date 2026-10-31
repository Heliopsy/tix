# Arrange columns and lanes

## Why

A board shows every state of the workflow and the lanes view every project with work on the page,
whether or not the reader cares about them today. The request was three dots on each lane and
column, shown on hover, with things to do to it: hide it, star it, and so on, kept as a preference
of the reader's own user rather than of one browser.

## What Changes

- A reader can hide, collapse, star and reorder the groups a view draws: the columns of the task
  screen's board, the lanes of the task screen, and the columns of a project's own board. The
  arrangement is **personal** and stored on the server per actor, per tenant and per view, so it
  follows the reader across browsers, devices, the command line and the terminal interface.
- A view names a grouping rather than a drawing. `board` is the task screen grouped by status,
  `lanes` the task screen grouped by project, `project:KEY` one project's board.
- The list view can group its rows by project or by status (a new View choice, kept per browser
  like the view switch). Its group headers carry the same menu and share the arrangement of
  `lanes` and `board` respectively.
- Every group heading carries a three dots button, shown on hover and keyboard focus and always on
  a coarse pointer, whose menu offers: hide, star (a starred group comes first), collapse to a
  strip with the name and count, move left/right (up/down in the list), and per kind:
  open the project, add a task to it and show only its tasks for a project; add a task in this
  state and show only this state for a status. Hidden groups are counted in a "Hidden: N" control
  at the end of the view and in the View menu, each with a Show button, and the View menu can
  reset the arrangement. Changes apply in place without reloading the page.
- Surfaces: `tix view ls|set|order|reset`, `GET /v1/views/{view}/groups`,
  `PATCH /v1/views/{view}/groups/{group}`, `PUT /v1/views/{view}/order`,
  `DELETE /v1/views/{view}/groups`, and in the terminal interface the project board honours the
  arrangement and changes it from the command palette.
- Each change is a mutation: rows, an audit entry and a `view.arranged` event in one transaction.
  A change that changes nothing writes nothing.

## Impact

- New table `group_prefs` (migration 0016), store methods on both engines.
- `core.GroupPref`, `core.GroupPrefInput`, `core.GroupOrderInput`, `core.ViewService` with four
  methods, one event type, one authz action, four registry entries.
- No change for a reader who arranges nothing: every view draws exactly as before.
