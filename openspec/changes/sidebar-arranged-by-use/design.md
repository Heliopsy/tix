# Design

## Pinning reuses the lane star

Group preferences (`group_prefs`, view `lanes`) already hold a per-reader star for each project, set
from a lane's menu, and the lanes draw starred projects first. A pinned project is a project whose lane
the reader starred. One mark, two places it shows: pinning on the projects screen moves the lane to the
front, and starring a lane pins it. A separate pin table would have been a second per-reader mark on
the same thing that could disagree with the first, plus a service method, a registry entry and a CLI
command for it. The cost is that the two cannot be set apart; nobody has asked for that.

The sidebar draws at most twelve, in the lanes' own order (a moved lane by its position, the rest as
stored), looking each one up so an archived project or one the reader can no longer read is left out
rather than failing the page.

## My work counts what is overdue

The badge could count every open task assigned to the reader or only the overdue ones. A badge is a
call to look; a count of everything assigned is never zero for anybody with work and so says nothing.
Overdue is usually zero, and when it is not it is the part of My work that wants attention now. It is
also the cheaper query: the overdue filter on the reader's open tasks, limited to 100 rows and counted,
with 99+ past that. No new service method was needed.

It is refreshed with the unread badge: `/inbox/badge` now answers both badges and `live.js` applies
each by id. A task assigned to the reader, or one of theirs changing status, is an inbox item, so the
`inbox.updated` event that moves the unread count is the one that can move this one. A deadline passing
is not an event at all; the next navigation or the page becoming visible again redraws it.

## No tenant switcher

A session belongs to the tenant its host resolves to. Finding the reader's accounts in other tenants
would mean looking an address up across tenants, which nothing else in the browser does. The name is
plain text; a switcher waits for cross-tenant membership to exist.

## Admin and the fold cookie

`tix_sidebar` keeps folded groups by name. The groups are now `saved`, `recent` and `admin`, with
`admin` folded by default. A cookie from the previous build named `configure` and `data` instead: both
named means both folded, so Admin is folded; either one missing means the reader had opened it, so
Admin is open. The server and `sidebar.js` read it the same way, and the script writes the new names
the next time it saves. The rule needs no version marker because the old and new readings agree on
every cookie that names neither old group.

Trash moves into Admin and stays hidden when the tenant deletes for good. A reader without Advanced
screens gets an Admin group holding Trash alone; one with neither gets no group.

## Account menu

A popover, the primitive every other menu here uses: it opens from its button with no script, Escape
and a click outside close it, and focus returns to the button. Inside the phone's menu, itself a
popover, it nests, so Escape closes only the account menu. A browser without popovers draws the list
in place. Keyboard shortcuts stays script-only, as before. The menu is anchored above the button, the
last thing in the column.
