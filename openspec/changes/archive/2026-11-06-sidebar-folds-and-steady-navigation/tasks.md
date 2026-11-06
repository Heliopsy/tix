# Tasks

## 1. Server

- [x] 1.1 Read `tix_sidebar` into the view: width clamped, rail, folds limited to the known groups
- [x] 1.2 Draw the rail class and width on `<html>` and the rail toggle's state from it
- [x] 1.3 Foldable headings for Saved filters, Recently viewed, Configure and Data

## 2. Browser

- [x] 2.1 `sidebar.js` keeps width, rail and folds in the cookie, adopts the old `localStorage` keys
- [x] 2.2 `js` class on the root; `data-needs-js` replaces `hidden` on the sidebar's script-only controls
- [x] 2.3 Recently viewed preserved across swaps; `recent.js` no longer manages its fold
- [x] 2.4 Recently viewed links handed to htmx, so opening one is a boosted navigation, not a reload

## 3. Proof

- [x] 3.1 Go guards for the server-drawn state, the group markup, the fold rule, `hx-preserve` and `data-needs-js`, each watched failing
- [x] 3.2 JS tests for the cookie, folding, the rail, migration and the in-place recent redraw, each watched failing
- [x] 3.3 Before and after measurements of sidebar geometry over 16 boosted navigations
- [x] 3.4 docs/web-ui.md
