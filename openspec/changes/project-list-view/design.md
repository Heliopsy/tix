# Design

## One screen, two scopes

The project page is not a second list. `buildTaskScreen` takes a scope: none for `/tasks`, a project
for `/projects/KEY`. The scope does four things and nothing else:

- it folds `project:KEY` into the filter ahead of the box, the way the project control does, and
  ignores a `project` parameter the address may still carry from the task list;
- it names the routes the screen writes into its own links (`Self`, the pager, show finished, Clear,
  the filter form's action, the live fragment) as `/projects/KEY` and `/projects/KEY/live`;
- it narrows the live subscription to the project;
- it replaces the heading with the project's own header, and hides the project select, which would
  offer a choice the path already made.

Everything else, from the board's merge rule to the bulk bar, is the task screen's code. The board
of one project draws that project's columns already, so the old `project-board` block is retired; its
arrangement key (`core.ProjectView(KEY)`) is kept for the scoped board so a reader's arranged
columns survive.

## The view in the address

`view` in the query is the source of truth for the drawing. Without it the task list reads the
`tix_task_view` cookie and the project page draws its board, which is what both screens did before.
A GET of `/tasks` with a known `view` also writes the cookie, so the next bare visit opens the same
way; the project page never writes it, because the two screens default differently and one cookie
cannot hold both answers.

The switch links are built server side from the request's query: `view` set, the cursor, its trail
and the flash removed (a cursor addresses a position in one drawing's walk, and the list narrows to
open work where the board does not). Every other parameter stays, including ones the target view
ignores, so a round trip restores them.

## Without scripting, and with it

The switch is plain links, so it needs nothing. With scripting, `feel.js` already redraws the summary
and the listing after an in-place filter; it now redraws the switch from the same response. The live
refresh keeps replacing only the summary and the listing, and the switch keeps its `show:none` swap
so following it does not scroll.

## Rejected

- Keeping the switch a form and patching its `next` from JavaScript: a form button cannot be copied
  as a link, and the address still would not say which view it shows.
- A separate cookie for the project page: a second preference would need its own settings control
  and would still disagree with a shared link.
