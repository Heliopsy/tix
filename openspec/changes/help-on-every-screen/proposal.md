# Help on every screen

## Why

The (i) tips explain one heading or field in a sentence. Nothing on a screen says where the full
explanation lives, so a reader who wants more goes to the Docs entry, lands on the index of the
documentation and has to find the page and the section on their own.

## What Changes

- Every screen gets a small (?) button at the right end of its header row, named "Help: " and the
  screen's title. It opens, in a new tab, the section of the docs that covers the screen, for the
  release the server runs.
- `GET /docs/{page}` takes `?section=ANCHOR` and redirects to the page with `#ANCHOR` on the end. Only
  an anchor shaped like a GitHub heading anchor is passed on.
- One Go table, `internal/web/help.go`, maps each screen's route to a docs file and anchor. Tests fail
  on a screen without the button, a file the redirect does not serve, or an anchor with no heading.
- docs/web-ui.md gains the sections the table needed: Signing in, Projects and Project settings.
- The (i) tips stay as they are.

## Impact

- New `help` icon and partial, `.helplink` styles, one line in each screen template.
- `view.Help`, `Help: %s` in Bulgarian.
- docs/web-ui.md.
