# Design

## Why not `$EDITOR`

The obvious answer to "the terminal cannot edit prose" is to hand the text to `$EDITOR` and read the file
back. Nothing in this repository shells out, and here it would be actively wrong.

`tix ssh` serves this interface over SSH. The process is on the server; the reader is not. An editor
spawned from a keystroke would open on the server's terminal, under the server's `$EDITOR`, with the
server's filesystem. For the reader that is useless: they would see nothing happen. For the operator it
is worse, because a keystroke from any remote reader would start a process of that reader's choosing on
the host. The same applies to a demo sandbox.

It is written down here so that nobody proposes it again as an obvious improvement.

## The widget

`charm.land/bubbles/v2/textarea`, already in the dependency tree and until now unused, so this adds no
dependency and keeps the build `CGO_ENABLED=0`.

It arrives configured for a standalone editor: a thick border character as its prompt, line numbers down
the left edge, a blinking reverse-video caret and its own palette. None of that belongs inside a panel
that already says what is being asked. The prompt and the line numbers are turned off unconditionally,
because the panel's label column is the structure; the colours and the caret are stripped where the theme
draws no colour, exactly as `styleInput` already did for `textinput`.

That last part is a promise the page makes and `tix ssh` into a `dumb` terminal depends on: "NO_COLOR
suppresses every escape". The textarea draws more than the text input did, so it needs every style state
replaced, not only the focused text. The guard reads the panel block rather than the frame, so it cannot
pass on a board that simply had no colour in it.

## The commit key

`enter` cannot both insert a newline and apply, and inserting a newline is the entire point of the field.

The key that applies is `ctrl+s`, added to the key map as `Commit`:

- `ctrl+s` is the save reflex almost everyone arrives with.
- A chord that applies is already established here: the `nano` scheme puts `Accept` on `ctrl+o`.
- `ctrl+d` was the alternative and is worse. In a terminal `ctrl+d` is end-of-input, which in a text field
  a reader reasonably expects to close the session, and it is the key that quits a shell.

`Accept` still applies from every field that is not prose, so the delete form, the tag form and every
single-line prompt are unchanged: `enter` ends them as it always did. The test is on the key that was
pressed rather than on the binding, so the `nano` scheme's `ctrl+o` still applies from inside a prose
field while its `enter` inserts a newline.

`ctrl+s` is the `emacs` scheme's `Filter` key. That is not a collision in practice, because a form owns
the keyboard while it is open and `Filter` is not dispatched there, and `Validate` does not see it because
the input-mode keys (`Accept`, `Cancel`, `Agree`, and now `Commit`, `NextField` and `PrevField`) are not
listed in `viewActions` — they belong to a panel, not to a view. That is the existing convention rather
than a new hole.

A key the reader cannot discover is the failure mode a multi-line field has, so the panel's legend states
it while the field is open: `tab shift+tab field   enter newline   ctrl+s apply   esc cancel`. The legend
is rendered per field, so a form sitting on a priority row says `←/→ value   enter apply` instead and
never advertises a key that would type a character.

## The arrows, and who owns them

In a form, up and down move between fields. Inside a textarea they move the cursor through the text.
Inside a single-line text field, left and right move the cursor rather than cycling a value. Both cannot
own the arrows, and the resolution has to be explicit because the failure mode is a reader trapped in the
body field, which reads as a hang.

**Resolution: `tab` and `shift+tab` always move between fields; the arrows belong to whatever field has
the cursor.** On a field drawn from a fixed set, the arrows keep doing exactly what they did, so the
delete form and the tag form are unchanged for anyone who already uses them.

`tab` was not free. The shipped map bound `Right` to `{right, l, tab}` and `Left` to `{left, h,
shift+tab}`, and the `emacs` and `nano` schemes carried the same pair. Checked against all five schemes,
nothing else claims `tab`. So `tab` and `shift+tab` are taken off column movement and given to two new
actions, `NextField` and `PrevField`. The board keeps `←`/`h` and `→`/`l`, which is two ways to move a
column; a form gets the one convention every terminal form in existence already uses. Rebinding stays
available: both are ordinary key map fields, so `--key NextField=ctrl+j` works like any other.

The alternative considered was leaving the arrows on the fields and giving field movement to `ctrl+n` and
`ctrl+p`. Rejected: those are `Up` and `Down` under the `emacs` and `nano` schemes, so the same two keys
would mean "move a field" in a form and "move a row" everywhere else, which is the ambiguity this repo
refuses elsewhere. `tab` has no such second meaning left.

## Height

A body reserving half the terminal and a body drawn into two lines are the same mistake from opposite
directions. `ProseHeight(lines, terminal)` clamps what the field holds between three lines and a third of
the terminal, itself clamped to ten. On the 24-row terminal `MinHeight` allows, that is at most eight
rows; on 40 rows, ten.

The frame already subtracts a tall footer from the body budget, which is what keeps the legend on screen
under a panel taller than two lines. That machinery is reused rather than duplicated, and the guard
asserts the last line of the frame is the legend with a forty-line body loaded.

## Showing before changing

`TaskEditForm` seeds every field from the task, so the form is a statement of what the task holds. The
answers live in the form, not in the service: a typed field's widget writes back into `Form.Fields` on
every keystroke and on every move between fields, and `esc` throws the whole form away. Nothing is sent
until the form is applied.

`applyTaskEdit` compares each answer against the task and sets only the `UpdateTaskInput` fields that
differ. A form that resent every field it drew would record an edit to the title on a trip that touched
the body, and the audit log is read.

## What is not on the form

**Custom fields.** They are per project and typed: a number, a date, an enumeration, each with its own
definition and its own validation. Gathering them here means either a typed editor per definition or a
free-text field that sends a string the service then refuses, and the second is worse than not offering
them. `tix task edit --field` takes them with the validation this screen cannot do. Left out deliberately.

**Tags and dependencies.** They are list operations, add and remove, not a value the form can hold and
apply once. `#`, `U`, `L`, `D` and `-` are what they are for.

**The due date.** `UpdateTaskInput.DueAt` is a `**time.Time`, so the form would need a date parser and a
way to express "clear it" distinctly from "leave it". That is a field of its own, not a line on this one.
