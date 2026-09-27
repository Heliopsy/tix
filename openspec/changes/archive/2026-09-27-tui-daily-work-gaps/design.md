# Design

## Why history is not the activity view

A previous review recommended closing `audit.list` by pointing it at the activity screen. The two look alike
and are not alike, and folding them would have produced an entry leading to a refusal, which is the thing
scope-aware navigation exists to prevent.

| | activity | history |
| --- | --- | --- |
| source | the live subscription | the stored audit log |
| scope | `event:subscribe` | `audit:read` |
| extent | the latest 200 events of this session | one page of everything ever recorded |
| before you connected | nothing | everything |
| `source:` terms | refused, an event carries none | answerable, an entry records it |

The scope line is the decisive one. A view is offered when the reader may perform one of its reads. Binding
`audit.list` to the activity view would have made the screen offered on either scope, which means a reader
holding only `event:subscribe` is shown a screen whose durable half the service refuses, and a reader
holding only `audit:read` is shown one whose live half never arrives. Two questions, two scopes, two views.

The subject follows what the reader is looking at, the way the project screen's subject does: the selected
card, the open project, or the tenant. One key, one meaning, wherever it is pressed.

## Why the board is the view of deleted tasks

A separate trash screen was the obvious answer and the wrong one. The board's filter bar already takes the
whole task filter grammar, `is:deleted` is already in it, and `taskFilter` already passes `IncludeDeleted`
through to the listing. A second screen would have been a second way to ask one question, and it would have
needed a read of its own in the registry, which `task.list` cannot supply twice.

What was actually missing was smaller and worse: a deleted card was drawn exactly like a live one. The
marker closes that, and the footer then offers the one action a deleted card accepts.

## Why the directory is a picker rather than a screen

`actor.list` records a listing. A screen that lists people and leads nowhere is navigation for its own sake.
The listing earns its place behind `A`, where it replaces a prompt that asked for a ULID.

The binding names the detail view rather than the board, and that is a gating decision rather than a
cosmetic one. `actor.list` needs no scope. Binding a scopeless read to the board would make the board
offered to every session, including one that may read no task on it, which is exactly the entry leading to a
refusal the gate exists to prevent. The detail view is already reachable without a scope, because resolving
an identifier to a handle needs none, so binding it there changes no gate.

## What the artifact form does not gather

`core.ArtifactInput` carries a kind, a name, a payload map, a content type and an inline blob. Only the kind
is required. The prompt gathers the name and the form gathers the kind; the other three are structured input
no fixed list of alternatives and no single line of text can hold.

That is recorded as a `Limitation` rather than left unsaid. A surface that serves half an operation while
the registry says it serves all of it is the same untruth as a missing binding, and the form itself says so
on screen so a reader who needs a payload learns it before recording an empty artifact rather than after.

## Keys

`H` for history, beside `v` for the live tail; lowercase `h` moves a column. `O` for the output a worker
records; lowercase `o` opens a new task under vim and helix. `u` for undoing a deletion, which is vim's own
undo and the reflex a reader arrives with. None of the three is bound by any shipped scheme, and all three
are in the collision set every view is validated against, so a rebinding that would make one mean two things
is refused rather than applied.
