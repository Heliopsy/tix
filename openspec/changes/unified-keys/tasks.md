# Tasks

## 1. Key table

- [x] 1.1 Replace the web's action vocabulary with the shared one, grouped Navigate, Lists, Task, General
- [x] 1.2 Default, Vim and Emacs tables per the shared decisions; vim `a` is assign, emacs ctrl+a is select all
- [x] 1.3 An Off scheme computed from Default with only the ctrl and meta chords, offered in settings
- [x] 1.4 Embed the go-to destinations from the route constants

## 2. Script

- [x] 2.1 `comboOf` names alt chords (from the physical key on a Mac), ctrl+space and AltGr symbols
- [x] 2.2 `keyStep` and `sequencePrefixes` for two-key sequences with a one second wait
- [x] 2.3 Handlers for go-to, PgUp and PgDn, refresh (`tix.liveRefresh`), switch view and copy reference
- [x] 2.4 Task keys press `data-shortcut-target` controls on the focused row or the page, else fall through
- [x] 2.5 Help overlay drawn under group headings, a sequence as its two keys

## 3. Markup, words and docs

- [x] 3.1 Mark the task screen's star, priority, assignee, due, tag, Snooze, description and reference
- [x] 3.2 Mark the list rows' star and priority and the view switch
- [x] 3.3 Bulgarian strings for every new label, retired ones removed
- [x] 3.4 `docs/web-ui.md`: the key table, asserted by a test, and the sequence and target rules

## 4. Tests

- [x] 4.1 Go: each scheme's table, Off, sequences, go-to screens, groups, the marks on the task screen and rows
- [x] 4.2 jstest: `comboOf`, `keyStep`, `pageIndex`, `nextView`, `refOfRow`, `targetPress` and the wiring
