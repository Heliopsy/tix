# What else is in the binary

Capabilities that are easy to miss from the command table, because they are not a command each.
[commands.md](commands.md) is the index of commands themselves.

**Statistics.** `tix stats`, `/stats` in the browser and `S` in the terminal interface, over one
window and optionally one project: completions per day, median and slowest lead time, where the work
is sitting, who moved it, what has waited longest. The leaderboard prints what it counts every time
it is shown, because a count of tasks moved to a terminal state is not a measure of work done and a
number presented without that sentence gets read as one. [docs/statistics.md](statistics.md).

**Theming.** A tenant names an accent and both the browser and the terminal render it. Palettes are
configuration rather than rows, so an operator writes one for the whole deployment. Light, dark and a
low-contrast scheme are a separate axis, chosen per browser, because that is a property of the person
reading. State and priority colours are deliberately not themeable: blocked is red whatever a tenant
brands itself. [docs/theming.md](theming.md).

**A directory of actors.** `tix actor ls` on the command line, `/actors` in the browser, and the same
listing behind the assignee field, which suggests handles instead of asking for an identifier from
memory. It suggests without constraining: an actor from another tenant is still assignable by
identifier. Agents are in the directory alongside people, because work is assigned to them as often.

**Due dates in your calendar.** `tix calendar create`, or Settings in the browser, issues a private
iCalendar address that Google Calendar, Apple Calendar or Outlook subscribes to: every task you are
assigned that has a due date and is not finished becomes an all-day event, or whatever a filter
expression selects. The address reads as you and nothing more, and stops the moment it is rotated or
revoked or you leave the tenant. [docs/calendar.md](calendar.md).

