# The terminal interface

`tix tui` opens a board in the terminal, over the same service layer the command line and the browser
call. A test enforces it: `internal/capability` lists `tui` among the surfaces every operation must
reach, and the parity test fails the build when a new operation names no terminal view or
records no reason for not having one.

The interface documents itself while it runs. `ctrl+k` opens a searchable list of every action, each named
in words and each showing its own key, and the footer names that key in every view so you do not have to
know it first. `?` opens an overlay listing the bindings of the view you came from, the bindings that work
everywhere, the card markers and both filter grammars. Both are built from the same `KeyMap` the program is
actually dispatching, so neither can describe keys you do not have. This page covers what they cannot:
what is here before you connect. Over SSH that is the whole problem, because you connect
first and find out afterwards.

## The eighteen views

The title bar names the open one, between the project and the connection state, because the start menu, projects, the
board, the task, the project screen, activity, history, the inbox, the tenant screen, people, integrations, the server,
global admin, your account, statistics, transfer and settings all render into the same frame and recognising the body
was the only way to tell where you were.

<!-- Rows asserted against the view list by TestDocsViewsTableListsEveryView in internal/tui. -->
| View | Opened by | Shows |
| --- | --- | --- |
| start | `~`, and a hosted SSH session that named nothing starts here | one row per interface a session can host: Boards and Watch, Watch with a project picker |
| projects | `W`, and every local session starts here | every project this tenant has, with its colour and icon (a built-in or uploaded icon as a text stand-in) |
| board | `enter` on a project | one project's tasks in columns, one column per workflow state |
| project | `w` | one project's attributes, the workflow it runs on, and its custom field definitions |
| detail | `enter` on a card | one task in full: body, custom fields, subtasks, dependencies, artifacts, comments |
| activity | `v` | the live event tail, filtered by the audit grammar |
| history | `H` | the stored audit log for the selected task, the open project or the tenant |
| inbox | `I` | your mentions, assignments, comments on tasks you follow and status changes, newest first |
| statistics | `S` | the same figures as `tix stats`, for the open project or the whole tenant |
| tenant | `T` | the tenant in force, its domains and its members, and a key to switch this session |
| people | `@` | the tenant's users with role, state and second factor, and the pending invites |
| integrations | `J` | webhooks and their deliveries, external sync sources, and retention, one tab each, as far as your scopes reach |
| server | `%` | the installation, the servers registered against it, the tenant's work and second factor, and its live connections |
| globaladmin | `!`, offered to a global admin only | every tenant and the global admins; a tenant's users; any account's log |
| account | `$` | who you are signed in as, your second factor, your API tokens, SSH keys and sessions, and your account log |
| settings | `,` | display preferences, and what this session is connected to |
| transfer | `V` | exports and imports of the open project's tasks, a bundle or a snapshot, by file path; over SSH, the `ssh` commands that do them instead |
| help | `?` | the bindings of the view underneath, and the legends |

Navigation is a stack. `esc` and `backspace` step back one level, and so does `q`: quitting from a
nested view would lose a place a reflex press only meant to step out of, so `q` ends the program only at
the top. `ctrl+c` always ends it, exiting 130. Entering the view already open is a refresh rather than a
step, so reloading never makes the way back one press longer.

`W` is different from `esc`. It discards the whole stack and returns to the project list. It is a capital
because `p` cycles the selected task's priority and `P` picks one outright, so neither case of the letter
the project list is named after was free.

**Cards.** A card leads with the title, because the title is what a person scans a board for, and it gets
the column's whole width. A title too long for one line wraps onto a second, indented to the same column,
and only the last line shown is cut. Under it, dim, sit the identifiers: the reference, the priority, then
whichever markers are true of the task, separated so that a priority and a marker never read as one token.

```text
┃ Default-deny network      │ Bring the platform
┃ policies in staging       │ onto one observ…
┃ INFRA-13 · P2 · due! · @me│ INFRA-8 · P2
```

The bar down the left edge carries the workflow category as a colour and the selection as a shape: the
selected card's bar is heavier. A terminal getting no colour has nothing else left to say which card the
keys will act on.

The priority is `P1` through `P5`, highest to lowest, the same digits `tix task list` prints. The rest of
the line is markers. The legend is in `?`, wrapped to the terminal so a narrow one shows all of it rather
than the first half of it:

<!-- Rows asserted against CardLegend by TestDocsMarkerTableMatchesTheLegend in internal/tui. -->
| Marker | Means |
| --- | --- |
| `*` | starred by you, which leads the meta line |
| `@` | claimed, by somebody |
| `@me` | claimed by you |
| `!` | blocked |
| `+` | has dependencies |
| `↻` | repeats when done |
| `†` | deleted, revealed by `is:deleted` or `is:trashed` |
| `zz` | snoozed, revealed by `is:snoozed` or `snoozed:any` |
| `due` | falls due within a week |
| `due!` | overdue |

`↻` marks a task that comes back when it is finished; the detail view states the rule as
`repeats: every week`. The terminal interface does not edit due dates and does not edit the rule either:
set it with `tix task edit --repeat` or in the browser.

`*` marks a task you starred with the `*` key. A star is yours alone: somebody else sharing the board sees
the same task unstarred and in its usual place, while on your board a starred card leads its column ahead of
the priority order. It is stored on the server, so the star you set here is the one `tix task ls` and the
browser show you too, and `is:starred` in the filter narrows the board to them.

`zz` marks a snoozed task. Snoozed tasks are off the board until they wake, so the marker shows only
under `is:snoozed` or `snoozed:any`. `b` on a card asks when it should wake (`later`, `tomorrow`,
`next-week`, a duration such as `3d`, a date or `YYYY-MM-DD HH:MM`, read in the local zone) and takes it
off the board; `b` on a snoozed card unsnoozes it. The detail view says until when and who snoozed it. A
snooze is for everybody, unlike a star, and a snoozed task is not handed to agents claiming work.

`@me` is separate from `@` because "is that me" is the first question anybody asks of a claimed card on a
shared board, and a single marker left it answerable only by opening the task.

The two due markers are drawn only while a deadline is pressing. A card carrying a date further out, and a
card carrying no date at all, both draw nothing: an earlier marker was drawn for any due date whatsoever,
which put it on nearly every card in a real backlog, so it distinguished nothing and was removed. `due!`
is drawn in the colour the theme gives an overdue deadline and `due` in the colour it gives a near one, but
neither is only a colour: the marker is a word, so a terminal getting no escapes keeps the meaning. The
detail view names the state in full beside the date, as `due 2026-02-01 (overdue)`.

`tix task ls --overdue`, and `due:overdue` in any of the three filter bars, list exactly the cards carrying
`due!`.

Columns follow the workflow's declared order and are coloured by the category a state belongs to, not by
its name, since workflow states are user defined. Nothing is conveyed by colour alone.

