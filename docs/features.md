# What else is in the binary

Capabilities that are easy to miss from the command table, because they are not a command each.
[commands.md](commands.md) is the index of commands themselves.

Capabilities that are easy to miss from the command table above.

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

**A status control that can move more than one step.** The status pill on a row offers the states
that row's own workflow can reach, including ones reachable only through another state. The whole
route is spelled out before it is applied, and each hop is an ordinary transition, so a task that
passed through a state really did pass through it and the audit trail says so.

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
