# Design

## One cookie, three positions

The view stays in `tix_task_view`. Lanes are a third value, `lanes`, beside `board`, and the list
still stores nothing. Keeping it in the cookie rather than in browser storage is what lets the server
render the right view on the first byte: a choice the server cannot read is a choice it can only
draw after a flash of the wrong one. Any value this build does not know still reads as the list.

## Which tasks, which lanes

Lanes draw the same page the list and the board draw. The tasks come from one `ListTasks` call over
one assembled filter, so a lane can never hold a task the list would not have shown on the same page.

Lanes are ordered by the project order the screen already uses for the quick-add menu and the
visibility choices, which is the order `ListProjects` answers in. Within a lane, tasks keep the
listing's sort.

Empty lanes were the real decision. The options:

- **A lane for every project the listing may draw from.** Honest about the selection, but a tenant
  with hundreds of projects would draw hundreds of empty columns, most of them because their tasks
  are on another page.
- **A lane only for projects with a task on this page.** Bounded by the page size, but a reader who
  typed `project:ops` and got nothing would see no lane at all and no sign the filter was heard.
- **Chosen: the second, plus a lane for every project a `project:` term names.** A named project is
  bounded by what the reader typed, and naming it is asking for it. An empty named lane says so.

A task whose project is not among the screen's projects (an archived project's task can still be
listed) is named in a notice under the lanes rather than dropped, for the board's reason.

## The status badge

The lane is the project, so the status has to be on the card. The badge is the existing status pill
with the state's label, coloured by the state's category class (`cat-*`) rather than by its key. A
tenant names its own states; only the category is a fixed vocabulary, so it is the one thing a
colour can mean everywhere. A state with no declared category takes the one statistics derives:
terminal is done, the initial state is todo, anything else is in progress.

## The tick box

Ticking means "this is finished", and that has to be one unambiguous move or it is not a tick box.

The box is offered when, from the task's current state in its own project's workflow, `core.Routes`
yields exactly one route ending in a terminal state whose category is done. Zero such routes means
there is nothing to finish into. Two means either two done states or two equally short ways into
one, and picking for the reader would write a history they did not choose.

The route may pass through other states (the built-in workflow reaches done from todo only through
doing). The box's accessible name and tooltip spell the whole journey out, the way every multi-hop
control here names its intermediate states before it is applied.

The box is also withheld, in favour of a link to the task screen, when the reader could not complete
the move from a lane:

- any step requires a comment, since a tick box has nowhere to type one;
- any step requires a scope this reader does not hold, or the reader cannot transition tasks at all.

The link was chosen over hiding the control outright. A lane where some tasks have a box and others
have nothing reads as broken; a hollow, dashed mark that opens the task and says why in its label is
the same shape doing a different, stated thing. The service still checks scope and comment on the
post, so the affordance is not the only protection.

The box posts `route=<value>` to the existing `POST /tasks/{ref}/transition`, with CSRF and `next`
set to this listing, so no new route or service method exists. With htmx it swaps only the card, the
way the list's tick swaps only its row, so the lane does not lose its scroll position. Without
JavaScript it is an ordinary form post that comes back to the same listing.

Already finished tasks (any terminal state) render ticked and inert. Unticking is out of scope: the
list's tick box reopens to the workflow's initial state, but in a lane the reverse of a multi-hop
completion is not obviously one move, and guessing would again write a history nobody chose.

## Finished work stays on the lanes

The list narrows to open work by default. Lanes are exempt, as the board is, because a lane's ticked
cards are part of what it draws and because Show completed is the control that hides them.

## Show completed, and why localStorage holds only that

Whether finished cards are shown in lanes is a presentation choice that no server rendering depends
on, so it does not need to cost a cookie on every request. It lives in `localStorage` under one key,
`tix.lanes.showCompleted`, holding `false` when finished cards are hidden; showing them, the default,
removes the key, so the default leaves nothing behind, the way the list position of the view switch
does.

`screens-forbid-storage` set `Cache-Control: no-store` on every screen because an authenticated page
written to disk is readable by the next person at that machine after the session is gone. Browser
storage is the same disk with no expiry and no logout hook. So nothing tenant-identifying goes
there: no project keys, no task references, no counts, no per-lane state. A boolean that says "this
browser prefers finished work hidden" discloses nothing about anybody's work. A per-lane collapse
state, which would have needed project keys, was deliberately not built for that reason.

`lanes.js` is loaded in the document head without `defer`, so it runs before the body is parsed and
sets a class on the root element. The stylesheet hides finished lane cards only under that class, so
a reader who hid them never sees them flash in, and because the class sits on the root rather than
on the lanes container it survives the live refresh and boosted navigation replacing the listing.
It is a few hundred bytes and touches nothing but the root class until the toggle is pressed.
Storage that throws, as it does in some private modes, reads as the default.

Without JavaScript the toggle is never revealed (it is rendered `hidden`) and finished cards are
shown, which is the default anyway.

## Layout

Lanes reuse the board's geometry: one row of fixed-height scrolling columns inside a frame that
scrolls sideways, never the page. At phone width one lane fills the screen and lanes snap as they
are swiped, the same gesture the board uses. They do not reuse the board's class names, because
`live.js` binds drag and drop to `.board` and `.column`, and a lane is not a drop target.

## Live refresh

The task screen's live fragment already re-renders the listing region from the same view, and the
view is read from the cookie the fragment request carries, so lanes refresh with no change to
`live.go` or `live.js`.
