# Design

## Which endpoint

`/healthz` already returns `{"status":"ok","version":"..."}`, needs no session, never touches the
database and sits on the same origin as the pages. A dedicated route would answer the same field and
need its own registry entry, OpenAPI entry and contract test, so the script reads `/healthz`.

## Which build the page is running

The version goes on `<body>` rather than in a meta tag. hx-boost swaps the body's contents and keeps
the element, so after any number of boosted navigations the attribute still names the build whose
scripts were loaded, which is the thing that goes stale. A full load replaces it along with the
scripts.

## When to ask

- The live socket reconnecting after a drop (`tix:live-reconnected`, fired by `live.js`). A deploy
  restarts the server, which drops every socket, so this is the moment the answer is most likely to
  have changed. The first connection of a page fires nothing.
- `visibilitychange` to visible, which covers a laptop waking up and a tab brought forward.
- Every seven minutes while the tab is visible, for screens with no socket.

Checks other than the reconnect are spaced at least fifteen seconds apart. Once a new version is seen
the script stops asking.

## When a reload is safe

`tix.reloadBlocked(document, inFlight)` answers what would be lost, or nothing:

1. focus is in an input, textarea, select or contenteditable element (`tix.isTypingTarget`)
2. a `dialog[open]` or an open popover
3. an htmx request counted between `htmx:beforeRequest` and `htmx:afterRequest`, or anything marked
   `.htmx-request`, `form[aria-busy="true"]` (busy.js) or `.is-pending` (feel.js)
4. select mode on with a row ticked
5. any non-hidden input, textarea or select whose value differs from its server default
   (`tix.fieldDirty`, the same test the live refresh uses). The rich text editor writes into its
   textarea on every change and a chosen file changes a file input's value, so both count.

Hidden inputs are skipped because scripts fill them in and the reader never typed into them.

If blocked, the bar is shown and the check runs again after `focusout`, `submit`, a dialog `close`,
a popover `toggle`, `htmx:afterRequest` and `htmx:afterSettle`. Closing the bar only hides it; the
tab still reloads by itself once it is safe. Reload in the bar is the reader's own choice and reloads
whatever the page holds.

## After the reload

Before reloading, the new version and the scroll offset (with the path and query it belongs to) go
into `sessionStorage`. The next load reads and removes both, scrolls back if the address matches and
shows the note as a `role="status"` paragraph for eight seconds. Without storage the reload still
happens; only the note and the scroll are lost.

## Without scripting

Nothing happens. The next navigation loads the new build.
