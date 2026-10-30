# The sidebar resizes and collapses, and every page starts the same distance from it

## Why

The sidebar is a fixed 224px (252px on a large screen). A reader with long project names or a narrow
laptop has no say in that: the column cannot be made wider to read a long entry, or narrower to give
a board another column. There is also no way to put it away while working a wide board.

Separately, the gap between the sidebar and the page content changes from screen to screen. At
1440px the task list starts 16px from the sidebar and the projects screen 34px; at 1920px the same
two are 32px and 276px apart, and at 2560px 122px and 542px. Settings and Projects, both screens of
forms, disagree with each other too. Moving between screens makes the whole page jump sideways.

The cause is one rule. The content column is centred (`margin-inline: auto`) under a `max-width`
that a list of `:has()` overrides changes per screen: a listing, a board, a feed or a table gets the
wide measure, everything else the page measure. Centring a box whose width depends on the screen in
a space whose width does not puts its left edge somewhere different on every screen that picks a
different measure, and even the vertical scrollbar shifted it by half its width.

## What Changes

- The sidebar can be resized by dragging its right edge. The width is clamped between 180px and
  480px, kept in `localStorage` as one number, and applied from the head before the body paints.
- The edge is a focusable `role="separator"` with its value, minimum and maximum, moved with the
  arrow keys (Shift for bigger steps), Home and End, and reset to the default by Enter or a
  double-click.
- Dragging it below a threshold collapses the sidebar to a rail that keeps the logo, the navigation
  icons, the Settings icon, the scheme glyph and a sign-out icon, each still carrying its name. A
  button beside the brand collapses and expands it on its own. The collapsed state is kept in
  `localStorage` and applied before paint.
- Both controls need scripting and are rendered hidden; without JavaScript the sidebar is what it is
  today. Below 900px nothing changes: the phone bar and its menu are untouched.
- The content column is anchored to the sidebar instead of centred, so the gap between them is the
  gutter on every screen. The per-screen measures still cap how wide a screen may grow; they no
  longer move where it starts.

## Impact

- `internal/web`: `templates/layout.html`, `templates/partials.html`, `assets/app.css`,
  `assets/icons.svg`, a new `assets/sidebar.js`.
- `internal/web/jstest`: a suite for `sidebar.js`.
- `docs/web-ui.md`.
- No route, service method or capability is added.
