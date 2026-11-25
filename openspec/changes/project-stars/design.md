# Design

## One mark

A project star is the `Starred` flag on the reader's own group preference for that project on the
`lanes` view. That is where Pin to sidebar already wrote, so every pin made before this change is a
star after it, with nothing to migrate. Starring from the projects screen still moves the project's
lane to the front, and starring a lane still lists the project in the sidebar.

Group preferences are stored per actor, in the tenant-scoped store, and every read names the caller,
so one person's stars are invisible to everybody else, and a person in two tenants has a separate set
in each.

## Service

`StarProject` and `UnstarProject` authorise as a project read of that project, then change the
preference in one transaction with the same audit entry and `view.arranged` event a lane star writes.
They do not call `SetGroupPref`, whose task-read authority would refuse a token holding only
`project:read`. Starring a starred project writes nothing.

`ListStarredProjects` returns the caller's starred projects in the lanes' order (those the reader
moved, by position, then the rest as stored). A project gone, or one the caller can no longer read,
is left out. The list is not paginated: it is bounded by what one person starred, and the sidebar
and the Starred section need all of it.

`GetProject` and `ListProjects` set `Starred` for the reader. The field is omitted when false, so it
stays out of exports and audit snapshots, which read the store and not these methods.

## Projects screen

The first page draws Starred, then Projects with the starred ones left out, so a project appears
once. Later pages draw only the rest. `?starred=1` draws only the Starred section. The toggle posts
to `/projects/{key}/star` or `/projects/{key}/unstar`; with htmx the response is the toggle itself
(`hx-swap=outerHTML`), without it a redirect back to the page.

## Sidebar cap

Twelve stays: each listed project is a lookup on every page. Past twelve the sidebar ends the list with
Show all, which opens `/projects?starred=1`.
