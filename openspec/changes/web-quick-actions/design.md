# Design

**Undo encoding.** The toast's undo already rode in `flashundo` as `restore`, `priority:N` or a bare route
of states. Each quick action adds a prefix (`star:`, `unstar:`, `snooze:`, `unsnooze:`, `assign:`, `due:`,
`tag:`, `untag:`) and one table maps the prefix to the route the Undo form posts to and the field carrying
the value. Route values are state keys joined by `>`, which never carry these prefixes.

**What the Undo restores.** Each handler reads the task before changing it and writes the undo only when
the change differs: the previous assignee id (empty unassigns), the previous due instant as RFC 3339
(empty clears), the previous wake time in the reader's zone in the snooze picker's layout, or the tag.

**Assign and due routes.** The lane card's assignee field posted the whole edit form with the version and
title. Dedicated routes change one field, like the priority route, carry no version, and accept `me` for
the reader. The due field reads what the task screen's Due field reads.

**Row menu.** The lane card's menu markup and stylesheet, as a popover in the row. The row is
`data-inplace`, so feel.js swaps the row and the toast for its forms. Move is a section of the menu rather
than a button opening the status popover, which is not drawn when the status column is hidden.

**Select.** A link to select mode, so it works without scripting; select.js turns it into entering the mode
with the row ticked.

**Bulk.** `assign_me=1` beside the Assign action sets the reader's id, so the free-text field beside it,
posted empty, does not win.
