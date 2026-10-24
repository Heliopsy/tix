# The first screen is the work

## Why

An adversarial review of the browser interface, run against a seeded
installation with ninety days of history, found that the task list opens on
finished work. Twenty-one of the first twenty-five rows were closed, and twenty
of those carried a red "overdue" badge.

The default sort is urgency, which is priority and then due date, and never
looks at status. It is the right order within a set of tasks and the wrong set
to be ordering. The badge is a separate fault with the same effect: a deadline
is compared to the present whatever the task's state, so a task finished late
stays overdue for the rest of its life, and a list of closed work comes up
almost entirely red.

The same review found two smaller faults with the same shape, where a rule
already written down was not the rule in force:

- A member refused at an administration screen was served the whole
  administration navigation on the refusal page, every entry of which would
  also refuse them. The function that decides this carries a doc comment
  saying an entry that leads to a refusal is worse than no entry; the path
  check simply ran before the scope check.
- The avatar on the user list takes the first letter of the first two words,
  so a seeded agent called "Atlas (build agent)" was drawn as "A(".

## What changes

- The task list shows unfinished work when nothing was asked for, and says so,
  with one link to show everything. The statuses come from the tenant's own
  workflows rather than from the built-in names.
- A finished task has no due state, so no badge.
- The administration navigation follows what the reader can reach, not the URL
  they typed.
- A monogram is made of letters.

A follow-up pass found the default being dropped by three of the listing's own
links, and the navigation fix stopping one scope short:

- The next-page control did not carry the request for finished work, so page
  two narrowed itself while still addressing a position in the unnarrowed
  walk, and the rows between were skipped with nothing on screen saying so.
- The board was handed the narrowed filter, so its terminal columns could hold
  no card and finishing one removed it from the board.
- The way off the default was written out as a fixed address, so following it
  dropped the reader's sort and page size.
- /workflows is gated on workflow:read, which the navigation's scope list did
  not hold, so a reader with that scope opened the screen and was handed the
  navigation that denies it exists. The list is now read against the
  capability registry rather than kept by hand.

## Impact

Affected specs: `web-ui`.

The narrowing is a default, not a filter: it applies only when the reader has
asked for nothing, it is visible on screen, and one link turns it off. An empty
narrowed list says "Nothing open" rather than "Nothing on the list", because a
project whose every task is finished is not a project with nothing in it.
