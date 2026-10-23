# Screens forbid storage

## Why

`renderStatus` is the one writer for every browser screen, and it set exactly
two headers: a content type and `nosniff`. No `Cache-Control`, on any page,
including an authenticated task list.

That is two faults in one gap.

**Staleness.** A response carrying neither a freshness lifetime nor a validator
may be cached heuristically, so a reader can be handed a task list that was
already wrong by a screen whose only job is to say what the state is now. This
surfaced while diagnosing a separate defect, where a transition made in the SSH
terminal did not appear in an open browser tab.

**Disclosure.** An authenticated board with nothing forbidding storage is one
any shared cache may keep, and one the next person at that machine may pull out
of the browser's own cache after a logout that cleared the session but not the
pages the session rendered.

The asset path was already careful here: `assets.go` carries a strong ETag and
a short `max-age`, with a comment explaining the upgrade that made it
necessary. The screens got none of that attention.

## What Changes

**Every rendered screen carries `Cache-Control: no-store`.**

- One header, set in `renderStatus`, so it covers every screen including the
  error screen and a form redrawn after a refusal. A refused form is exactly
  the response most likely to carry what the user just typed.
- `no-store` rather than `no-cache`, because the objection is to the response
  being written down at all, not to it being reused without revalidation.
- Assets are untouched. They are the opposite case: their URLs never change
  between releases, so they keep their validator and lifetime.

## Trade-off, stated

`no-store` costs the back-forward cache, so a back-button restore re-fetches
rather than painting instantly. That is a real loss, and it is the right trade
for a page carrying another tenant's work.

## What this does not do

It does not make any screen update itself. The staleness this removes is the
browser's own cache. A screen that does not subscribe to the event stream is a
separate defect with a separate fix.
