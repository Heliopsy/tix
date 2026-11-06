# Foldable sidebar groups, and a sidebar that stays still on navigation

## Why

The Configure group is nine entries long and is in the way for anybody who opens it twice a week.
Recently viewed already folded, through a `<details>` that remembered itself in `localStorage`; the
other groups could not.

Going from one screen to another made the sidebar jump. hx-boost swaps the whole body, sidebar
included, and the sidebar arrived from the server in a state its scripts then corrected: the Search
button, the rail toggle and the shortcuts hint were rendered `hidden` until a script revealed them,
and Recently viewed was rendered empty and hidden until `recent.js` filled it. Measured in Chrome on
16 boosted navigations between Tasks, Projects, Activity, Settings, Inbox, My work and Statistics,
every one had a 20 to 60ms window where everything below the Search button sat 58 to 119px higher,
then dropped back.

## What Changes

- Saved filters, Recently viewed, Configure and Data get a disclosure button on their heading, with
  a chevron, `aria-expanded` and `aria-controls`. A folded group still shows the entry for the page
  being read. The rail ignores folds.
- The sidebar's width, rail and folds move into one cookie, `tix_sidebar`, which the server reads to
  draw the page, so the first paint of a load or a boosted navigation is final. The script moves the
  old `localStorage` keys into it once.
- Controls that need a script are shown by a `js` class the head script puts on `<html>`, rather than
  rendered `hidden` and revealed after the swap has painted.
- Recently viewed carries `hx-preserve`, so a swap keeps the list already drawn, and its links are
  handed to htmx: they were added after htmx processed the page, so opening one reloaded it.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: the sidebar's groups fold, and a navigation draws the sidebar as the reader left it.

## Impact

`internal/web/sidebar.go` (new), `render.go`, `templates/layout.html`, `templates/partials.html`,
`assets/sidebar.js`, `assets/recent.js`, `assets/app.css`, the JS and Go tests beside them, and
`docs/web-ui.md`. No service, store, CLI, HTTP API or TUI change.
