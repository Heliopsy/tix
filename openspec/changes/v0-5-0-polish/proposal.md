# Make a release someone can sit in front of

## Why

v0.4.0 shipped themes, statistics, shell completion and self-update. Using it for an afternoon turned up a
set of defects that share a shape: each one was covered by a test that asserted the code returned the right
value, and none of them asserted the thing a person actually experiences.

The accent was tested by checking what `brandFor` returned, so a dark scheme that never picked up the
tenant colour shipped green. The tick was tested by checking the row came back done, so a tick that
repainted eighteen rows and threw the scroll position away shipped too. The external sync is tested for the
records it writes, so a failed run that erases the watermark, an audit entry that records no cursor, an
unreachable source reported as "internal error", and a result printed as a raw Go struct all shipped
together.

The demo seed has the same problem from the other side. It creates tasks with titles and nothing else, so
every screenshot in the README and on the site shows a product with no descriptions, no tags, no due dates
and no history, which is the opposite of what the screens are for.

## What Changes

- **A tick animates one checkbox.** The pop and draw were attached to the done state rather than to the
  act of ticking, so every already-done checkbox replayed them on every render: a load, a filter change or
  a column switch sent a wave of ticks down the list, none of which corresponded to anything the reader had
  done.
- **An unthemed tenant gets the product default**, and the documentation says so. The behaviour changed
  when the default became the logo's green; the theming page still described the old derived colour, and
  pointed at a `tix tenant set` command that does not exist.
- **External sync stops losing and misreporting its own state.** Four defects, one per requirement below.
- **`tix demo seed` produces data worth looking at**: backdated history, descriptions, tags, due dates,
  assignees and custom fields, so the screens demonstrate the product rather than an empty shell.

## Impact

- `internal/web/assets/app.css`, `docs/theming.md`, `cmd/tenant.go`.
- `internal/service/sync.go` and `internal/output` gain a `SyncResult` renderer; a new error kind
  distinguishes an upstream failure from an internal one.
- `cmd/demo.go` and its fixtures.
- Screenshots regenerate from the new seed.
