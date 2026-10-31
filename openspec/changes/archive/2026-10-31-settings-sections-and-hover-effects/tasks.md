# Tasks

## 1. Hover effects preference

- [x] 1.1 `internal/web/render.go`, `session.go`, `routes.go`, `prefs.go`: `tix_hover`, its route and
      toggle, declared in `Preferences`
- [x] 1.2 `internal/web/templates/layout.html`: `data-hover="off"` on `<html>` when off
- [x] 1.3 `internal/web/assets/app.css`: the hover layer, gated on the attribute, `(hover: hover)` and
      reduced motion

## 2. Settings screen

- [x] 2.1 `internal/web/templates/settings.html`: section navigation, rows with a short description
      and an (i), switches, save feedback per section
- [x] 2.2 `internal/web/assets/app.css`: the navigation, the control column, stacking at 900px

## 3. Configure screens

- [x] 3.1 `internal/web/templates/tokens.html`, `assets/app.css`: the token table in a scrolling
      frame, row disclosures never broken per letter, (i) notes inside the viewport

## 4. Guards and docs

- [x] 4.1 `internal/web/hover_test.go`, `settings_test.go`: the preference, the attribute, the gated
      rules, the section navigation, the switches and the save feedback
- [x] 4.2 `docs/web-ui.md`: the preferences table and the settings screen
