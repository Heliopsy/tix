# Quick actions on list rows, Assign to me, a due date on lane cards, and Undo for each

## Why

The app offers a task's quick actions wherever a task is drawn: an actions menu on every row, Assign to me
in one press, a due date from the menu, and an Undo on each. The web had the menu on lane cards only, no
due date there, no one-press assign anywhere, and an Undo only after a move, a priority change or a delete.

## What Changes

- Every list row gets a More actions menu: open, complete or reopen, star, move, snooze, assignee, Assign to
  me, due date, tags and Select (select mode with the row ticked).
- A lane card's menu gains the due date field and Assign to me.
- Assign to me in the task screen's actions menu and in the bulk bar's Assignee menu.
- Two web routes: `POST /tasks/REF/assign` and `POST /tasks/REF/due`, each changing that one field.
- The toast offers Undo after a star, unstar, snooze, unsnooze, assignee change, due date change, tag add
  and tag remove, posting the old value back. An action that changed nothing offers none.
- A lane card's quick forms bring the toast with them, so their Undo is seen.

## Impact

Web only (`internal/web`): no service, API or CLI change. Docs in `docs/web-ui.md`, Bulgarian strings.