**Project keys that can change.** A project's key starts every task reference, and it can be changed with
`tix project edit --key`, `PATCH /projects/{ref}`, the project's settings in the browser or the terminal's
project screen. Every reference changes with it, and the old key stays an alias, so old references, filters
and links keep resolving and the browser is redirected to the new address.
[docs/workflows.md](workflows.md#changing-a-projects-key).

**A status control that can move more than one step.** The status pill on a row offers the states
that row's own workflow can reach, including ones reachable only through another state. The whole
route is spelled out before it is applied, and each hop is an ordinary transition, so a task that
passed through a state really did pass through it and the audit trail says so.

**Quick-add syntax.** One line can carry a task's attributes: `Fix login !high #auth @raj due:fri +web
^infra-3` is a task titled "Fix login", priority high, tagged `auth`, assigned to `raj`, due next Friday,
in project `web`, under `infra-3`. The same parser reads the browser's one-line add box (always), the
terminal's new-task prompt (always), `tix task add --parse` and `POST /v1/tasks` with `"parse": true`.

| Token | Sets | Notes |
| --- | --- | --- |
| `!highest` `!high` `!normal` `!low` `!lowest`, `!1` to `!5`, `priority:X` | priority | 1 is highest |
| `#tag`, `tag:X` | a tag, repeatable | needs a letter, so `#42` stays text |
| `@handle`, `assignee:X` | assignee | |
| `due:X` | due date | `today`, `tomorrow`, a weekday (`fri`, `friday`), `+3d`, `+2w`, `2026-11-20`, RFC 3339 |
| `+key`, `project:key` | project | takes the place of the box's project picker |
| `^ref`, `parent:ref` | parent task | |
| `status:X` | initial status | any state of the project's workflow |

A weekday is the next one after today, never today itself, which is `due:today`. Dates count in the
reader's calendar: the local zone on the command line and in the terminal, the zone chosen under Settings
in the browser, and UTC over the API. A date with no time is midnight UTC of that day, as `--due` gives.

A token is a whole word and only counts when its value fits, so `C#`, `a@b.c`, `#42`, `+1`, `!important`
and the `#` inside a URL stay in the title. A keyed form with a bad value (`due:someday`) is refused
rather than kept. Put a backslash before a token (`\#news`) or quotes around text (`"#main"`, `„#main“`)
to keep it as written; quotes stay in the title. Naming one attribute twice with different values, or
contradicting a flag or field given alongside the line, is refused. An unknown project, handle or parent
is refused the way the explicit field is, and the browser and terminal keep the line for you to fix.

The command line parses only when asked, because a script passing a title it did not write, an issue
title or a commit subject, must get that title back. Single-quote the line so the shell leaves `!` and `#`
alone:

```sh
tix task add --parse 'Fix login !high #auth @raj due:fri'
```

**Descriptions and comments in Markdown, with checklists.** A task's description and its comments are
CommonMark with the GitHub tables, task lists, autolinks, strikethrough and fenced code, and a line break
you type stays a line break. The browser draws them; the command line and the terminal show the text as
written. A reference such as `infra-8` links to the task when the project is one you can see, and an
`@handle` of this tenant links to the tasks assigned to it. A line written `- [ ]` or `- [x]` is a
checklist item: the task screen draws it as a checkbox anyone with `task:write` can tick in place, rows and
cards carry a `3/5` count (cards add a thin progress bar), `tix task show` reports it, a task read carries
`"checklist": {"done": 3, "total": 5}`, and `tix task check REF N` / `tix task uncheck REF N`, the
terminal's `[` key and `POST /tasks/{ref}/checklist` tick and clear item N. A tick rewrites only that
item's marker and carries the version it was made against, so a tick on a description somebody changed
since is refused with the usual conflict rather than landing on the wrong line. `has:checklist`,
`checklist:open` and `checklist:done` filter by it. The editor stays a plain textarea, with a Preview
toggle that renders on the server.

What is never rendered, by design: raw HTML is shown as the text that was typed, a link survives only
when it is `http`, `https` or `mailto` and opens in the same tab with `rel="noopener noreferrer
nofollow"`, and an image is never loaded. It becomes a link to its address labelled by its alt text. An
image would make every reader's browser fetch from whatever origin the writer named, which tells that
origin who opened the task and when; a link fetches only when somebody chooses to follow it. The markup
carries no inline style or script, so it holds under a strict content security policy.
[docs/filtering.md](filtering.md#checklists), [docs/api.md](api.md#checklists).

**Tasks that repeat.** A task can carry a repeat rule, and finishing it puts the next one on the list, the
way Google Tasks does it: nothing is generated ahead of time, so there is only ever one open occurrence of
a series. Set it with `tix task add --repeat`, change it with `tix task edit --repeat`, remove it with
`--no-repeat`, or use the Repeat field on the browser's add and edit forms. Recurring rows, board cards and
terminal cards carry a `↻` marker, and `is:recurring` lists them.

| Rule | Next due date |
| --- | --- |
| `every day`, `daily`, `every 3 days` | N days on |
| `every week`, `weekly`, `every 2 weeks on mon,thu` | N weeks on, or the next listed weekday of every Nth week |
| `every month`, `monthly`, `every month on day 31` | same day of the month, clamped to the month's end |
| `every year`, `yearly` | same date; 29 February becomes 28 February in a common year |
| `... after done`, as in `3 days after done` | counted from the day it was finished instead |
| `... until 2027-06-30` | no occurrence after that date |
| `... for 5 times` | five occurrences including this one, counting down on each |

Without `after done` the next date is counted from the previous due date, skipping any occurrence that has
already passed, so a weekly chore finished three weeks late comes back once, next week, rather than three
times overdue. A task with no due date counts from the day it was finished. The time of day of the due date
is kept. A plain `every month` from the 31st clamps to the 30th or the 28th and then stays there, because
each occurrence counts from the last; `every month on day 31` keeps going back to the month's last day.

The next occurrence keeps the title, description, priority, assignee, parent, tags and custom fields, starts
in the workflow's first state, and carries the rule. It does not copy comments, artifacts, dependencies or a
lease. It is created only by moving to a state whose category is `done`, never `cancelled`, so cancelling
ends the series. Each finished task records the one it spawned and never spawns again, however often it is
reopened and finished. Creating a task already done, importing a snapshot and an external sync never spawn,
and the rule travels in an export. The parser is strict: anything it cannot read exactly is refused with
what was wrong, rather than guessed at.

**Per-browser dates and times.** Format and timezone are chosen in the settings screen and stored in
a cookie, not on the tenant. Two people sharing a tenant are frequently in different zones.

**Lease badges.** A task list says which rows an agent is holding and which ones were claimed by
somebody who never came back, how long ago that claim lapsed and how many times the task has been
picked up. The second of those is read from evidence the sweeper leaves on the task rather than from
the lease columns it clears, which is what makes an abandoned claim visible at all rather than for
the minute before the next sweep. [docs/web-ui.md](web-ui.md) covers these and the rest of the
browser interface.

**Stars.** Star a task and it lists ahead of the rest for you, on every surface: the star button on a
row, a card or the task screen in the browser, `*` in the terminal interface, `tix task star` and
`tix task unstar` on the command line, `PUT` and `DELETE /api/v1/tasks/{ref}/star` over HTTP. A star is
yours alone, stored with your actor rather than in a cookie, so it follows you between the browser, the
terminal and the command line and never reorders a colleague's queue. Your starred tasks lead under
every sort, and `is:starred` narrows a listing to them. [docs/filtering.md](filtering.md#stars).

**Lanes.** Besides the list and the board, the task screen can draw one lane per project, each lane
holding that project's open work as cards. A card ticks, changes status and changes priority where it
sits, without leaving the screen. [docs/web-ui.md](web-ui.md#lanes).

**A task over the list, and titles renamed where they are listed.** Two per-browser settings: a task
followed from a listing can open in a dialog over it instead of its own page, and a click on a listed
title can rename it in place. [docs/web-ui.md](web-ui.md#opening-a-task-in-a-dialog).

**Undo.** A status change, a priority change and a soft delete in the browser come back with a toast
offering Undo. Elsewhere the same undo is an ordinary command: `tix task mv` back, `tix task edit
--priority`, and `tix task restore` for a soft delete. [docs/web-ui.md](web-ui.md#acting-in-place).

**Two languages.** The browser draws every screen in English or Bulgarian, following `Accept-Language`
unless Settings picks one for that browser. [docs/web-ui.md](web-ui.md#language-and-info-tips).

**An activity feed.** Every change the audit log records, as a live feed at `/activity` that narrows by
record, kind, surface and actor, and as `tix audit ls --filter` and the terminal's history view.
[docs/filtering.md](filtering.md).

**A second factor.** A time-based code on top of a password or an SSH key, required per tenant and per
surface, with recovery codes and an administrator's clear for a lost phone. API tokens are exempt.
[docs/second-factor.md](second-factor.md).

**Custom field defaults.** A field definition can carry a `default`, which a task created without a value
for that field is given. It is part of the definition the API's `PUT /projects/{ref}/fields` takes and
travels in a bundle. [docs/workflows.md](workflows.md#custom-fields).

**Columns and lanes you arrange.** Every column of a board and every lane of the task screen carries a
three dots menu: hide it, star it so it comes first, collapse it to a strip, or move it. The arrangement
is yours alone and kept on the server, so it follows you to another browser, to the terminal board
(through its palette) and to `tix view ls|set|order|reset` and `/api/v1/views` over HTTP.
[docs/web-ui.md](web-ui.md#arranging-columns-and-lanes), [docs/api.md](api.md#arranging-views).

**A command palette.** ctrl+k (command+k on a Mac) in the browser opens one search box over tasks,
projects, every screen and settings section, and actions such as New task in a project, switching the
view or the colour scheme, and signing out. Task search runs the filter language on the server once
something is typed; everything else draws instantly. [docs/web-ui.md](web-ui.md#the-command-palette).

**Saved filters, My work and recently viewed.** A filter keeps its expression, sort and view under a
name, private or shared with the tenant: saved from the browser's filter bar and listed in its sidebar,
picked in the terminal, and kept with `tix filter` and `/api/v1/filters`. `me` in an actor term is the
reader, so `assignee:me` and `tix task ls --mine` are your own work; My work in the sidebar draws it under a
heading per deadline. The sidebar also lists the last ten tasks the browser opened.
[docs/web-ui.md](web-ui.md#my-work-saved-filters-and-recently-viewed), [docs/api.md](api.md#saved-filters).

**Copying a task.** A task copies into any project you can create tasks in, its own included: "Copy to…"
on the task screen in the browser, `Y` in the terminal interface, `tix task copy REF --to PROJECT` on the
command line and `POST /api/v1/tasks/{ref}/copy` over HTTP. The copy gets a fresh reference and keeps the
title (editable), description, priority, tags, assignee, due date, repeat rule and the custom fields the
target defines. It starts in the target workflow's first state, or keeps its state when asked and the target
has one of the same name. Comments, subtasks (recursively) and dependencies come along only when asked, and a
linked copy shows "Copied from" with the original. Stars, claims, history and artifacts stay behind.
[docs/api.md](api.md#copying-a-task).

**Bulk actions.** Tick rows on the web task list and a bar applies one action to all of them: move,
priority, add or remove a tag, assignee, star, unstar, copy to a project, or delete with one Undo for the
batch. It is one request, `POST /api/v1/tasks/bulk` or `tix task bulk ACTION REF...`, and each task is still
changed on its own with its own history entry and event, so the answer names any task that was refused and
why. `tix task edit`, `tix task copy`, `tix tag add` and `tix tag rm` take many references too.
[docs/web-ui.md](web-ui.md#acting-on-many-tasks), [docs/api.md](api.md#bulk-actions).

**CSV export.** Export CSV in the task list's View menu, `tix task ls -o csv` and
`GET /api/v1/tasks?format=csv` write a listing as CSV, with any cell a spreadsheet would read as a formula
prefixed with a quote. [docs/web-ui.md](web-ui.md#exporting-the-list), [docs/api.md](api.md#csv).

**Shell completion.** `tix completion install` detects the shell, writes the script where that shell
looks for it and never edits a startup file. It completes task references, project keys, tags and the
states your workflows actually define, not only flag names.
[docs/shell-completion.md](shell-completion.md).

**Self-update.** `tix update` replaces this binary with a release built for its platform, checked
against the checksum published beside it, and refuses a binary the Go toolchain or a package manager
owns. `--check` reports without writing. [docs/upgrading.md](upgrading.md).

**Demonstration data.** `tix demo seed --db /tmp/demo.db` writes a backlog with history: projects,
people, agents, custom fields, tasks with bodies, tags, due dates and comments, and completions
spread across the window by several actors. It is replayed through the ordinary service calls on a
clock the command advances, so the statistics have real audit entries to attribute against, including
two claims an agent took and never gave back so that state can be seen without arranging it by hand.
It refuses a database that already holds work unless `--reset` is passed.
