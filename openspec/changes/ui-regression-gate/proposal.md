# UI regression gate

## Why

Layout regressions kept merging. The strict layout check in `scripts/screenshots/shoot.mjs` and the
tests that drive a real browser ran only when somebody remembered `just screenshots --strict`, and the
demo seed it shot was too tidy to break anything: a long Markdown description, a long tag or a pasted
URL never reached it. A project row running over its own actions on a phone sat on `main` unnoticed.

## What Changes

- A **UI regression** CI job on every pull request and push to `main`: the strict check over every
  route at phone and Full HD in both themes on the hostile seed, the task row checks, the
  `*InABrowser` Go tests and the JavaScript suite. `just ui-regression` runs the same steps.
- The strict check gains checks for every screen: no sideways page scroll, no content spilling out of
  a box that neither clips nor scrolls, every `<time>` with a `datetime`, no raw Go time in visible
  text or a tooltip.
- `--hostile` adds sixty-four tasks with hostile titles, tags and Markdown in one project, a long
  history, long user, project and token names and many expiring tokens.
- `scripts/screenshots/known.json` lists findings already owned elsewhere; a stale entry fails.

## Capabilities

### Modified Capabilities

- `web-ui`: every screen holds its layout and shows readable times whatever the content.

## Impact

`.github/workflows/ci.yaml`, `justfile`, `scripts/screenshots/`, `docs/testing.md`, `AGENTS.md`.
