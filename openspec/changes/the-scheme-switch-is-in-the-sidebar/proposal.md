# The colour scheme switch is a button in the sidebar

## Why

The colour scheme was a labelled `select` with an Apply button under an Appearance heading on
`/settings`. Changing it cost a navigation to a screen nobody otherwise opens, a reading of four
option labels, a choice and a submit, to answer a question a reader asks while looking at the screen
that is too bright. Every other preference on that page is one a reader sets once; the scheme is the
one they change by time of day.

Nothing about it needed a page. It has four values and no parameters, the control that sets it is
smaller than the heading above it, and the sidebar foot is the one strip of the interface present on
every signed-in screen and already holds the Settings link.

## What Changes

- **The scheme is a switch at the foot of the sidebar, beside the Settings link.** A small button
  carrying the scheme's glyph and its name, on every signed-in screen.
- **It cycles.** One press posts the next of System, Light, Dark and Low contrast as a hidden field
  and returns to the page it was pressed on. A four-position control that works with scripting off
  is a select plus an Apply button, which is a form and does not fit a 224px column; cycling is one
  button and reaches any scheme in at most three presses.
- **Its accessible name says both halves**: the scheme in force and the one a press moves to. The
  glyph says neither, and a `title` attribute never reaches a keyboard user who does not hover. The
  visible word is part of that name, so speaking the control and reading it agree.
- **Settings loses the form and keeps the subject.** It names the scheme in force and points at the
  switch. One cookie behind one control, which is the rule the three screen-local preferences are
  already built to.
- **`Preference` gains `Sidebar`.** The guard that holds the settings screen to carrying a control
  for every preference now reads the main column for the ones it owns and the sidebar for the one it
  does not, instead of reading the whole document. The sidebar renders inside the settings page, so
  the old assertion would have passed for a preference with no control on the screen at all.

## Impact

- `internal/web`: `render.go`, `prefs.go`, `templates/layout.html`, `templates/partials.html`,
  `templates/settings.html`, `assets/app.css`.
- No route, handler or cookie changes: the switch posts to `/theme`, the route the form posted to,
  with the same fields and the same validation.
- `web.ThemeLabel` is new and exported, for the same reason `web.ColumnListings` is: a test holds the
  switch to naming the scheme it is drawn in, and the words are read off the code rather than
  written down twice.
- Observable change for a reader with scripting off: none. The control was a plain form and still is.