There are six categories: `todo`, `in_progress`, `blocked`, `waiting`, `done` and `cancelled`. Each draws
in its own hue, and the two that are easy to mistake for something else are the point of having six.
`blocked` draws in the colour that already means urgent rather than in the yellow of ordinary waiting
work, and `cancelled` draws muted rather than in the green of `done`, so an abandoned task does not read
as a finished one. A column whose state names no category, or a category this build does not know, draws
plainly. See [workflows](workflows.md#categories).

A column is as wide as it needs to be and as tall as what it holds. The width is shared out by what each
column has to show, so the column holding eight tasks with long titles is wider than the one whose entire
content is the word `empty`, and a column with nothing in it is a short box rather than a screenful of
blank frame. Every column keeps a floor wide enough for its own heading.

The footer advertises only the keys the selected task can accept: `c` while the task
is free, `x` and `R` while this session holds the lease, neither while another worker does, and `t` only
where the workflow permits a move out of the current state. A deleted card is offered the restore and
none of the editing keys, because the service answers a deleted task as not found for an edit, a claim, a
comment or a transition, and bringing it back is the one thing left to do with it. A task held elsewhere
says who holds it in the status bar instead. `?` still documents every binding the view has, so hiding a
key from the footer never hides it from the help.

The footer always leads with `ctrl+k commands`, before the view's own keys. A terminal too narrow for the
whole line is truncated from the right, so what it loses is the view's bindings rather than the one key that
reaches all of them. Below the width that hint's own text needs it is dropped whole, because `ctrl+k comm…`
names a key nobody can press.

## Keys

Five schemes ship, for people arriving with muscle memory from somewhere else. The scheme is
`tui.keymap` in configuration, `tix tui --keys NAME` for one run, or the first row of the settings
screen.

| Scheme | What it changes |
| --- | --- |
| `default` | the shipped bindings below |
| `vim` | `o` new, `i` edits the task, `a` comment, `:` also filters, `e` refreshes, `ctrl+k` the palette |
| `emacs` | `ctrl+p`/`ctrl+n`/`ctrl+b`/`ctrl+f` move, `ctrl+a`/`ctrl+e` ends, `ctrl+g` backs out, `ctrl+s` filters, `ctrl+t` edits, `ctrl+l` redraws |
| `nano` | `ctrl+w` searches, `ctrl+g` helps, `ctrl+x` quits, `ctrl+o` applies, `ctrl+l` redraws, `ctrl+k` deletes, `:` the palette |
| `helix` | `x` opens the row, `d` releases, `o` new, `i` edit, `a` comment, space opens settings, `ctrl+k` the palette |

There is deliberately no `mac` scheme. A terminal never sees the command key, and the chords macOS
applies to every text field are Cocoa's emacs bindings, so a mac scheme would be `emacs` under a second
name. The emacs description says so instead.

`nano` puts `ctrl+k` on the delete key. It cuts a line in nano, and for as long as the interface could
remove nothing it had nothing to mean here, so it was left unbound rather than given an invented meaning;
now that `X` removes a task or a comment, `ctrl+k` carries nano's own meaning. `ctrl+c` is left alone,
because interrupt already owns it.

A scheme lists only the actions it moves. Everything else keeps its default keys, so a scheme cannot leave
an action unbound.

### The default bindings

<!-- Keys asserted against DefaultKeyMap by TestDocsBindingsTableNamesEveryAction in internal/tui. -->
| Keys | Action | Where |
| --- | --- | --- |
| `↑`/`k`, `↓`/`j` | move the selection | everywhere |
| `←`/`h`, `→`/`l` | previous, next column | board; steps a value on settings, switches the tab on integrations, moves the cursor inside a text field |
| `tab`, `shift+tab` | next, previous field | wherever a form is open |
| `g`/`home`, `G`/`end` | first, last | lists |
| `enter` | open, or apply an input; inserts a newline inside a multi-line field; pings an endpoint, redelivers, runs a source or prunes on integrations | everywhere |
| `ctrl+s` | apply an input, including from a multi-line field | wherever one is open |
| `esc`/`backspace` | back one level | everywhere |
| `/` | filter | board, activity; cycles the window on statistics; asks for an email whose account log to read on global admin |
| `C` | clear the filter, and on activity the project the start menu picked | board, activity |
| `space` | pause the tail, holding new events back; again to resume and draw them | activity |
| `s` | apply one of your saved filters, or one shared with the tenant | board |
| `K` | keep the board's filter as a saved filter, under a name | board |
| `c` | claim | board, detail |
| `x` | release | board, detail |
| `R` | renew the lease; re-issues the selected invite on people; rotates the selected token on account | board, detail, people, account |
| `F` | reclaim the lease another worker holds, leaving the task where it is; clears the selected user's second factor on people | board, detail, people |
| `N` | claim next from the queue | board, detail |
| `t` | transition | board, detail |
| `n` | new task, new project on the project list, new field or domain or member, new user or invite, new endpoint or sync source; a tenant, global admin, user or account on global admin; a token or key or second factor on account | board, detail, projects, project, tenant, people, integrations, global admin, account |
| `e` | edit the whole task on one form; edits the project, the tenant, a user, an endpoint, a source, the retention windows or a tenant's user on their own screens | board, detail, project, tenant, people, integrations, global admin |
| `P` | set priority, from the five | board, detail |
| `p` | cycle priority one place down, wrapping from `P5` back to `P1` | board, detail |
| `A` | assign, from the tenant's directory | board, detail |
| `m` | comment | board, detail |
| `M` | edit the selected comment | detail |
| `#`, `U` | add tag, remove tag, by name | board, detail |
| `L` | pick a tag from the ones the tenant has | board, detail |
| `D` | add a dependency, by reference | board, detail |
| `-` | remove one of the dependencies the task waits on | detail |
| `O` | record an artifact | board, detail |
| `u` | restore a deleted task | board, detail |
| `*` | star the task, or unstar a starred one; starred cards lead their column | board, detail |
| `[` | tick a checklist item of the description by its number, or clear a ticked one | board, detail |
| `b` | snooze the task until a time you type, or unsnooze a snoozed one | board, detail |
| `Y` | copy the task to any project, this one included, with a form for the title and what travels | board, detail |
| `^` | share the task: copy its link, a Markdown link or its key and title to the terminal's clipboard | board, detail |
| `B` | pick a task template: create a task from it, show it, rename it or delete it | board, detail |
| `E` | save the task as a template, for this project or the whole tenant | board, detail |
| `X` | delete the task or the selected comment; removes the selected domain or member; deletes the selected user or revokes the selected invite; removes the selected endpoint or source; ends a connection; deletes a tenant or user, revokes a global admin; revokes the selected token, key or session | board, detail, project, tenant, people, integrations, server, global admin, account |
| `Z` | hide the selected column, for you, on this project's board everywhere | board |
| `+` | star the selected column, or unstar it; starred columns lead | board |
| `z` | collapse the selected column to its name and count, or expand it | board |
| `<`, `>` | move the selected column left, right | board |
| `=` | show every hidden column again | board |
| `pgdown`, `pgup` | next, previous page of the project's tasks, when it holds more than one read | board |
| `space` | mark the selected item read, or unread; tick a scope in the token scope picker | inbox, account |
| `a` | mark every item read; tick or clear a whole group in the token scope picker | inbox, account |
| `V` | import and export: tasks, bundles and snapshots, by file path, or the `ssh` commands over SSH | everywhere |
| `y` | agree to a confirmation; writes a previewed import | wherever one is open, transfer |
| `w` | project setup | everywhere |
| `f` | custom field definitions | project |
| `~` | the start menu | everywhere |
| `W`, `,`, `v`, `H`, `I`, `S`, `T`, `@` | projects, settings, activity, history, inbox, statistics, tenant, people | everywhere |
| `J` | integrations: webhooks, external sync and retention | everywhere |
| `$` | your account: tokens, SSH keys, sessions, second factor and account log | everywhere |
| `%`, `!` | the server, global admin | everywhere they are offered |
| `ctrl+k`, `:` | the command palette: every action by name | everywhere |
| `r` | refresh | everywhere |
| `?` | help | everywhere |
| `q` | back, or quit at the top level | everywhere |
| `ctrl+c` | interrupt, exit 130 | everywhere |

`S` is capital because lowercase `s` is spoken for on the board, and a capital is what the other
cross-view keys already use when their letter is taken. `V` opens transfer for the same reason, with less
excuse: every letter a transfer could be named by was taken.

### Moving data in and out

`V` opens the transfer view on the project the board is open on, or the row under the cursor on the project
list. It lists the six transfers the reader's authority reaches: export and import of the project's tasks,
of the project as a bundle, and of a snapshot of the whole tenant. `enter` opens a form that asks for a file
path and the choices the command line offers: the format of a task export (CSV by default, JSON, NDJSON or a
Markdown table), the collision policy of a bundle import, and whether a snapshot import merges or replaces.
A relative path is read from the directory `tix tui` started in, and `~` is your home. Exporting onto a file
that exists asks before overwriting it, and the file holds exactly the bytes `tix export tasks`,
`tix bundle export` or `tix export` would have written.

No import writes on the first press. A task import runs as a dry run first and the view shows what
`tix import tasks --dry-run` prints: how many rows would be created, updated, skipped or refused, every
column of the file with the field it fills and whether that is tix's guess, the first rows as they would be
written, and every row in error. The form takes a mapping file by path when the guess is wrong; editing the
mapping column by column is the browser's import screen and `--save-mapping`. While any row is in error
nothing can be written. Otherwise `y` asks once more and then writes the same bytes with the mapping that
was shown. A bundle import shows its preview and a snapshot import its dry run the same way before `y`
writes them. `esc` discards a preview.

Over SSH the server cannot read or write a file on your machine, so the hosted view asks for no path. It
names, for the same project, the command that does each transfer through standard input and output from
your own shell, such as `ssh acme@tix.example export tasks -p infra > infra.csv` and
`ssh acme@tix.example import tasks -p infra --dry-run < tasks.csv`. The registry records this as a
limitation of the six terminal bindings rather than as a gap.

### The command palette

Seventy bindings is more than anyone holds in their head, and the letters ran out: `W` lists projects
while `w` opens project setup, `H` reads history while `v` draws the live tail, `L` picks a tag, `O` records
an artifact, `N` claims the next task, `Y` copies one. The capitals were handed out by elimination rather than by meaning.

`ctrl+k` opens every action by name, and the footer says so in every view. The keys were always there;
nothing advertised them.

```text
tix │ as local │ @default │ projects │ ● live
  6 projects

▸ 🏗  infra   Infrastructure
  🖥  web     Web UI
  🤖 agents  Agent Fleet
  📚 docs    Documentation
  🛠  ops     Operations
  📊 data    Data Platform
6 projects │ ✓ connected
ctrl+k commands  ↑/k up  ↓/j down  enter open  n new project  ? help  q quit
```

Pressing it opens the same panel every other input mode uses, with the actions in it:

```text
6 projects │ ✓ connected
────────────────────────────────────────────────────────────────────────────────
commands:
  type to narrow; every entry shows the key that does the same thing
  match
  ?             help
  r             refresh
  W             projects · here
  w             project setup
  ,             settings
  v             activity
  H             history
  ↓ 5 more
  enter run   ↑/↓ move   esc cancel
```

Every entry shows its own key, so the palette teaches its own way out: a reader who reaches
for `ctrl+k` and then `statistics` has read `S` beside it, and eventually stops needing the trip. The open
view is marked `· here`, in a word rather than a colour, so a list whose whole purpose is going somewhere
else still says where you are.

Typing narrows it. Every whitespace-separated word of the query has to appear in the action's name,
ignoring case, and the words may come in any order, so `task new` finds `new task`:

```text
commands:
  type to narrow; every entry shows the key that does the same thing
  match         stat
  S             statistics
  enter run   ↑/↓ move   esc cancel
```

`enter` runs the highlighted entry, `esc` leaves with nothing changed, and `↑`/`↓` move. The arrows only:
`k` and `j` also move the selection everywhere else, and inside this field they are the letters k and j.
A query that matches nothing says so rather than drawing an empty box:

```text
  match         zzz
  no action matches "zzz"
```

Each entry is the action itself. Selecting one makes the same call the key press
makes, through one switch that both reach, so a palette entry cannot do something slightly different from
the key it is teaching. On the board, with a card selected, that includes everything that acts on the task:

```text
14 tasks in 5 columns, showing 1-4 │ ✓ connected
────────────────────────────────────────────────────────────────────────────────
commands:
  type to narrow; every entry shows the key that does the same thing
  match         artifact
  O             record artifact
  enter run   ↑/↓ move   esc cancel
```

`enter` there opens exactly what `O` opens:

```text
artifact name:
  what this output is called
  name
  enter apply   esc cancel
```

Nothing is listed that would be refused. The entries are filtered by the same authority the `?` overlay
is filtered by, so a reader who may not subscribe to events is not offered `v`, and a reader who may not
claim is not offered `c`. An action that acts on the selected task is left out while nothing is selected,
and a new task is left out while no project is open, rather than listed in a form that cannot be pressed.

The order is fixed: the cross-view keys in the order `?` prints them, then the actions on the selection.
Narrowing never reorders what is left, because a row that moved as the query grew would no longer be
under `enter` where the reader left it.

Two things are deliberately not filtered. Lease state is one: the palette offers `x` on a task this session
does not hold, because the `?` overlay documents every binding a view has regardless of the lease, and the
palette's list and the overlay's list are held equal by a test. The palette itself is the other: `?`
documents `ctrl+k`, and the palette does not list itself.

`ctrl+k` and `:` both open it. Two schemes have already spent one of the two, so `vim` and `helix` keep `:`
for the filter, which is what their own `:` opens onto, and `nano` keeps `ctrl+k` for the cut and takes `:`.
`g` was not available: it is bound to "first", and `gg` is muscle memory for exactly the readers who would
reach for a list like this.

Every action that acts on a task is offered wherever a task is selected, so the board and the detail view
run the same set. Two of them need a sub-item only the detail view has a cursor
over, the comment thread and the dependency list, so `M` and `-` pressed on the board say to open the task
rather than acting on a guess.

Whatever a key opens, it opens in the same panel: a rule the width of the terminal, the name of what is
being asked, one labelled row per answer, and the keys that end it. A single-value prompt is a form with
one field rather than a different control with its own shape, so a reader who has used the delete form
has used all of them. While a panel is open the board behind it is dimmed and the view's own bindings are
not advertised at all, because a legend offering `n new task` under a text field is describing a
keystroke that will type the letter n.

What a key gathers depends on what the operation can accept, and there are three things to gather. A
single-line input gathers a value: a title, a tag name, a reference, a hostname. It is seeded with the
value it would replace where there is one, and an empty input cancels, so a stray keystroke never sends a
blank edit. Transition and priority open a numbered picker. Transition offers the states the
task's own workflow reaches: the ones it permits directly first, so a single move still costs one digit,
then the ones reachable only through another state, each named with the path it takes and how many moves
that is. Picking a route applies it one ordinary transition per hop, so the task's history records every
state it passed through. The assignee and an artifact's kind open a form whose every
field picks from the values the operation accepts, because neither is free text: a reader knows a
colleague by handle and has never seen the identifier the service stores, and an artifact kind is one of a
fixed set. `O` gathers the artifact's name at the prompt first and classifies it in the form afterwards,
which is the one place the two controls run in sequence.

### Prose fields

A task body, a comment and a project description are prose, and prose is not a value. They open a field
several lines tall inside the same panel, seeded with what is there, newlines and all. It grows with what
you type, between three lines and a third of the terminal, so a one-line note does not reserve half the
screen and a long one is not read through a two-line slot.

`enter` inserts a newline there, so it cannot also apply. `ctrl+s`
applies, and the panel's own legend says so on screen the whole time the field is open. Nothing here shells
out to `$EDITOR`: `tix ssh` serves this interface over SSH, so an editor spawned from a keystroke would run
on the server rather than on the reader's machine, which is useless to them and hands a remote reader a
process on your box.

### Editing a whole task

`e` opens one form over the selected task: title, body, priority and, when the tenant has a directory to
read, assignee. Every field shows what it holds before anything is sent, and `esc` leaves having written
nothing. Only the fields you changed are sent, so a trip that touched the body records an edit to the body
and not to the title.

`tab` and `shift+tab` move between the fields. The arrows cannot: inside the title and the body they move
the cursor through the text, which is what the field needs them for. That is why they no longer step a
column with `tab` on the board, where `←`/`h` and `→`/`l` still do. The panel's legend names the keys for
whichever field the cursor is on, because a reader who cannot leave the body field is stuck in a way that
reads as a hang.

There is no second key for the title alone. The form already holds it, and a title-only prompt was a
slower way to do part of what `e` does. The single-key actions that change something the form does not
gather are untouched: `P` for priority, `A` for the assignee, `#` and `U` for tags, `D` for a dependency.
Custom fields are not on the form either: they are per project and typed, and `tix task edit --field`
takes them with the validation this screen cannot.

### Copying a task

`Y` copies the selected task, to the project on screen or any other this tenant has. The form opens on the
board's own project and the task's title, so `Y` and `enter` puts a copy beside the original; change the
project or the title first to send it elsewhere. The switches add what a plain copy leaves behind: the
status (kept only when the target workflow has a state of that name), the comments, the subtasks and the
dependencies. Link to the original starts on, and records on the copy which task it came from.

Description, priority, tags, the custom fields the target defines, the repeat rule, the due date and the
assignee always travel. Stars, claims, history and artifacts never do. The status bar names the new task,
since it may have landed in a project the board is not showing.

### Sharing a task

`^` shares the selected card, or the task on screen, the way the web's Share menu does. A picker offers
`1` the link, `2` the Markdown link, `[INFRA-12 Fix the build](https://tix.example.com/tasks/infra-12)`,
with `\`, `[` and `]` escaped, and `3` the key and title, `INFRA-12 Fix the build`. The pick goes to the
clipboard of the terminal you are sitting at, through the OSC 52 escape sequence, so it works the same in a
local terminal and over SSH, where the clipboard is still your own.

Not every terminal honours OSC 52 (some ask first, some need it turned on, tmux needs `set-clipboard on`),
and nothing reports whether it arrived. So the copied text is also drawn in full under the board, wrapped
rather than cut, and stays until the next key: select it with the mouse if the clipboard stayed empty.

The link is the task's `web_url`, which a server configured with `server.public_url` sets on every task
([configuration.md](configuration.md#the-public-web-address)), else the server a remote context talks to.
A local database or an SSH listener without the key has no web address, so the picker offers the key and
title and the Markdown label without a link, and says it has no link and why.

### Task templates

`E` keeps the selected task as a template. The form asks for a name (the task's title to start with),
whether the template is for this project or the whole tenant, and whether the subtasks go with it. The
title, description, priority, tags, custom fields and repeat rule are kept; status, assignee and due date
are not, because they describe one piece of work rather than its shape.

`B` lists the templates the open project can use: its own and the tenant-wide ones. Pick one and an
action. Create a task makes a task, with the template's subtasks, in the board's project; leave the title
empty to keep the template's, and give due in as `3d` or `12h` to set a due date that far from now. A
title may carry `{N}` (the use number), `{DATE}` (today) and `{PROJECT}` (the project key), in any case,
and the service fills them in. Show states the template on the status bar, rename gives it a new name,
and delete asks first. The status bar names the new task.

### Cycling a priority

`p` moves the selected task one place down the scale and sends it at once, with nothing to pick: `P1` to
`P2`, and `P5` back round to `P1`. It wraps rather than stopping at the bottom, because a cycle that
stopped would leave a reader who overshot unable to climb back with the key they overshot on.

`P` still opens the picker, which is the shorter trip when the priority is already decided: one press and
a digit, rather than up to four presses of `p`. There is no key that cycles the other way, because the
shifted `p` is the picker, and wrapping means four presses reach anywhere the reverse would.

`X` opens a form for what the delete removes and how far it reaches, and then a confirmation naming the
subject. `y` answers that, not `enter`: a question answered by the key every other input is accepted with
is answered by reflex, which is the habit a destructive confirmation exists to interrupt. The same
confirmation stands in front of archiving or deleting a project and removing a custom field.

Deleting a project asks one more thing on its form, shown only once `delete` is chosen: what happens to its
tasks, `delete them` or `move to KEY` for each other live project. The confirmation then states the plan
the service returns, either `with its 3 tasks, permanently` or `moving its 3 tasks to ops: todo to todo (2),
review to doing (1)`, so the status mapping is read before anything is written.

### Arranging the board's columns

The board is one project's board, and its columns can be arranged the way the web board's can: `Z` hides
the selected column, `+` stars it so it leads, `z` collapses it to its name and count, and `<` and `>`
move it past its neighbour. `=` brings every hidden column back. The arrangement is yours alone and is kept
on the server under the view `project:KEY`, so a column hidden here is hidden on the same project's board
in the browser, and the other way round. The command line reads and changes it with `tix view`.

A starred column only moves among starred ones and an unstarred one among the rest, so `<` and `>` are not
offered at the line between them, nor at either end. A collapsed column is still a column: it can be
selected, which is how it is expanded again, and a task moved into its state lands in it. A collapsed
heading carries `▸` and a starred one `★`, so neither depends on colour.

A reader without the authority to read the arrangement gets the board in workflow order, and the keys are
neither drawn nor sent.

### Paging a long project

One read of the board is 500 tasks, the most one page of a listing holds. The listing counts every task the
filter selects, so a project holding more says where you are in the status bar:
`500 tasks in 5 columns, page 1/3`. A count that stopped at its cap of 10,000 is written as a floor,
`page 1/20+`. `pgdown` reads the next 500 and `pgup` the previous, by page number, the way `page=N` does in
the browser; both are offered only where there is a page to go to, and in the palette. A new filter or
another project starts on page 1, and a live refresh rereads the page you are on. A project that fits one
read draws no indicator and offers neither key. There is no typed jump: at 500 a page, a project that needs
one is rare.

### Rebinding one action

`--key ACTION=KEYS` for a run, or `tui.keys` in configuration for good. The action names are the fields
of the interface's own key map, so `--key New=o` and `--key Filter=ctrl+s,/` are the shape.

A per-invocation flag wins for the action it names and leaves the rest of a configured set alone. A
rebinding keeps the action's description, so the footer goes on saying what the key does.

A rebinding that would make one key mean two things in the same view is refused, and the refusal names
the collision. Applying it would leave a key that quietly stops doing what its owner expects.
The same check runs per view, because the same key may mean two things in two views and often should.

An unusable scheme name or an override naming an action that does not exist is reported.
`tix tui --keys nonsense` refuses before a terminal is opened at all, so a typo in `tui.keymap` reports
the typo and not a TTY error.

## The settings screen

`,` opens five preferences and a statement of what this session is connected to. `↑`/`↓` move between
them and then scroll the body, because on a short terminal the session facts below the settings were
otherwise unreachable: four settings meant four presses and nothing moved after the fourth. `←`/`→`, or
`enter`, step the selected value, wrapping at both ends.

| Row | Configuration key | Values |
| --- | --- | --- |
| keys | `tui.keymap` | `default`, `vim`, `emacs`, `nano`, `helix` |
| time format | `output.time_format` | `iso`, `rfc3339`, `short`, `long`, `dotted`, `dayfirst`, `us`, `relative` |
| timezone | `output.timezone` | `local`, `UTC`, and a selection of named zones |
| colour | `output.color` | `auto`, `always`, `never` |
| motion | `tui.motion` | `on`, `off` |

A time format or timezone the machine leaves unset shows the account's value instead, with a note
naming where it came from (your own choice, the tenant default or the server default) and that a
value picked here applies to this machine only. The interface draws every date in the account's
style otherwise, over SSH as well, where the signed-in person's choice applies; see
[web-ui.md](web-ui.md#dates-and-times) and `tix user time`.

These are the command line's own keys, not a second store. Somebody at a terminal already has a
configuration file, and a separate place to write a time zone down would be a second place to disagree
with `tix task ls`. A change applies to the frame it is read in and is written to the file at once: a
preference that only takes effect after a restart is worse than none.

Each row is shown with what choosing it would mean, rendered by the thing that will render it. The time
format and zone rows show the same instant formatted, so an example cannot claim a layout the interface
does not draw, and `auto` says what it currently decides for this terminal rather than what it decides in
general.

A row can say that writing it will change nothing. The screen is given the configuration layer each
value arrived from, and a value coming from anything above the file, an environment variable for
instance, carries a line saying that layer still wins after a restart. Without it the sequence is: change
a row, watch it apply, see the file written, restart, find the old value back, and conclude the screen is
broken.

The zone list is a selection rather than every name the database carries, for the same reason the browser
offers a selection: six hundred entries cycled one press at a time is not a control anybody can use. A
zone or a scheme already configured is folded into the list so it is never cycled away and lost, and a
value this build cannot render is refused rather than adopted.

The session section is read only. It names the target, the tenant, the actor, the build and the
configuration file. The target is taken from the redacted view of the resolved configuration, so a DSN
carrying a password is not drawn on a screen somebody is sharing, and a file that does not exist yet says
so rather than reading as one that failed to load. Changing a target from inside a session already pinned
to it would apply to nothing, so the screen points at `tix config show --sources` and `tix tenant use`
for everything that is configuration rather than a reading preference.

A session with nowhere to write, which is what an SSH session is, keeps the screen usable and says the
choices last until you quit.

## Adding a task

`n` opens the new-task prompt, which reads [quick-add syntax](features.md): `Fix login !high #auth
@raj due:fri` creates "Fix login" with that priority, tag, assignee and due date, in the open project
unless the line names another with `+key`. A row under the field names what the line will set while you
type. A line the parser refuses keeps the prompt open with the reason, and one the service refuses, an
unknown handle or project, reopens the prompt holding the line as typed. Relative dates count in the
local zone. Quick-add syntax names no lease length, so a task added here inherits its project's: set
one with `tix task add --lease`, `tix task edit --lease` or in the browser.

## Filtering

`/` on the board takes the expression language of [filtering.md](filtering.md), through the same parser,
so an expression that selects a set of tasks here selects the same set from a shell. `C` clears it. A bad
expression keeps the filter already in force and reports itself in the status bar, with the text still in
the bar, rather than emptying the board. The expression in force is named in the title bar.

`assignee:me`, `creator:me` and `claimed-by:me` name whoever is signed in, here as on every other surface,
so `/ assignee:me due:week` is your week on this board.

### Saved filters

`s` lists your saved filters, your own first in the order you keep them, then the ones others shared with
the tenant, each with the expression it applies. The arrows walk the list, `enter` puts the filter on the
board with its sort, and `esc` closes the list and leaves the board's filter as it was. With none saved
yet, the status bar says how to keep one instead of opening an empty list.

`K` keeps the filter in force under a name you type, as one of your own, unshared. It is the same list
the browser's sidebar shows under Tasks and `tix filter ls` prints, so a filter kept here is a link there.
Renaming, sharing, ordering and removing one are done in the sidebar or with `tix filter`; the terminal
applies and keeps them. Both keys are offered only to a reader who may read tasks, and `K` only while a
filter is in force.

`is:deleted` reveals the deleted cards the board otherwise leaves out, each marked `†`. It is the term the
shared parser already had, so it is the terminal's equivalent of `tix task ls --include-deleted`, and `u`
on one of those cards is what brings it back. `is:trashed` shows the deleted cards alone, the board's
trash, the way `tix task ls --trashed` does; `u` on a live card refuses and names that term.

`/` on the activity view takes a different grammar: the audit filter `tix audit ls --filter` takes,
because an event has a kind and an actor and no status, tag or due date to ask about. A term the live tail
cannot answer is refused by name, pointing at `tix audit ls --filter` for the question that needs the
stored log:

```text
activity: kind:task actor:ada -action:task.updated word
```

The text the filter is answered against is the text the line on screen says, so a word somebody can read
in the tail is a word they can filter it by.

`/` on the statistics view cycles the window, over 7, 14, 30 and 90 days. A statistics screen has one
thing worth narrowing and it is the period, so rather than open a text prompt for a single number, the key
that means "narrow this" everywhere else steps through the windows.

## The start menu

`~` opens a menu with one row per interface a session can host. Today that is Boards, which opens the
project list, and Watch, which opens the activity view. On the Watch row `←` and `→` step through
`all projects` and every project you can read, and `enter` opens the tail narrowed to the one picked;
`v` from anywhere else always watches every project. `esc` goes back to the menu from whichever
interface it opened.

The rows come from one registry in `internal/tui/interfaces.go`, the same one the SSH listener reads its
command words and its refusal message from, so an interface added there appears on the menu and as an
`ssh` command together, and a test fails if either is missing. A hosted SSH session that names no command
starts on this menu; `tix tui` starts on the project list as it always has, and a demo sandbox opens its
seeded board. See [deployment.md](deployment.md#the-command-picks-the-interface) for the commands.

## The activity view

`v` draws the live event tail. The subscription is already running for every view, so opening this one
changes what is drawn and never what is fetched, and each line is rendered from the same field extraction
`tix watch` uses, so the two surfaces cannot describe one event two different ways.

The view keeps the most recent 200 events and drops the rest, so a long session watching a busy tenant
cannot grow without limit, and the view says so rather than silently dropping what it held. It follows the
newest event until the selection is pulled back to read an older one, and then stays where it was put.
`space` pauses it outright: new events are held back, the title bar says `paused, N new`, and a second
`space` draws them and follows the newest line again. The held events count against the same 200 cap.

The status bar counts what is held, and what the filter is keeping when one is in force, so a short list
is never mistaken for a quiet tenant. The empty state distinguishes the three cases for the same reason:
a filter that matches nothing, a subscription that has actually dropped, and a tenant with nothing
happening are three different problems, and a long silence must not look like a stalled program.

The title bar carries `● live` or `○ disconnected, retrying`. A dropped subscription is reopened from the
last sequence seen, so reconnecting does not skip events.

## The history view

`H` reads one page of the durable log for whatever you are looking at: the selected task, the open
project, or the tenant when neither is named. The heading says which, because a listing of changes with no
subject named is a listing of changes to whatever happened to be selected. There is no filter bar here:
the subject is the filter, and it is chosen by what you press `H` from rather than typed.

It is kept apart from the activity tail for two reasons. Activity is a live
subscription: it starts empty, holds the latest two hundred events of this session, and is offered to a
reader holding `event:subscribe`. History is the stored log: it answers what happened before this session
opened, it is bounded by a page rather than by a session, and reading it needs `audit:read`. One view for
both would offer a reader holding `audit:read` no history at all, and would promise a reader holding only
`event:subscribe` a record the service refuses.

Each line is the same reduction `tix audit ls` prints, so the two surfaces cannot describe one change two
different ways: the sequence it was written at, when it happened, who did it, what was done and to what,
and the surface it arrived through. Actors resolve to handles, because the identifier the log stores names
nobody a reader recognises.

The status bar names the bound rather than leaving a page of a long log looking like the whole of one, and
points at `tix audit ls` for the older entries. A subject with nothing recorded against it says so in
words, since a heading over blank space reads as a read that failed, and a read that did fail opens no
view at all. `r` re-reads the same subject, rather than refreshing a screen you are not looking at.

## The inbox view

`I` reads the newest hundred items of your inbox: somebody mentioned you as `@handle`, assigned you a task,
commented on a task you are assigned to or starred, or moved a task assigned to you. Unread items carry a
dot and full brightness; read ones are dimmed. `enter` marks the selected item read and opens its task,
`space` toggles it between read and unread, and `a` marks everything read. `r` reads the inbox again. The
status bar counts the items and the unread ones, and points at `tix inbox ls` for older pages.

Muting a kind is a setting kept with your account, set from the browser's Settings or `tix inbox prefs`;
the terminal has no screen for it, which the registry records as an exemption rather than a gap. The
same holds for notification channels (`tix inbox channel`).

## Creating a project

On the project list `n` asks for a key and a name. When the tenant has more than one workflow, or built-in templates to offer,
a form follows that picks the workflow: the tenant's own first, then the templates. While a template is
selected the form says what it is for first, then its name marked `[Built-in]`, its states and the fields the
project will get. Applying a template under another key, or over one that is taken, is `tix workflow template
apply`.

## The project screen

`w` opens the project the board is on. `e` edits it: the colour and, when there is another one to move to,
the workflow from their lists, and the name, the key, the description and the icon from the single-line
input seeded with the value it would replace. A new key renames every reference on the board at once; the
old key stays an alias, so a reference written before the change still opens the task.

## The tenant screen

`T` reads the tenant this session is pinned to and states it: its key, its name, the theme it presents
itself with, the tenants this session can see, the hostnames that resolve to it and the actors who belong
to it. Each of those is a separate read under a separate authority, so a reader refused one gets the rest
of the screen and a line saying what is missing, rather than a screen that fails to open over a listing
they were never going to see.

One cursor runs over the domains and the members together, because `X` removes either and a removal needs
a named subject rather than a typed identifier. The confirmation says the hostname or the handle out loud.

`e` edits the tenant: the theme from the palettes the build carries, including the word for having none
and `custom`, which asks for a `#rrggbb` accent at the single-line input, and the name from the
single-line input seeded with the name it would replace. `n` asks whether a domain or a member is being
added. A domain takes its hostname at the prompt and is added carrying no certificate
of its own, because a certificate is a pair of paths on the server that no control here can gather;
`tix domain add` takes them. A member is picked from the tenant's own directory, read at the keystroke
rather than held for the session, with one of the three roles beside it.

### Switching tenant

For a person in more than one tenant the screen lists their memberships, read from whoami, under
`memberships`: each tenant's key and the role held there, the current one marked. The cursor runs onto these
rows after the domains and the members, and enter on one switches to it when a credential for it is at hand:
a context holding its token on the same server (`tix login --tenant KEY` stores one), or a database opened
directly, which reaches every tenant. A tenant with no credential says so on its row, and enter names the
login command and leaves the session where it is. A person in one tenant, or a local caller whose actor has
no account, is offered no list.

Over SSH the username picked the tenant, so the rows carry `ssh KEY@HOST` instead and enter repeats it; there
is no switch inside an SSH session.

A key can still be typed: enter anywhere else on the screen opens the input. It is checked the way
`tix tenant use` checks one, by opening a connection pinned to it and asking who you are there.

A key that cannot be reached leaves the session exactly as it was, naming the key in the refusal. The key
the session is already on is refused before anything is dialled. A switch that lands replaces everything
the previous tenant's rows produced, the project list, the board, the open task and the event tail, and an
event the previous stream was already holding is dropped rather than drawn under the new tenant's name.

**A switch drops the leases this session holds, and does not release them.** A lease is held by the tenant
it was taken in, so walking away from it does not give it back: it stays held until it expires. The status
bar says so, counting them:

```text
✓ switched to tenant acme; 2 leases dropped from this session and will expire unreleased
```

Release before switching if you want the work back in the queue now. See [agents.md](agents.md) for what a
lease is and why the token matters, and `tix claim sweep` for what returns a lapsed one.

The switch lasts for the session. `tix tenant use KEY` writes it down. A session that was opened with no
way to dial another tenant, which is what an SSH session is, says that plainly and points at
`tix tenant use` rather than offering a switch that cannot happen.

## The people screen

`@` lists the people of the tenant this session is pinned to: each user's address, display name, role,
whether they are active or disabled, whether they hold a second factor and whether their next sign-in must
choose a new password. The pending invites follow, expired ones included so they can be re-issued, each with
its role, when it stops working and who issued it. One cursor runs over both lists. It is the browser's
Admin > Users screen and `tix user` in one place.

`enter` reads the selected user in full, the way `tix user show` does. `n` opens one form for a new person,
with a choice of how they sign in. With a password, the password is masked as it is typed and as it is
shown, and the change at first sign-in is on by default, as in the browser. With an invite link, the form
asks how long the link works instead, from one to thirty days.

**An invite link is shown once.** Only its hash is kept, so the link is drawn at the top of the screen when
it is issued and is gone as soon as you act again or leave the screen. tix sends no email: copy the link and
send it yourself. A session against a local database knows no address of its own, so the link is a path,
and the screen says to put the address people open tix at in front of it.

`e` edits the selected user: display name, role, disabled, a new password at next sign-in, and the password
itself, which is left alone while the field is empty. Only the fields you changed are sent. `R` re-issues the
selected invite with a fresh expiry and shows the new link once; the old one stops. `F` clears the selected
user's second factor, for somebody who lost both the device and the recovery codes. `X` deletes the selected
user or revokes the selected invite. Clearing, deleting and revoking each ask first, naming the person.

A person can belong to more than one tenant. Their password, their second factor and their account belong to
all of them, so a tenant administrator who is not a global admin may change only their standing here: the
role, the display name and whether they are disabled in this tenant. A change that reaches further is
refused, and the status line says why in words naming them:

```text
✗ cannot edit ada@example.com: ada@example.com also belongs to another tenant, so only they or a global admin can change their password, second factor or account
```

The roles are read from the tenant's memberships, which needs `tenant:admin`. A reader who holds
`user:admin` without it gets the list with the roles marked unknown and a line saying why.

## The integrations view

`J` opens the parts of a tenant that talk to the world outside it, on three tabs: webhooks, sync and
retention. `←` and `→` move between them. Each tab needs its own scope, `webhook:admin`, `sync:admin` and
`tenant:admin`, and a tab you lack the scope for is not drawn or read at all, so a reader holding only
`sync:admin` opens the view on the sync tab and never sees the other two named. The tab strip at the top
brackets the open one, which is how a terminal with no colour tells them apart.

```text
tabs: [webhooks]  sync  retention

endpoints (1):
▸ https://hooks.example.com/tix   active  task.*

deliveries (1, newest first):
  failed    3 attempts  connection refused  to https://hooks.example.com/tix  01J...
```

**Webhooks.** One cursor runs over the endpoints and then the newest fifty deliveries, the way the tenant
screen runs one over domains and members. `n` registers an endpoint, `e` edits the selected one, `X`
deletes it behind a confirmation naming its URL, and `enter` on a delivery queues it again. The form takes
the URL, the event types comma separated (`task.*` matches a family, blank is every event), whether it is
active, and the signing secret. Leave the secret blank and tix generates one, which the tab shows once,
until you leave the view; a blank secret on an edit keeps the stored one. Older deliveries are
`tix webhook deliveries --cursor`.

**Sync.** Each source is listed with its system, its last run, its stored settings, which credentials are
stored (by name and the last four characters of a long one, never more), where a run on the server reads
its mapping and credentials from, and what it still lacks there. `n` configures a source, `e` sets up the
selected one, `X` deletes it behind a confirmation, and `enter` runs it from a form whose dry run is on
until you turn it off; full ignores the stored cursor. The source form offers the settings the chosen
system takes, the mapping file on the server, a mapping document pasted into a multi-line field, and the
credentials. Credentials are write-only, as on the web: their fields draw a mark per character, are never
filled in from what is stored, and a blank one keeps what is there. `check only` validates the source on
the server, saves nothing, and draws what a run would use. A run's counts, warnings, unmatched people and
cursor, or the reason it was refused, are drawn under the sources.

**Retention.** The three windows, in the form `30d` or `12h`. `e` edits them; a blank window keeps its
value, which is what `tix retention set` does with a flag left out. `enter` prunes from a form whose dry
run is on by default, with an optional limit per kind. A prune that writes asks first, and either way the
counts are drawn on the tab.

## The server view

`%` reads the same report `tix status` prints and the tenant's live connections, and is offered to a reader
who may read the status report, which is a tenant admin. It states the installation (version, schema,
engine, store), every server registered against it with whether it still heartbeats and which one
answered, the tenant's work counts, and the state of the second factor. The two reads are separate, so a
refused one says why in its own section and costs nothing else.

The connections are the tenant's on the server that answered, the same partial view `tix connection ls`
gives when several servers share one database. The cursor runs over them and `X` ends the selected one
behind a confirmation naming its surface, handle and address. Ending is not revocation: the holder may
reconnect. Over SSH this session is in the list too, and ending it closes it.

## The global admin view

`!` is offered only to a session whose actor the service reports as a global admin when the session
opens, which is the same report the web sidebar uses to offer its Global admin area. A session holding the
database directly is one. Revoking global admin takes effect on the next call, because the service asks
the store every time; the view stays offered until the session ends and then refuses.

The session is pinned to one tenant and that does not get in the way: every global admin operation names
the tenant it acts on, exactly as the web screens do from a session pinned the same way. Nothing is dialled.

The view has three parts, and `esc` walks out of a part before it leaves the view:

- **Tenants and global admins.** One cursor runs over every tenant and then every global admin. `n`
  creates a tenant from a key, a name and the first admins' addresses separated by commas, or grants global
  admin by email. A new address gets a generated password, shown once under its own heading until the view
  is read again or left. `X` deletes the selected tenant, or revokes the selected global admin, after a
  confirmation naming it. The tenant this session is on is marked and not deleted from here, because that
  would end the session's own access; `tix tenant rm` can.
- **A tenant's users.** `enter` on a tenant lists its users with their role and whether they are disabled.
  `n` creates a user (email, handle, display name, password, role) or adds an existing account by email
  with a handle and role. The password is masked as it is typed and on the form. `e` edits the display
  name, the role and the disabled state, sending only what changed. `X` deletes the user after a
  confirmation.
- **An account log.** `enter` on a global admin or a user, or `/` and an email anywhere in the view, reads
  that account's log: every change, the tenant it concerned, who made it and through what.

## The account screen

`$` opens what the browser spreads over Settings, Tokens, SSH keys and Your account: who this session is
signed in as, your second factor, your own API tokens and SSH keys, your sessions in this tenant, and the
newest page of your account log. Each section is its own read, so a refused one says why and the rest of
the screen still draws. An agent's session, for one, has no second factor and no account log to show.

One cursor runs over the second factor, the tokens, the keys, the sessions and the log entries. `X` on a
token, a key or a session revokes it and on the second factor removes it, each behind a confirmation naming
what goes; a revoked token or key has nothing left to revoke and is not offered. The service decides
whether the factor may be removed from the surface you came in on: a tenant that requires one over SSH
refuses it there, and the status bar carries that refusal.

The sessions are your browser and terminal sign-ins in this tenant, newest first, each with how and when it
signed in, when it expires and its user agent. Revoking one signs whatever holds it out at once. The
terminal itself runs on a token or an SSH key, so none of them is marked as this one; a token needs
`account:read` to see them, which the token `tix login` stores carries.

`R` on a live token rotates it: a form asks how long the old value keeps working, from the browser's choices
(stop it now, proposed, or 10 minutes, 1 hour or 24 hours), a confirmation names the token and the window,
and the new value, same name, scopes and lifetime, is drawn once at the top of the screen the way an issued
token is. A token already rotated and still in its window is listed as `rotated, stops ...` and offers `X`
but not `R`. Rotating the token this session runs on with no window ends the session's own credential, so
pick a window and point the context at the new value first.

`n` asks what to add, offering only what your authority reaches: an API token, an SSH key, and either a
second factor while none is on or new recovery codes once one is.

A token takes a name, an expiry from the browser's own choices (7, 30 or 90 days, a year, or never, with
90 days proposed) and a preset to start from. The scope picker then takes the screen: the browser's groups
in the browser's order, each scope with the same note, `*` first for a reader who holds it, and a scope you
do not hold drawn as `[-]` and refused when ticked. `space` ticks one, `a` ticks or clears its whole group,
`enter` issues and `esc` gives up. The token's value is drawn once at the top of the screen and stays until
`esc` dismisses it; it never goes to the status bar, where the next message would replace it.

