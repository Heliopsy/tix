# One look for table rows

## Why

Clicking Manage on the user listing left a ring round the user's row that never went away. The
Manage dialog had the same id as the row (`user-ID`), so the script that opens a dialog found the row
instead, the click fell through to the link, and the address kept `#user-ID`. The deep-link mark that
fragment put on the row was an outline that stayed until the next navigation.

The tables and the lists drawn as tables also hovered seven different ways: a grey on the tables, a
different grey (white, in light mode, so nothing) on the task list and the user listing, an accent bar
on a table row's first cell, an accent bar on the user and webhook rows, a border and a shadow on the
project rows, and a tint on settings rows.

## What Changes

- A dialog's id no longer matches the row it acts on: Manage opens `manage-user-ID`.
- Closing a dialog takes its `?dialog=` parameter and its fragment out of the address.
- The row a fragment or a toast names flashes once and lets go instead of staying ringed.
- Every table and every list drawn as one carries the class `rows` and takes the same states:
  one tint under the pointer, the same tint while a control in it has keyboard focus, one colour for a
  picked or keyboard-focused row, and the flash. No other row hover, border, bar or shadow remains.
