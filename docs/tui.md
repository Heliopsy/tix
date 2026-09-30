# The terminal interface

`tix tui` opens a board in the terminal, over the same service layer the command line and the browser
call. That is enforced rather than asserted: `internal/capability` lists `tui` among the surfaces every
operation must reach, and the parity test fails the build when a new operation names no terminal view or
records no reason for not having one.

The interface documents itself while it runs. `ctrl+k` opens a searchable list of every action, each named
in words and each showing its own key, and the footer names that key in every view so you do not have to
know it first. `?` opens an overlay listing the bindings of the view you came from, the bindings that work
everywhere, the card markers and both filter grammars. Both are built from the same `KeyMap` the program is
actually dispatching, so neither can describe keys you do not have. This page exists for the part they
cannot cover: what is here before you connect. Over SSH that is the whole problem, because you connect
first and find out afterwards.

## The ten views

The title bar names the open one, between the project and the connection state, because projects, the
board, the task, the project screen, activity, history, the tenant screen, statistics and settings all
render into the same frame and recognising the body was the only way to tell where you were.

<!-- Rows asserted against the view list by TestDocsViewsTableListsEveryView in internal/tui. -->
| View | Opened by | Shows |
| --- | --- | --- |
| projects | `W`, and every session starts here | every project this tenant has, with its colour and icon |
| board | `enter` on a project | one project's tasks in columns, one column per workflow state |
| project | `w` | one project's attributes, the workflow it runs on, and its custom field definitions |
| detail | `enter` on a card | one task in full: body, custom fields, subtasks, dependencies, artifacts, comments |
| activity | `v` | the live event tail, filtered by the audit grammar |
| history | `H` | the stored audit log for the selected task, the open project or the tenant |
| statistics | `S` | the same figures as `tix stats`, for the open project or the whole tenant |
| tenant | `T` | the tenant in force, its domains and its members, and a key to switch this session |
| settings | `,` | display preferences, and what this session is connected to |
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
┃ infra-13 · P2 · @me       │ infra-8 · P2
```

The bar down the left edge carries the workflow category as a colour and the selection as a shape: the
selected card's bar is heavier. That is deliberate rather than decorative, since a terminal getting no
colour has nothing else left to say which card the keys will act on.

The priority is `P1` through `P5`, highest to lowest, the same digits `tix task list` prints. The rest of
the line is markers. The legend is in `?` and is short on purpose, so it still fits a narrow terminal:

<!-- Rows asserted against CardLegend by TestDocsMarkerTableMatchesTheLegend in internal/tui. -->
| Marker | Means |
| --- | --- |
| `@` | claimed, by somebody |
| `@me` | claimed by you |
| `!` | blocked |
| `+` | has dependencies |
| `†` | deleted, revealed by `is:deleted` |

`@me` is separate from `@` because "is that me" is the first question anybody asks of a claimed card on a
shared board, and a single marker left it answerable only by opening the task.

Columns follow the workflow's declared order and are coloured by the category a state belongs to, not by
its name, since workflow states are user defined. Nothing is conveyed by colour alone.

A column is as wide as it needs to be and as tall as what it holds. The width is shared out by what each
column has to show, so the column holding eight tasks with long titles is wider than the one whose entire
content is the word `empty`, and a column with nothing in it is a short box rather than a screenful of
blank frame. Every column keeps a floor wide enough for its own heading.

**The footer is a promise.** It advertises only the keys the selected task can accept: `c` while the task
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

A scheme lists only the actions it moves. Everything else keeps its default keys, which is what makes it
impossible for a scheme to leave an action unbound.

### The default bindings

<!-- Keys asserted against DefaultKeyMap by TestDocsBindingsTableNamesEveryAction in internal/tui. -->
| Keys | Action | Where |
| --- | --- | --- |
| `↑`/`k`, `↓`/`j` | move the selection | everywhere |
| `←`/`h`, `→`/`l` | previous, next column | board; steps a value on settings, moves the cursor inside a text field |
| `tab`, `shift+tab` | next, previous field | wherever a form is open |
| `g`/`home`, `G`/`end` | first, last | lists |
| `enter` | open, or apply an input; inserts a newline inside a multi-line field | everywhere |
| `ctrl+s` | apply an input, including from a multi-line field | wherever one is open |
| `esc`/`backspace` | back one level | everywhere |
| `/` | filter | board, activity; cycles the window on statistics |
| `C` | clear the filter | board, activity |
| `c` | claim | board, detail |
| `x` | release | board, detail |
| `R` | renew the lease | board, detail |
| `N` | claim next from the queue | board, detail |
| `t` | transition | board, detail |
| `n` | new task, new project on the project list, new field or domain or member | board, detail, projects, project, tenant |
| `e` | edit the whole task on one form; edits the project or the tenant on their own screens | board, detail, project, tenant |
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
| `X` | delete the task or the selected comment; removes the selected domain or member | board, detail, project, tenant |
| `y` | agree to a confirmation | wherever one is open |
| `w` | project setup | everywhere |
| `f` | custom field definitions | project |
| `W`, `,`, `v`, `H`, `S`, `T` | projects, settings, activity, history, statistics, tenant | everywhere |
| `ctrl+k`, `:` | the command palette: every action by name | everywhere |
| `r` | refresh | everywhere |
| `?` | help | everywhere |
| `q` | back, or quit at the top level | everywhere |
| `ctrl+c` | interrupt, exit 130 | everywhere |

`S` is capital because lowercase `s` is spoken for on the board, and a capital is what the other
cross-view keys already use when their letter is taken.

### The command palette

Forty-nine bindings is more than anyone holds in their head, and the letters ran out: `W` lists projects
while `w` opens project setup, `H` reads history while `v` draws the live tail, `L` picks a tag, `O` records
an artifact, `N` claims the next task. The capitals were handed out by elimination rather than by meaning.

`ctrl+k` opens every action by name. The footer says so in every view, which is the point: the keys were
always there and nothing advertised them.

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

**Every entry shows its own key.** That is what makes the palette a way out of itself: a reader who reaches
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

**The entries are the actions, not a menu beside them.** Selecting one makes the same call the key press
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

**Nothing is listed that would be refused.** The entries are filtered by the same authority the `?` overlay
is filtered by, so a reader who may not subscribe to events is not offered `v`, and a reader who may not
claim is not offered `c`. An action that acts on the selected task is left out while nothing is selected,
and a new task is left out while no project is open, rather than listed in a form that cannot be pressed.

The order is fixed: the cross-view keys in the order `?` prints them, then the actions on the selection.
Narrowing never reorders what is left, because a row that moves as the query grows is a row that is not
under `enter` where the reader left it.

Two things are deliberately not filtered. Lease state is one: the palette offers `x` on a task this session
does not hold, because the `?` overlay documents every binding a view has regardless of the lease, and the
palette's list and the overlay's list are held equal by a test. The palette itself is the other: `?`
documents `ctrl+k`, and the palette does not offer the door it is already standing in.

`ctrl+k` and `:` both open it. Two schemes have already spent one of the two, so `vim` and `helix` keep `:`
for the filter, which is what their own `:` opens onto, and `nano` keeps `ctrl+k` for the cut and takes `:`.
`g` was not available: it is bound to "first", and `gg` is muscle memory for exactly the readers who would
reach for a list like this.

Every action that acts on a task is offered wherever a task is selected, so the board and the detail view
run the same set rather than drifting apart. Two of them need a sub-item only the detail view has a cursor
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

`enter` inserts a newline there, which is the whole point of the field, so it cannot also apply. `ctrl+s`
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

`,` opens five preferences and a statement of what this session is connected to. `↑`/`↓` move between
them and then scroll the body, because on a short terminal the session facts below the settings were
otherwise unreachable: four settings meant four presses and nothing moved after the fourth. `←`/`→`, or
`enter`, step the selected value, wrapping at both ends.

| Row | Configuration key | Values |
| --- | --- | --- |
| keys | `tui.keymap` | `default`, `vim`, `emacs`, `nano`, `helix` |
| time format | `output.time_format` | `iso`, `rfc3339`, `short`, `us`, `relative` |
| timezone | `output.timezone` | `local`, `UTC`, and a selection of named zones |
| colour | `output.color` | `auto`, `always`, `never` |
| motion | `tui.motion` | `on`, `off` |

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

`is:deleted` reveals the deleted cards the board otherwise leaves out, each marked `†`. It is the term the
shared parser already had, so it is the terminal's equivalent of `tix task ls --include-deleted`, and `u`
on one of those cards is what brings it back.

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
cannot grow without limit, and the view says so rather than silently dropping what it held. It follows the
newest event until the selection is pulled back to read an older one, and then stays where it was put.

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

It is not the activity tail, and folding it in would have been wrong twice over. Activity is a live
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

## The tenant screen

`T` reads the tenant this session is pinned to and states it: its key, its name, the theme it presents
itself with, the tenants this session can see, the hostnames that resolve to it and the actors who belong
to it. Each of those is a separate read under a separate authority, so a reader refused one gets the rest
of the screen and a line saying what is missing, rather than a screen that fails to open over a listing
they were never going to see.

One cursor runs over the domains and the members together, because `X` removes either and a removal needs
a named subject rather than a typed identifier. The confirmation says the hostname or the handle out loud.

`e` edits the tenant: the theme from the palettes the build carries, including the word for having none,
and the name from the single-line input seeded with the name it would replace. `n` asks whether a domain
or a member is being added. A domain takes its hostname at the prompt and is added carrying no certificate
of its own, because a certificate is a pair of paths on the server that no control here can gather;
`tix domain add` takes them. A member is picked from the tenant's own directory, read at the keystroke
rather than held for the session, with one of the three roles beside it.

### Switching tenant

The same screen takes a key to switch this session to another tenant. There is no list of other tenants to
pick from, and that is not an omission: the listing above it is the tenants this session can see, which is
its own, because an actor belongs to one tenant and both listing tenants and reading one are scoped to it.
From inside `default` a tenant named `acme` and a tenant that was never created are the same answer. The
key is checked the way `tix tenant use` checks it, by opening a connection pinned to it and asking who you
are there.

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
| project | the actor may read projects, or read workflows |
| detail | always in practice: resolving an identifier to a handle needs no scope |
| statistics | the actor may read tasks |
| activity | the actor may subscribe to events |
| history | the actor may read the audit log |
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
- **The tenant switch is read only.** Which database or server a key resolves against is configuration, and
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
a reason as a gap when the binding ought to exist. As of this writing there are 32 such gaps against 90
operations, and a test asserts the exact number so it cannot grow quietly and cannot be mistaken for
zero. The number is coming down, so treat `internal/capability/registry.go` as the live answer rather
than any list here:

```sh
grep 'gap(SurfaceTUI' internal/capability/registry.go
go test ./internal/capability/...
```

Every one of the 32 now has one of two shapes, which is worth knowing before you try:

- **Administration.** Users, tokens, credentials, webhooks and their deliveries, retention and the server
  itself have no terminal screen at all, and neither the live connections a server holds nor the servers
  registered against the store can be listed here, and creating or destroying a tenant is not something a
  session pinned to one tenant can do. That is twenty-four of the count.
- **Bulk and data movement.** Import, export, bundles and external sync are command line and API only.
  That is the other eight.

Four shapes that were on this list have closed, and how they closed is the part worth keeping. Working
on definitions rather than instances: the project screen edits a project, defines and removes custom
fields, and renders the workflow it runs on. Input a single line of text cannot gather: a prompt now hands
over to a form whose every field picks from the values the operation accepts, which is how the assignee
and the artifact arrived, and what the terminal still cannot gather is recorded as a limitation on the
binding rather than as a gap, so `artifact.put` is bound and says in the same breath that a payload, a
content type and an inline blob belong to `tix artifact put` and the API. Listings needing a selectable
sub-item: the comment thread, the tag list, the dependency list, the actor directory and the deleted cards
`is:deleted` reveals all have a cursor over them now. And the tenant itself: the screen that stated only
which tenant a session was pinned to now reads the tenant, the tenants the session can see, its domains
and its memberships, and edits all three, which closed nine.

Seven absences are not gaps and will not close. Closing the service is process lifecycle; hostname
resolution runs in the server request path; lease sweeping is a background loop; signing in and out
bracket the program rather than happening inside it, since the interface opens on a session that already
exists and ending it would revoke the credential the running program is using; and the two workflow writes
cannot be served here rather than have not been yet. A workflow is a state machine, which is a graph, and
neither a line of text nor a form over fixed lists gathers one, so the project screen names
`tix workflow put` instead. A deletion would only ever be offered on the workflow the open project runs
on, which is the one workflow the service refuses to delete, and an entry leading to a refusal is worse
than no entry.

What is bound is daily work, and that is asserted rather than hoped for: a test names the operations a
board has to reach and fails if any of them loses its terminal binding.

The honest summary is that the terminal interface is where you work on tasks, and the command line is
where you configure the thing you are working in. Every gap names a reason, and `tix <command> --help` or
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

At the floor itself the footer keeps the palette hint and loses the view's own keys, which is the trade it is
ordered for:

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
