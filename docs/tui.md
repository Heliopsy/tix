# The terminal interface

`tix tui` opens a board in the terminal, over the same service layer the command line and the browser
call. That is enforced rather than asserted: `internal/capability` lists `tui` among the surfaces every
operation must reach, and the parity test fails the build when a new operation names no terminal view or
records no reason for not having one.

The interface documents itself while it runs. `?` opens an overlay listing the bindings of the view you
came from, the bindings that work everywhere, the card markers and both filter grammars, built from the
same `KeyMap` the program is actually dispatching, so it cannot describe keys you do not have. This page
exists for the part `?` cannot cover: what is here before you connect. Over SSH that is the whole
problem, because you connect first and find out afterwards.

## The nine views

The title bar names the open one, between the project and the connection state, because projects, the
board, the task, the project screen, activity, the tenant screen, statistics and settings all render into
the same frame and recognising the body was the only way to tell where you were.

| View | Opened by | Shows |
| --- | --- | --- |
| projects | `p`, and every session starts here | every project this tenant has, with its colour and icon |
| board | `enter` on a project | one project's tasks in columns, one column per workflow state |
| project | `w` | one project's attributes, the workflow it runs on, and its custom field definitions |
| detail | `enter` on a card | one task in full: body, custom fields, subtasks, dependencies, artifacts, comments |
| activity | `v` | the live event tail, filtered by the audit grammar |
| statistics | `S` | the same figures as `tix stats`, for the open project or the whole tenant |
| tenant | `T` | the tenant in force, and a key to switch this session to another |
| settings | `,` | display preferences, and what this session is connected to |
| help | `?` | the bindings of the view underneath, and the legends |

Navigation is a stack. `esc` and `backspace` step back one level, and so does `q`: quitting from a
nested view would lose a place a reflex press only meant to step out of, so `q` ends the program only at
the top. `ctrl+c` always ends it, exiting 130. Entering the view already open is a refresh rather than a
step, so reloading never makes the way back one press longer.

`p` is different from `esc`. It discards the whole stack and returns to the project list.

**Cards.** A column draws one line per task: the reference, a priority badge, then markers. The legend is
in `?` and is short on purpose, so it still fits a narrow terminal:

| Marker | Means |
| --- | --- |
| `@` | claimed, by somebody |
| `@me` | claimed by you |
| `!` | blocked |
| `+` | has dependencies |
| `*` | has a due date |

`@me` is separate from `@` because "is that me" is the first question anybody asks of a claimed card on a
shared board, and a single marker left it answerable only by opening the task.

Columns follow the workflow's declared order and are coloured by the category a state belongs to, not by
its name, since workflow states are user defined. Nothing is conveyed by colour alone.

**The footer is a promise.** It advertises only the keys the selected task can accept: `c` while the task
is free, `x` and `R` while this session holds the lease, neither while another worker does, and `t` only
where the workflow permits a move out of the current state. A task held elsewhere says who holds it in the
status bar instead. `?` still documents every binding the view has, so hiding a key from the footer never
hides it from the help.

## Keys

Five schemes ship, for people arriving with muscle memory from somewhere else. The scheme is
`tui.keymap` in configuration, `tix tui --keys NAME` for one run, or the first row of the settings
screen.

| Scheme | What it changes |
| --- | --- |
| `default` | the shipped bindings below |
| `vim` | `o` new, `i` edit title, `I` edit body, `a` comment, `:` also filters, `e` refreshes |
| `emacs` | `ctrl+p`/`ctrl+n`/`ctrl+b`/`ctrl+f` move, `ctrl+a`/`ctrl+e` ends, `ctrl+g` backs out, `ctrl+s` filters |
| `nano` | `ctrl+w` searches, `ctrl+g` helps, `ctrl+x` quits, `ctrl+o` applies, `ctrl+l` redraws |
| `helix` | `x` opens the row, `d` releases, `o` new, `i` edit, `a` comment, space opens settings |