An SSH key takes the line `ssh-keygen` wrote to the `.pub` file, and a label.

Enrolling a second factor draws the `otpauth://` address as a QR code, the same half blocks `tix login`
prints, when the terminal has colour and room for the whole symbol, because a terminal that strips escapes
gets a block of identical characters and half a symbol does not scan. When either is missing the screen
says which, and how large the terminal needs to be. The key, grouped in fours, and the address are always
drawn as text, so enrolment never depends on the code. `enter` asks for the code the authenticator shows;
the recovery codes are then drawn once, the same way a token's value is. `esc` abandons an enrolment,
which grants nothing until it is confirmed.

The tokens and keys listed are yours. The browser's Tokens screen widens to the whole tenant for a reader
holding `tenant:admin`, and `tix token ls --actor` and `tix user key ls --actor` reach anybody's; the
registry records that as a limitation on both terminal bindings. The account log reads the newest fifty
entries; `tix account log --cursor` reads further back.

## You are offered the views your permissions reach

A key that opens a view the service would refuse tells the reader the refusal is their mistake. So the
set of views a session is offered is resolved once from the connecting actor's authority, and both the
navigation keys and the `?` overlay obey it: a reader who may not subscribe to events is not told about
`v`, and pressing it does nothing.

The set is derived rather than written down. `capability.TUIAccess` offers a view when the actor may
perform at least one of the reads the registry binds to it, and the scope each read needs is the scope
the service actually enforces, which `internal/capability/authority_test.go` proves against the real
service. There is no second table of permissions in the terminal package to fall out of step with the
first.

