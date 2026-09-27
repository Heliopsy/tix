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

**Per-browser dates and times.** Format and timezone are chosen in the settings screen and stored in
a cookie, not on the tenant. Two people sharing a tenant are frequently in different zones.

**Lease badges.** A task list says which rows an agent is holding and which ones were claimed by
somebody who never came back, how long ago that claim lapsed and how many times the task has been
picked up. The second of those is read from evidence the sweeper leaves on the task rather than from
the lease columns it clears, which is what makes an abandoned claim visible at all rather than for
the minute before the next sweep. [docs/web-ui.md](web-ui.md) covers these and the rest of the
browser interface.

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
