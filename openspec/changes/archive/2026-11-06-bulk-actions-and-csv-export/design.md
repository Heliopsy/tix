# Design

## One request, a transaction per task

`BulkTasks(BulkTaskInput)` loops over the references and calls the existing per-task method
for each one: `TransitionRoute`, `UpdateTask`, `AddTag`, `RemoveTag`, `StarTask`, `UnstarTask`,
`CopyTask`, `DeleteTask`, `RestoreTask`. Each call is its own transaction writing its rows, its
audit entry and its event, exactly as a single-task call does. The alternative, one transaction
for the whole batch, would make one refused task roll back every other one and would need a
second copy of each rule inside a batch path. The cost is that a batch is not atomic, which is
the point: the answer lists what landed and what was refused and why.

Before the loop the service checks the caller may perform the action at all (the scope the
per-task method needs), so a reader without the scope gets one refusal for the request rather
than five hundred identical ones. The scope therefore varies with the action and the registry
records it as varying by input.

A move to a status walks the shortest route through the task's own workflow, one transition
per hop, as `tix task mv --hops` does. A listing mixes projects and workflows, so the browser
offers every state on the page and the service refuses per task where no route exists; the
toast lists those refusals.

A batch holds at most 500 references and names each at most once; duplicates are folded.

## Undo

A bulk delete answers with the references it deleted. The browser's toast carries them in the
redirect and its Undo posts a bulk restore for exactly those references, one request.

## Selection in the browser

Each list row gets a checkbox that belongs to the action bar's form through the `form`
attribute, so selecting and submitting works with scripting off: the bar is drawn whenever the
list has rows. With scripting on (`assets/select.js`) the bar hides until something is
selected and counts the selection; shift-click selects a range; `x` toggles the focused row;
Ctrl or Cmd+A selects every row while focus is in the list; Escape clears the selection before
it does anything else. A checked box is a dirty field, so a live refresh waits while a
selection exists, the existing rule for work in progress. Lanes and the board are not given
checkboxes: their cards are drag targets and already carry a menu each.

## CSV

`internal/output` writes CSV. A column list decides the header and cells; the CLI and API use
a fixed set (`ref, project, title, status, priority, assignee, tags, due, created, updated`),
the browser uses the title plus whichever optional columns the reader's column picker shows.
The guard against formula injection prefixes `'` to any cell starting with `=`, `+`, `-`, `@`,
tab or carriage return. The browser export follows every page of the current filter, capped at
10000 rows; the API answers one page and names the next cursor in `X-Tix-Next-Cursor`.
`-o csv` is accepted by `tix task ls` only, since other records have no column set.
