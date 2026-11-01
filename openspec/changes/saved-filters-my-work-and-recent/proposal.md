# Saved filters, My work and recently viewed

## Why

A reader who looks at the same few slices of work every day retypes the same filter every day, and the
sidebar offers nothing between "Tasks" (everything) and a filter typed by hand. "What is mine and when is
it due" is the most asked question and needed `assignee:<handle>` with the reader's own handle. A task
opened a minute ago could only be found again through the browser's history.

## What Changes

- **Saved filters**: a named filter expression with the view (list, board or lanes) and the sort the task
  screen opens with, per actor, optionally shared with the tenant. Saved from the filter bar ("Save this
  filter"), listed in the sidebar under Tasks, renamed, shared, moved and deleted from a three dots menu.
  A shared filter is listed for everybody and changed only by its owner or an administrator.
  Surfaces: `tix filter ls|show|save|set|rename|order|rm`, `tix task ls --saved NAME`,
  `/api/v1/filters`, `/api/v1/filters/{filter}`, `/api/v1/filters/order`, and a picker in the terminal
  interface that applies one, plus saving the current filter.
- **`me`**: in `assignee:`, `creator:` and `claimed-by:` (and the API's `assignee` parameter) `me` is the
  caller. `tix task ls --mine` is `assignee:me`.
- **My work**: a built-in sidebar entry (`/mine`) drawing the caller's open assigned tasks as the task
  list grouped by deadline: Overdue, Today, This week, Later, No date. A switch takes in the tasks the
  caller holds a claim on. The buckets are a view (`mine`) of the #121 arrangement machinery, so each
  heading stars, collapses, moves and hides, kept per reader.
- **Recently viewed**: the last ten tasks this browser opened, per tenant and actor, kept in
  localStorage, in a collapsible sidebar section, and exposed as `tix.recentTasks()` for other scripts.
- The collapsed sidebar rail draws My work and each saved filter as an icon with its name kept for
  assistive technology; menus and the recent list fold away.

## Impact

- New table `saved_filters` (migration 0021) and store methods on both engines.
- `core.SavedFilter`, its inputs, `core.SavedFilterService` (six methods), `core.MeRef`, the `mine` view
  and `core.DueBucketOf`; three event types; two authz actions (`filter.save`, `filter.manage`); six
  registry entries.
- No change for a reader who saves nothing: every screen draws as before, with My work and an empty
  saved list added to the sidebar.
