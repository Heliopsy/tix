# Settings reads as sections, and hovering a thing says it can be used

## Why

The settings screen is one long column of six groups, each row a paragraph of justification over a
control, with no way to get to Dates and times or Second factor short of scrolling past everything
above them. Controls sit at whatever width their content gives them: two selects in one list are
180px and 320px wide, and each on/off preference is a word followed by a button whose label is the
opposite of that word. The Database row breaks at 1440px, where the target path claims the whole row
and squeezes its description into a column one word wide.

Several configure screens break between 900px and 1200px, where the sidebar is a column and the
content is narrow: the API token table squeezes its Revoke disclosure into one letter per line and
grows wider than the page when it opens, and a field's (i) note is positioned from the icon, so on a
phone it runs off the right edge of every form that has one.

Separately, almost nothing in the interface responds to the pointer beyond a colour change on a
button. Rows, cards and lane cards give no sign that they are a thing to act on until clicked. Some
readers find motion and highlight under the pointer distracting, so this needs to be something a
reader can turn off.

## What Changes

- The settings screen gets an in-page section navigation: a sticky column beside the sections when
  there is room, a wrapped row of links above them when there is not.
- Each setting is a row with its name and a one-line description on the left and its control on the
  right, in a control column of one width so every select and switch lines up. Rows stack at 900px
  and below. The longer explanation each row used to print moves behind an (i) beside its name.
- On/off preferences are drawn as switches (`role="switch"` with `aria-checked`), still plain form
  posts.
- Saving a preference from settings returns to the section it was saved in with a toast naming what
  was saved.
- A new per-browser preference, Hover effects, on by default, kept in the `tix_hover` cookie like the
  other appearance preferences. Off puts `data-hover="off"` on `<html>`, which removes the hover
  highlight from rows, cards, lane cards, navigation items, buttons and links. Every transition the
  effects use is dropped under `prefers-reduced-motion: reduce`.
- The configure screens' rows (`setrow`) and tables hold together from 1440px down to 390px: the
  token table scrolls inside its own frame rather than widening the page, a row's disclosure never
  breaks its label one letter per line, and an (i) note stays inside the viewport.

## Impact

- `internal/web`: `templates/settings.html`, `templates/layout.html`, `templates/tokens.html`,
  `assets/app.css`, `prefs.go`, `render.go`, `routes.go`, `session.go`.
- `docs/web-ui.md`: the preferences table and the settings section.
- No database migration: the preference is a cookie, like every other display preference. No
  service method or capability is added.
