# A project screen, for the container tasks live in

## Why

The terminal interface could open a board and work on the tasks in it, and could say nothing about the thing
the board is. Nine operations that configure a project were recorded as terminal gaps: the project could be
created and opened but not read, edited, archived or deleted; the workflow was rendered as columns but never
read as a state machine; and the custom field values shown on a task had no definition anywhere a reader could
see, let alone change.

They belong together because they are one subject. A project is its attributes, the workflow its tasks move
through and the fields those tasks carry, and a reader who wants to know why a column exists or why a task
demands a value has one question, not three.

The two primitives that landed with the previous change are what make this reachable: a confirmation that
names its subject, for the three destructive operations here, and a form whose every field picks from a fixed
list, for the answers a single line of free text cannot gather.

## What Changes

- **A project screen.** `w` opens the project under the cursor on the listing, or the one the board has open.
  It states the project's own attributes, the workflow its tasks move through with every state, edge, terminal
  flag and default lease, and the custom field definitions those tasks carry. It is gated by the same
  registry-derived authority the other views are, so a reader who may not read a project is not offered it.
- **The project is fetched, not remembered.** The screen reads the one project it shows rather than reusing the
  row the listing held, because an archive returns nothing and the listing's copy is stale the moment the
  screen acts on it.
- **Every attribute an edit accepts.** The edit form's first field is the attribute. The colour and the
  workflow are answered from its own lists; the name, the description and the icon hand over to the
  single-line prompt, seeded with the value they would replace.
- **Archiving and deleting behind one key and two confirmations.** `X` asks which, and the confirmation names
  the project. A deletion says how far it reaches; an archive does not claim a deletion's reach. An already
  archived project is not offered archiving again, because the service refuses it.
- **Custom field definitions.** `n` names a new field and then asks what its values look like; `f` offers the
  definitions the project already has, to redefine or to remove. A redefinition is seeded from the definition
  it replaces and carries through everything the form never asked about, because a put replaces the whole
  definition.
- **A workflow is read here and edited elsewhere.** The screen names `tix workflow put` rather than leaving a
  reader looking for a key that was never bound.

## Impact

- `internal/tui`: `project.go` with the screen and its forms; the project view, its handlers and its commands;
  two new bindings, `w` and `f`; three new confirmations; four new forms.
- `internal/capability/registry.go`: seven terminal gaps become bindings and two become exemptions, so the
  recorded terminal gap count falls from 53 to 44.
- No change to `internal/service`, `internal/store`, `internal/core`, `internal/web` or `cmd`. Every operation
  already existed; what is new is that the terminal can reach it.
