# Tasks

## 1. The scheme's words and its cycle

- [x] 1.1 `internal/web/render.go`: `themeLabels` and `ThemeLabel`, the words each scheme is named
      by, read off the code rather than written into a template four times
- [x] 1.2 `internal/web/render.go`: `nextTheme`, the scheme one press moves to, cycling through
      `Themes`
- [x] 1.3 `internal/web/render.go`: `view.ThemeToggle`, built from the scheme the render already
      resolved, so the switch cannot name one the page is not drawn in

## 2. The control

- [x] 2.1 `internal/web/prefs.go`: `themeToggleView`, beside the other preference control views
- [x] 2.2 `internal/web/templates/partials.html`: the `themeswitch` and `themeglyph` partials, a
      posting form with the CSRF token, `hx-boost="false"`, the next scheme as a hidden field and
      an accessible name carrying the scheme in force and the one a press moves to
- [x] 2.3 `internal/web/templates/layout.html`: the switch on the Settings link's row in
      `.side-foot`
- [x] 2.4 `internal/web/assets/app.css`: `.themer`, `.theme-switch` and `.theme-ico`, in the
      sidebar's own tokens so it reads the same in light, dark and dim

## 3. Settings keeps the subject, not the form

- [x] 3.1 `internal/web/templates/settings.html`: the Appearance section names the scheme in force
      and points at the switch, replacing the select and its Apply button
- [x] 3.2 `internal/web/prefs.go`: `Preference.Sidebar`, so the list says which preference the
      settings screen does not own

## 4. Guards

- [x] 4.1 `internal/web/settings_test.go`: `sidebarOf`, `contentOf` and `themeSwitch`, so an
      assertion about where a control lives reads one region and not the document
- [x] 4.2 `internal/web/settings_test.go`: the switch is in the sidebar of every signed-in screen,
      with a submit button, a CSRF token and `hx-boost="false"`
- [x] 4.3 `internal/web/settings_test.go`: four presses cycle through every scheme and back, the
      cookie and `<html>` follow each one, and the name says both halves at each step
- [x] 4.4 `internal/web/prefs_test.go`: `TestSettingsOwnsEveryPreference` reads the main column for
      the preferences settings owns and the sidebar for the one it does not
- [x] 4.5 `docs/web-ui.md`: the preferences section says the scheme is the exception and why