There is deliberately no `mac` scheme. A terminal never sees the command key, and the chords macOS
applies to every text field are Cocoa's emacs bindings, so a mac scheme would be `emacs` under a second
name. The emacs description says so instead.

`nano` leaves `ctrl+k` alone. It cuts a line in nano, and tix has no action that removes the selected
task; `x` gives up a lease rather than deleting anything, so binding it would invent a meaning nano does
not have. `ctrl+c` is left alone too, because interrupt already owns it.

A scheme lists only the actions it moves. Everything else keeps its default keys, which is what makes it
impossible for a scheme to leave an action unbound.

### The default bindings

| Keys | Action | Where |
| --- | --- | --- |
| `↑`/`k`, `↓`/`j` | move the selection | everywhere |
| `←`/`h`/`shift+tab`, `→`/`l`/`tab` | previous, next column | board; steps a value on settings |
| `g`/`home`, `G`/`end` | first, last | lists |
| `enter` | open, or apply an input | everywhere |
| `esc`/`backspace` | back one level | everywhere |
| `/` | filter | board, activity; cycles the window on statistics |
| `C` | clear the filter | board, activity |
| `c` | claim | board, detail |
| `x` | release | board, detail |
| `R` | renew the lease | board, detail |
| `N` | claim next from the queue | board |
| `t` | transition | board, detail |
| `n` | new task, or new project on the project list | board, detail, projects |
| `e`, `E` | edit title, edit body | board, detail |
| `P`, `A` | set priority, set assignee | board, detail |
| `m` | comment | board, detail |
| `#`, `U` | add tag, remove tag | board, detail |
| `D` | add a dependency, by reference | board, detail |
| `w` | project setup | everywhere |
| `f` | custom field definitions | project |
| `p`, `,`, `v`, `S`, `T` | projects, settings, activity, statistics, tenant | everywhere |
| `r` | refresh | everywhere |
| `?` | help | everywhere |
| `q` | back, or quit at the top level | everywhere |
| `ctrl+c` | interrupt, exit 130 | everywhere |

`S` is capital because lowercase `s` is spoken for on the board, and a capital is what the other
cross-view keys already use when their letter is taken.

Every action that acts on a task is offered wherever a task is selected, so the board and the detail view
run the same set rather than drifting apart. Each of them opens one single-line input, seeded with the
value it would replace, and an empty input cancels, so a stray keystroke never sends a blank edit.
Transition and priority open a numbered picker instead, and transition offers only the states the task's
own workflow permits from where it is.

### Rebinding one action

`--key ACTION=KEYS` for a run, or `tui.keys` in configuration for good. The action names are the fields
of the interface's own key map, so `--key New=o` and `--key Filter=ctrl+s,/` are the shape.

A per-invocation flag wins for the action it names and leaves the rest of a configured set alone. A
rebinding keeps the action's description, so the footer goes on saying what the key does.

A rebinding that would make one key mean two things in the same view is refused with the collision
named, rather than applied. The other behaviour is a key that quietly stops doing what its owner expects.
The same check runs per view, because the same key may mean two things in two views and often should.

An unusable scheme name or an override naming an action that does not exist is reported rather than
swallowed. `tix tui --keys nonsense` refuses before a terminal is opened at all, so a typo in
`tui.keymap` reports the typo and not a TTY error.

## The settings screen

`,` opens four preferences and a statement of what this session is connected to. `↑`/`↓` move between
them and then scroll the body, because on a short terminal the session facts below the settings were
otherwise unreachable: four settings meant four presses and nothing moved after the fourth. `←`/`→`, or
`enter`, step the selected value, wrapping at both ends.

| Row | Configuration key | Values |
| --- | --- | --- |
| keys | `tui.keymap` | `default`, `vim`, `emacs`, `nano`, `helix` |
| time format | `output.time_format` | `iso`, `rfc3339`, `short`, `us`, `relative` |
| timezone | `output.timezone` | `local`, `UTC`, and a selection of named zones |
| colour | `output.color` | `auto`, `always`, `never` |

