# Tasks

## 1. The prose field

- [x] 1.1 `internal/tui/prompt.go`: `PromptSpec.Multiline`, set on the comment, the comment edit and the
      project description; every other spec stays one line
- [x] 1.2 `internal/tui/model.go`: `area textarea.Model`, seeded, focused and cleared beside `input`
- [x] 1.3 `internal/tui/model.go`: `styleInput` strips the textarea's prompt character, its line numbers,
      its palette and its reverse-video caret
- [x] 1.4 `internal/tui/model.go`: `ProseHeight`, `fitArea`, re-run on every keystroke and on resize
- [x] 1.5 `internal/tui/view.go`: `proseRows` and `proseHelp`, drawing the field into the panel's own
      label column

## 2. The commit key and field movement

- [x] 2.1 `internal/tui/keys.go`: `Commit` on `ctrl+s`, `NextField` on `tab`, `PrevField` on `shift+tab`
- [x] 2.2 `internal/tui/keys.go`: `tab` and `shift+tab` removed from `Left` and `Right`
- [x] 2.3 `internal/tui/scheme.go`: the same removal in the `emacs` and `nano` tables
- [x] 2.4 `internal/tui/model.go`: `handlePromptKey` and `handleFormKey` route enter by the field the
      cursor is on, and hand every other key to the widget while a typed field has it

## 3. The whole-task form

- [x] 3.1 `internal/tui/form.go`: `FieldKind`, `FormField.Kind`, `FormField.Limit`, `Form.Typing`,
      `Form.Prose`, `Form.SetValue`, `FieldSummary`
- [x] 3.2 `internal/tui/form.go`: `TaskEditForm`, `PriorityOptions`, `PriorityFor`
- [x] 3.3 `internal/tui/keys.go`: `EditBody` becomes `EditTask`; `E` opens the form
- [x] 3.4 `internal/tui/model.go`: `openTaskEditForm`, `openForm`, `focusField`, `stashField`,
      `moveField`, `applyTaskEdit` sending only what changed
- [x] 3.5 `internal/tui/view.go`: `formPanel` draws a typed field's widget and `formHelp` names the keys
      for the field the cursor is on

## 4. Guards

- [x] 4.1 `internal/tui/prose_test.go`: enter inserts a newline and sends nothing
- [x] 4.2 `internal/tui/prose_test.go`: the commit key applies, the legend names it, the body reaches the
      service with its newlines
- [x] 4.3 `internal/tui/prose_test.go`: a colourless panel with a prose field open carries no escape
- [x] 4.4 `internal/tui/prose_test.go`: `ProseHeight` boundary table, and the legend is still the last
      line of the frame under a forty-line body
- [x] 4.5 `internal/tui/prose_test.go`: up stays inside the body, shift+tab leaves it
- [x] 4.6 `internal/tui/prose_test.go`: cancelling writes nothing, applying names only the changed field
- [x] 4.7 `internal/tui/prose_test.go`: the detail view draws a body with newlines as written
- [x] 4.8 `internal/tui/prose_test.go`: the audit of which specs are multi-line, asserted against
      `promptSpecs`

## 5. Documentation

- [x] 5.1 `docs/tui.md`: the bindings table gains `tab`/`shift+tab` and `ctrl+s`, and loses `tab` from the
      column row
- [x] 5.2 `docs/tui.md`: "Prose fields" and "Editing a whole task", including why not `$EDITOR`
- [x] 5.3 `docs/tui.md`: the colour section names what a colourless run strips from the prose field
