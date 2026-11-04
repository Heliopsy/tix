# Settings that read plainly, on every screen size

## Why

A phone review of Settings turned up nine small defects that share a cause: each control was right on
its own and wrong next to its neighbours. An (i) note opened over the row under it. A switch read
"Page" while off, which says neither what off is nor what on would be. The Database row showed every
signed-in member the server's database path. The calendar feed said Active twice and never said
where the address had gone. A placeholder read like a saved value. Inbox and Notifications put their
description outside the card every other section kept inside, and the existing notification channel
sat loose above an always-open form. Hover effects and keyboard shortcuts were offered on a phone
that has neither. The Advanced screens row still named the Configure and Data groups that one Admin
group replaced in 0.36.0.

## What Changes

- An (i) note opened beside a label, a setting's name or a heading flows in the page and pushes what
  follows down, instead of floating over the next row.
- Task opens in and Clicking a title become two-position segmented controls naming both positions
  (Page | Dialog, Open the task | Rename it). The forms still post to the same routes; a post naming a
  position sets it, and a post naming none still flips it, so an old form keeps working.
- The Database section and its jump link are drawn for a tenant administrator only, and the target is
  set at body size and breaks anywhere rather than as large monospace.
- The calendar feed's state row says when the feed was created and that its address is shown once, at
  creation or rotation, with Active stated once.
- Placeholders are italic in the muted colour everywhere, in the body font even in a monospace field,
  and the ones that read like a value are written as examples.
- Every Settings section has its heading, then its card; descriptions sit inside the card as a lead
  row; rows carry their own (i) tips and no section heading carries one. Existing notification
  channels are a card of rows, and Add a channel, like Issue a token, Enrol a key, Map a hostname and
  Add a member, is folded behind the same Add disclosure Users and Webhooks use.
- On a device whose main input is a finger, Hover effects, the Keyboard section and the
  keyboard hints elsewhere (the account menu's shortcuts entry, the palette's key hints and the
  create forms' Ctrl+Enter line) are not drawn.
- The Advanced screens row, its tip, the command palette's hints and the code comments name the
  Admin group.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: the settings screen's controls, the database target's audience, the calendar feed's state
  row, the information disclosure's placement, placeholders and touch-only devices.

## Impact

`internal/web` templates, `taskopen.go`, `session.go`, `palette.go`, `render.go`, `app.css`, the
Bulgarian catalogue and `docs/web-ui.md`. No service method, route, cookie or migration changes; the
two converted preferences keep their cookies and values.
