# The new task and new project forms read as forms

## Why

The owner's verdict on both forms was short: the new project form and the "Add with details" form
are bad, improve them. Screenshots of both, at 1440px and 390px in light and dark, show why.

Add with details opened a three-column grid where Title, Project and Priority started on one line
with the same weight, so nothing said where to begin. Notes, tags and the repeat rule were stacked
under it at full width, the repeat rule (a rarely used, syntax-bearing field) with the same standing
as the title. Due date, assignee, parent and dependencies could not be set at all, though the service
accepts every one of them. Nothing was focused when the form opened, nothing said which field was
required, and a refused submission replaced the page with the error screen and lost everything typed.

The new project form sat folded at the bottom of the list. Its labels did not line up: the Key and
Workflow tips wrapped under their labels, so the three controls of the first row started at three
heights. Key came before Name, so the reader had to invent a code before naming the thing. Colour and
Icon were a second row of the same weight as the essentials, and the button sat alone at the far
right of a 1200px panel. A bad key ended on the full-page error screen. The project settings on the
board repeated every one of these problems.

## What Changes

- Add with details is a form with a hierarchy: a large title field first, focused when the form
  opens, then the description, then the essentials (project, priority, due date, assignee, tags) in
  a side column on a wide screen. The repeat rule, the parent task and the dependencies sit behind a
  "More options" disclosure that is open whenever one of them holds a value or a refusal.
- The project the listing is filtered to is preselected in both add forms.
- The new project form opens from a button in the projects screen's header. Name comes first and is
  focused; the key is suggested from the name and stays editable, with a line saying what tasks will
  be numbered. Workflow and description follow. Colour and icon are grouped under "Appearance". The
  project settings on the board use the same layout.
- Required fields say so. Every form carries its primary action at the bottom right with Cancel beside
  it, Ctrl+Enter or Cmd+Enter submits, and Escape cancels.
- A refused create or save is answered with the same screen, the form shown open, every value kept,
  and the message under the field it is about. Refusals from the service name that field.
- Adding still stays on the list by default and still honours "Open a new task".

## Impact

- `internal/core`: `OnField` attributes a refusal to a submitted field; project and task input
  validation names its fields.
- `internal/service`: task and project create and update attribute their refusals.
- `internal/web`: the task and project handlers re-render on refusal; new form templates, styles and
  `assets/forms.js`; Bulgarian entries for every new string.
