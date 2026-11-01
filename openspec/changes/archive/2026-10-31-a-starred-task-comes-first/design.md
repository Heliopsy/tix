# Design

## Personal, not shared

A star is a property of the reader, the same way the list/board switch and the column picker
are. A shared "pinned" flag on the task would let one person reorder everybody's queue, and on
a queue shared with agents that is the wrong default: a pin set by a person would move work up
an agent's listing that the agent has no reason to prefer.

So the row is keyed by `(tenant_id, actor_id, task_id)`. The actor is whatever the service
layer already resolved as "who is asking": the actor behind the session, token or SSH key. One
user holds one actor per tenant today, so a person's stars follow them across every surface of
that tenant and do not leak into another tenant they belong to. An agent token is its own
actor and has its own (usually empty) set.

Stored server-side rather than in a cookie, because the request is that the star follows the
reader into the command line and the terminal interface, which no cookie reaches.

## Authority

Starring changes nothing anybody else sees in a listing, so it is gated on reading the task,
not on writing it: a new action `task.star` requiring `task:read`, project-confinable like
every other per-task action. A viewer can star. A project-pinned token can star only inside
its project, and starring a task the caller cannot read is a not-found, exactly like reading
it.

## Mutation, audit and events

Star and unstar write the row, an audit entry (`task.star`, `task.unstar`) and an outbox event
(`task.starred`, `task.unstarred`) in one transaction, like every other mutation.

Both are idempotent. Starring a starred task, or unstarring one that is not starred, succeeds
and writes nothing: no row, no audit entry, no event. An event for a change that did not
happen would make a webhook consumer count stars that never moved.

The event is tenant-visible, like the audit entry. A star is personal, not private: the audit
log already names who did what, and hiding the event would mean filtering the stream per
actor, which nothing else in the stream does. Its payload carries the task ref and the actor,
so a consumer can tell whose star it was. Because the type sits under `task.*`, the live
screens already subscribed to task events refresh on it, which is what makes a star set in one
tab reorder the reader's other open tabs without new machinery. Another reader's screen also
refreshes and draws the same order, which costs one fragment fetch and is accepted.

## Ordering and the cursor

Starred-first is a grouping, not a sort field. Under every sort the reader picks, their starred
tasks come first and the chosen order holds inside each group. There is no opt-out in this
change: the sort language takes a field and a direction and has no slot for a modifier, the
grouping is empty for a reader who stars nothing, and `is:starred` / `is:unstarred` already
give either group on its own.

The store joins `task_stars` for the reader (a `LEFT JOIN` on tenant, task and actor) and
orders by a rank expression first:

- ascending: `CASE WHEN task_stars.task_id IS NULL THEN 1 ELSE 0 END`
- descending: `CASE WHEN task_stars.task_id IS NULL THEN 0 ELSE 1 END`

The rank flips with the direction so the whole ordering stays one direction, which is what
lets the keyset predicate remain a single row-value comparison:
`(rank, key1[, key2], id) > (?, ?[, ?], ?)` (or `<` descending). Mixing an always-descending
star key with an ascending sort key would need an expanded OR-chain predicate per direction,
which is the kind of predicate that skips rows when one branch is wrong.

The cursor gains one additive field, `st` (the starred state of the last row on the page).
An older cursor without it decodes as "last row unstarred", which resumes inside the unstarred
group; that is the correct position for every cursor issued before stars existed, since
nobody had any.

When the listing has no reader (export, internal sweeps, workflow checks), no join and no rank
are emitted, and the order is exactly what it was.

### Starring mid-walk

A star toggled while somebody is paging moves that task between groups, the same way changing
a task's priority moves it under a priority sort. The cursor stays valid and the walk
continues; the toggled task may be seen twice (unstarred after it was listed in the starred
group) or not at all (starred after the walk passed the starred group). Rows that did not
change are never skipped or repeated. This is the guarantee the existing keyset requirement
already gives ("no duplicates and no gaps among unchanged rows"), and it is stated again for
stars so it is a decision rather than an accident.

## Which reads carry `starred`

`ListTasks` and `GetTask` set `starred` for the reader. Other responses that happen to contain
a task (a create, an update, a claim) do not look it up and report it unset; the field is
`omitempty`, so an absent value means "not starred, or not asked". This keeps audit snapshots
and exported tasks free of a per-reader field.

## Deletion

Hard deleting a task (and anything that cascades to it: deleting its project, a replace-mode
import, the tenant) removes every star on it through the foreign key. Soft deletion keeps the
row: a soft-deleted task is absent from every listing that does not ask for deleted tasks, so
its star has no effect, and a restore is an undo that should bring back what was there.
Removing an actor removes that actor's stars.

## Snapshots

Stars do not travel. A snapshot carries no actors, so a star could only be imported by keeping
an actor identifier the target may not have, or by handing it to the operator running the
import, who would then inherit everybody's stars. Both are worse than leaving a personal view
preference behind, which is what view preferences already do.

## Surfaces

| Surface | Star | Unstar | Marker |
| --- | --- | --- | --- |
| CLI | `tix task star REF...` | `tix task unstar REF...` | an asterisk before the ref in the `task ls` and `task show` tables; `"starred": true` in JSON |
| HTTP | `PUT /v1/tasks/{ref}/star` | `DELETE /v1/tasks/{ref}/star` | `starred` on the task |
| Web | `POST /tasks/{ref}/star` | `POST /tasks/{ref}/star/remove` | a toggle button, `aria-pressed`, on list rows, board cards and the task screen |
| TUI | `*` toggles | `*` toggles | an asterisk before the ref on a card; starred lead each column |
