# Prose fields, and one form for editing a task

## Why

"i cant edit description only title on a task from tui really????"

`E` was wired to a single-line `textinput`. Pressing it on a seeded task rendered the whole body into one
line and clipped it at the terminal edge:

```text
body          Everything can talk to everything, which means a compromised sidecar reaches the database
enter apply   esc cancel
```

You cannot see the end of it, you cannot move through it comfortably, and you cannot insert a newline,
because `enter` applies. A task body is prose and often paragraphs. The widget was wrong for the field,
and the same widget was gathering comments and project descriptions, which are prose too.

The second half of the report is that editing a task costs six separate keys and six round trips: `e` for
the title, `E` for the body, `P` for priority, `A` for the assignee, `#` for a tag, `D` for a dependency.
Each asks one question and applies it before the next can be asked, so changing three things is three
trips through the service and three audit entries. There is no way to see what a task holds and change
several of them together.

## What Changes

- **A prose field.** The `textarea` already in the dependency tree, drawn inside the panel every input
  mode now uses: same label column, same rule, same legend last. It grows with what it holds, between
  three lines and a third of the terminal.
- **Enter inserts a newline there**, so it cannot also apply. `ctrl+s` applies, and the panel's legend
  names it on screen every time the field is open.
- **The body, a comment and a project description become prose fields.** Every other input stays one
  line, because a tag name, a reference, a hostname and a filter expression are values rather than prose.
- **`E` opens one form over the whole task**: title, body, priority and assignee, each showing what it
  holds before anything is sent. `esc` writes nothing, and only the fields the reader changed are sent.
- **`tab` and `shift+tab` move between fields**, taken off column movement, because the arrows now belong
  to whichever field has the cursor.
- **The single-key actions are untouched.** `e`, `P`, `A`, `#`, `D`, `m` all do exactly what they did.
- **Nothing shells out to `$EDITOR`.** `tix ssh` serves this interface over SSH, so an editor spawned
  from a keystroke would run on the server rather than on the reader's machine.

## Impact

- Affected specs: `tui-input`
- Affected code: `internal/tui/prompt.go`, `internal/tui/form.go`, `internal/tui/model.go`,
  `internal/tui/view.go`, `internal/tui/keys.go`, `internal/tui/scheme.go`, `docs/tui.md`
- No service, store or capability change. `E` calls `UpdateTask`, which it already called.
- One behaviour change outside the panel: `tab` and `shift+tab` no longer step a column on the board.
  `←`/`h` and `→`/`l` still do.
