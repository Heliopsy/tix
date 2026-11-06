# A history row's raw entry opens in a dialog

## Why

Raw on an activity row was a `<details>` placed on the row's flex line. The line gave it `flex: 0 0 auto`
and the disclosure was `width: fit-content`, so when it opened it kept the width of its summary, the word
"Raw", squeezed against the right edge of the row. The fields inside (a 60px label column and values that
break anywhere) were read down a column a few characters wide. The task's History showed the same fields
in a popover menu capped at 32rem. Neither showed the recorded snapshots at all.

## What Changes

- Raw on an activity row and on a task's History row is a link to the same screen at
  `?dialog=raw-SEQ`, drawn as a button. With scripting it fetches that one dialog into a slot after the
  list and opens it as a modal action dialog; Escape and Close shut it and focus returns to Raw.
- The dialog is wide (up to 56rem, at most 80% of the window tall, its body scrolling) and a bottom sheet on
  a phone. Its header names the entry's action, subject and time. Each entry shows actor, action, subject,
  full timestamp, source and sequence, the changed fields side by side when there is a before and an after,
  and each snapshot as indented JSON with a Copy button. Copy all as JSON copies the whole entry.
- Without scripting the link draws the dialog open in place, with Close leading back to the screen.

## Capabilities

### Modified Capabilities

- `web-ui`: the raw entry behind a history row.

## Impact

- `internal/web/rawentry.go` (new), `activity.go`, `taskdetail.go`, `templates/partials.html`,
  `templates/activity.html`, `templates/task.html`, `assets/app.css`, Bulgarian strings.
- No route, API or service change.