| View | Offered when |
| --- | --- |
| projects, settings, help | always |
| board | the actor may read tasks, or read workflows |
| project | the actor may read projects, or read workflows |
| detail | always in practice: resolving an identifier to a handle needs no scope |
| statistics | the actor may read tasks |
| activity | the actor may subscribe to events |
| history | the actor may read the audit log |
| tenant | always: asking a tenant who you are there needs only a session |
| people | the actor may administer users (`user:admin`, which the admin role holds) |
| integrations | the actor may list webhooks (`webhook:admin`), list sync sources (`sync:admin`) or read retention (`tenant:admin`); each tab only for its own |
| server | the actor may read the status report, which is tenant administration |
| global admin | the service reports the actor as a global admin when the session opens |

Global admin is the one row not derived from a scope. The service decides it from the store on every call,
and no session holds it as a scope, so the session asks the service once, when it opens, whether its actor
is a global admin, and `capability.Operation.Permits` answers operations needing `global:admin` from that
report.

The project list, settings and help are exempt because they need no authority. Settings and help read
nothing from the service at all, and the project list is where every session starts and where going back
ends up, so there is nowhere to send a reader who is refused it. A reader who cannot list projects is
told so by the list's own empty state instead.

The default is closed. A caller that supplies no access set loses navigation, which is visible, rather
than gaining screens the service will refuse. Every entry into a view goes through one function, so a
view added later is gated by existing and not by its author remembering to ask.

