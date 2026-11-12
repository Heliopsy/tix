# Narrow Activity to a time window and an action

## Why

The audit log can be narrowed by text, kind, channel, token and actor on the web, but not by when a change
happened or by what was done. The tix-mobile audit screen has From and To days and an action box, and
`tix audit ls` takes `--since`, `--until` and `--action`. On the web the only way to look at last Tuesday was
to page back to it.

## What Changes

- The Activity filter gains an action box (exact match, suggesting the actions on the page) and From and To
  date-and-time pickers, read in the reader's time zone. They are carried in the query string as `action`,
  `from` and `to`, shown back as removable chips, kept by paging and the live refresh.
- A bare date in a typed link works too: `from` starts at its midnight, `to` includes the whole day.
- A bound that is not a date, or a window that ends before it starts, is refused with a 400.
- No service, store or API change: `AuditFilter` already has `Since`, `Until` and `Actions`.
