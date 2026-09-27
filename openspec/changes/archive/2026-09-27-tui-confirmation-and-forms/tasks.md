# Tasks

## The primitives

- [x] `tui/confirm.go`: `Confirm`, `Question`, `DeleteNote`, `ConfirmHelp`
- [x] `tui/form.go`: `Form`, `FormField`, `FieldCondition`, `Visible`, `Move`, `Cycle`, `Lines`, `FieldHint`
- [x] `tui/keys.go`: the `Agree` binding, and `Delete`, `Undepend`, `CommentEdit` and `Tags`
- [x] `tui/keys.go`: `taskBindings` pairing each action with the operation it calls
- [x] `tui/model.go`: the confirmation and the form ahead of the prompt in the key router
- [x] `tui/view.go`: the confirmation line and the form block in the footer
- [x] `tui/scheme.go`: the new actions in every view's collision set, and nano's `ctrl+k`

## The five operations

- [x] `task.delete`: the delete form's reach switches, then the confirmation that names the task
- [x] `comment.delete`: the delete form's comment subject, then the confirmation that names the comment
- [x] `comment.edit`: `M` on the selected comment, seeded with its own body
- [x] `dependency.remove`: `-` offers every dependency the task waits on
- [x] `tag.list`: `L` offers the tenant's tags and names the ones on the task
- [x] `tui/view.go`: the thread marks the selected comment, and `CommentTarget` names it
- [x] `tui/model.go`: the column keys step the thread in the detail view

## Authority

- [x] `tui/authority.go`: `ActionAccess`, `resolveActions`, `mayPerform`
- [x] `tui/model.go`: `mayPress` refuses a keystroke the reader's scopes do not reach
- [x] `tui/keys.go`: the footer and the overlay offer only what the reader may press

## The registry

- [x] `capability/registry.go`: `task.delete` bound to the board
- [x] `capability/registry.go`: `dependency.remove`, `comment.edit`, `comment.delete`, `tag.list` bound to the detail view
- [x] `capability/parity_test.go`: the terminal gap count down from 58 to 53

## Guards

- [x] `tui/confirm_test.go`: a confirmation names its subject, and one that cannot name it asks nothing
- [x] `tui/confirm_test.go`: only the agreement key runs a destructive action
- [x] `tui/confirm_test.go`: the reach the form gathered reaches the service
- [x] `tui/confirm_test.go`: deleting the open task leaves its detail view
- [x] `tui/confirm_test.go`: the delete form offers no subject the reader may not remove
- [x] `tui/form_test.go`: a form hides the fields its own answers make irrelevant
- [x] `tui/form_test.go`: the cursor never rests on a hidden field
- [x] `tui/form_test.go`: every scheme drives and cancels both primitives
- [x] `tui/form_test.go`: the tag and dependency pickers reach the service with the chosen value
- [x] `tui/thread_test.go`: the thread marks the comment the actions act on, and the column keys step it
- [x] `tui/thread_test.go`: the new actions are offered only to a reader who may use them
- [x] `tui/thread_test.go`: every operation this change bound has a key that reaches it
