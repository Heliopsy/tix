# Design

## Storage

`saved_filters (tenant_id, id, actor_id, name, query, view_name, sort_field, shared, sort_order,
created_at, updated_at)`, keyed by tenant and id, indexed by owner and by `shared`. A listing reads the
caller's own rows and every shared row in one bounded query (`store.MaxListedFilters`); a reader owns at
most `core.MaxSavedFilters`, so the list is small and is read whole rather than paged, as group
preferences are. Names are unique per owner without regard to case, enforced in the service because a
`lower(name)` index is not portable through the builder.

## Who sees and changes what

| Filter | Owner | Another reader | Administrator |
| --- | --- | --- | --- |
| private | lists, changes | never sees it (`not_found`) | never sees it |
| shared | lists, changes | lists, opens | lists, changes, deletes |

Reading and saving need `task:read` (`filter.save`), because a filter selects nothing by itself;
changing somebody else's shared filter needs `tenant:admin` (`filter.manage`). Neither is confinable to a
project: a filter spans projects. Order is the owner's own; filters shared by others follow the reader's
own, by name.

A reference is an id, then the caller's own name, then a shared name (ambiguous shared names are
refused with a hint to use the id).

## The expression

The query is parsed when it is saved, so a filter never opens on a syntax error. The names in it
(projects, states, handles) are resolved when it is used, like any typed filter, so a filter naming a
project that is later deleted reports that rather than silently selecting nothing.

## Opening one in the browser

`GET /filters/{filter}` writes the view switch's cookie when the filter names a view and redirects to
`/tasks?q=...&sort=...`. The view is a per-browser preference already; putting it in the URL would make
the View switch fight the link. The sidebar marks a filter current when the task screen shows exactly
its expression and sort.

## `me`

The service rewrites `me` in the actor terms to the caller before resolving handles (`bindMe`), on a
copy of the filter, so it works on every transport. It wins over an actor whose handle is literally `me`.
The terminal interface matches in memory and binds `me` to its own actor the same way.

## My work

My work is the task screen built with a fixed question (`assignee:me`, open states, sorted by deadline,
one page of 200) and the list grouping of #121 with a new grouping, by deadline. The buckets are read
in the reader's chosen time zone: overdue before now, today until local midnight, this week within seven
days, later after that, none without a deadline. They are the groups of a new arrangement view `mine`,
so `tix view set mine overdue --collapsed` and the heading menus work unchanged. "Include tasks I hold"
asks a second question (`claimed-by:me`) and merges the answers, because the filter language has no
disjunction. The filter box shows `assignee:me`, so Apply carries the reader to the full listing and Save
keeps it.

## Recently viewed

Per browser by design: no migration, no server state. The task screen marks itself with
`data-recent-task`; `assets/recent.js` records it on load and after every htmx swap (the task dialog
loads through htmx), keeps ten entries under a key derived from a digest of tenant and actor, and draws
the sidebar list. `tix.recentTasks()` returns `[{ref, title, href, at}]`, newest first, and a
`tix:recent` event fires on the document when the list changes.
