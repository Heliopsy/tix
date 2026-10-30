# Tasks

## 1. One content inset

- [x] 1.1 `internal/web/assets/app.css`: the content column is anchored to the sidebar, so its left
      edge is the gutter on every screen

## 2. Resize and collapse

- [x] 2.1 `internal/web/assets/sidebar.js`: stored width and collapsed state, clamped, applied to the
      root before paint; drag, keyboard and the toggle, delegated so a boosted swap keeps them
- [x] 2.2 `internal/web/templates/layout.html`, `internal/web/templates/partials.html`: the separator,
      the toggle, labels the rail can hide, icons for Settings and Sign out
- [x] 2.3 `internal/web/assets/app.css`, `internal/web/assets/icons.svg`: the handle, the rail, all
      of it inside the desktop breakpoint

## 3. Guards

- [x] 3.1 `internal/web/sidebar_test.go`: the inset rule, the head script order, the hidden controls,
      the separator's attributes, the rail confined to the desktop breakpoint
- [x] 3.2 `internal/web/jstest/sidebar.test.mjs`: clamping, storage, collapse threshold, keyboard
- [x] 3.3 `internal/web/assets_test.go`: `sidebar.js` is served with a validator

## 4. Documentation

- [x] 4.1 `docs/web-ui.md`: resizing, the rail, and the two storage keys
