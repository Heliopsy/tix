# Colour follows category, and cancelled work is not done

## Why

A workflow state carries a `Category`, and category is what every surface derives colour, grouping and
reporting from. The vocabulary was three words — `todo`, `in_progress`, `done` — and the shipped workflow
has five states, so two of them were forced into a category that misdescribes them:

- `blocked` was categorised `todo`. A task nobody can move read as ordinary work waiting its turn, in the
  same colour and the same statistics row as a task that simply has not been picked up.
- `cancelled` was categorised `done`. Abandoned work read as finished work: green on the board, green in
  the browser, and counted in the `done` row of *where the work is*.

The second is not cosmetic. An operator reading that row was reading a number with two meanings in it,
and there was no way to separate them, because the distinction did not exist in the data.

The alternative considered was a free per-state `Color` field. It was rejected: a colour that each tenant
picks means nothing across deployments, degrades to nothing on a monochrome terminal, and turns a shared
vocabulary into per-workflow decoration. A widened fixed vocabulary keeps one meaning per hue everywhere,
and keeps the word as the fallback where there is no hue. A "category default plus per-state override"
hybrid was rejected for the same reason: it is the free-colour option with extra steps.

## What Changes

- The `StateCategory` vocabulary gains `blocked`, `waiting` and `cancelled`, making six. `waiting` is
  added alongside `blocked` so that stalled-on-us and stalled-on-someone-else are separable, which is the
  distinction a board actually wants and the one a three-word vocabulary could not express.
- `core.WorkflowInput` validates the category it is given. A workflow naming a word outside the
  vocabulary is refused, and the refusal lists all six. Nothing validated categories before, so a typo
  used to store cleanly and then colour nothing and count into no row.
- The shipped workflow recategorises its `blocked` and `cancelled` states.
- **Stored workflows are not migrated.** A tenant's workflow is the tenant's statement about its own
  states, including one that deliberately says `category: done` on a cancelled state. It keeps reporting
  that way. Only what tix itself ships changes.
- The statistics breakdown reports all six categories, so `done` counts only finished work and
  `cancelled` gets a row of its own.
- Each category draws in its own colour on the board, on the command line and in the browser, with
  cancelled muted rather than green and blocked in the colour that already means urgent.

## Impact

- `internal/core`: the vocabulary, its validation, and the accepted-set message.
- `internal/service`: the shipped workflow, and the category order the statistics breakdown reports in.
- `internal/sshd`: the seeded workflow, which mirrors the shipped one.
- `internal/output`, `internal/tui`, `internal/web`: the three colour surfaces and the browser stylesheet.
- `docs/workflows.md`, `docs/statistics.md`, `docs/tui.md`.

Not changed, and deliberately: `Completed`, the per-day series, the lead times, the leaderboard and the
`completed_at` column. All of them read terminal-ness, which is the `Terminal` flag and has never been
category. A cancelled task is still terminal, still carries a completion timestamp and still releases its
dependents. Recategorising it moves it between rows of one breakdown and changes no throughput figure.
