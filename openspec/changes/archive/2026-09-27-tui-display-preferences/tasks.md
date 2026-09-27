# Tasks

## The screen

- [x] `tui/settings.go`: the preferences, their keys, their values and what each value means
- [x] `tui/settings.go`: the rendered screen as lines carrying how they are drawn, not the drawing
- [x] `tui/settings.go`: the session facts, with a fact nobody supplied saying so
- [x] `tui/settings.go`: the cursor and the window, so the bottom of the screen is reachable
- [x] `tui/model.go`: stepping a preference applies it to the keys, the timestamps and the theme
- [x] `tui/view.go`: draw the windowed screen with a hint for what is off it
- [x] `tui/keys.go`: relabel the value keys for this view

## Persistence

- [x] `tui`: a preference writer supplied by the caller, nil meaning a session that cannot write
- [x] `cmd/tui.go`: write through `config.SaveChanges` so nothing else is pinned
- [x] `cmd/tui.go`: pass the layer that supplied each preference
- [x] `cmd/tui.go`: pass the redacted target, tenant, build and configuration file

## Tests

- [x] test: each row renders its example through the chosen layout and zone
- [x] test: stepping wraps, and a value the build does not ship stays on offer
- [x] test: a change is applied, written, and reported with the file it went to
- [x] test: a session with no writer says the change is temporary
- [x] test: a failed write is reported
- [x] test: a value that cannot be rendered is refused and nothing changes
- [x] test: a layer above the file is announced on its own row and nowhere else
- [x] test: the session facts name the tenant in force and never a secret
- [x] test: the bottom of the view is reachable on a 24-row terminal
- [x] test: writing a preference pins no other key and keeps what the file held
