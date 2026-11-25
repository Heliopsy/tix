# Inbox days, kind filters, summary and Stop on the web

## Why

The app's inbox groups items by day, filters them by kind, opens with a summary of what is late and what is
open, and stops a kind from an item's menu. The web inbox had none of it: one undivided list, All and Unread
only, and muting a kind meant a trip to Settings. Filtering by kind in the browser alone would break keyset
paging, so the listing needs the filter on the server.

## What Changes

- `ListInbox` takes `Kinds`; `GET /api/v1/inbox?kind=` (repeatable) and `tix inbox ls --kind` pass it. An
  unknown kind is refused as invalid.
- The web inbox draws its items under Today, Yesterday and Earlier in the reader's zone.
- One chip per kind beside All and Unread narrows the list, as `/inbox?kinds=mention,comment`.
- A summary card counts the reader's overdue and due-today unfinished work, the tenant's open tasks and the
  tasks finished in seven days, each linking to the screen that lists them.
- Each row carries Stop, posting to the Settings, Inbox switch with a toast naming what stopped.
