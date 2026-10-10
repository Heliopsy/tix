# Tasks

## 1. One state, one form, one reader

- [x] 1.1 `internal/web/prefs.go`: `prefsView`, `columnFormView`, `visibilityFormView`,
      `taskViewFormView` and `prefsFor`, the only constructor of any of them, with the doc comment
      saying what the singularity is for
- [x] 1.2 `internal/web/prefs.go`: `Preference` and `Preferences`, the declared set the settings
      screen and the documentation table are both decided against
- [x] 1.3 `internal/web/prefs.go`: `columnPages`, naming and ordering the listings settings offers a
      picker for, and `newColumnForm`, the one way a picker's state is built
- [x] 1.4 `internal/web/render.go`: `view.ColumnForm`, so a listing's own picker comes through the
      same constructor the settings screen's do
- [x] 1.5 `internal/web/columns.go`: `ColumnListings`, read off `columnSets`, so a guard can hold
      the screen to offering a picker for each

## 2. The shared partials

- [x] 2.1 `internal/web/templates/partials.html`: `column-choices` takes a `columnFormView` rather
      than the page's view, so a second rendering would need a second value and only one constructor
      makes one
- [x] 2.2 `internal/web/templates/partials.html`: `project-choices`, extracted from `tasks.html`,
      naming each project as well as keying it and accounting for keys no project answers to
- [x] 2.3 `internal/web/templates/partials.html`: `taskview-switch`, extracted from `tasks.html`,
      two positions
- [x] 2.4 `internal/web/templates/tasks.html`: renders all three shared partials instead of its own
      copies

## 3. The screens

- [x] 3.1 `internal/web/session.go`: `showSettings` lists the tenant's projects and builds the shared
      controls; `settingsView` carries them and whether the walk finished
- [x] 3.2 `internal/web/templates/settings.html`: a Listings section holding the view switch, the
      project visibility control and one column picker per listing, each with the `muted` help the
      rest of the screen uses and an `aria-describedby` pointing at it
- [x] 3.3 `internal/web/tasks.go`: the task screen reads its view and its visibility state off the
      shared constructor rather than resolving either a second time

## 4. The stale key

- [x] 4.1 `internal/web/visibility.go`: `liveHiddenKeys`, so a key naming a deleted project excludes
      nothing instead of being refused by the service and taking the task screen down
- [x] 4.2 `internal/web/prefs.go`: `staleHiddenProjects`, so such a key is named rather than being a
      shorter listing with no stated cause
- [x] 4.3 `internal/web/tasks.go`: the exclusion is built from the live keys

## 5. Guards

- [x] 5.1 `internal/web/prefs_test.go`: every declared preference has a control on settings
- [x] 5.2 `internal/web/prefs_test.go`: every declared preference has a documentation row, and every
      row a preference
- [x] 5.3 `internal/web/prefs_test.go`: settings offers one column picker per configurable listing
      and no more
- [x] 5.4 `internal/web/prefs_test.go`: a change made on settings reaches the screen that uses it,
      asserted on the effect and on the other control
- [x] 5.5 `internal/web/prefs_test.go`: a change made in context is reflected in the settings
      control's rendered state, never in the cookie
- [x] 5.6 `internal/web/prefs_test.go`: a key naming a deleted project leaves both screens standing,
      is named, has no checkbox, and is forgotten by the next submission
- [x] 5.7 `internal/web/prefs_test.go`: every preference form is a plain posting form and stores its
      value when submitted as one
- [x] 5.8 `internal/web/settings_test.go`: the database-target guard reads the Database section
      rather than the document, since the preference sections above it now set code spans

## 6. Documentation

- [x] 6.1 `docs/web-ui.md`: the settings screen owns every preference; what the three new controls
      are; why they are one control in two places; what a key naming no project does
- [x] 6.2 `docs/web-ui.md`: where a preference lives on the terminal and the command line, and that
      the two surfaces do not share a store
