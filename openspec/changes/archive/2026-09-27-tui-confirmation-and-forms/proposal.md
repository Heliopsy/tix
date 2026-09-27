# A confirmation and a form, built with the five operations that needed them

## Why

A parity review found fifty-eight operations the terminal interface cannot perform, and roughly half of them
were blocked on two inputs that did not exist.

`grep -rn 'confirm\|Confirm' internal/tui/*.go` returned nothing outside tests. Seventeen of the gaps are
destructive, and the registry already said so: the reason recorded against `task.delete` was "deleting a task
needs a confirmation step the interface does not have yet".

Input was one `textinput` with sixteen prompt kinds. Anything needing more than one line of free text had no
way in, which is why `task.delete` could not ask whether a deletion is permanent or takes the subtasks with
it, and why the tag actions took a name the reader had to already know.

Both were built together with the first operations that need them rather than on their own, because a
primitive with no caller is a guess. The five are the most one-sided gaps, where the interface could already
do the opposite half: `dependency.remove` against a `D` that adds one, `comment.edit` and `comment.delete`
against an `m` that writes one, `task.delete` against everything else a task accepts, and `tag.list` against
tag actions that were typed blind.

## What Changes

- **A confirmation that names what it will do.** `Confirm.Question` renders nothing without a target, and the
  interface refuses to open a confirmation that renders nothing, because "are you sure?" is a question about
  whatever happened to be selected. It is answered by `y` and not by `enter`: a destructive question answered
  by the key every other input is accepted with is answered by reflex.
- **A form of enumerated fields**, moved and cycled with the keys the settings screen already uses, so every
  keybinding scheme drives it without rebinding anything. A field can be hidden by another field's answer, so
  the delete form stops asking about subtasks once its subject is a comment.
- **One destructive key.** `X` opens the delete form, whose first field is the subject. A key whose subject
  depends on what is selected is the ambiguity a confirmation exists to remove, and the interface had one key
  to spend where a board has two things on it a reader may want gone.
- **A comment thread that can be selected from.** The column keys, idle in the detail view, step through it,
  and the selected comment carries the marker a selected card carries. `M` edits it; `X` removes it.
- **A tag picker.** `L` lists the tags the tenant has and names the ones already on the task, which the typed
  prompts cannot.
- **A dependency picker.** `-` offers every dependency a task waits on. It is a form rather than the numbered
  picker because that picker reads one digit, and a task waiting on more than nine would have had the rest
  unreachable.
- **Affordances gated by the reader's authority.** Every action on the selected task is paired with the
  registry operation it calls, and the footer, the help overlay and the keystroke all ask the same question.
  An interface that shows a key and then reports the service's refusal has told the reader the refusal was
  their mistake.

## Impact

- `internal/tui`: `confirm.go` and `form.go`; the delete, comment, tag and dependency wiring; the thread
  cursor; `ActionAccess` in `authority.go`; `KeyMap.taskBindings` pairing each action with its operation.
- `internal/capability/registry.go`: five terminal gaps become bindings, and the recorded gap count falls
  from 58 to 53.
- No change to `internal/service`, `internal/store`, `internal/core`, `internal/web` or `cmd`. The operations
  all existed; what is new is that the terminal can reach them.
