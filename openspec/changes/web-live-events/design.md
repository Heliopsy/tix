## Decisions

- The rows are drawn in the browser, from the WebSocket frames, rather than by re-fetching a server
  fragment the way Activity and the task list do. The point of the screen is the raw event as sent, and
  nothing about it is stored to render again.
- The script is its own asset, loaded only by this screen and idempotent across boosted navigations, so
  the shell's live updates (assets/live.js) are untouched. It reuses `tix.backoffDelay` and
  `tix.resumeRefused` from decide.js.
- Types and the project go to the server, as the app and `tix watch` send them; actors are matched in the
  browser, as both of those do. The project is chosen by key in the address and resolved to its id.
- The Copy JSON button is the site's copy mechanism (assets/copy.js, `data-copy-text`), like the audit
  rows' Copy JSON.
- An unknown project key or a malformed replay point is refused on the server-drawn page with a way back,
  so the page never subscribes to a filter it knows is wrong. An event type the server does not know is
  refused by the server and shown as a notice.
- The screen keeps 500 events, the app's cap, and holds at most as many while paused.
