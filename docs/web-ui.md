# The web interface

`tix serve` puts the browser interface on the same service layer the command line calls, so a screen
can do everything a command can. The build enforces it: the capability registry in
`internal/capability` names a web binding for every operation, and the parity test fails when one is
missing.

The pages are server-rendered Go templates with no build step. htmx swaps fragments where a full
reload would lose the reader's place, but every form carries a real `method` and `action`, so a
browser that runs no script submits it normally and follows the redirect.

## Signing in

`/login` takes an email and a password. An account with a second factor is asked for its code on a second
page ([The second factor](#the-second-factor)), and a deep link followed while signed out lands after both
steps. An account flagged for a password change gets a Choose a new password page first, and an invite
link opens a page of its own ([Inviting people and required password
changes](#inviting-people-and-required-password-changes)). Can't sign in? opens the `tix user` commands
that create an account or reset a password from the machine serving tix.

Below the password sits Keep me signed in, with how long it lasts, where the tenant offers it (see
[configuration.md](configuration.md#sign-in-sessions)). Unticked, the sign-in lasts the tenant's session lifetime,
12h unless the operator or tenant says otherwise, and its cookie ends when the browser closes. Ticked, it lasts
the remembered lifetime, 30d by default, and the cookie persists until then. The choice rides through the
second factor and password change steps. Where the tenant turned it off, or the server did, the box is not
drawn. An idle timeout, where one is set, ends a remembered session left unused like any other.

On a host that serves several tenants (see [tenancy.md](tenancy.md#switching-tenant)) the sign-in goes by
the account, and somebody who belongs to more than one tenant there lands on Choose a tenant (`/switch`)
after the password and any code: one row per tenant with its role, the current one marked, a Switch button
for each other tenant this host serves, a link to the sign-in page of a tenant at its own address, and
Continue here.

## The sidebar

The sidebar is ordered by how often each part is used. At the top is the tenant's name with the rail
toggle, then Search (ctrl+k). Then Inbox with its unread badge, My work with a badge counting your open
tasks that are overdue, Tasks, Projects with the projects you starred under it, Activity, Statistics,
Connect an agent and Docs.
Your own views follow, Saved filters and Recently viewed, then Admin, and at the foot your account menu
with the version under it.

**Switching tenant.** For somebody in more than one tenant, the first sidebar entry is the current
tenant's name. It opens Your tenants, the same rows as Choose a tenant, plus links to Your account and, for a
global admin, the Global admin area; tenants a global admin is no member of are reached from there, not
listed here. Without scripting the entry is a link to `?dialog=tenants`, which draws the list in place.
Switch mints a session in that tenant and ends the one you had; when that tenant requires a second factor
your sign-in did not give, it asks for a code first. With one tenant the name at the top is plain text and
there is no switcher.

**My work's badge** counts your open assigned tasks past their deadline, not all of them: a count of
everything assigned is never zero for anybody with work, so it says nothing. It reads 99+ past 99 and is
hidden at none. The server draws it, and it is refreshed with the unread badge: `/inbox/badge` answers
both, and a task assigned to you or one of yours changing status is an inbox item, so the same event moves
both. A deadline passing is not an event; the next navigation, or the page becoming visible again,
redraws it.

**Starred projects.** The star beside a project's name, on each row of the projects screen and in the
project page's header, lists the project under Projects: its icon, or the first letter of its key, in
its colour, and its name the way you asked projects to be named in Settings, Listings, the key
upper-cased. It links to the project's board and is marked current there. A star is the one a project's
lane already has in the lane menu (the `lanes` view's group preferences, see Arranging columns and
lanes), not a second mark: starring a project moves its lane to the front, and starring a lane lists it
here. Pin to sidebar, which wrote the same mark before it was called a star, is gone, and every project
pinned with it is starred. Stars are yours alone, kept with your account in this tenant; nobody else sees
them on any screen, and the API and `tix project ls` mark only your own. The first twelve are listed, in
the lanes' order, then Show all, which opens the projects screen with only the starred ones; each listed
project is a lookup on every page, and a longer list is what that screen is for. An archived project, or
one you can no longer read, is left out.

**Admin** holds Trash and the screens Configure and Data used to: Workflows, Templates, Tenant, Domains,
Users, Tokens, SSH keys, Status, Connections, Webhooks, Import and export, and Sync. The configuration
screens need Advanced screens on, and Trash leaves when the tenant deletes for good, so a reader with
neither gets no Admin heading at all. A global admin also gets Global admin there, whatever the
preference says: every tenant with Create a tenant, Rename and Delete, each opening a dialog; the global
admins with Grant and Revoke; and a Users button per tenant that opens that tenant's accounts with the same
Manage dialog as Users. Below the accounts, a Domains section lists the tenant's hostnames, each marked
shared or this tenant only, with Share this host or Stop sharing beside it (see
[tenancy.md](tenancy.md#shared-domains)); a tenant's own Domains screen shows the same mark in its Shared
column and cannot change it. A tenant created with first admins brings the screen back with their generated
passwords at the top, each with a Copy button, once; a reload shows none. See
[tenancy.md](tenancy.md#global-admins).

**The account menu.** Your initials and handle at the foot open one menu: Settings, Docs, the colour scheme
switch, Keyboard shortcuts and Sign out. These were four blocks stacked under the navigation. It is a
popover, like every other menu here, so it opens and closes with no script, Escape closes it alone (inside
the phone menu too, which stays open) and focus returns to the button. A browser without popovers draws
the list in place. Keyboard shortcuts needs scripting and is hidden without it. On the rail the button is
your initials, named by your handle.

On a window 900px or wider the sidebar is a column, and its right edge can be dragged to make it
anywhere from 180px to 480px wide. The edge is also a keyboard control, a vertical separator in the
tab order: the arrow keys move it 16px (64px with Shift), Home and End jump to the limits, and Enter
or a double-click puts it back to the default width.

Dragging it below 120px, pressing the left arrow at the minimum, or pressing the button beside the
brand collapses it to a 64px rail holding the logo, the navigation icons, each starred project's mark
and your initials, which open the account menu. The labels are clipped rather than removed, so every
entry keeps its accessible name, and each gets a tooltip while the rail is showing. The same button, or
the right arrow on the edge, expands it to the width it had.

Admin starts folded, since it holds screens you visit far less than your tasks; open it once and it
stays open. Both are kept in the `tix_sidebar` cookie, with the groups folded below:
`w=320&rail=1&fold=recent.admin` holds a width in pixels, the rail, and the folded groups by name
(`saved`, `recent`, `admin`). A cookie written before Admin existed names `configure` and `data`
instead: both named folds Admin, and either one missing opens it, since you had opened that group. The
server and the script read it the same way, and the script writes the new names the next time it saves.
A default is left out, and a cookie holding only defaults is removed. None of it says anything about
anybody's work. The server reads the cookie and draws the page with it: the rail as a class on `<html>`,
the width as `--side-w` on its style, each folded group folded. `assets/sidebar.js` changes the same
places and rewrites the cookie when the reader drags, presses or folds. An earlier build kept the width
and the rail in `localStorage` (`tix.sidebar.width`, `tix.sidebar.collapsed`) and the recent list's fold
under `tix.recent.closed`; the script moves those into the cookie once and removes them. Without
JavaScript neither control is shown and the sidebar is its default width. Below 900px neither applies: a
phone gets the brand bar and its menu whatever a desktop chose.

**Folding a group.** Saved filters, Recently viewed and Admin each have a heading that is a
button: pressing it, or Enter or Space on it, folds the group under a chevron and again unfolds it. The
button says `aria-expanded` and names the group's body with `aria-controls`. Because the server draws the
fold from the cookie, a folded group arrives folded and nothing animates or moves on load. A folded group
still shows the entry for the page you are on, so the sidebar never hides where you are: folding Admin
while on Users leaves Users under the heading. On the collapsed rail folding does not apply, and every
group is its icons as before; the Recently viewed clock opens the sidebar with the list unfolded. Without
JavaScript the heading is plain text and every group is open.

**Navigating without the sidebar moving.** Every link is boosted by htmx, which swaps the whole body,
sidebar included. The sidebar used to arrive from the server in a state the scripts then corrected: the
Search button, the rail toggle and the shortcuts hint were rendered `hidden` until their scripts revealed
them, and Recently viewed was rendered empty and hidden until `assets/recent.js` refilled it. Between the
swap and those scripts, about 20 to 60ms on every navigation, everything under them sat up to 119px higher
and then dropped back. Now nothing in the sidebar waits for a script to look right. A control that needs
scripting carries `data-needs-js` and is shown by the `js` class `assets/sidebar.js` puts on `<html>` from
the head, before the body paints; a swap never touches `<html>`, so it is right in the first frame. The
inbox badge, the saved filters, the current entry and the folds are server-rendered. Recently viewed lives
only in this browser, so it carries `hx-preserve` and a swap keeps the list already drawn; recent.js then
moves only its current mark, or adds the task just opened. Its links are handed to htmx as they are
drawn, so opening a recent task swaps the page like every other entry instead of reloading it, which it
used to. On a full reload the deferred script still draws the list, a step after the first paint.

The page content starts the gutter's width from the sidebar on every screen. It used to be centred in
the space beside the sidebar, under a width cap that differs between a listing and a form, which put
its left edge somewhere different on almost every screen; it is anchored now, and a screen narrower
than the window leaves the spare width on its right.

## Projects

`/projects` lists the tenant's projects, each with its icon, name, key and description, linking to its
board. An archived project stays listed with an archived mark. Each row starts with a star, always drawn:
an outline to star the project, filled to unstar it, with `aria-pressed` saying which and a tooltip (Star
this project, Unstar). It is a plain form, so it works without scripting; with scripting the page is
boosted and the answer swaps in place, the sidebar with it. The projects you starred come first, under
Starred, and the rest under Other projects, so a project is listed once; later pages list only the rest.
All projects and Starred only above the list switch between the two, and Starred only is `?starred=1` in
the address. The row's tools open its tasks in the task list, its fields and its settings. New project in
the header asks for a key, a name, a workflow and a description; colour and icon are set afterwards in the
project's settings. The workflow picker lists the tenant's workflows and then the built-in templates, each
template leading with its one-line purpose, then its name with a Built-in label, its states and its
fields; picking a template creates its workflow if the tenant does not have it yet and adds its fields to
the new project. The Workflows screen's Add from template does the same without a project. See
[workflows.md](workflows.md#templates). The key starts every task
reference in the project, as in `INFRA-5`.

The name has first claim on a row: when the row's links do not fit beside it, in the two-column grid
from 1200px or with longer Bulgarian labels, they take a line of their own rather than squeeze the name,
and a name breaks only between words unless one word is wider than the whole row. A description is cut
at two lines and its full text is the tooltip. Table cells and the record and people lists wrap the same
way: words break between words, and only an identifier (`code`, a monospaced value, or a cell marked
`.token` for a name that may be one long token) breaks anywhere, so it cannot push a table past the page.

`/projects/KEY` is the project's page; see [The project page](#the-project-page). Settings in its
header opens the project's settings.

### The project page

The project page is the task screen narrowed to one project, not a second listing: the same handler and
template draw it, so it has the List, Board and Lanes views, the [filter toolbar](#the-filter-toolbar)
with its box, Filters, Sort and More menus and the chips of the terms in force, the bulk
bar, quick add (set to the project), the pager and the live refresh. Its header is the project's own: its
icon, name and star, and Import, Export and Settings. One muted line under the name says the workflow it
runs (a link to the workflow editor), how much is left, and links the field definitions. The task count
does not add "across 1 project" here, and the board draws no workflow line of its own: it is this
project's workflow, which the header already named. Quick add here has no project picker, since the
page is the project; `+key` in the line still adds to another one. On a phone Import, Export and Settings
are icons on the name's row, and the line under the name runs the full width.

The project comes from the path, so the Filters menu has no project picker and no chip names the
project, and a `project` parameter
carried over from the task list is ignored rather than added to it. Everything the screen writes back to
itself stays on the project page: the filter form, Clear, the pager, show finished too, a row's way back,
and the view switch. Its live refresh fetches `/projects/KEY/live` with the page's own query and listens
to that project only.

The page opens on the board, which is what it has always shown, unless the address names a view:
`/projects/infra?view=list&q=tag:ops&sort=title` is the infra list filtered by a tag and sorted by title,
and a link to it opens exactly that. Choosing a view here does not change the task list's remembered
view. A board of one project draws that project's workflow, and the columns a reader arranged on the old
project board keep their arrangement.

Its lanes are grouped by assignee unless the address says otherwise (`lanes=priority`, `lanes=tag`,
`lanes=due`): one lane per project would be a single lane here, so Project is not offered, and the page
neither reads nor writes the grouping the task list remembers. See [Lanes](#lanes).

### Project settings

`/projects/KEY/settings` has four sections, always drawn:

- **Project**: name, key, workflow, description, colour and icon. A new key keeps the old one as an
  alias ([Changing a project's key](#changing-a-projects-key)).
- **Claims**: how long a claim on the project's tasks holds and the longest one allowed
  ([Claim length](#claim-length)).
- **Housekeeping**: when the project's done, cancelled and trashed tasks are cleaned up, overriding the
  tenant ([Housekeeping](#housekeeping)).
- **Danger zone**: Archive project, which hides it and keeps its tasks, and Delete project
  ([Deleting a project](#deleting-a-project)).

## The task list

The heading counts the whole selection, every page of it, not the page in front of you: how many are
still open, how many are done, how many projects they span, how many an agent is holding, and how many
are waiting on other tasks. The service counts them with the listing's own filter (one grouped count per
project and state, and a count each for waiting and held), so the numbers agree with the pager's total.

"Waiting on other tasks" means a task that depends on an unfinished task (`depends_on`). It is not a
task in a workflow state called Blocked, which is a column on the board; the two used to share the word
and read as a contradiction ("5 blocked" next to "Blocked (0)"). The count is a link to the same view
narrowed with `is:waiting-on`, which `is:blocked` still spells. Every card, lane card and row whose task
is waiting carries a small hourglass mark, "waiting on AICP-12" for one dependency or "waiting on 2
tasks" for several, with every open dependency in its tooltip and a link to the first.

**Filtering.** The box takes the same expression language the CLI takes, documented in
[filtering.md](filtering.md), through the same parser, so an expression that selects a set of tasks
here selects the same set from a shell. A project chip on a row writes a filter into that box in one
click, so there is a Clear all control to get back out in one click. Clear all keeps the sort, because
a sort is not a filter. The box and its menus are the [filter toolbar](#the-filter-toolbar).

### The filter toolbar

One row above the list on a desktop, at most two on a phone, and the same on the task list and a
project's page: the filter box, **Filters**, **Sort**, the List, Board and Lanes switch, **Select**
where the screen has it, and **More** (the three dots). On a phone, Filters, Sort and Select keep their icons and
counts and drop their words, and the switch drops its (i). With a filter in force, a slim row of chips
under it names each term, and Clear all ends it.

- **The box** takes the filter language and applies on Enter. With scripting on it narrows the list
  as you type, and it suggests what the word under the caret completes to: a term key for a bare word
  (`pr` offers `project:` and `priority:`), a handle after `@`, and the tenant's own values after
  `project:`, `tag:`, `assignee:`, `status:`, `priority:`, `due:` and `is:`. The suggestions come from
  the projects, tags, people and workflows the page already loaded, so typing sends nothing extra. Down
  and Up move through them, Enter or Tab takes the one reached, Escape closes the list. Nothing is
  reached until an arrow is pressed, so Enter on its own still applies what you typed. `@raj` is
  `assignee:raj` ([filtering.md](filtering.md)).
- **Filters** opens pickers for the project (not on a project page), assignee, tag, status, priority,
  the deadline window and Include finished tasks, with Apply and Clear all. The count on the button is
  the number of terms in force. A picked status, priority or assignee replaces the box's terms of that
  key (negated ones are left alone), the empty choice removes them, and a tag is added; the server
  writes the result into `q` and redirects, so the address stays the expression and a link shares it.
  A picker you did not touch changes nothing, so a term typed into the box in the same go wins. When
  the box holds several terms of one key the picker says "as typed" and leaves them be. The deadline
  window and the project stay the `due` and `project` parameters they always were, and Include
  finished tasks is `all=1`.
- **Sort** holds the sort field, the direction (`order=desc`), Group the list on the list, and Group
  lanes by on the lanes. Stars still lead under every sort.
- **More** holds the columns and Export CSV, the page size (`limit`), Save this filter while one is in
  force, the saved filters as links (renamed, shared, moved and deleted in the sidebar), the hidden
  columns, lanes or groups with Reset the arrangement, and project visibility.
- **Chips.** One per term, in order: the deadline window, each project of the project control, each
  word of the box as typed, and Include finished. A chip's × is a plain link to the same screen
  without that one term, so it works without scripting and the other terms keep their quoting and
  negation.

Every control is a link or a GET form. The menus are popovers opened with `popovertarget`, so they work
with scripting off and close on Escape or a press outside. Every link the toolbar writes keeps the sort,
the direction, the view, the lanes grouping, the page size and select mode, and a change of filter (a
submitted form, a chip, Clear all) drops the cursor and the page number, so it starts at page one.

Where the content column is at least 84rem wide (a Full HD window with the sidebar open), quick add and
the toolbar share one row: quick add on the left, with Add with details and From template as icon buttons,
and the box and its menus on the right. Narrower, they stack, quick add first. What either draws under
itself, the quick-add preview, the chips or a refused filter, comes after that row. On a window 900px or
wider the pager over the listing is one slim line: where you are, the numbered pages and Go to page, then
Previous and Next at the end.

`scripts/screenshots/shoot.mjs` prints the toolbar's height and how far below the top of the content the
tasks start on every task screen it captures. Its strict check fails a toolbar over an eighth of a desktop
window, on more than one row of controls on a desktop, or on more than two below 900px, and tasks that
start more than 150px down a Full HD window or 240px down a phone, with no filter in force and not
counting the pager, or a pager over the tasks taller than one line on a window 900px or wider.

**Adding a task.** The one-line box and Add with details both create the task and leave you on the
listing, filter, sort, view and page as they were. The toast says `task created` and links the new task,
and its row or card is ringed. Settings has "Open a new task" for the other way of working, where each add
is opened to write it out in full. That used to be the default; a browser that never touched the
preference now stays, and one that turned it on keeps opening.

**The trash.** Trash, under Admin in the sidebar, is the task list filtered by `is:trashed`: the
soft-deleted tasks and nothing else. It is a filter rather than a screen of its own, so the box says
what it is, the address can be bookmarked, and a term added to it narrows the trash. Tasks is not
marked current while it is open. The screen is headed Trash and counts the deleted tasks on the page.
It is always drawn as the list, whatever the reader chose for the task screen, because a board's
columns are moves a deleted card cannot make; it does not narrow to open work, since a finished task
that was deleted is still worth finding; and it has no quick-add row.

A deleted row, there or on an `is:deleted` listing, carries a `deleted` badge, whose tooltip gives the
time, and a struck-through title that is text rather than a link, since the task screen answers not
found for a deleted task. It has no tick, and its status and priority are labels rather than menus, since
the service refuses all three on a deleted task. Restore at the end of the row posts to the task's
restore route with the listing as the place to come back to, so the reader stays on the trash, the
row leaves it, the count drops, and the toast says `task restored`. With scripting on, the listing and
its count are swapped in place; without it, the form posts and the page reloads on the same trash.

Beside Restore, a red Delete permanently button opens a dialog that names the task and says it cannot be
undone. Its red button deletes the task for good, with its subtasks that are in the trash too, the way the
trash cleanup rule would when they expire, and comes back to the trash with `task deleted permanently`.
The same is `tix task purge` and `DELETE /api/v1/tasks/{ref}/trash`.

A tenant admin can turn the trash off on the Tenant screen ([tenancy.md](tenancy.md#the-trash)). Then the
Trash entry leaves the sidebar, and the trash's address answers the task list with "trash is turned off
for this tenant" in its filter bar rather than an error page, since it may have been bookmarked. Every
Delete, wherever it is drawn, answers with a confirmation, "Delete N tasks permanently", instead of
deleting and offering Undo; the bulk bar's Delete does the same for the whole selection. The question is
asked by the delete handlers rather than by the buttons, so a Delete drawn anywhere asks. Delete
permanently on the task screen, which was already explicit, does not ask again.

**Quick add.** The one-line box reads [quick-add syntax](features.md): `Fix login !high #auth @raj
due:fri +web` sets the priority, a tag, the assignee, the due date and the project, and the title is what
is left. A moment after you stop typing, a row of chips under the box shows what the line will set, and a
project or handle the tenant does not have is drawn in red with the reason, before anything is sent. A
refused line comes back in the box with the reason under it. The box's (i) tip lists the syntax, with
`due:` examples whose date is written in the reader's date format. The
project picker beside the box is where the task goes when the line names no project. Dates count in the
zone chosen under Settings. With scripting off the add still parses; only the preview needs a script.
Add with details has a field per attribute and stores its title as typed. Its Due date field, like the
task screen's Due row, is a text field taking a word (`tomorrow`, `fri`), an offset (`+3d`, `+2w`),
`YYYY-MM-DD`, or a date in the order the reader's date format writes one (`06.11.2026` under Dotted); its
(i) tip shows those examples before anything is typed, the date among them in the reader's format, and
the placeholder shows three of them. The calendar button beside the field opens the browser's date picker
and writes the picked day into the field in the reader's format (YYYY-MM-DD for a format that writes the
month as a word); without scripting the button is not drawn and the field still takes a typed date.
Its More options fold holds the repeat rule, parent, dependencies and the task's lease length, blank to inherit;
a lease above the maximum is refused on that field and nothing is created.

**Repeating tasks.** The add form's details and the task screen's Repeats row carry a Repeat field taking
a rule such as `every week` or `3 days after done`, emptied to stop the task repeating. Its (i) tip lists
`daily`, `every 2 weeks`, `every week on mon,thu`, `every month on day 15` and `3 days after done`. The task screen's
Repeats row states the rule, links the occurrence it created once the task is finished, and before that
gives the date finishing it now would make the next one due, or says that this is the last occurrence.
A recurring row or card carries a `↻` marker with the rule beside it, linking every recurring task. The
Repeats field under View turns it off. Completing one from the browser creates the next exactly as the
command line does; see [features.md](features.md).

**Deadlines.** The control beside the sort narrows the list to a deadline window: overdue, today, this
week, this month. What it selects is answered as a `due:` term by the same parser the box uses, rather
than as a bound of its own, so the same window is sayable by hand in the box, the selection travels in
the URL like the rest of the query, and a window the language does not have is refused under the box like
any other bad term. Whatever is in the box is ANDed with it, and a `due:` term typed there wins.

A row whose deadline has passed carries an `overdue` badge, one falling due within the week a
`due this week` badge, and a row whose deadline is further out or absent carries nothing, since a badge
on every dated row marks nearly every row in a real backlog. The badge is a word as well as a colour and
links to the listing it belongs to. The task screen says the same thing beside the date in the Details
rail. The `Due` column can be turned off in View, like any other optional column.

An expression the listing cannot answer is reported under the box, with the expression still in the
box. A filter naming a project, status or handle this tenant does not have is a typo in a control,
not a missing page, and the full-page error screen threw the query away and made the reader retype it
to find out what was wrong. Every refusal is treated alike, whether the expression failed to parse or
named something that does not resolve, because a rule written per term goes stale the next time the
filter language learns to refuse something. The page is still served as a success: it is the listing
that was asked for, and htmx swaps nothing out of a response that is not a 2xx, so an error status
would leave the previous page on screen carrying no message at all. A failure that is not the
expression's fault still fails the page.

**Who has it.** The list carries an assignee column, shown by default, naming the actor by handle and
saying `unassigned` where nobody holds it rather than leaving the cell blank, which reads as a value
that failed to load. The handle is a link that filters the listing to that actor. Handles are
resolved once for the page rather than once per row, so the column costs one lookup pass whatever the
page size.

**More** holds two choices that are really one decision, which columns the listing shows
and which projects it includes. They were two separate disclosures and deciding what was on screen
cost two openings and two page loads. Both are per browser, stored in cookies, and both record what
is *hidden* rather than what is shown, so a project created tomorrow, or a column a later release
adds, appears on its own instead of waiting for somebody to tick it. A filter that names a project
explicitly overrides the hiding, and the page says so rather than quietly returning nothing. Both are
also on Settings, where the same two forms are rendered over the same stored values rather than
copied, so the panel here and the screen there cannot come to disagree.

The task fields apply to every drawing of the tasks, not only the list: a board or lane card draws its
star, status (on a lane card), priority, tags, repeat rule and reference only when that field is ticked.
Star and Repeats are fields like the others, both on by default. Hiding Star removes the toggle and
nothing else; starred tasks are still listed first, and the task screen keeps its star. A lane card's
more actions menu (below) is drawn whatever is hidden, so hiding a field never takes its control away.

**Lease badges.** A lease is the one thing on a row that changes by itself, so the row says which
state it is in:

| Badge | Means |
| --- | --- |
| `held · handle` | An agent holds a live lease. The title carries who and until when |
| `claim expired · 3h ago` | Somebody claimed it and never came back. It is claimable again. A row claimed more than once also says how many times, and the title names the holder and the instant it lapsed |

`held` is computed from the lease expiry against the clock as the page renders, not from whether the
sweeper has been round yet, so a lease that ran out a minute ago stops reading as held.

`claim expired` cannot be computed that way, and trying to was what made it unreachable in practice.
The sweeper clears the lease columns within a minute of a lease lapsing, so from then on a task an
agent took and stopped answering for looked exactly like a task nobody had ever touched, which is the
opposite of what the badge is for. It reads the durable evidence the sweep leaves on the task
instead, `lease_expired_at` and `lease_expired_by_actor_id`, and treats that evidence as current for
a day (`core.Task.ClaimExpiredRecently`, over `core.LeaseExpiryEvidenceWindow`). A day spans the gap
between one person looking and the next, so a claim that lapsed overnight still says so in the
morning, and it is a full maximum lease, so no lease can outlive its own evidence. The columns
themselves are not cleared when the window passes; the bound is a judgement made at read time, and an
operator can still ask about an older expiry directly. A task claimed again reports the live claim,
because the question the badge answers is whether the work was dropped and left dropped.

The claim count is shown only beside an expiry, where it distinguishes two different situations: a
task claimed seven times whose last claim was dropped is a holder that keeps dying, while the same
seven claims with no expiry is ordinary work changing hands.

`tix claim sweep`, and the sweeper a server runs on a ticker, returns the task to the queue. See
[agents.md](agents.md) for what a lease is and why the token matters.

**Ticking a task** swaps that one row rather than the page. The completion animation belongs to the
act of completing, not to the completed state, so opening a list of finished tasks animates nothing.
Fifteen checkboxes popping in sequence on load says fifteen things just happened when nothing did.

## List, board or lanes

The switch in the filter toolbar draws the same tasks as rows, as columns or as lanes, and remembers
which per browser (`tix_task_view`). The list is what an untouched install shows. Settings shows the
remembered view from the same resolved value, as a form, since it has no listing to link to.

It is one selection drawn three ways. The filter, the deadline window, the project control, the sort
and the project visibility choice are resolved once and apply to all of them, so switching changes the
drawing and never which tasks are selected.

**Only the list is paged.** Paging a board cut every column at the same row of one shared listing, so
page 2 could say "Doing (0)" while the doing tasks sat on page 1. The board reads each column on its
own instead: a column draws its first 50 cards, its heading counts every task in that state under the
filter, and a **Show N more** link at its foot draws 50 more of that column alone (`?more=STATE`, once
per step, swapped into the column in place with scripting on). Tasks in a state the workflow no longer
has are read separately and listed under the board. The lanes draw the first 200 tasks of the
selection with a **Show N more** under them (`?more=lanes`). Neither view renders the pager.

**The view is in the address.** Each position of the switch is a link to the same screen carrying
the current query with its own `view` (`?q=tag:ops&sort=title&view=board`), so switching never resets
the filter, the search text, the deadline window, the sort or the project, and works without
scripting. The page number stays, so the reader keeps their page; the cursor, its trail and a flash are
left behind, since a cursor addresses a place in one drawing's walk and the list narrows to open work
where the board does not. A parameter the target view does not use
stays in the link, so switching back restores it. `view` in the address wins over the remembered choice;
following the switch on the task list also remembers it in `tix_task_view`, so the next visit to
`/tasks` opens the same way. The filter form carries the view the address named, so Apply keeps the
drawing. With scripting on, the filter bar narrows the listing in place as you type, and the switch is
redrawn from the same answer so its links carry what was just typed; the live refresh still replaces only
the summary and the listing. Settings keeps a form for the remembered view, and the command palette's
view entries follow the switch's links when the screen has them.

**Filtering by project.** The Filters menu's project picker is the same on the list, the board and the
lanes. It submits `project=KEY` (several keys comma separated, `project=web,ops`), which is read as
`project:` terms ahead of whatever is in the box, the way the deadline select is read as a `due:` term,
so `?project=web` and `?q=project:web` answer the same question. The view switch returns to the same
address, so the choice follows you between List, Board and Lanes, and the pager, a live refresh, Export
CSV and Save this filter keep it (a saved filter stores it as `project:` terms). It is part of the
address rather than a cookie: a filter is what a link shares, while the More menu's project
visibility is the remembered, per-browser choice. A screen filtered to projects listens for changes in
those projects only. On the board, one project draws its own workflow's columns, several projects draw
one board when their workflows agree, and projects whose workflows disagree get the explanation below
with the list in place of the board. The project page at
`/projects/KEY` is this same screen narrowed to one project ([The project page](#the-project-page)).

**Columns come from a workflow, and the list mixes projects.** So a board over several projects is
only drawable when those projects agree about their states, and agreement is decided structurally,
not by name: two projects can each run a workflow called `default` whose
states differ, and the state-category vocabulary now has six words where it had three, so two
same-named workflows can differ in ways that decide where a task may go. What is compared is
everything a board draws or a move depends on:

| Compared | Why |
| --- | --- |
| The states, in declared order | Declared order is the column order |
| Each state's key | What a task's status is matched against, and what a drop submits |
| Each state's label | The column heading. A heading misnaming a state for half its cards is dishonest |
| Each state's `terminal` flag | What completing means, and what satisfies a dependency |
| Each state's `category` | What colour and grouping are derived from everywhere |
| The transitions, in declared order | The edges the route finder walks, in the order it enumerates them |
| Each transition's `from` and `to` | Whether a move is legal at all |
| `requires_scope`, `requires_comment` | A move legal here and refused there is not one control |

Not compared, because none of them can change a column or a move: `initial`, `default_lease`, and the
per-state `revert_on_lease_expiry` and `revert_to`. Not compared either, and deliberately: the
workflow's name, key and identifier. Two projects pointing at one stored workflow agree trivially;
two pointing at separately stored workflows that describe the same machine agree too, which is the
ordinary case in a tenant that made a project before it had a shared workflow.

The difference in cost decides the borderline cases. Being too strict costs a refusal that
explains itself and is one click from a board that draws. Being too loose costs a card that looks
draggable to a column it can never reach, with the refusal arriving from the service after the reader
has already dropped it. So anything not demonstrably harmless is compared.

**Every card's moves come from its own project's workflow**, resolved per card, never from the
definition the columns were drawn from. On a board whose workflows agreed those are the same answer;
asking the card's own project anyway means a change to the merge rule can never start offering a
foreign move. Drag-and-drop reads the card's own move control to decide which
columns light up, so a column outside that list is refused before any request is made.

**A card is two lines.** The star and the title share the first, and the second is small: the reference,
then the project on a merged board, the priority, the marks and the tags. Move is a small button in the
card's corner. With a mouse it shows on the card under the pointer or holding focus, so tabbing to it
shows it; on a touch screen it is always drawn, at a 44px target. It is still a fold holding a plain
form, so it moves the card with scripting off, and opened it drops the form into the card. A board of
several projects keeps its line naming the workflow and the projects sharing it, with its (i) on that line.

**When the projects do not agree the board is refused and the list is drawn**, with a notice naming
each distinct workflow and the projects running it. Each group links to the same screen narrowed to
exactly its own projects, filter and sort intact, so a board is one click away. Where two of those
workflows carry the same name, the case the merge rule exists for, the entry adds the workflow's key:
two lines both reading `Default` would be a refusal that explains nothing. Drawing whichever workflow
came first would show a subset of the selected tasks without saying so, which is the failure
this screen exists to avoid; the same reason a task whose status is in none of the columns is named
in a notice rather than dropped. The preference is not touched by a refusal, so narrowing draws the
board with no second press of the switch.

The columns are decided by the set the listing may draw from (the keys an explicit `project:` filter
names, or every project not put away), not by the projects the current page happens to carry. Deciding
on a page's contents would merge on page one and refuse on page three, and creating the first task in a
disagreeing project would turn a working board into a refusal with no visible cause.

A merged board carries more columns than one project's does, so the board is one row of columns
scrolled sideways inside its own frame rather than scrolling the page sideways. At phone width it
snaps one column to the screen. It does not wrap onto further rows and does not stack: a column is a
fixed-height scroller, and either arrangement grows past that height and runs over whatever the page
draws beneath the board. Cards carry their project's key, icon and accent only on a merged board,
where a column holds more than one project's work.

## Lanes

The third position of the same switch draws the page in lanes, by default one per project, the way
Google Tasks draws one column per list. It is the same selection again: one filter, one page, one
visibility choice and one pager, except that lanes, like the board, are exempt from the list's
open-work default, because a finished task is drawn in its lane, ticked.

**Group lanes by.** In the Sort menu, and only while lanes are drawn, a select picks what a lane is:

| Grouping | One lane per | Order | An empty lane is drawn when |
| --- | --- | --- | --- |
| Project | project | the order the screen lists projects in | a `project:` term names it |
| Assignee | person, plus Unassigned | A to Z by handle, Unassigned last | an `assignee:` term names the person |
| Priority | priority | highest to lowest | a `priority:` term names it |
| Tag | tag, plus No tag | tags the filter names, then A to Z, No tag last | a `tag:` term names it |
| Due | deadline bucket | overdue, today, this week, later, no date | never |

The choice is the `lanes` parameter beside `view`, so a link keeps it, and the view switch, the filter
form, Clear, the pager, the live refresh and a card's own actions all keep it. The control is a GET form
carrying the rest of the address: without scripting it has an Apply button, with scripting a change
submits it. The task list opens on Project and remembers the last choice in this browser
(`tix_lanes_group`, a cookie, removed when Project is chosen again); a project page opens on Assignee
(see [The project page](#the-project-page)).

**A task with several tags is drawn once**, in the first lane among its tags as you see them, left to
right, after your arrangement. Tags a filter names come first, so `tag:ops` puts every card tagged ops in
the ops lane, and starring a tag's lane pulls its cards into it; hiding one sends them to their next tag.
Drawing a card in every tag's lane would make the lanes add up to more than the page and repeat the
card's controls, whose forms, menus and focus are addressed by the card's id. The card still lists all
its tags, and a line over the lanes says the rule.

There is no grouping by status, on purpose: lanes by status are the board, which already merges the
workflows of several projects, refuses the ones that disagree and moves a card by drag.

Lanes have no drag and drop. A card changed in a way that moves it to another lane (a new priority in
lanes by priority, a new assignee in lanes by assignee, a tag in lanes by tag) is swapped in place
first and moves to its new lane with the live refresh that change triggers, as anybody else's change
would; without scripting the page is drawn again with it in its new lane.

A lane heading never splits a word: a name too long for the lane is cut short with an ellipsis, its
full text in the tooltip, and the open count beside it stays whole. A lane heading other than a
project's is a link to the screen filtered to that lane's tasks
(`assignee:lena`, `priority:high`, `tag:ops`), still in lanes grouped the same way, and its menu offers
the same link. Unassigned, No tag and the deadline buckets have no filter term that says exactly them,
so they are plain headings.

Project lanes follow the order the screen lists projects in, and a lane keeps the listing's sort. A project
gets a lane when a task on this page is in it, or when a `project:` term names it, in which case an
empty lane says so. No other empty lane is drawn: a tenant with hundreds of projects would otherwise
draw hundreds of columns of nothing, most of them because their tasks are on another page. A task
whose project the screen does not list is named under the lanes rather than dropped.

The lane is the project, so the status moves onto the card as a badge: the state's label, coloured by
its category, which is the one part of a state a tenant cannot rename.

**The tick completes and reopens.** On an unfinished card it considers the shortest route from the
task's state into each terminal state of the done category. On a finished card it considers the
routes back to an open state that take the fewest steps: in the built-in workflow that is `done →
todo`, so unticking a card simply reopens it. A route is usable when no step of it needs a comment
and the reader holds the scope to move tasks and every scope a step requires.

- One candidate, usable: the tick is a box that applies it. Its label spells out any state it passes
  through.
- More than one: the tick opens a menu of the usable ones, each naming its journey. It is never a box
  in that case, even if only one of them is usable, because picking for the reader writes a history
  nobody chose.
- None usable: the circle is dashed and opens a panel that says why in words (no route, a comment is
  needed, a scope is missing, the reader cannot move tasks) and links to the task. It used to be a
  link whose reason sat in a tooltip, which a touch screen never shows.

**The badge and the priority are menus**, the same popover controls a list row has. The status menu
offers every state the card's own workflow reaches from here, leaving out routes that need a comment
or a scope the reader lacks, since a lane has nowhere to type one. The priority menu offers every
priority and marks the current one; a normal priority is drawn faintly so it can still be raised. A
reader who may not move tasks sees the badge as a label, and one who may not write tasks sees the
priority as the filter link it was, normal not drawn at all.

**Everything happens on the card.** Each of those forms is a plain post that works without
JavaScript and returns to the listing at the card. With htmx it replaces that card and nothing else:
the response is the redrawn listing and `hx-select` takes the one card out of it, so the card is drawn
by the same template as every other. While the request is out the card dims and takes no second press.
Focus returns to the control that was pressed, on the new card. A refusal, say because somebody else
moved the task meanwhile, leaves the card where it is and says why underneath the title rather than
swapping the error page in, which used to make the card disappear.

**More actions.** Every lane card has three dots in its corner, drawn whatever the More menu hides.
They open a menu with the whole task in it: open the task (in the dialog when Task opens in is set
to Dialog), copy its link or its reference, complete or reopen it along the tick's routes (or the
tick's reason when it offers none), move it to any state the status menu offers, follow a link to the
task for a move that needs a comment, change the priority, star or unstar it, snooze or unsnooze it,
rename the title in place, set the assignee (the field suggests the tenant's handles), add and remove
tags, and delete it.
A reader sees only what their scopes allow: moves need `task:transition`; priority, snooze, rename,
assignee and tags need `task:write`; delete needs `task:delete`. A reader who may only read gets
open, copy and their star.

Each item is a form on an existing route or a link, so it works without JavaScript and returns to
the listing at the card, and with htmx it swaps the card like the controls above, dimmed while it
waits and with a refusal said on the card. The edit, tag and restore routes return to a posted `next`
for this, exactly as moves already did; without one they go to the task screen as before. A delete is
soft, the card goes, focus moves to the next card in the lane, and the toast offers Undo, which brings
the card back on the same listing. Rename starts the same in-place editor a title click does with
Clicking a title set to Edit the title, whatever that setting says; without JavaScript it is a link
to the task. A due date is not offered: the web has no form that sets one.

The menu behaves as the badge's does: Escape closes it and puts focus back on the dots, no shortcut
acts while it is open, and a live refresh waits for it. It takes focus when it opens, and the arrow
keys move through it (Home and End go to its ends), in every lane menu; a text field keeps its own
arrows. On a touch screen the dots, the fields and every row answer a 40px press.

A live refresh holds back while a lane request is in flight or any menu in the listing is open, and
puts focus back on the same control when it lands. It also hands what it drew to htmx: it writes the
listing with `innerHTML`, which htmx never sees, so before this every form a refresh had drawn posted
as a whole page, and the second tick on a card reloaded the screen and threw the reader to the top.

On a phone (below 640px) the lanes are one row that snaps a lane at a time, each lane 86% of the
screen wide so the next one shows its edge, and each lane is as tall as its cards so the page, not the
lane, scrolls down. It was a frame of fixed height whose lanes scrolled inside it, three scroll axes
for one thumb, and the guessed height cut the last card off below the fold. On a touch screen the
tick, the badge and the priority each answer a press 40px tall, and the menu rows are 40px.

**Show completed** hides finished cards in every lane. It is kept in the browser's `localStorage` rather
than a cookie, as a single key, `tix.lanes.showCompleted`, which holds `false` when finished cards are
hidden and is removed when they are shown. The sidebar's width and collapsed state used to sit beside it
and are a cookie now, because the server has to draw them (see The sidebar). Nothing about work goes
there: no project key, no task reference, no count. Screens are served `no-store` so that a page does
not outlive the session on a shared machine, and browser storage is the same disk with no logout to
clear it, so only a choice that says nothing about anybody's work may live in it. The script that
applies it loads in the head, before the body paints, and sets a class on the root element, so hidden
cards never flash in and a live refresh of the lanes does not bring them back. Without JavaScript the
toggle is not shown and finished cards are.

## My work, saved filters and recently viewed

The sidebar holds three things of the reader's own: My work, under Inbox, and the saved filters and
recently viewed tasks, under Statistics.

**My work** is your open tasks assigned to you, drawn as the list under a heading per deadline:
Overdue, Today, This week, Later and No date, read in the time zone you chose in Settings. An empty
heading is left out. **Include tasks I hold** adds the tasks you hold a claim on, assigned or not. The
headings carry the same three dots as any list group and are kept per reader as the view `mine`
(`tix view set mine overdue --collapsed`). It draws one page of 200 by deadline and says so when
there are more, linking the full listing. The filter box reads `assignee:me`, so Apply takes you to
the task list on the same question and Save keeps it.

**Saved filters.** With an expression or a deadline window in the filter toolbar, **Save this filter** in the
More menu opens a dialog that asks for a name and whether to share it with everyone in the tenant, and
keeps the expression, the deadline window, the sort and the view the screen is drawn in. The filter is
listed in the sidebar, your own in your order and then the ones others shared; opening it sets the view
switch to its view and lands on the task screen filtered and sorted the same way, marked current there.
The three dots beside a filter you may change rename it, share it or stop sharing it, move it up or down
among your own, and delete it. A shared filter is changed only by its owner or an administrator. The
forms work without scripting; with it the sidebar is redrawn in place, focus goes back to the same
filter's dots, and a refusal (a name you already use) is said inside the menu. `tix filter` keeps the
same filters from the command line.

**Recently viewed** lists the last ten tasks this browser opened, as a page or in the dialog, newest
first, in a group that folds like the others (see The sidebar). It is kept in `localStorage`
under `tix.recent.<digest>`, a digest of the tenant and the reader so two people sharing a browser
keep apart lists, and is drawn by `assets/recent.js`; with scripting off it is not offered. The same
list is `tix.recentTasks()` for other scripts, an array of `{ref, title, href, at}` newest first,
and the document receives a `tix:recent` event whenever it changes.

On the collapsed rail My work, each starred project and each saved filter are icons with their names
kept as accessible names and tooltips, the menus are put away, and Recently viewed folds to a clock that opens the
sidebar with the list unfolded.

## Arranging columns and lanes

Every column of a board, every lane and every heading of a grouped list carries three dots. They
show while the pointer is over the heading or focus is inside it, and always on a touch screen. The
menu behind them acts on that group:

- **Star** puts the group before every unstarred one. A starred heading carries a star.
- **Collapse** draws the group as a narrow strip with its name and count and no cards. A collapsed
  column still takes a card dragged onto it.
- **Move left** and **Move right** (up and down in a list) swap the group with its neighbour. A
  starred group moves among the starred ones and the rest among the rest.
- **Hide** stops drawing the group. **Hidden: N** at the end of the view, and the More menu, list
  every hidden group with a Show button; the More menu also resets the whole arrangement.
- A project heading opens the project, shows only its tasks and adds a task to it. A status heading
  shows only that status and adds a task in that state, to a project the board draws.

The arrangement is yours and kept with your account on the server, not in the browser, so it
follows you to any browser and device, to `tix view` on the command line and to the terminal board,
and changes nothing anybody else sees. It is kept per view: `board` is the task screen grouped by
status (the board and the list grouped by status), `lanes` is the task screen grouped by project
(the lanes and the list grouped by project), `lanes:assignee`, `lanes:priority`, `lanes:tag` and
`lanes:due` are the lanes grouped those ways, `mine` is My work grouped by deadline, and
`project:KEY` is one project's own board. A lane is kept by the project, so renaming a project's key
keeps it; a person's lane is kept by the actor and named by the handle. The other groupings are shared
by the task list and every project page, so a person you starred is starred on both.

Every action is a form, so it works without scripting. With scripting it redraws the view in place
and puts focus back on the same heading's dots, or on Hidden after a hide. While a menu is open the
keyboard shortcuts stand down and a live refresh waits; Escape closes only the menu.

The list can group its rows: View, **Group the list**, or Settings, Listings. The grouping splits the
page being shown, so paging is unchanged and a group can run on to the next page.

## The inbox

The first entry in the sidebar is Inbox, with a badge counting your unread items. An item arrives when
somebody mentions you as `@handle` in a comment or a task description, assigns you a task, comments on a
task you are assigned to or starred, or moves a task assigned to you to another status. Your own changes
never reach it, and one change gives you at most one item per task: a comment that mentions the assignee
is a mention, not a mention and a comment. Editing a comment or a description notifies only handles the
earlier text did not name, so fixing a typo does not mention everybody again.

The badge stays current on every screen: it subscribes to the event stream for `inbox.updated` alone and
asks `/inbox/badge` for the count when one arrives. The page carries no actor identifier for the script to
compare against, so the server, which knows who is reading, answers. Without JavaScript it is as current
as the page. On the collapsed rail it is a dot on the icon, and on a phone a dot on the menu button.

`/inbox` lists items newest first, each saying who did what (mentioned you, assigned you, commented,
moved it from one status to another), the task's reference and title, the text quoted from the comment or
description, and when. Unread items carry a dot and bold text. All and Unread switch the list; Mark read
and Mark unread on each row, and Mark all read above the list, are plain forms that redraw the list in
place with htmx and work without it. Opening an item marks it read and opens the task the way your
dialog setting says, over the inbox or as its own page; without JavaScript the link opens the task and
the row's button marks it read. The listing pages by keyset like every other.

Settings has an Inbox section with one switch per kind: Mentions, Assignments, Status changes and
Comments. A kind turned off creates no item for you at all. Unlike the rest of Settings these are kept
with your account rather than the browser, because they decide what the server writes, so they apply in
every browser, the terminal and the command line (`tix inbox prefs`).

The list keeps itself current. It is bound to the event stream for `inbox.updated`, and when one arrives
it fetches its own page again and swaps the bar and the list in place, the way the task list does: the
scroll position and focus stay where they were, a refresh waits while you are in the middle of something,
and the rows the redraw brought are washed in the accent colour for a few seconds (shown without the fade
when your system asks for reduced motion).

### Notification channels

Settings has a Notifications section for sending your inbox items elsewhere as they arrive: a Slack
incoming webhook, a Discord webhook, an ntfy topic URL, or your own endpoint taking signed JSON. Each
channel has a name, a URL, a format, the kinds it sends (mentions, assignments, status changes, comments)
and an on switch. The add form uses the same layout as New project. These are yours, kept with your
account, and only your items reach them; a kind you turned off above is not sent either. They are not the
tenant's webhooks under Admin.

Adding a channel opens its own screen, which shows the signing secret once (signed JSON deliveries carry
`X-Tix-Signature` made with it), then the channel's settings, Send test, and its recent deliveries with
their state, attempts and the last answer, each not yet delivered with a Redeliver button. A refused
delivery is retried on the same schedule as a tenant webhook, and private network addresses are refused
the same way. Each message says who did what to which task and links it when the tenant has a domain.

### Mentioning someone

Typing `@` in a comment box, the task description or the new task form's description offers the handles
of the tenant's directory, the same list the assignee field suggests, narrowed as you type: handles that
start with what you typed first, then those containing it. The arrow keys move through the list, Enter
or Tab inserts `@handle` and a space and does not submit the form, and Escape closes the list and
nothing else, so a comment in the task dialog stays open. The box is a combobox while the list is open,
so a screen reader announces the option under the arrows. On a phone the list is a bottom sheet, and
each option is at least 40px tall on a touch screen. Without JavaScript the box is a plain textarea, and
a handle typed in full mentions just the same. An `@` after a letter, as in an email address, is not a
mention.

## Starring a task

Every task the browser draws carries a star: beside the tick on a list row or a lane card, before the
reference on a board card (the task screen's board and a project's own board alike), and beside the
title on the task screen. Pressing it stars the task, pressing it again unstars it, and the reader
lands back on the screen they pressed it from, filters and all, at the row they pressed.

A starred task comes first. The list puts the reader's starred tasks above everything else and keeps
the chosen sort inside each group, and each board column and each lane does the same, because they
are the same listing split by state or by project. Paging stays exact across the point where the
starred tasks end.

A star is personal. It belongs to the reader who set it, so two people sharing a queue each see their
own stars first and never reorder each other's screen. It is kept on the server, so it follows the
reader into the command line and the terminal interface, and starring in one tab refreshes the
reader's other open task screens through the same live updates any other change uses.

The button is a plain form posting to `/tasks/{ref}/star` or `/tasks/{ref}/star/remove` with the
CSRF token, so it works with no script. It is a real button, `aria-pressed` says whether the task is
starred, and its name says what pressing it does: "Star INFRA-2" or "Unstar INFRA-2". The icon is an
outline until pressed and filled after, so the state is a shape and not a colour alone. Starring
needs only what reading the task needs.

## Snoozing a task

Snooze hides a task from every listing, for everybody, until a time you pick, and then it comes back
by itself. Nothing has to run for that: every listing compares the wake time with the clock. Agents
cannot claim a snoozed task until it wakes.

Snooze is in two places. On the task screen it is in the actions menu (the three dots in the header); on
a lane card it is a section of the three dots menu. Both offer **Later today** (three hours on, rounded
up to the hour), **Tomorrow morning** (09:00), **Next week** (09:00 next Monday) and **Pick a date**, a
date and time field with its own Snooze button. Every choice is read in the time zone chosen under
Settings, or the server's when none is, so tomorrow morning is your morning. A time that has passed is
refused and nothing changes: on the task screen the refusal is a toast, in the lane it is said inside
the card. List rows have no snooze control of their own; open the task or use the lanes.

Each choice is a plain form posting to `/tasks/REF/snooze` with `until` set to `later`, `tomorrow` or
`next-week`, or with the picked time in `at`, so it works with no script. With htmx a lane card's
snooze swaps the card, and because a snoozed task is not in the listing the card leaves the lane and
the toast names the task and when it wakes.

A snoozed task is drawn only where it was asked for, under `is:snoozed` or `snoozed:any` in the filter,
and there it carries a dashed badge saying until when: "until 3 Nov 09:00" on a row or a card, "Snoozed
until ..." in the Snoozed row of its own screen, where hovering names who snoozed it. Its task screen's
actions menu offers **Unsnooze** in place of Snooze, as does its lane card's menu; it posts to
`/tasks/REF/snooze/remove` and the task is back at once. Snoozing and unsnoozing need `task:write`, and
a reader without it is offered neither. See [filtering.md](filtering.md#snoozed-tasks).

## Moving a task through its workflow

The status pill on a row is a control. Opening it lists the states that row's own workflow can reach
from where it is, which is per project: a listing mixes projects, and a state legal in one need not
be legal in another.

States that are not adjacent are offered too. The panel spells out the whole route before it is
applied, `todo → doing → review`, and says how many steps it takes, so the reader agrees to the
states it passes through rather than finding them in the history afterwards.

Each step is then an ordinary transition. A three-state route performs three service calls, writes three
audit entries and emits three events, exactly as making those moves one at a time would. That is
intended: a task that passed through `doing` did pass through it, and the trail should say so. If a step
is refused after earlier ones have applied, the message names the state actually reached and says it
stopped there, rather than reporting a move that did not happen.

The panel is a popover. It was an absolutely positioned disclosure first, inside a list that clips
its overflow, so on a low row the choices were cut out of the scrollable area entirely: they took no
pointer events and could not be scrolled to at any position, which reads as a frozen page. The top
layer is outside every clipping ancestor, and `popovertarget` gives light dismissal and Escape with
no script. A browser that does not understand the attribute gets the panel inline, which is the list
of choices it always was.

## Walking a listing

Every paged listing renders the same control. It says where the reader is and how much there is,
"Page 4 of 18 · 437 tasks" in the listing's own noun, and offers First, Previous, Next and Last, the
numbered pages around this one with a gap for each run it leaves out (`1 … 3 4 5 … 18`), and a small
**Go to page** field. The field is a plain GET form whose hidden inputs carry the listing's own
parameters, so it works without scripting and keeps the filter; a number past the end opens the last
page. A listing that fits on one page renders no control at all, rather than a set of steps that lead
nowhere. There were six controls before the shared one, five of them a bare paragraph holding a link,
and none said where the reader was.

**The page is in the address.** Every link the control writes is `?page=N` and the listing's own
parameters, with no cursor, so a page can be linked, reloaded and bookmarked, and page one is the bare
listing. On the task screen the view switch and the Group lanes by control keep `page`, so a reader on
page 3 of the list who opens the board and comes back is on page 3 again; the board and the lanes
ignore it (see [List, board or lanes](#list-board-or-lanes)). The filter form submits no
page, and Clear, the links that narrow a refused board to some projects, and Show all drop it, so any
change to the selection opens on page one.

**How the count and the jump work.** Listings page by keyset and never by `OFFSET`, and that is
unchanged. A task listing is counted by the service with the same filter, tenant scope and access as
the page (a capped `COUNT` over the same predicates, so PostgreSQL's row-level security applies to it
as well), and page N is turned into a cursor by a seek: the same ordering, star rank included, read
`(N-1) × size` rows deep, and the last of them becomes the cursor the page starts from. The page itself
is then read exactly as Next would have read it. The other paged listings (Activity, the directory,
users, tenants, inbox, deliveries, sync runs, projects, the account log) have no count of their own, so
the browser counts them by walking their own listing in 500-row stretches.

**Caps.** The task count stops at 10,000 and the walked count at 1,000. Past the cap the control
writes the page count and the total as floors, "Page 3 of 400+ · 10000+ tasks", leaves out Last, and
Next keeps working beyond it with a cursor link, the way the control worked before it counted.

**Live updates move rows, not the reader.** A live refresh fetches the screen's own address, so a
reader on page 4 is redrawn on page 4 with the new total. A numbered page is a position rather than a
snapshot: a task created ahead of page 4 shifts its rows by one, as on any numbered pager, and a page
that no longer exists after deletions draws the last one. That is the trade-off for a page that can be
named; a cursor still addresses an exact place.

**Cursor links still work.** A link carrying `cursor` opens the page that cursor names, as before, and
the API's cursor is unchanged. Such a link also carries a trail of the cursors it came through, which is
how Previous worked before pages were counted:

```text
/tasks?cursor=C3&trail=C0.C1.C2
```

The trail arrives from the address bar, so it is whatever anybody cares to type. Every entry has to
decode as a cursor this build issues, and the whole value has to be within forty entries and 4096
bytes. A value failing any of those is discarded entirely rather than repaired: half a trail would
send Previous to a page the reader was never on, which is worse than sending them back to the first
page. Changing the filter or the sort discards both the cursor and the trail, since they name
positions in one ordered result set.

## Keys in upper case

Project keys and task references are drawn upper case wherever the interface shows them to a reader:
`INFRA`, `INFRA-8`. They are stored, linked and sent in lower case, so links keep pointing at
`/tasks/infra-8`, a form value or a data attribute holds `infra-8`, and the API and every export carry
`infra-8` as they always have. Anything typed or followed takes either case: `/tasks/INFRA-8` and
`/projects/INFRA` show the task and the project without a redirect, and `project:INFRA` filters the
same tasks as `project:infra`.

## Naming a project in a picker

Every control a project is chosen in names it the same way: the add box, Add with details, the bulk Copy
to, a heading's New task in, Copy to, New from template, a template's project, the move target of a
project delete and the statistics filter. The Project row on the task screen reads the same way.
Settings, under Listings, picks the style: `Name (KEY)` by default, `KEY · Name`, `Name` or `KEY`. The
choice is a cookie, `tix_project_label`, like every other display preference, and the value each option
submits is the key whatever the label says. A name past 24 characters is shortened with an ellipsis so
the key after it is never cut off, and on a phone the add box and a heading's add field, which are only
a few characters wide, list projects by key alone. A project's text icon leads the label, since a native
option holds text only.

## Changing a project's key

The project board's settings carry the key beside the name and the description, with an (i) that says what a
change does. Saving a different key does not change it straight away: the next screen names the old and the
new key and shows two of the project's references before and after, such as `INFRA-1 → OPS-1`, and the key
changes only when that is confirmed. The confirmation carries the rest of the form, so the name or
description changed in the same save are kept, and so is the icon. A browser cannot carry an uploaded file to
a second screen, so when the same save uploaded an icon image the confirmation asks for the file again. It needs
no script.

The old key stays an alias of the project for as long as the project exists. Opening `/projects/infra`,
`/projects/infra/fields`, `/tasks/infra-8` or a screen under it answers `301 Moved Permanently` to the same
screen under the new key, query kept, so bookmarks and pasted links keep working. A form posted to an old
address is served where it was sent, so a tab left open across the rename still saves. The text of task bodies
and comments is not rewritten: a reference typed there stays as it was typed, and still resolves.

## The task screen

The link above the title goes back to the listing the task was opened from, and says which: "Back to
tasks" for the task list, with its filter, sort and page, or "Back to the infra board" for a project
board. A list row carries the listing in its link; anything else, a board card, the toast, a subtask, is
answered from the page that linked here. Every save on the screen returns to it with the listing still
attached, so the link still goes to the same place after a tag, a comment or an edit, and deleting the
task returns there too.

The screen reads like an issue. The header holds the reference, the title in large type with **Rename**
beside it, your star, **Hand to an agent**, **Share** ([Sharing a task](#sharing-a-task)), and one actions
menu behind the three dots: **Copy the task id**, **Copy to…**, **Move to…**, **Save as template**, **Snooze…** or
**Unsnooze**, **Reclaim the lease…** when you may take a live lease, and **Delete task** with its two options.
Nothing in that menu is drawn anywhere else.

On a wide screen the main column holds the description, subtasks and dependencies, then a tab strip for the
task's activity, and the column on the right holds the properties: status and priority, which open the same
menus a list row does, the snooze and claim state when there is one, assignee, due date, project, tags, repeat
rule and each custom field. Each is a label and a value, and a writer changes it where it is read: point at or
focus the value, type, and press Enter or the Save that appears. Each row posts only its own field, so a save
never touches anything else, and with scripting off it is a plain form post. An enum field is a dropdown that
saves when chosen. The record's facts follow in small type: who created the task, when it was created and last
changed, when it started and finished, its version and its identifier with a Copy button. On a phone the
properties come first, straight under the title, and the facts come last. The task dialog a listing can open
draws the same screen.

**Comments, History and Artifacts** are tabs under the dependencies, each with its count; a zero count is
drawn muted, and History's count is its newest page, with a plus when older changes exist. Click a tab,
or focus one and use the arrow keys, Home and End. The browser remembers the tab you last chose, in
`localStorage` under `tix.taskTab`. The address wins over that: `#comments`, `#history` or
`#artifacts`, a link to one comment or artifact, an older page of history (`?history=CURSOR#history`)
and a raw entry the address names each open the tab holding it. A live update redraws the lists and the
counts but never the tabs, so the one you are reading stays. With scripting off there is no strip:
the three panels stand one under another, each under its own heading.

The Artifacts tab's form can attach a file. Above the button it states the largest file and the types this
tenant accepts, and the file picker offers only those types. A file over the limit or of another type is
refused with both named. An attached file has a Download link, and it is always a download: an HTML or SVG
file is never opened as a page. A tenant administrator sets the tenant's own limits under Tenant settings,
Largest artifact file and Accepted artifact types, each empty to use the server's (shown greyed) and never
past it.

Tenant settings also carries the tenant's session settings: Sign-in lasts, Sign out when idle for, and Keep me
signed in lasts (a duration or `off`). Each is empty to use the server's, shown greyed, and can only be
shorter. A new lifetime applies to the next sign-in; the idle timeout applies to every session as soon as it
is saved.

The star is a shape in the header; its name says whether you starred the task (Star or Unstar). A subtask
names its parent beside its reference and in a Subtask of row.

**Subtasks** draws the whole tree under the task, as `tix task tree` prints it. Three levels show on load;
a subtask on the third level with subtasks of its own offers **Show N subtasks**, which loads three more
levels in place from `/tasks/REF/tree`, and without scripting is a link to that subtask, whose own screen
draws the rest. The arrow beside a subtask folds its subtasks away and back. Without scripting there is no
arrow and the tree is open. A tree too large to walk is drawn one level deep, with a line saying so. The
count beside the heading is the direct subtasks.

When a page fails, the error screen offers the page that linked to it and the task list, each named for
where it goes. It used to offer "the dashboard", which was `/`, a redirect to the projects list.

**Edit** beside the description's heading turns it into the editor in
place, reading **Cancel** until you save; a task with no description offers **Add a description** instead.
Both are disclosures, so they work with no script, and the `e` shortcut opens Rename and focuses the title.
The description is drawn from its Markdown: lists, tables, code, links, and references such as
`infra-8` and `@handles` linked to their pages. A checklist written `- [ ]` draws as checkboxes. With
`task:write` each one ticks in place: it is a button a screen reader announces as a checkbox, Space and
Enter press it, it flips at once and the screen swaps in place when the answer arrives, and a description
somebody changed in the meantime is refused like any other stale save, with the box put back. Without a
script the press posts the form and the page reloads with the item ticked. Comments render the same way,
their checkboxes always read-only. The comment box is a plain textarea, with a Preview link beside it
that shows the text as it will be drawn without saving it. The description editor, on the task screen and
in the New task form, has two modes, Rich text and Markdown, switched at the end of its label row and
remembered per browser. Markdown is the textarea with its Preview. Rich text edits the same text
formatted, with a toolbar for bold, italic, strikethrough, headings, lists, checklists, quotes, code,
links and tables, and `@` completion as in the textarea. The form still sends Markdown, and only an edit
rewrites it: opening and closing Rich text changes nothing. A text Rich text could not give back as it
found it, such as a list item that opens with a code block or a single-tilde strikethrough, stays in
Markdown with a line saying so. Without a script there is only the textarea. Rows and cards in every
listing carry a `3/5` checklist count, and board cards a thin bar under it. What is shown as typed rather
than rendered, and why images are linked rather than loaded, is in [features.md](features.md).

Dependencies name each task this one waits for by reference, title and status, linked to the task,
with the full identifier on the row's title. They read back a raw identifier while the field directly
under them was placeholdered `infra-2`, so the screen disagreed with itself about what identifies a
task. A dependency this reader may not read keeps its shortened identifier and gains no link, so a
dependency they cannot open is still visible as one rather than missing from the list.

### Reacting to a comment

Under each comment sit its reactions: one chip per emoji with how many people left it. A chip you left is
highlighted and reads as pressed to a screen reader, and hovering or focusing one names everyone behind
it. Pressing a chip adds that emoji as yours, or takes it back when it is already yours. The `+` button
after the chips opens a small picker over the thread with eight quick reactions (👍 👎 ❤️ 🎉 😄 😕 👀 🚀)
and a box for any other emoji; each is a button, so Tab and Enter work it without a mouse, and Escape
closes it. The server refuses anything that is not a single emoji. Someone without `comment:write` sees
the chips but cannot press them. Reactions arrive live through the same event stream as comments, and
notify nobody.

## Copying a task

**Copy to…** in the task screen's actions menu, offered to a reader who may write tasks, opens a
small form: the project the copy goes to, the reader's own project selected, the title ready to edit, and
what else to bring. The copy is a new task with its own reference, in that project or the same one. Its
description, priority, tags, assignee, due date and repeat rule come with it, and so do the custom fields
the target project defines. It starts in the target workflow's first state unless **Keep the status** is
ticked and the target has a state of that name. **Comments** copies each comment as a new one by you that
names who wrote it and when, **Subtasks** copies the whole live subtree under the copy, and
**Dependencies** makes the copy wait for what the original waits for. **Link the copy to this task**, ticked
by default, records where the copy came from: its properties then show **Copied from** with a link back.
Stars, claims, history and artifacts never travel.

The form is laid out like New task: the title is the large field, the project sits beside it, the options
are one group of checkboxes, and Cancel and Copy close the form at the bottom right, with Ctrl+Enter
(Cmd+Enter on a Mac) to copy and Escape to cancel. A copy that is refused, such as one into a project that no longer
exists, comes back with every choice kept and the reason under the field it is about.

When tasks open in a dialog the form opens in the same dialog. Copying returns to the original with a
toast linking the copy, or opens the copy when the "Open a new task" setting is on, exactly as adding a
task does. A lane card's three dots menu offers **Copy to…** too, opening the same form over the lanes
when tasks open in a dialog; copying from there returns to the lanes, filter and all, with the toast. The
command line has the same operation as `tix task copy`, and the API as `POST /api/v1/tasks/{ref}/copy`.

## Moving a task

**Move to…** sits under **Copy to…** in the task screen's actions menu, for a reader who may write tasks. It
opens a small form with one choice, the project to move to, listing every project but the task's own. The
task itself moves: it keeps its comments, artifacts, history, dependencies, tags, assignee and stars, and
takes the next number in the target project. Its old reference keeps working, so a link, a mention or a
bookmark of `/tasks/INFRA-4` redirects to the task under its new reference.

The status stays when the target's workflow has a state of the same name; otherwise the task starts in the
target's first state and the toast says so. Custom fields the target project does not define are left
behind. A task somebody else holds a claim on is refused with the reason in the form until the claim is
released; a claim of your own moves with the task. Moving opens the task under its new reference. Boards
open on either project refresh on their own. The command line has the same operation as `tix task move`,
the API as `POST /api/v1/tasks/{ref}/move`, and the bulk bar's **Move to…** moves every selected task.

## Handing a task to an agent

**Hand to an agent** in the task screen's header, beside the star and the actions menu, opens a dialog of
text that hands this one task to a coding agent. The same button is in the header when the task is open in
a dialog. On a phone the button is its icon alone. Every piece has a Copy button:

- **Prompt**: an instruction for an agent that runs the tix CLI. It names the task's reference, its title
  and the address this browser reached the server at, and the commands that do the work: `tix claim task`
  to claim it, `tix task show` to read it, `tix comment add` for progress and `tix claim release` with
  `--status done` to close it.
- **Start it from your client**: one tab per client [mcp.md](mcp.md#wiring-it-into-a-client) documents.
  Claude Code, Codex, OpenCode, Gemini CLI, Cursor and VS Code have a command that starts them with a prompt
  (`claude`, `codex`, `opencode --prompt`, `gemini -i`, `agent` and `code chat`), shown with the prompt
  after it. Claude Desktop and Zed have none, so their tab holds the MCP prompt to paste into the chat.
- **With the tix MCP tools**: the same prompt for an agent that has `tix mcp` connected, naming the tools
  `claim_task`, `task_show`, `comment_add` and `claim_release`.

A command passes the prompt as one argument in single quotes, quoted for sh, bash and zsh, so a title
holding quotes, `$(...)`, backticks or `$` reaches the agent as text and is never run. A title over several
lines is folded onto one. Nothing in the dialog carries a token: the agent needs tix connected first, with
a token of its own ([agents.md](agents.md#tokens-and-scopes), [mcp.md](mcp.md)). The prompt is in English
whatever language the interface speaks, since a model reads it.

The browser remembers the last client tab chosen, so the next task opens on it. Without scripting the button
draws the dialog in place, with no tabs and every client's block one under another.

## Sharing a task

**Share** in the task screen's header, beside Hand to an agent, opens a small menu. The same button is in
the header when the task is open in a dialog, and on a phone it is its icon alone:

- **Share…** opens the system share sheet, on a phone or wherever the browser has one
  (`navigator.share`), with the reference and title and the address. It is not drawn where there is
  no sheet. Dismissing the sheet does nothing; a browser that refuses it copies the link instead.
- **Copy link** copies the task's address.
- **Copy as Markdown** copies `[INFRA-12 Fix the build](https://tix.example.com/tasks/infra-12)`, the
  title on one line with `\`, `[` and `]` escaped, ready for a pull request or a wiki.
- **Copy key and title** copies `INFRA-12 Fix the build`, for a chat message or a commit.

Each copy says what it did in a toast that a screen reader announces ("Link copied"), the menu closes and
focus goes back to the Share button. The menu is a popover like the actions menu: Escape closes it, and
the arrow keys, Home and End move along it. Its foot shows the address itself as a plain link that
selects whole, with a (?) that opens this section. With scripting off the menu still opens, the copy
buttons are not drawn, and the link is there to select. A lane card's three dots menu offers Copy as
Markdown, Copy key and title and Share… beside its Copy link and Copy reference.

The address is the task's screen, `/tasks/REF`, on the scheme and host the browser is on, so behind a
proxy it is the address the reader typed. When the server is configured with `server.public_url`
([configuration.md](configuration.md#the-public-web-address)), a host serving several tenants copies the
link under that address instead, so a reader who reached the server by an internal name or a proxy's
backend port still hands out the address everybody else opens; on a tenant's own domain the link stays on
that domain. A reference under a project's old key still opens the task
([Changing a project's key](#changing-a-projects-key)). It names no tenant, and needs none: on a shared
host the tenant comes from the reader's session ([tenancy.md](tenancy.md#switching-tenant)), so another
member of the same tenant, signed in to it on that host, opens the task. Someone signed in there to a
different tenant of theirs switches tenant first, and someone not signed in signs in and lands on the
task. A `?tenant=` on the link would not help: the interface refuses a tenant named in the request.

The command line prints the same address with `tix task link REF`, and `--as markdown` or `--as text`
print the other two forms. The address is the task's `web_url` when the server sets one, else the server
the context talks to; a local database without `server.public_url` has no web address, so there it needs
`--base`. Over SSH `tix task link` prints the `web_url` and refuses `--base`, and it exits 2 naming
`server.public_url` on a server without it. The terminal interface shares a task with `^`, through the
terminal's clipboard ([tui.md](tui.md#sharing-a-task)). Over the API and MCP every task carries the same
link as `web_url` while `server.public_url` is set.

## Acting on many tasks

A row shows one round control, the tick that marks it done. Selecting rows for a bulk action is a mode
you turn on, so the square selection boxes never sit beside the circle on every row and get pressed by
mistake. **Select** in the screen's toolbar turns it on, on the list, the board and the lanes alike; so
does `x` on a focused row (which also selects it), Shift-click or Ctrl/Cmd-click on a row's or card's
empty space, and on a touch screen a long press of half a second that does not scroll. A press on a
title, a button or a field keeps its own meaning, so Ctrl-click on a title still opens a tab.

In select mode every row, board card and lane card shows a square box labelled "Select INFRA-12", the
tick stays but is dimmed so the box reads as the control, and a bar appears under the listing saying
how many are selected (announced to a screen reader) and offering **Move**, **Priority**, **Tag** (add
or remove), **Assignee** (a handle, or empty to unassign), **Star**, **Unstar**, **Copy to…**,
**Move to…** (another project), **Delete** and **Cancel**. Shift-click ticks every row between the last box
you clicked and this one;
`x` toggles the focused row; Shift+Up and Shift+Down extend the selection from the focused row and
move focus with it; Ctrl+A (Cmd+A on a Mac, `*` in the Emacs scheme, where Ctrl+A goes to the top)
ticks every row while focus is in the list, and leaves a text field's own select all alone. `d` marks
the focused task done in any mode (`e` stays Edit title, as in the terminal). Escape, Cancel, Select
pressed again, or unticking the last selected task leaves the mode and clears the selection; an open
menu or the task dialog keeps its own Escape. While anything is ticked the live refresh waits, the
same rule that keeps it from drawing over a half typed comment.

The mode is the toggle's `aria-pressed`, and the boxes and the bar are always in the page, shown by
the stylesheet only while it is pressed. The toggle sits outside everything a live refresh, a row's
own swap or the in-place filter redraws, so none of them drops the mode, and a row redrawn while
ticked (marked done in select mode, say) comes back ticked. Leaving the screen leaves the mode.

Each action is one request for the whole selection. The server changes each task on its own, with its own
history entry and event, so a task that cannot take the change does not hold back the rest: the toast
says how many changed and names each refused task with the reason, such as a status its workflow cannot
reach. Move offers every state the projects on the page run and walks each task along the shortest route
of its own workflow. A copy is linked to its original. Delete is a soft delete, and the toast's one
**Undo** restores every task it removed in one request.

With scripting off, Select is a link to the same address with `select=1`, which draws the screen in
select mode, and Cancel is the link back without it. The boxes belong to the bar's form, so ticking
rows and pressing an action posts the selection, and the action returns to the address it came from,
still in select mode. `select` is kept like the rest of the screen's state: by the view switch, the
Group lanes by control, the pager, the filter form, Clear and Show all. `tix task bulk` and
`POST /api/v1/tasks/bulk` are the same operation.

## Exporting the list

**Export CSV** under the Columns choices in the More menu downloads the listing you are looking at: the
filter, the deadline window, the sort, the open work default and the projects you have not put away, every
page of it up to 10000 rows. The columns are the title and then whichever columns are ticked just above,
in the order the list draws them. Cells are quoted the RFC 4180 way, and a cell starting with `=`, `+`,
`-`, `@`, a tab or a carriage return gets a leading `'`, so a spreadsheet shows it as text instead of
running it as a formula. `tix task ls -o csv` and `GET /api/v1/tasks?format=csv` export with a fixed set of
columns.

## Exporting a project

**Export** in a project's header opens a dialog that writes the project's tasks as CSV, JSON, NDJSON or a
Markdown table. It takes a filter in the task list's language, whether finished tasks, subtasks and the trash
are included, the date style (ISO, the day alone, or the date format and timezone you chose on the settings
screen, for a spreadsheet), the CSV separator and the columns. Every column has an up and a down arrow, and
the ticked ones are written in the order shown. The download streams, so a large project starts at once. A
filter that does not parse brings the dialog back with your choices and the reason under the field. See
[import-export.md](import-export.md).

## Importing into a project

**Import** in a project's header, for readers who may import, opens five steps, each a page of its own so
the browser's Back walks them: **Upload** a CSV, JSON or NDJSON file or paste a list of tasks, one per line
in quick-add syntax; **Map columns**, where every column of the file has a field chosen for it; **Map
values**, the statuses, priorities and people of the file against this project's; **Preview**, the counts
and the first rows as they will be written, every row in error and a CSV report of them; and **Result**.
Every step has Back, which keeps what was chosen, and Cancel, which leaves; Escape in the form is Cancel.
A note on the first step says what is easy to miss: a few tasks need no import at all, and the mapping is
optional. When every column is recognised, the columns step says so and **Skip to preview** goes straight
there. Nothing is written until **Import**, and then every row or none. See
[import-export.md](import-export.md#importing-tasks-into-a-project).

## Task templates

A template is a named shape of a task that is not itself work: it never shows in a listing, a claim or the
statistics.

**Save as template**, in the task screen's actions menu and in a lane card's three dots menu,
opens a small form drawn like the copy form: the template's name (the task's title to start with), whether
it is for **This project** or **All projects**, whether the subtasks come along (ticked when the task has
any), and an optional **Due in** such as 3d or 12h. The template keeps the title, description with its
checklists, priority, tags, custom fields and repeat rule. Status, assignee, due date, comments and history
stay with the task. A name another template already holds, in any case, is refused under the name field.
When tasks open in a dialog the form opens in the same dialog; saving returns to the task with a toast.

**Templates**, under Admin in the sidebar, lists every template the reader can see with its project
(or All projects), the title it creates, how many tasks it holds, its due offset and how often it was used.
Each row links to **Use** and **Edit**. The edit screen renames the template, moves it to a project or to
all projects, and changes the title, description, priority, due offset, tags, repeat rule and custom fields
(one `key=value` per line; a line removed removes the field). Its subtasks are listed for reading; save a
task as a template again to change them. **Delete template** is the red button beside the title, and
it asks in a dialog first; tasks already made from it stay as they are.

**New from template** creates a task, and a subtask for each one the template holds. It is offered by the
**From template** button at the end of the add box, a menu listing the templates with All templates… at
the bottom, by a link at the top of Add with details, and by Use on the Templates screen. The form takes
the template, a title (empty keeps the template's), the project (the template's own to start with, which a
template for all projects needs when the tenant has several) and **Due in** (empty for the template's own
offset, 0 for none). Titles may carry placeholders, matched in any case and filled in at each use:
`{DATE}` is today's date as YYYY-MM-DD, `{N}` the use number and `{PROJECT}` the project's key. The task
starts in the project's first state and keeps the custom fields the project defines. Creating returns to
the listing the form was opened from with a toast linking the new task, or opens it when "Open a new task"
is on. A refusal comes back with every value kept and the reason under its field.

The add box menu is a popover like the lane card menu: arrow keys move through it, Escape closes it and
returns focus to its button, and on a phone it rises from the bottom of the screen with 40px rows. Every
entry is a link, so it all works with scripting off.

## Acting in place

Every action on a list row and on the task screen answers where it was taken. A row's star, status
and priority swap that row and nothing else, as the tick always did, and a lane card's star swaps that
card, as its other controls already did; a star, move, priority, tag or comment on the task screen
swaps the page content and leaves the sidebar alone. The list keeps its
scroll position and the control you used keeps focus. The element the forms sit in declares this
with `data-inplace` (`templates/tasks.html`, `templates/task.html`) and `assets/feel.js` reads it at
submit; with scripting off the forms post and redirect exactly as before. In the task dialog the
dialog's own body is what its forms swap, so nothing there declares it.

The answer is drawn before the server gives it: the star fills, a moved row's pill names its new
state, a priority chip shows the one chosen, and the row fades slightly while the request is out (only
after 80ms, so a quick answer does not flicker). A refusal or a lost connection puts back what was
there and says why in a toast.

**The toast** sits in `#toast`, which every signed-in screen carries even when empty so an in-place
answer can bring its toast with it. It leaves on its own after five seconds, ten when it offers Undo,
and stays while the pointer or focus is on it. Undo is offered for a move whose way back the task's
workflow allows (directly or through other states; a workflow with no way back offers none), for a
priority change, and for a soft delete, which it restores. It is an ordinary form, so it works with
scripting off. The redirect carries it as `flashundo`, which every address the screen builds from the
request drops, as it drops `flash`.

Undo is a browser convenience over ordinary operations, not an operation of its own, so the command line
and the API undo the same things directly: a move with `tix task mv` (or `--hops` when the way back passes
through another state), a priority with `tix task edit --priority`, and a soft delete with
`tix task restore`, `POST /api/v1/tasks/{ref}/restore`. `tix task ls --trashed` finds what was
deleted. A delete with `--hard` has no way back on any surface, and neither has any delete on a tenant
whose trash is off.

**Live updates** never take the screen from you. A refresh waits for a request in flight, an open
menu or a row drawn ahead of its answer (`is-pending`) in the region it redraws. When it lands it
patches the region in place rather than replacing it (`tix.morph` in `live.js`): every row, card,
board column, lane and scrolling box still on screen stays the same element, and only rows that are
genuinely new are inserted. Replacing them was what made scrolling under a burst of changes snap
back (the wheel was scrolling a box that had just been thrown away), what made every scrollbar on the
screen blink, and what replayed every row's entrance. Each scrolled box, down and sideways, and the
page itself stay on the row and the column you were looking at, so a task added above you does not
push what you are reading down; instead a **N new, show** pill floats over the top of the box, and
pressing it scrolls up to them. It goes away on its own when you scroll to the top. A listing redraws
at most twice a second under a burst (the first change at once, the last after 500ms), a background
refresh never shows a busy pointer, and the list frame, the board's columns and the lanes keep their
scrollbar's room so a row that makes them scroll does not reflow the rest. Focus is put back by the
same rule everywhere (`tix.focusMemo` in `decide.js`, used by live updates, lane cards and in-place
actions): the nearest id and the control's `data-focus` name, and a choice made in a menu counts as
the button that opened it. A refresh that keeps the focused element leaves focus alone.

**A new release** reaches an open tab by itself. Each page names its build on `<body data-build>`,
and the tab compares it with the `version` from `/healthz` when the live connection comes back after
a drop, when you return to the tab, and every seven minutes. If the server runs a newer build and
nothing on the page would be lost, the tab reloads, keeps its scroll position and says
**Updated to vX** for a few seconds. If you are typing, have changed a field, have a dialog or menu
open, are waiting on a request or have rows selected, a bar says **A new version is available** with
a **Reload** button instead, and the tab reloads by itself once you have sent or left what you were
doing. Without scripting the next page you open is simply the new build.

**A pressed button** says so in the same frame: its spinner takes its icon's place and the label stays
readable, or, on a button of text alone, takes the label's place with the label hidden but still
holding the width, so nothing moves. Nothing about it fades in, which is what used to leave the
spinner drawn over the words for a moment. A second press while it waits sends nothing, a button
with its own `hx-post` is marked the same way, and the mark clears when the request ends wherever its
answer is drawn. The pointer over a submitted form turns to progress only after 300ms.

**Moving between screens.** A link into a task or a listing starts on the mouse press rather than the
release, which takes 80 to 150ms off each one (main button, no modifiers, never touch or keyboard; the
click that follows is swallowed). Screens cross-fade over 150ms where the browser has view
transitions, and not at all for a reader whose system asks for reduced motion. Back to the list
restores where it was scrolled: the list scrolls inside its own box, which htmx's history does not
remember, so the positions are kept in `sessionStorage` per address. The filter box applies as you
type, 300ms after the last key, and the deadline and sort selects apply when changed; Apply stays for
a browser without scripting. The list, board and lanes switch swaps the page instead of reloading it.

**Keyboard shortcuts** are behind "?" on every screen, and behind Keyboard shortcuts in the account
menu at the foot of the sidebar for anyone who does not know to press it. The overlay used to be
looked up once when the page first loaded, so after the first boosted navigation "?" opened a copy
nobody could see.

## The command palette

ctrl+k, or command+k on a Mac, opens the command palette on every screen, in every key scheme and
from inside a text field too; the Search button under the brand in the sidebar, and in the bar on a
phone, opens it as well. It is one search box over four groups:

- **Tasks**, searched on the server once something is typed: the query is read by the same filter
  language as the filter bar, so `is:starred deploy` narrows the way it does there, and a half-typed
  term is searched as plain words. A ref, in any case, puts its task first. Ten at most, most
  recently changed first.
- **Projects**, read once when the palette opens, each going to its board.
- **Screens**, every destination the sidebar shows (the configuration and data ones under Admin only
  with Advanced screens on) and every section of Settings.
- **Actions**: New task, New task in a project, New project, Open the filter, Show tasks as a list, a
  board or in lanes, Switch the colour scheme, Keyboard shortcuts and Sign out. Each one presses a
  control the screen already has; from a screen without it, the palette goes to the one that has it.

Screens and actions are matched as you type with a forgiving match (`stg` finds Settings) and draw the
moment the palette opens, with no request behind them. Nothing is asked of the server for tasks until
something is typed, and then only 150ms after the last key. With nothing typed, the last things chosen
in this browser come first under Recent, along with recently viewed tasks when the sidebar keeps them.

Up and Down move through the options, Enter chooses, Escape closes and focus goes back where it was.
A task chosen here opens the way **Task opens in** says: in the dialog over the page, or as its own page.

The palette reads `GET /palette/tasks?q=` and `GET /palette/projects`, two JSON routes behind the
session and the tenant like every screen. They are not a new capability: over the API the same
questions are `GET /api/v1/tasks?q=...&limit=10`, `GET /api/v1/tasks/{ref}` and `GET /api/v1/projects`.
The terminal interface has its own palette on the same ctrl+k.

## The directory

`/actors` lists everyone and everything work can be assigned to in this tenant: handle, kind, display
name and identifier. Kind is on every row because it is the one thing a handle does not tell you, and
work is assigned to agents as often as to people.

The same listing feeds the assignee field on the task screen. It is a `datalist` rather than a
`select`, so the field suggests without constraining: an actor from another tenant is deliberately
assignable by identifier, and a control that only accepted what it listed would remove that without
saying so. A listing that fails leaves the suggestions empty rather than failing the page, because
the field takes a typed value either way.

The field takes a handle as readily as an identifier. The reference is resolved in the service, so
the browser, the command line and the API obey one rule rather than each transport obeying its own,
and it is classified by shape: a value of exactly the length and alphabet of a generated identifier
is an identifier and is passed through without a lookup, which keeps an actor of another tenant
assignable. Anything else is a handle, and one this tenant does not have is refused as not
found, naming the reference, rather than reaching storage, where it used to fail a foreign key and
surface the constraint text to the reader.

The listing answers who is here and never what anybody may do: scopes, roles and token identifiers
are dropped before it leaves the service, so being signed in is the only thing it asks for. It orders
by handle and refuses any other sort rather than returning rows in an order that is not the one asked
for.

On the command line the same directory is `tix actor ls`, and `tix actor show` resolves one
identifier.

### The actor screen

`/actors/ID` is one actor, opened from its handle in the directory, a task's assignee and creator, a
comment's author or a token's owner. It takes a handle as readily as an identifier, as `tix actor show`
does. It names the handle, display name, kind and identifier, links to the tasks assigned to the actor and,
for a reader holding `token:admin`, to its API tokens and SSH keys, and lists its twenty latest audit
entries worded as the activity feed words them, with **All activity** leading to the feed narrowed to it.
A reader without `audit:read` is told the list needs that scope and still sees the rest. Like the
directory it never says what the actor may do: that is its own business, and yours is in Settings under
Profile.

## Statistics

`/stats` carries the same figures as `tix stats` and the terminal interface's `S` view, over one
window and optionally one project. [statistics.md](statistics.md) covers what each figure means and
what the leaderboard does and does not measure.

## Activity

`/activity` is the tenant's audit log as a feed, newest first, grouped by record, and it updates as
events arrive. It is the browser's `tix audit ls` and the terminal's history view. The filter bar is a
plain `GET` form, so a narrowed feed is a link somebody can send: `q` searches the records (a task
reference such as `infra-5` works), `kind` picks a kind of record (tasks, comments, artifacts,
projects, workflows, users, memberships, sign-ins, API tokens, webhooks, the tenant), `via` the
channel a change came through (web, API token, CLI on the database, CLI through a server, SSH, MCP, sync,
import, system, or unknown for entries older than the channel), `token` the API token by name, `action`
the action an entry records, matched exactly (`task.create`, `task.update`; the box suggests the actions
on the page), and `from` and `to` a time window, picked as a date and time and read in your own time zone.
`to` includes the minute it names, and a bare date in a typed link (`to=2026-11-12`) includes that whole
day. The actor's name on any row narrows to that actor. A `source` link from before the channel existed
still filters by surface.

Every row ends with its channel as an icon and a few words: "via web", "via SSH", "via token ci-agent",
"via MCP (token scout-runner)", "via CLI", and "via sync (token ci-runner)" for a sync or import an agent
started with a token. Hovering it shows what identified the caller: the client
address, the user agent, the SSH key fingerprint, the token's name and ID, the session reference, the job
for background work. The row's Raw view lists the same details, and the channel itself links to the feed
narrowed to that channel and token. A task's History has the same channel in its Channel column, linked the same way.
[audit-log.md](audit-log.md) says what each channel means and what is and is not stored.

Each active filter is a chip that removes itself, and the live refresh keeps the filter rather than
widening back out. On the command line the same narrowing is `tix audit ls --filter "kind:task via:web"`;
[filtering.md](filtering.md) has the expression.

Raw on a row, and on a row of a task's History, opens the entry as recorded in a dialog: who, what, the
full time, the source and the sequence number, the fields that changed side by side, and the before and
after snapshots as indented JSON, each with a Copy button and a Copy all as JSON button for the whole
entry. Without JavaScript Raw is a link to `?dialog=raw-SEQ`, which draws the same dialog in place.

Beside Raw, every row on Activity, on an actor's screen and in a task's History has a Copy JSON button.
It copies the row's audit entry exactly as the API returns it, pretty-printed (an array when the row
groups several steps), and says Copied for a moment. The JSON is part of the page, so the copy needs no
second request, and where the browser offers no Clipboard API (a page served over plain HTTP) it falls
back to the older copy command. Audit entries never hold a token's secret, so neither does the copy. On
the command line `tix audit ls -o json` prints the same entries.

## Housekeeping

The Tenant screen has a Housekeeping section: how many days a done task, a cancelled task and a task in the
trash wait before cleanup acts, each Off or After a number of days. Each project's settings screen has a
Housekeeping section that overrides any of them, with an Inherit choice naming what the tenant has set.
Under every rule that is on, both say how many tasks it would act on now, and both say cleanup is paused while
the trash is off. Clean up now on the Tenant screen runs one pass; otherwise `tix serve` runs it hourly. See
[configuration.md](configuration.md#housekeeping).

The project's Housekeeping section has its own Clean up now, a red button that runs one pass on that
project alone, the way `tix cleanup run --project KEY` does. It opens a confirmation dialog first that
says how many of the project's tasks go to the trash and how many are deleted for good, and the toast
after it says how many were. It is greyed out while nothing is due or the trash is off. Like the rest of
the section it is shown to tenant admins only.

## Claim length

A task's Lease row sets how long a claim on it holds, and under it the screen says how long a claim taken now
would last and which level decided it, such as "Claims last 4h, from the project". The Claims section of a
project's settings screen (`/projects/KEY/settings`, Settings on the board) sets the project's lease length and its
longest lease; both blank inherit the tenant's, and the tenant's are on the Tenant screen. The order is in [agents.md](agents.md#how-long-a-lease-lasts).

## Server administration

Three screens under Admin answer for the installation rather than the work in it, and each
is the browser's view of a command. `/admin/status` is `tix status`: what the installation holds and
which servers are running against it. `/admin/connections` is `tix connection ls` and `kill`: the live
event streams and SSH sessions this server holds, and ending one. `/admin/ssh-keys` is `tix user key`:
the public keys the SSH listener accepts, enrolled and revoked. The screen enrols a key in the tenant
you are in; enrolling one key in every tenant you belong to is `tix user key add --all-tenants`, or
`"all_tenants": true` on the API (see [tenancy.md](tenancy.md#ssh-keys-in-several-tenants)).
[deployment.md](deployment.md) covers the listener and live connections.

## Connect an agent

`/connect`, in the sidebar and on the API tokens screen, has every command a person needs to point a CLI, an
MCP client or an agent at this server, filled in for it. The server address is the one the browser reached,
with the scheme taken from `server.trusted_proxies` the way the calendar feed takes it; the tenant key is
filled in for a reader allowed to read the tenant, and left out otherwise, since a token already names its
tenant. It reads as three steps:

1. **Get a credential.** The token form with the scope picker below, drawn once for every tab that needs a
   token. Only a reader holding `token:admin` gets the form; it also shows `tix token create` and
   `tix token rotate --grace 10m`. SSH needs no token, and the step says so.
2. **Pick how you connect.** A strip of five tabs: CLI, MCP, SSH, API and Skill.
3. **Copy the commands** in the chosen tab. Each tab has a help button (?) opening its documentation section.

| Tab | Holds | Help opens |
| --- | --- | --- |
| CLI | the install (`install.sh`, PowerShell, `go install`, the container image, pinned to the server's version), `tix ctx add`, `tix task ls` to check, and `tix login --tenant` for a reader whose address and tenant are known | [configuration.md](configuration.md#contexts) |
| MCP | `tix mcp` for Claude Code, Codex, OpenCode, Gemini CLI, Claude Desktop, Cursor, VS Code and Zed, a tab each, and `tix mcp --list`. These are the snippets in [mcp.md](mcp.md), from one Go source a test holds to that page | [mcp.md](mcp.md#wiring-it-into-a-client) |
| SSH | enrolling a key, the terminal interface, a tix command over ssh (`ssh acme@host task ls -o json`), an export and the event stream. Drawn only when `tix serve` runs with `--ssh-listen`, at `ssh.public_address` when set ([configuration.md](configuration.md)); otherwise the tab says the server runs no listener | [deployment.md](deployment.md#commands-over-ssh) |
| API | `curl` with the token, and the OpenAPI document | [api.md](api.md#authentication) |
| Skill | installing the agent skill for Claude Code, in the project or for every project, or pointing another agent at it | [agents.md](agents.md#the-shape-of-a-worker) |

Every tab is a link, `/connect?tab=mcp`, and so is each nested one: `install=`, `client=` and `skill=` name
the install method, the MCP client and where the skill goes, so `/connect?tab=mcp&client=codex` opens the
Codex commands. The server draws the named tab selected and hides the rest, so the page works without
scripting, and an unknown name falls back to the first tab. With scripting the strips are the task screen's
tabs (`assets/tasktabs.js`): a click switches without a reload, the arrow keys, Home and End move along a
strip, and the address is rewritten to name the tab on screen. Unlike the task screen, the choice is kept in
the address only, not in the browser. Issuing a token comes back to the tab it was issued from.

Before a token is issued the commands read `$TIX_TOKEN`. Issuing one draws the value once, with the scopes it
got, and the page's script writes it into every command, in every tab. It is not stored and not put in the
URL, so a reload puts `$TIX_TOKEN` back. Every `tix` command the screen shows, the ones run over ssh
included, is checked against the command tree by `cmd/connect_test.go`.

**Docs** in the sidebar and the account menu, and the help button on each tab, go through `/docs/PAGE`, which
redirects to that page on GitHub at the running release's tag (`main` for a development build). Rendered pages
name no external origin, and only a page that exists under `docs/` is redirected.

### Choosing scopes

The token form here and on the API tokens screen use one picker: a checkbox per scope, grouped as Tasks,
Projects, Workflows, Comments and artifacts, Events and Administration, each saying what it allows. A scope you
do not hold is drawn disabled with the reason, since you cannot grant what you do not have; the server refuses
it too. With scripting there are presets (Agent, Read only), Select all, Clear, a checkbox per group and a
line saying what the token will be able to do. Select all ticks each scope you hold, never `*`; an
administrator gets `*` as its own choice with a warning. The issued token's confirmation lists the scopes it
was granted.

### Rotating a token

On the API tokens screen every live token has a Rotate button beside Revoke, on the whole-tenant listing, on
one actor's listing (`/admin/tokens?actor=ID`, where the directory's token links lead) and on your own. It
opens a dialog naming the token and, when it is not yours, whose it is, with a choice of how long the old
value keeps working: stop it now (proposed), 10 minutes, 1 hour or 24 hours. Rotate issues a new token with
the same name, scopes, project and lifetime and shows its value once with a copy button, the same block an
issued token gets, back on the listing you came from.

While its window is open the old token is listed as *retiring*, with the time it stops, and offers Revoke but
not Rotate: revoking it ends the window early, and its replacement is the one to rotate next. See
[agents.md](agents.md#rotating-a-token) for the lifetime and who may rotate what.

## The release screen

`/release` is linked twice from the sidebar footer: the running version opens its "What's new"
section, and a "vX is available" line under it, shown when a newer release exists, opens the top. It
answers one question first: is there something newer? A single card at the top says which of five
things is true:

| State | When |
| --- | --- |
| Update available | The newest published release is ahead of the running version |
| Up to date | It is the running version, or behind it |
| Not comparable | One of the two versions is not a `major.minor.patch` this build can order |
| Check failed | The last check got no usable answer and none ever has |
| Not checked yet | The server has not finished its first check |

The check runs on the server, in the background, at most every six hours, and never from the
browser. A failed check is not logged and is drawn quietly: an isolated network is the normal case
for a self-hosted tracker.

With an update on offer the screen lists one copyable command per install method: `tix update` for
a release binary, `podman pull` of the new image tag for the container, and `go install` of the new
version for the Go toolchain. None of them restarts a running `tix serve`. [upgrading.md](upgrading.md)
explains what `tix update` checks and what it refuses.

When a newer release exists, its notes are drawn as Markdown by the same renderer as a task
description, with every link taken out first, since the text comes from a server this installation
does not control. When the installation is current they are left out, because they would repeat the
section below.

"What's new" (`/release#changes`) is read from the `CHANGELOG.md` embedded in the binary when it was
built, so it answers with no route out. The running version's entry comes first, marked "You are
running this", and the ten releases before it follow, folded. A build that is not a tagged release,
such as `dev` or `0.31.0+dirty` from an edited tree, says so and lists the newest entries with none
marked as running. Entries go through the same link stripping and Markdown renderer as the feed's
notes: the text of a link stays, its target and the trailing commit hash do not.

## Choosing a colour

Every place the browser asks for a colour, a project's on the new project form and in its settings, and
the tenant's accent on the tenant screen, uses one control: the predefined colours as swatches, none or the
default where the colour is optional, and Custom, with a colour picker, a `#rrggbb` box and a preview. The
box alone is enough with scripting off. A custom project colour is stored as `#rrggbb` in the same field a
palette name is, and is drawn wherever a palette colour is, in a shade per colour scheme that clears 4.5:1
on that scheme's page and surface, the same floor the palette is tuned to. A colour that already clears it
is drawn exactly as chosen.

## Actions and dialogs

What a screen offers to do is a button, never a fold at the foot of the page:

- **Making a record** (Issue a token, Create a user, Map a hostname, Enrol a key, Register an endpoint, Add a
  member, Add a channel, Define a field, Configure a source, Save this filter) is a primary button with a plus
  in the page header, or beside the heading of the section it adds to, and an empty list carries the same
  button. It opens the form in a dialog: focus stays inside it, Escape or Cancel closes it, and a refused
  submission comes back with the dialog open, the message under the field and what was typed kept.
- **A step that cannot be undone** (Revoke a token, Delete a user, Delete template, Remove channel, Clean up
  now) is a red button that opens a dialog saying what happens, whose own red button takes the step.
- **A project's settings** are their own screen at `/projects/{key}/settings`, reached from **Settings** in
  the board's header: Project (name, key, workflow, description, colour and icon), Claims, Housekeeping and,
  last, the Danger zone with Archive project and Delete project. Every section is always drawn.
- **Reference text** (what a snapshot or a bundle carries, what the sync run list leaves out) is drawn as
  text, and the sign-in help and the key scheme comparison open in a dialog from a button.

Without scripting each button is a link to the same screen with `?dialog=ID`, which draws the dialog in
place at the top of the screen.

## Deleting a project

Delete project in the danger zone of a project's settings opens a confirmation screen at `/projects/{key}/delete`; nothing
is deleted by the button itself. A project with no tasks gets one button. A project with tasks, soft
deleted ones counted, offers two choices:

- **Move them to another project.** A select lists the other live projects, and the screen shows the
  status mapping that target implies: the same state when its workflow has it, otherwise its first state
  of the same category, otherwise its initial state. Choosing another target redraws the mapping (with
  scripting off, through the Show the mapping button). Each task gets the target's next number and keeps
  its comments, attachments, tags, dependencies, stars and history. The reader lands on the target's board.
- **Delete them with the project, permanently.** The project key has to be typed. Nothing is restorable
  afterwards: project deletion is permanent, and Archive is the reversible way to put a project away.

The command line spells the same choice `tix project rm KEY --move-tasks-to OTHER` or `--delete-tasks`, and
the API `DELETE /api/v1/projects/{ref}?move_tasks_to=OTHER` or `?delete_tasks=true`.

## Display preferences

Every display preference under Settings is a property of the person reading, not of the tenant, so
each one is stored in a cookie on that browser and reaches no other reader, except the date format and
the timezone, which are kept with the account (see Dates and times below). The Second factor and
Database sections are not preferences: the first is your enrolment, kept by the service and the same
from every surface, and the second only shows a tenant administrator what this server is talking to.
Two people sharing a tenant are frequently in different timezones for the same reason they may want
different keyboard shortcuts.

Settings carries all but one of them. Three are also editable from the screen they affect, because
that is where the decision is made: whether a column reads better absent is answerable only by
looking at the listing, and ticking projects off a task list is something you do while watching the
list redraw. Those three are one control rendered in two places, not two controls: one constructor
builds the state, one template renders the form, one function reads the cookie and one handler
writes it, so changing a preference on either screen is what the other shows. A preference
with two editors has two defaults, two validations and two readings of one value, and the drift that
follows is not noticed until a reader is looking at two screens that disagree.

The exception is the colour scheme, whose only control is the switch at the foot of the sidebar, on
every signed-in screen. It is the preference a reader changes while looking at the screen that made
them want to change it, which is the same argument as the three above, and it is the one that could
be made small enough to live beside the Settings link. It cycles: one press posts the next of System,
Light, Dark, OLED, Low contrast and System, OLED when dark, and returns to the page it was pressed on,
so it stays a button rather than becoming a select with an Apply beside it, which is what a
six-position control costs with scripting off. Its accessible name says both the scheme in force and
the one a press moves to, since the glyph says neither and a `title` never reaches a keyboard user
who does not hover. Settings names the current scheme and points at the switch rather than carrying
a second form.

A shortcut never takes a key from the control it is pressing. Enter on a focused button or link, and
Space on a focused button, do what they do natively; the `open` shortcut is bound to Enter, and it
used to claim the key everywhere, so no button on the page could be pressed from the keyboard. While a
status or priority menu is open no shortcut acts at all, so Escape closes the menu rather than also
going back a page.

| Preference | Cookie | Values |
| --- | --- | --- |
| Colour scheme | `tix_theme` | empty follows the system, else `light`, `dark`, `oled`, `dim`, `system-oled` |
| Keyboard shortcuts | `tix_keyscheme` | the shipped schemes, `?` shows the active one |
| Advanced screens | `tix_advanced` | whether the configuration screens are in the sidebar's Admin group |
| Drag to move | `tix_drag_move` | `0` stops board cards being dragged between columns; absent or anything else lets them |
| Open a new task | `tix_open_after_add` | `1` opens the new task after adding. Absent or `0` stays on the listing. An empty value, which earlier builds wrote for "on", also opens it |
| Hover effects | `tix_hover` | `0` turns hover highlights off and puts `data-hover="off"` on `<html>`; absent or anything else leaves them on |
| Columns | `tix_columns` | which optional columns each listing leaves out |
| Hidden projects | `tix_hidden_projects` | which projects a listing leaves out |
| Task opens in | `tix_task_open` | `dialog` opens a task followed from a listing in a dialog over it; absent or anything else opens its own page |
| Clicking a title | `tix_title_click` | `edit` makes a click on a listed title rename it in place, with an open link beside it; absent or anything else opens the task |
| Task view | `tix_task_view` | `board` draws the task screen as columns, `lanes` as one lane per project; anything else, including no cookie, is the list |
| List groups | `tix_list_group` | `project` or `status` draws the list under a heading per project or per state; anything else, including no cookie, is one ungrouped list |
| Language | `tix_lang` | `en` or `bg` draws every screen in that language; anything else, including no cookie, follows the browser's `Accept-Language` |
| Colour rows by | `tix_tint` | What the stripe down a task row and the edge of a board card stands for: `status`, `priority` or `none`. Anything else, including no cookie, is the project's own colour |
| Project names in pickers | `tix_project_label` | How a project reads in every control that chooses one: `key-name` (`INFRA · Infrastructure`), `name` or `key`. Anything else, including no cookie, is the name with the key, `Infrastructure (INFRA)`. The option's value is always the key |

`tix_advanced` has three states rather than two: shown, hidden, and absent, which leaves the answer
to who is reading. A tenant administrator who has never chosen gets the configuration and data
screens under Admin, because the tenant screen was otherwise reachable only by typing its URL: the
only entry to it lived behind a preference on a settings page they had no reason to open. Anybody else
gets the short navigation, since every screen in those groups refuses them and an entry leading to a
refusal is worse than no entry. Being on one of those screens expands its group whatever the
preference says, so a reader is never on a page the navigation beside them denies exists. Once the
preference is set, in either direction, it decides.

`tix_columns` is per listing, so Settings offers one picker per listing rather than one global
control: the listings share no column vocabulary, and the columns worth keeping on Tasks are not the
ones worth keeping on API tokens. Each picker is the same form that listing carries beside itself.
Every listing keeps the column naming its rows whatever is chosen, so no choice can put a record out
of reach, and hiding a column withholds nothing, since every value stays on the record's own screen.
Reset returns a listing to its declared defaults, which is not the same as unticking everything: a
column declared off by default is shown by the first and not the second.

`tix_hidden_projects` names each project as well as keying it, because on Settings there is no task
list in front of the reader to decode a key against. A key naming a project that no longer exists
(deleted, or archived since the choice was made) has no checkbox, since the project it names is
gone, so the control names it in a sentence instead and the next submission of the form forgets it.
Such a key excludes nothing: an unknown project key in a filter is refused as not found by the
service, deliberately, so sending one would answer the task screen with a not-found page and leave
the cookie unreachable from the only screen that could clear it.

`tix_task_view` has three positions and no unset one. The board and lanes are the stored values;
absent, empty, stale or anything another program left behind is the list, which is what an untouched
install shows. Choosing the list stores nothing and expires the cookie, so both switches offer three
positions and neither has an "unset" to show.

`tix_columns` carries a version marker, `v2~`, and names the columns each listing *hides*. Recording
the columns shown cannot tell a column the reader turned off from a column that did not exist when
the reader chose, so every column added afterwards read as one that reader had refused, and the
readers it silenced were exactly those who had used the picker at all. A value written in the older
form is recognised by the absence of the marker and converted rather than read as it stands, which
would mean the opposite: every column a reader put away stays away, every column they kept stays,
and the columns that form could not name are shown. The marker covers the whole value rather than
each column, because the inversion changes what the empty-list sentinel means as well as what a key
means.

An unusable value is never stored and never reaches a page. A format the build does not implement, a
zone that does not resolve, a scheme that is not shipped, an oversized cookie some other program left
behind: each falls back to the deployment's own configuration rather than taking the screen down.

The binary embeds the zone database (`time/tzdata`) rather than trusting the host to carry one, so
the zones on offer are the same on a distroless image as on a developer's laptop, and a reader's
chosen zone is not quietly replaced by the deployment default on a base image with no `tzdata`. The
fallback stays regardless: a value that does not resolve must not take a page down.

### The settings screen

Settings is eleven sections (Profile, Appearance, Listings, Keyboard, Dates and times, Inbox, Notifications,
Second factor, Sessions, Calendar feed, Database) with a navigation of plain anchors to each. Database, which
names the server's database path or a redacted DSN, is drawn for a tenant administrator only, and the
command palette offers it to nobody else either. Where the settings block has room the navigation is a
column that stays in view beside the sections; where it does not, it is a wrapped row of links above
them. The choice is a container query on the block rather than a viewport breakpoint, because the
sidebar is resizable and collapsible and the room beside it is what decides.

The navigation follows the reader: as the page scrolls, the link for the section in view carries
`aria-current="true"`, and once the page is at its end the last section's link does, however short that
section is. A section counts as in view once its top reaches the line a jump to it lands on, so a click
and a scroll mark the same link. Clicking a link, or pressing Enter on it, scrolls smoothly to the
section (a jump when the system asks for reduced motion), puts its id in the address and moves focus to
the section, so the next Tab carries on from there. This is `assets/tocspy.js`, which drives any
`nav[data-toc]` from two IntersectionObservers and no scroll handler; the settings screen is the only
one with such a navigation. Without scripting the links are plain anchors, and the section last jumped
to is marked through `:target` instead.

Profile is who you are signed in as, the browser's `tix whoami`: handle, email, display name, actor id
with a Copy button, kind, role, tenant, whether you may act as a global admin, what this request
authenticated with (the browser session, or an API token by name), every tenant your account is a member
of with your handle and role there, and every scope you hold. It links to your actor screen. The tenant
shows its key and name for every credential, an agent's token included, as whoami reports them.

Each setting is a row: its name and one line saying what it does on the left, its control on the
right in a column every row shares, so selects are one width and switches line up. The longer
explanation is behind the (i) beside the name. At 900px and below each row stacks its control under
its description. On/off preferences are switches: a submit button with `role="switch"` and
`aria-checked`, its state in words (On and Off, Shown and Hidden), so they still post with scripting
off. A choice between two named positions is not on or off, so it is a segmented control naming both:
Task opens in is Page or Dialog, Clicking a title is Open the task or Edit the title, as Task view and
List groups name theirs. Each segment is a submit button posting its own position (`open=page`,
`click=edit`); a post naming none flips the setting, as the switch did.

Every section is a heading and then its card. A section with a description (Inbox, Notifications,
Calendar feed) draws it as the card's first row, and every row carries its own (i); no heading does.
Notification channels are rows of the Notifications card, and **Add a channel** beside the section's
heading opens the form in a dialog, as Create a user, Issue a token, Enrol a key, Map a hostname and Add a
member do on their screens. A refused form comes back with the dialog open.

On a phone or tablet, where the main input is a finger (`(hover: none) and (pointer: coarse)`), the
Hover effects row, the Drag to move row and the Keyboard section are not drawn, nor are the account
menu's Keyboard shortcuts entry, the palette's key hints, its Keyboard and Keyboard shortcuts entries
and the create forms' Ctrl+Enter line. A board card is not lifted by a long press there; its Move menu
is how a finger moves it. A touch laptop keeps all of them.

Every form on the screen returns to `/settings?flash=...#section`, so after the full-page reload the
reader is back at the section they changed with a toast naming what was saved. The shared Listings
controls (task view, projects, columns) are pointed back the same way when rendered on Settings and
keep returning to their own screen elsewhere.

### Hover effects

Rows, cards, lane cards, sidebar links, buttons and links respond to the pointer: a row tints and
gains an accent edge, a card lifts and gains a shadow, a link in running text thickens its underline.
The rules are written once under `:root:not([data-hover="off"])` and inside `@media (hover: hover)`,
so a touch screen never gets a sticky highlight. Turning the preference off sets `data-hover="off"`
on `<html>`, which removes them and resets the older hover tints to their resting look; focus rings
are not hover effects and stay. Movement and transitions are declared only under
`prefers-reduced-motion: no-preference`, so a reader whose system asks for less motion gets the
colour without the lift whatever the preference says.

### Opening a task in a dialog

With Task opens in set to Dialog, following a link to a task from a listing, a board, the lanes, the
activity feed, search results or the toast after adding opens the task screen in a modal `<dialog>`
over the page instead of leaving it. It is the same screen: the server answers a request carrying
`X-Tix-Dialog: 1` with the task screen's content block and its flash, without the layout, inside an
element marked `data-task-dialog`. The ids the task screen shares with a listing's rows (the status
and priority menus, the description, priority and repeat fields) are prefixed in that answer, and the
task's own live binding is left out, so nothing in the dialog addresses the page behind it.

The address changes to the task's, with an entry of its own in history, so a reload or a copied link
draws the full page, Back closes the dialog, and Forward opens it again over the same page. Escape,
the Close button where the page has its back link, and a click on the backdrop all close it by going
Back. A popstate leaving the dialog's entry is stopped before htmx sees it, so htmx does not restore
an older snapshot of the page over the live one. Focus returns to what opened the dialog, or to the
task's row when a refresh has redrawn it; while the dialog is open the rest of the page is inert,
which is the focus trap, and the keyboard shortcuts stand down. A link to another task inside the
dialog replaces the entry; any other link leaves for its own page.

Every form in the dialog posts as it does on the page, and the request's header survives the
redirect, so the answer redraws the dialog in place. The listing underneath is refreshed through its
live binding (`tix:refresh`), whether or not its socket is open. A save that redirects somewhere other
than the task, which is what deleting it does, closes the dialog and loads that page; a refusal keeps
the task and shows the error screen's message above it. A task that cannot be opened at all opens as
its own page, which explains why. Under 600px the dialog fills the screen.

A click with a modifier, a middle click, and any click without scripting open the full page.

### Renaming a title in place

With Clicking a title set to Edit the title, the title of a task in the list, on a board card and on
a lane card carries its reference and version, and a small open link is drawn beside it. A click turns
the title into a text field. Enter or leaving the field posts `version` and `title` to the task's
edit address (`POST /tasks/{ref}`), the same address and the same version check as the task screen's
form; Escape puts the title back, and an unchanged title sends nothing. A refusal, an empty title or
a conflict with a newer version included, is shown under the field and the field stays open; leaving
it does not resend a refused value. Enter or F2 on a selected row edits its title, and the open link
opens the task as Task opens in says. The edit address changes the description and assignee only
when the submission names them, so renaming leaves them alone. Without scripting a title is a link.

### Dates and times

The date format and the timezone are kept with the account, not the browser, so every browser and
device a person signs in from shows dates the same way, and so do `tix task ls` and the terminal
interface. They resolve in three levels: the person's own choice under Settings, else the tenant
default an administrator sets on the Tenant screen, else the server's `output.time_format` and
`output.timezone`. Each control offers Inherit, and names what Inherit would give and from where, and
the line under it says what is in use and where it came from, such as "from tenant default". The
format and the zone resolve separately.

Earlier builds kept these two in the `tix_time_format` and `tix_timezone` cookies. The first request a
browser still carrying them makes moves each value onto the account, unless the account already holds
one for that field, and expires both cookies. There is no per-browser override any more.

Every timestamp a screen shows goes through one template helper, which writes a `<time>` whose
`datetime` stays RFC 3339 and whose text is in the reader's format. Its title, its accessible label and
a tip shown on focus carry the full instant: the date, the time to the second, the zone and its offset,
and how long ago it was (`06.11.2026 14:05:32 EET (UTC+02:00) · 2 hours ago`). Each one takes focus, so
a tap on a touch screen or Tab from the keyboard shows the tip, which a `title` never does. A relative
time such as "2 hours ago" in the activity feed or a task's history shows the absolute instant the same
way. A timestamp inside a link or another element's title stays plain text, because nothing focusable
may sit inside a link.

A test renders every screen in `scripts/screenshots/routes.json` on a seeded database as a reader in
`dotted` and `Asia/Tokyo` and fails on a timestamp in another layout, one written as text instead of
through the helper, or a `<time>` missing part of its tooltip. A new field that prints a date raw fails
it.

### The terminal and the command line

A browser preference is a cookie on that browser. The terminal interface's settings screen writes the
CLI's own configuration keys instead, because somebody at a terminal already has a configuration file
and a second place to write a timezone down would be a second place to disagree with the command
line. The two surfaces therefore do not share a store, and setting a preference in one does not
change the other:

| Preference | Browser | Terminal and CLI |
| --- | --- | --- |
| Date format | the account | the account, unless the machine sets `output.time_format` |
| Timezone | the account | the account, unless the machine sets `output.timezone` |
| Keyboard scheme | `tix_keyscheme` | `tui.keymap` |
| Colour | `tix_theme`, which palette | `output.color`, whether colour at all |
| Advanced screens, drag to move, open a new task, hover effects, task opens in, clicking a title, columns, hidden projects, task view, colour rows by, language | cookie | no equivalent |
| Selected-row pulse | no equivalent | `tui.motion` |

So a timezone chosen in the browser is what `tix task ls` prints too, unless that machine's
configuration names one, and one chosen in the terminal's settings screen stays on that machine.
See [tui.md](tui.md) for the terminal screen and [configuration.md](configuration.md) for the keys.

The zone list is a selection rather than every name the system carries; a zone set from the command
line that is not on it is still shown, rather than replaced by the first option. A `select` of six hundred
entries is not a control anybody can use, and a fixed list also bounds how many template sets one
process can be made to parse: the formatting helpers are bound into a template at parse time, so a
per-reader zone has to be a separately parsed set rather than a value handed to `Execute`.

The preference forms, the sidebar's scheme switch among them, deliberately opt out of htmx boosting.
`data-theme` lives on `<html>` and a boosted form swaps only the body, so the cookie changed while
the page kept its old scheme, which reads as a button that does nothing. A full navigation costs one
round trip and is correct.

The colour scheme is a separate axis from the tenant accent, which is the same everywhere and covered
in [theming.md](theming.md).

### OLED

OLED is a black page for screens that light every pixel they draw. The page is `#000` and the
surfaces are `#0a0b0d`, close enough to black that a card or a field reads as raised through its
border rather than a grey slab, so the strong line is brighter than dark's and clears 3:1 on both.
Ink is a shade softer than dark's to cut the halo white text gets on black. The accents, the state
colours, the priority colours and the project palette are dark's, which were tuned for 4.5:1 on a
lighter surface and clear it with more room on black; the soft tints behind a selected row or a
badge are darker, so a highlight does not light a band across the screen. The sign-in card drops
its accent glow. A tenant's dark accent and a custom project colour's shade apply as they do under
dark.

System, OLED when dark is offered as its own position rather than as a second setting. Most readers
who want OLED are on a phone that turns dark at night: pinning OLED paints the page black at noon
too, and a separate "prefer OLED" toggle would be a second cookie that only means anything under one
value of the first. Plain System still maps to light and dark and stays the default.

The scheme is a cookie on the browser like the rest, so it follows no tenant or account default, and
no link carries it. The terminal interface has no light or dark scheme, only the tenant accent and
`NO_COLOR`, so OLED is browser only.

`TestEveryDarkTokenHasAnOLEDValue` fails when a token is added to dark and not to OLED, or when the
OLED block and its System, OLED when dark copy differ; `TestTheOLEDSchemeClearsTheContrastFloor`
computes its contrast; `TestEverySchemeRuleKnowsOLED` fails on a dark rule elsewhere in the
stylesheet with no OLED twin.

## Language and info tips

Every screen is drawn in the language the browser asks for in `Accept-Language`, and says so in the
root element's `lang`. English and Bulgarian ship; a header naming neither gets English. Keys are the
English text (`internal/i18n`), so a word a catalog lacks reads in English rather than as a blank.
Settings can override the browser's choice for this browser (`tix_lang`).

A small (i) beside a heading, a filter or a figure opens a sentence or two on what it means and names
the page in `docs/` that says more. It is a disclosure, so it works with no script, from the keyboard
and on touch. Beside a heading, a label or a setting's name the note opens under that line and pushes
what follows down, rather than floating over the next row. One in a toolbar, the view switch or the
task screen's header, floats on a wide screen with a mouse and opens under the toolbar the same way
on a phone, a tablet or any window narrower than 900px.

With scripting an open note closes when you move on: a click or tap anywhere outside it and its (i),
Escape (which puts focus back on the (i)), or opening another tip. A click inside the note leaves it
open.

A (?) at the right end of each screen's header opens the section of these docs about that screen, in a
new tab, for the release the server runs: it links to `/docs/PAGE?section=ANCHOR`, which redirects to the
page on GitHub at the running version's tag (or `main` for a development build) with `#ANCHOR` on the end.
`internal/web/help.go` holds the one table of screens and sections, and its tests fail on a screen
without a button or a section whose heading is gone.

A placeholder is italic, in the muted colour and the body font even in a monospace field, so it does
not read as a value already saved, and one that gives an example says so: "For example, laptop".

To add a language, add `internal/i18n/<lang>_*.go` files registering one entry per key. The tests in
`internal/web/i18n_internal_test.go` name every key it is missing.

## The second factor

A tenant can require a time-based code in the browser, and any reader can enrol in one without being
asked to. Either way the sign-in screen grows a second step: the password form answers with a code
form instead of a session, carrying a signed ticket in a hidden field. The ticket says only that the
password was accepted in the last five minutes, so it grants nothing on its own, and no session
exists until the code is taken. The destination a deep link was bounced from rides through both
steps, so the reader still lands where they were going.

An account the tenant requires a factor from that holds none is taken through enrolment on that same
step rather than refused. That keeps turning the requirement on from locking out the
administrator who turned it on, and it is why the recovery codes appear there: they exist for exactly
one response and are shown before the reader goes on.

The code field also takes a recovery code. A verifier that has been locked by repeated failures says
so and says to wait, because a reader told their code is wrong tries again immediately, which is what
the lock exists to stop.

Under the tenant's second factor policy (Tenant screen, Second factor: Off or Required for everybody, with
a grace period) the sign-in does not enrol inside the second step. A reader with no factor is signed in, and
every screen they open sends them to `/settings/totp`, which says why, offers Set up a second factor, the
account history and Sign out, and nothing else. An htmx request or a form post from an older page answers
403 with `HX-Redirect: /settings/totp`. Once the code is confirmed the same session works everywhere. During
the grace every page carries a banner naming the deadline and linking to that screen. Removing your own
factor is not offered under the policy. The Users screen marks each person 2FA or no 2FA and can be narrowed
to Without a second factor (`?without_totp=1`), which the Tenant screen links to beside the deadline. See
[second-factor.md](second-factor.md#the-tenant-policy).

Settings carries the rest: enrolment, how many recovery codes are unused, replacing them, and
removing the factor where the tenant does not require one. The user administration list offers an
administrator holding `totp:admin` the clear, which is the way back in for somebody who lost both the
authenticator and the codes. No secret and no recovery code appears on either screen afterwards.

The `otpauth://` code is drawn as inline SVG by the server, by the encoder in `internal/web/qr.go`
rather than by a client-side library. Every form here is a plain form: a sign-in that needed a script
would be a sign-in some readers could not complete, and a script that drew the code would need the
enrolment secret handed to it.

## Inviting people and required password changes

Create a user on the Users screen offers two ways in, as one segmented control: Set a password and Send
an invite link. The choice is a radio pair, and the fields the other way needs are hidden by the stylesheet
alone, so it works with scripting off. With a password, Require a password change at first sign-in is ticked
by default: a password an administrator typed for somebody else is one they should replace.

Send an invite link takes the address, a display name, a role and how long the link works (1, 3, 7, 14 or
30 days, 7 by default). The account is created at once, without a password, and the screen comes back with
a dialog holding the link, its expiry and a Copy button. The link is shown that once: only its hash is kept.
tix sends no email, so the dialog says to send the link on. The link's address is built from the scheme and
host the request reached the server at, the same way calendar feed addresses are.

Pending invites lists the links issued and not used, with the address, role, who invited them and when the
link expires, or that it has. Re-issue stops the old link and shows a new one in the same dialog. Revoke is a
red button behind a confirm dialog; it stops the link and keeps the account, without a password.

The link opens `/invite/TOKEN`, a page outside the application shell like the sign-in screen. A new account
chooses a password, twice, and may change the display name; an account that already had a password confirms
it instead and keeps it. Accepting signs the person in, through the second factor step when the tenant asks
for one. Every link that cannot be used, whether unknown, expired, used or revoked, answers the same page with
the same words and a 404, so a link says nothing about any other. These pages send `Referrer-Policy:
no-referrer`, because the address carries the secret.

The Manage dialog of each account carries Ask to change the password at next sign-in. An account with it set
signs in to a Choose a new password page instead of a session. Nothing else is reachable from there, because
nothing is signed in yet: the page asks for the current password again beside the new one and its
confirmation, and its only other way out is back to sign-in. The new password must be at least 12 characters
and differ from the current one; a refusal redraws the page with the reason beside the field. Saving clears
the flag and signs in.

## Sessions

Settings carries a Sessions section listing your own browser and terminal sign-ins in this tenant, newest
first: the user agent each signed in with, when it signed in and how (password, SSH key or a tenant switch,
and whether a second factor passed), and when it expires, with Kept signed in on one made with Keep me
signed in. The one this browser is using is marked This browser. Each row has a red Revoke button that
opens a dialog naming what happens; the form is only in the dialog. Revoking another session signs whatever
holds it out at once and returns to the section with a notice; revoking the one in use signs this browser
out. Session tokens are never shown, only an identifier that cannot sign anybody in. Sessions that predate
this list show Unknown client, because the user agent was not recorded then.

## Calendar feed

Settings carries a Calendar feed section. Its first row states the feed: No feed, with a filter field and
Create feed below it, or Active, with when it was created and that its address is shown only once, at creation
or rotation, so Rotate is how to get a new one; the filter and Save filter, Rotate and Revoke follow. The
address itself is never on Settings: only its hash is kept. Revoking returns to the section with a notice.
Creating and rotating answer a screen that shows the address once, as text a reader with scripting off can
select, beside a Copy button and a link that asks the device's calendar to subscribe. That screen is rendered
rather than redirected to, so the secret never travels in a URL the browser keeps, and it is sent
`Cache-Control: no-store`. A filter the service refuses draws Settings again with the text kept in the field
and the reason beside it. [calendar.md](calendar.md) covers the feed itself.

## Project icons

The new-project form and a project's settings carry an icon picker. Emoji takes one emoji or one or two
letters, Icon offers a grid of built-in symbols drawn in the same style as the navigation, Upload takes a PNG,
JPEG, WebP or SVG of up to 64 KB, and No icon removes it. The preview beside the choice shows what the project
will carry; saving the form with Upload chosen and no new file keeps the image already there.

An uploaded image is drawn through `<img>`, where an SVG runs no script and fetches nothing, and an SVG that
carries script is refused on upload anyway. It is served from `/projects/{key}/icon` with its own content
type, `nosniff`, a sandboxing content security policy and an ETag; the screens address it with the image's
digest, so a browser keeps it until the image changes.

## Static assets

The stylesheet, the scripts and the icon are compiled into the binary and served from memory with a
strong `ETag` derived from their bytes and `Cache-Control: public, max-age=60, must-revalidate`. A
conditional request for an unchanged asset gets a 304 with no body.

The validator matters because the asset URLs do not change between releases. The responses used to
carry no validator at all, so a browser cached them heuristically and after an upgrade a reader got
the new markup with the previous release's stylesheet. That happened during review and was reported
as the interface being broken. Each file is fingerprinted independently, so changing one does not
invalidate the others.

**Compression.** A browser that accepts gzip gets every screen, fragment and asset compressed. The
task list was about 185KB of HTML per load, swap and live refresh, and is about 17KB on the wire
now; the stylesheet goes from 200KB to about 56KB. Screens are compressed as they are written, at the
fastest level; assets once, at startup, at the best level, and each encoding carries the digest of
its own bytes as its `ETag`. Every response says `Vary: Accept-Encoding`.

Compressing a page that echoes input (the filter box) beside a constant secret leaks the secret to
anyone who can steer requests and watch their sizes, which is the BREACH attack. So the CSRF value a
form carries is masked with a fresh one-time pad on every response, and the cookie's own value is
never on a page. The check accepts the masked value or the plain one.

## Something to look at

An empty install is a poor demonstration of screens whose whole job is showing work in progress.

```sh
tix demo seed --db /tmp/demo.db          # 90 days by default
tix demo seed --db /tmp/demo.db --days 30 --reset
tix serve --db /tmp/demo.db
```

It writes projects, people, agents, custom field definitions, and tasks carrying descriptions, tags,
priorities, assignees, due dates and comments, with completions spread across the window by several
actors and lead times that vary.

Two of those tasks are claims an agent took and never gave back, one of them picked up and dropped
four times, so the `claim expired` badge and the claim count beside it can be seen without arranging
that state by hand. They are produced by the ordinary claim and sweep calls like everything else
here, and placed against the end of the window rather than on a fixture day, because the evidence
only reads as recent for a day and the fixture days are backdated by months.

A fresh installation is given four starter lists before any command runs. Any of them still holding
no task is dropped, so the projects screen and the project filter name only the lists the backlog
works out of, and a first seed and a re-seed show the same thing rather than ten lists and six. Only
a list with no task at all is removed, so nothing that has been worked in can be lost to a seed.

The summary says what it wrote: projects, actors, custom fields, tasks, completions, comments,
dependencies and `ABANDONED CLAIMS`. `-o json` carries the same counts, the last of them as
`abandoned`.

The history is replayed through the ordinary service calls against a clock the command advances,
not written behind them. Statistics attribute a completion by matching the transition entry written
in the same instant, so data inserted underneath the service would leave every leaderboard and
lead-time figure empty.

It refuses a database that already holds tasks or users unless `--reset` is passed, exiting 4 like any
other conflict, so nobody discovers this command by finding a few dozen invented tasks in their real
tracker. Point `--db` at a scratch file regardless.

## See also

- [deployment.md](deployment.md) for running `tix serve`, TLS, proxies and the bind guard
- [filtering.md](filtering.md) for the expression the filter box takes
- [theming.md](theming.md) for the tenant accent, and what is deliberately not themed
- [statistics.md](statistics.md) for the figures behind `/stats`
- [tenancy.md](tenancy.md) for what a tenant owns and how isolation is enforced
