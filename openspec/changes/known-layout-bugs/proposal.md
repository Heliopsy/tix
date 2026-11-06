# Known layout bugs

## Why

The UI regression gate (#206) started green by listing the layout bugs already on `main` in
`scripts/screenshots/known.json`. Each is a real defect a reader hits: project names running under
their own links and past a phone's edge, an activity line that does not wrap a long user name, a long
tag running out of a board or lane card, a project picker overflowing a column's menu, (i) notes
opening past the window's right edge, and a pixel of sideways scroll from the (i) circle's tap zone.

## What Changes

- A project row's name wraps, and on a phone its links take their own line under it.
- An activity line breaks a long name instead of widening the page.
- A tag on a board or lane card ends in an ellipsis inside the card and carries its name as a title.
- The project picker in a column's menu shrinks to the menu.
- An open (i) note that would run past the window is moved back inside it (`assets/tipplace.js`); a
  docs path inside a note breaks rather than widen it.
- The (i) circle's touch zone reaches the page gutter and no further.
- Every entry is gone from `known.json`, so `--strict` fails if any of these comes back.

## Impact

`app.css`, `board.html`, `tasks.html`, `layout.html`, the new `tipplace.js` and its test. No route,
service or storage change.
