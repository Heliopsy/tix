# One keyboard table for the web and the app

## Why

The web and the app grew their shortcuts apart. The same key meant different things on the two
surfaces: Vim's `a` was comment on the web and assign in the app, Emacs's ctrl+a was the top of a list
on the web and select all in the app, and the app's `t` tagged while the web's moved. The web also had
no go-to keys, no paging, no refresh and none of the task keys (star, priority, assign, due, snooze,
tag, copy), and nothing to turn single-key shortcuts off for somebody they keep surprising.

## What Changes

- One key table, shared with the app, lists every action. Each surface binds the part it can do.
- The web gains the go-to sequences (`g i` Inbox, `g m` My work, `g t` Tasks, `g p` Projects, `g a`
  Activity, `g s` Settings, `g S` Statistics), PgUp and PgDn, `r` refresh, `v` switch view, and the
  task keys `s`/`*` star, `p` priority, `a` assign, `u` due, `z` snooze, `#` tag, `y` copy reference
  and `E` edit description. A lone `g` waits a second for a second key and then jumps to the top.
- Vim gains `h`/`l`, `g g`, ctrl+d/ctrl+u, `v` select, `J`/`K` extend, `g v` view and `D` description;
  its `a` is assign again. Emacs gains alt+x, alt+< and alt+>, ctrl+v/alt+v and ctrl+space, and
  gives ctrl+a back to select all.
- A fourth scheme, Off, keeps only the ctrl and command chords: the palette and select all.
- The help overlay groups the keys as Navigate, Lists, Task and General, with labels the app uses
  word for word, and the settings comparison has an Off column.

## Capabilities

### Modified Capabilities

- `web-ui`: the keyboard scheme's actions, sequences and the Off scheme.

## Impact

`internal/web/shortcuts.go`, `assets/decide.js`, `assets/shortcuts.js`, a refresh hook in
`assets/live.js`, `data-shortcut-target` marks on the task screen and the list rows, the help
overlay's markup and stylesheet part, Bulgarian strings, `docs/web-ui.md`. No route, service method
or HTTP change. The terminal interface is unchanged.