These are the command line's own keys, not a second store. Somebody at a terminal already has a
configuration file, and a separate place to write a time zone down would be a second place to disagree
with `tix task ls`. A change applies to the frame it is read in and is written to the file at once: a
preference that only takes effect after a restart is worse than none.

Each row is shown with what choosing it would mean, rendered by the thing that will render it. The time
format and zone rows show the same instant formatted, so an example cannot claim a layout the interface
does not draw, and `auto` says what it currently decides for this terminal rather than what it decides in
general.

**A row can say that writing it will change nothing.** The screen is given the configuration layer each
value arrived from, and a value coming from anything above the file, an environment variable for
instance, carries a line saying that layer still wins after a restart. Without it the sequence is: change
a row, watch it apply, see the file written, restart, find the old value back, and conclude the screen is
broken.

The zone list is a selection rather than every name the database carries, for the same reason the browser
offers a selection: six hundred entries cycled one press at a time is not a control anybody can use. A
zone or a scheme already configured is folded into the list so it is never cycled away and lost, and a
value this build cannot render is refused rather than adopted.

**The session section is read only.** It names the target, the tenant, the actor, the build and the
configuration file. The target is taken from the redacted view of the resolved configuration, so a DSN
carrying a password is not drawn on a screen somebody is sharing, and a file that does not exist yet says
so rather than reading as one that failed to load. Changing a target from inside a session already pinned
to it would apply to nothing, so the screen points at `tix config show --sources` and `tix tenant use`
for everything that is configuration rather than a reading preference.

A session with nowhere to write, which is what an SSH session is, keeps the screen usable and says the
choices last until you quit.

## Filtering

`/` on the board takes the expression language of [filtering.md](filtering.md), through the same parser,
so an expression that selects a set of tasks here selects the same set from a shell. `C` clears it. A bad
expression keeps the filter already in force and reports itself in the status bar, with the text still in
the bar, rather than emptying the board. The expression in force is named in the title bar.

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

## The activity view

`v` draws the live event tail. The subscription is already running for every view, so opening this one
changes what is drawn and never what is fetched, and each line is rendered from the same field extraction
`tix watch` uses, so the two surfaces cannot describe one event two different ways.

The view keeps the most recent 200 events and drops the rest, so a long session watching a busy tenant
cannot grow without limit, and the view says so rather than silently losing history. It follows
the newest event until the selection is pulled back to read history, and then stays where it was put.

The status bar counts what is held, and what the filter is keeping when one is in force, so a short list
is never mistaken for a quiet tenant. The empty state distinguishes the three cases for the same reason:
a filter that matches nothing, a subscription that has actually dropped, and a tenant with nothing
happening are three different problems, and a long silence must not look like a stalled program.

The title bar carries `● live` or `○ disconnected, retrying`. A dropped subscription is reopened from the
last sequence seen, so reconnecting does not skip events.

## Switching tenant

`T` states the tenant in force and takes a key to switch this session to another. There is no list to
pick from, and that is not an omission: an actor belongs to one tenant, and both listing tenants and
reading one are scoped to the caller's own, so from inside `default` a tenant named `acme` and a tenant
that was never created are the same answer. The key is checked the way `tix tenant use` checks it, by
opening a connection pinned to it and asking who you are there.

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

## You are offered the views your permissions reach

A key that opens a view the service would refuse tells the reader the refusal is their mistake. So the
set of views a session is offered is resolved once from the connecting actor's authority, and both the
navigation keys and the `?` overlay obey it: a reader who may not subscribe to events is not told about
`v`, and pressing it does nothing.

The set is derived rather than written down. `capability.TUIAccess` offers a view when the actor may
perform at least one of the reads the registry binds to it, and the scope each read needs is the scope the
service actually enforces, which `internal/capability/authority_test.go` proves against the real service
rather than taking on trust. There is no second table of permissions in the terminal package to fall out
of step with the first.