This matters most over SSH, where the person connecting may be a demo visitor in their own sandbox or an
enrolled member with their real membership, and both arrive down the same code path.

## Over SSH

`tix ssh` serves this interface over SSH, with tix itself as the SSH server. The client proves a public
key and lands on a board. [deployment.md](deployment.md#the-terminal-interface-over-ssh) covers enrolling
keys, how the username selects the tenant, the enrolled and demo modes, and the flags.

Four things about the interface are different in a session served that way, and all four follow from the
session belonging to a listener rather than to a shell:

- **Settings do not persist.** There is no configuration file to write, so the screen says the choices
  last until you quit.
- **The tenant switch is read only.** Which database or server a key resolves against is configuration, and
  the listener hands the session no way to dial another, so the view says so and names `tix tenant use`.
- **The session facts are mostly unstated.** The target, the version and the configuration file are
  things the command layer supplies, and a listener does not; each says it was not named by this session
  rather than rendering blank.
- **Colour comes from what the client said its terminal is.** Each session builds its own renderer from
  its own environment, so one client's colour depth cannot be applied to another's.

The clipboard is the same as locally: `^` copies through your own terminal with OSC 52, so a link copied
from a board on a server lands on the machine you are typing at ([sharing a task](#sharing-a-task)).

A demo sandbox prints one line on disconnect, after the alternate screen is gone: the board is owned by
that key, reconnecting with the same key finds it as it was, and it is deleted after a period without a
visit.

The same listener runs tix commands as well as this interface: `ssh acme@host task ls -o json` prints what
`tix task ls -o json` would, as the key's actor, with nothing configured on the client. That is the way to
move files over SSH, since the listener never reads or writes a path on your machine:
`ssh acme@host export tasks -p infra > infra.csv`. See
[deployment.md](deployment.md#commands-over-ssh).

## What the terminal does not do

The registry records, for every operation, either a terminal binding or a reason there is none, and marks
a reason as a gap when the binding ought to exist. As of this writing there are 0 such gaps against 182
operations: everything the terminal ought to do, it does, and what it does only in part is recorded as a
limitation on the binding. A test asserts the exact number, so a new operation that arrives without a
binding or a written reason fails it rather than growing the count quietly. Treat
`internal/capability/registry.go` as the live answer rather than any list here:

```sh
grep 'gap(SurfaceTUI' internal/capability/registry.go
go test ./internal/capability/...
```

The calendar feed is absent on purpose rather than as a gap: its address is pasted into a calendar app,
which the terminal has no part in, so its six operations are managed from `tix calendar` and the settings
screen.

Every shape that was on this list has closed. Your own credentials: the account screen issues and
revokes your API tokens, enrols and revokes your SSH keys and runs your second factor from enrolment to
removal, which closed eleven. Working on definitions rather than instances: the project
screen edits a project, defines and removes custom fields, and renders the workflow it runs on. Input a
single line of text cannot gather: a prompt now hands over to a form whose every field picks from the
values the operation accepts, which is how the assignee and the artifact arrived, and what the terminal
still cannot gather is recorded as a limitation on the binding rather than as a gap, so `artifact.put` is
bound and says in the same breath that a payload, a content type and an inline blob belong to
`tix artifact put` and the API. Listings needing a selectable sub-item: the comment thread, the tag list,
the dependency list, the actor directory and the deleted cards `is:deleted` reveals all have a cursor
over them now. And the tenant itself: the screen that stated only which tenant a session was pinned to
now reads the tenant, the tenants the session can see, its domains and its memberships, and edits all
three, which closed nine. The people screen closed ten more: users, their invites and clearing somebody
else's second factor. And the parts of a tenant that talk to the world outside it: the integrations view
lists, registers, edits, pings and deletes webhook endpoints and redelivers from their log, configures, checks,
runs and deletes external sync sources, and edits the retention windows and prunes, which closed twelve.

The server and the installation have closed too. The server view reads the status report and the
tenant's live connections and ends one, which closed three, and the global admin view creates and deletes
tenants, which closed the last two that a session pinned to one tenant was said not to be able to do. The
same view turned nine absences that were never counted as gaps into bindings: a tenant's users, adding an
existing account, the global admins and any account's log. `tenant.delete` records one limitation: the
session's own tenant is not deleted from here.

Import and export closed last: the transfer view moves tasks, bundles and snapshots by file path, and over
SSH names the command that does it instead (see [moving data in and out](#moving-data-in-and-out)). That
closed the final six.

Eight absences are not gaps and will not close. Closing the service is process lifecycle; hostname
resolution runs in the server request path; lease sweeping is a background loop; signing in and out
bracket the program rather than happening inside it, since the interface opens on a session that already
exists and ending it would revoke the credential the running program is using; proving a second factor
happens in the ssh listener before the interface starts, which is the point of it, since a view inside the
board would be drawn after the tenant's data; and the two workflow writes cannot be served here rather
than have not been yet. A workflow is a state machine, which is a graph, and neither a line of text nor
a form over fixed lists gathers one, so the project screen names
`tix workflow put` instead. A deletion would only ever be offered on the workflow the open project runs
on, which is the one workflow the service refuses to delete, and an entry leading to a refusal is worse
than no entry.

Bulk task actions are absent on purpose as well: the board acts on the card under the cursor, and each
action a bulk request applies (move, priority, tags, assignee, star, copy, delete, restore) is bound
there for that card. Selecting many rows at once belongs to the web task list and `tix task bulk`.

Housekeeping accounts for three more, by choice: its rules are tenant administration, set on the Tenant
screen, `tix cleanup` and the API, and `tix serve` applies them on its own; the board only shows what they
moved, in the trash.

Saved filters account for four more, by choice: renaming, sharing, ordering and removing one belong to
the browser's sidebar and `tix filter`, reading one by name is `tix filter show`, and the board applies
them from the list with `s` and keeps a new one with `K`.

What is bound is daily work: a test names the operations a board has to reach and fails if any of them
loses its terminal binding.

The terminal interface is where you work on tasks, and the command line is where you configure the thing
you are working in. Every gap names a reason, and `tix <command> --help` or
`tix docs` will have the command.

## Narrow terminals, and colour

The board draws as many columns as fit at twenty characters each, keeping the selected column in view,
and the status bar says which slice is on screen only when the board is wider than the terminal. The width
those columns then share is weighted by what each has to show rather than split evenly. Below two
columns' worth of width it draws one column. Below 24 by 8 it stops drawing and says what size it needs
and what it has, rather than rendering something unreadable:

```text
terminal is 20x6; tix tui needs at least 24x8
```

At the floor itself the footer keeps the palette hint and loses the view's own keys:

```text
tix │ as local │ @defaul
  6 projects

▸ 🏗  infra   Infrastruc…
  🖥  web     Web UI
  🤖 agents  Agent Fleet
  📚 docs    Documentati
  ↓ 2 more
6 projects │ ✓ connected
ctrl+k commands  ↑/k up
```

Every scrolling list gives up one row to a hint counting what is hidden above and below, so a list that
runs off the screen is never mistaken for the whole list, and a list that fits claims nothing.

`NO_COLOR` or `TIX_NO_COLOR` suppresses every escape, as does `output.color = never` and `--no-color`. A
destination that is not a terminal gets none by default. State is never carried by colour alone: every
marker, badge and category is also a word or a character. The selected card is drawn with a heavier bar,
not only a brighter one, and an open input panel is separated from the board by a rule, so neither the
selection nor where the keyboard is being captured depends on an escape sequence. A run with no colour
also loses the text cursor inside an input and inside a prose field, both drawn in reverse video, along with
the prose field's line numbers and its own prompt character: the panel's own rule and its legend say where
the typing is going. The tenant's accent is resolved the same way
the browser resolves it, so one tenant is one colour on both surfaces, and a tenant whose branding cannot
be read keeps the built-in accent rather than failing to open a board. See [theming.md](theming.md).

## Something to look at

An empty install is a poor demonstration of a screen whose job is showing work in progress.

```sh
tix demo seed --db /tmp/demo.db
tix tui --db /tmp/demo.db
tix tui --db /tmp/demo.db -p infra --filter "status:todo is:unclaimed"
```

`--db` is not optional advice. A command with no `--db` resolves to the zero-configuration store, which is
somebody's real work, and `tix demo seed` refuses a database that already holds tasks unless `--reset` is
passed.

The seed writes several projects with their own colours, people and agents, custom field definitions, and
tasks carrying descriptions, tags, priorities, assignees, due dates, dependencies and comments, so the
card markers, the column categories and a full detail view can all be seen without arranging any of it by
hand. [web-ui.md](web-ui.md#something-to-look-at) covers what it writes and why it replays the history
through the ordinary service calls.

## See also

- [filtering.md](filtering.md) for the expression the board's filter bar takes, and the activity grammar
- [configuration.md](configuration.md) for `tui.keymap`, `tui.keys` and the five configuration layers
- [agents.md](agents.md) for what a lease is, and why switching tenant with one held matters
- [deployment.md](deployment.md#the-terminal-interface-over-ssh) for serving this interface over SSH
- [statistics.md](statistics.md) for what the `S` view's figures mean
- [tenancy.md](tenancy.md) for what a tenant owns and how isolation is enforced
- [theming.md](theming.md) for the tenant accent and what is deliberately not themed
- [web-ui.md](web-ui.md) for the browser interface over the same service layer
