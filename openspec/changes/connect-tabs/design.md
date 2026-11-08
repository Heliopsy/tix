# Design

## One tab component

The task screen's Comments, History and Artifacts strip (`assets/tasktabs.js`, `[data-task-tabs]`) is the
component reused. It already picks a tab from the address, a forced value and a remembered one, and handles
arrows, Home and End. Three additions let it carry the connect screen:

- **Link tabs.** A tab may be an `<a role="tab" href="?tab=cli">`. The click handler prevents the
  navigation when it switches in place, so without scripting the link reloads with the tab named.
- **`data-tab-param`.** A box naming a query parameter writes the chosen tab into the address with
  `history.replaceState`, keeping every other parameter, and does not touch localStorage. A box without
  it remembers the choice in localStorage as before, so the task screen is unchanged.
- **Nested boxes.** A box's tabs and panels are only those whose nearest box is itself, so the MCP client
  strip inside the MCP panel is a box of its own.

The connect screen's own tab code in `assets/connect.js` is removed, leaving it the token filler.

## Server-drawn selection

The server reads `tab`, `install`, `client` and `skill`, falls back to the first tab for an unknown value,
and draws the selected tab with `aria-selected="true"` and the others' panels `hidden`. Every tab href is
the current query with one parameter changed, so following a sub-tab keeps the main tab. Without
scripting this is the whole behaviour; the strip is not `data-needs-js`.

The box gets `data-tab-forced` whenever the server drew it, so the script keeps the server's choice on load
rather than consulting storage.

## Credential first

The token is shared by CLI, MCP and API, so it sits above the tabs as step 1 rather than repeated in three
panels. SSH needs a key, not a token; step 1 says so and the SSH tab carries the enrolment command. The
issued token is still drawn once and copied into every slot by `assets/connect.js`, hidden panels included.

## Remembering the tab

There is no generic per-person preference on the account (only time and inbox preferences, each with its
own columns), and adding one means a migration and a service method for a convenience. The tab lives in the
URL only.

## Alternatives

- Fragment tabs (`#cli`). A fragment cannot be read by the server, so the no-script page would have to draw
  every panel, which is the page the owner found confusing.
- A new tab partial. Two tab scripts already existed (task and connect) and a third (handoff dialog); this
  removes one rather than adding one.