| View | Offered when |
| --- | --- |
| projects, settings, help | always |
| board | the actor may read tasks, or read workflows |
| detail | always in practice: resolving an identifier to a handle needs no scope |
| statistics | the actor may read tasks |
| activity | the actor may subscribe to events |
| tenant | always: asking a tenant who you are there needs only a session |

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
- **The tenant view is read only.** Which database or server a key resolves against is configuration, and
  the listener hands the session no way to dial another, so the view says so and names `tix tenant use`.
- **The session facts are mostly unstated.** The target, the version and the configuration file are
  things the command layer supplies, and a listener does not; each says it was not named by this session
  rather than rendering blank.
- **Colour comes from what the client said its terminal is.** Each session builds its own renderer from
  its own environment, so one client's colour depth cannot be applied to another's.

A demo sandbox prints one line on disconnect, after the alternate screen is gone: the board is owned by
that key, reconnecting with the same key finds it as it was, and it is deleted after a period without a
visit.

## What the terminal does not do

The registry records, for every operation, either a terminal binding or a reason there is none, and marks
a reason as a gap when the binding ought to exist. As of this writing there are 44 such gaps against 88
operations, and a test asserts the exact number so it cannot grow quietly and cannot be mistaken for
zero. The number is coming down, so treat `internal/capability/registry.go` as the live answer rather
than any list here:

```sh
grep 'gap(SurfaceTUI' internal/capability/registry.go
go test ./internal/capability/...
```

The gaps have a shape worth knowing before you try. Roughly:

- **Administration.** Tenants, domains, memberships, users, tokens, credentials, webhooks, retention and
  the server itself have no terminal screen at all. This is most of the count.
- **Definitions rather than instances.** The project screen edits a project, defines and removes custom
  fields, and renders the workflow it runs on, but a workflow's own states and transitions are edited
  with `tix workflow put` or in the browser: a graph is not something a single-column form gathers.
- **Bulk and data movement.** Import, export, bundles and external sync are command line and API only.
- **Edges of what a single-line input can gather.** An operation needing structured input, several fields
  at once, or a confirmation step reaches the terminal later than it reaches the other surfaces.
- **Listings that would need a selectable sub-item.** A thread of comments, a tag list to pick from, a
  task's history, the actor directory and the deleted tasks a restore would come from are the recurring
  shape here.

Seven absences are not gaps and will not close. Closing the service is process lifecycle; hostname
resolution runs in the server request path; lease sweeping is a background loop; and signing in and out
bracket the program rather than happening inside it, since the interface opens on a session that already
exists and ending it would revoke the credential the running program is using.

What is bound is daily work, and that is asserted rather than hoped for: a test names the operations a
board has to reach and fails if any of them loses its terminal binding.

The honest summary is that the terminal interface is where you work on tasks, and the command line is
where you configure the thing you are working in. Every gap names a reason, and `tix <command> --help` or
`tix docs` will have the command.

## Narrow terminals, and colour

The board draws as many columns as fit at twenty characters each, keeping the selected column in view,
and the status bar says which slice is on screen only when the board is wider than the terminal. Below two
columns' worth of width it draws one column. Below 24 by 8 it stops drawing and says what size it needs
and what it has, rather than rendering something unreadable:

```text
terminal is 20x6; tix tui needs at least 24x8
```

Every scrolling list gives up one row to a hint counting what is hidden above and below, so a list that
runs off the screen is never mistaken for the whole list, and a list that fits claims nothing.

`NO_COLOR` or `TIX_NO_COLOR` suppresses every escape, as does `output.color = never` and `--no-color`. A
destination that is not a terminal gets none by default. State is never carried by colour alone: every
marker, badge and category is also a word or a character. The tenant's accent is resolved the same way
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
