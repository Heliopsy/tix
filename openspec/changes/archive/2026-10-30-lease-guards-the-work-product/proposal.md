# The lease guards the work product

## Why

`LeaseToken` exists on `core.TransitionInput` and nowhere else, and
`requireLeaseToken` has exactly one caller. `PutArtifact` takes no token and
makes no holder check, so the lease protects a task's *status* and does not
protect its *results*.

Demonstrated against a real binary: an agent whose lease has expired, and whose
task has since been re-claimed by somebody else, writes an artifact at exit 0.
The live holder's `result` is no longer the newest one on the task, so anything
reading "what did the worker find" reads the dead predecessor.

`docs/agents.md`, in a section titled *"Why a zombie cannot write"*, says the
token "is required on every write that touches a claimed task". That is the
property the product promises agents and it is not the property the code has.
The spec already asks for it: *Stale token rejection* says a stale token "SHALL
NOT be able to renew, transition, release, or write a result", and that "every
lease-bound write SHALL reach" the one guard. The artifact write does not reach
it.

## What Changes

**One guard is added, on `PutArtifact`, and the other two write paths are left
alone deliberately.** tix is one queue shared by humans and agents; a rule that
refused a person touching a task an agent holds would be worse than the defect
it fixed. So the three paths are decided separately rather than together.

- **`PutArtifact` requires the lease token, under exactly the rule
  `TransitionTask` already uses.** `core.ArtifactInput` gains `LeaseToken`, and
  the service calls the existing `requireLeaseToken`. A task carrying a live
  lease needs a matching token; a task whose lease has lapsed needs none, and
  refuses one that is offered. `tix artifact put` gains `--lease-token`.

  Artifacts are the holder's work product, and the result slot is the thing the
  user asked to have protected. This is not a new product rule: the browser
  already cannot transition a claimed task, for want of a token it cannot mint,
  and that consequence has shipped. Applying the same rule to the artifact list
  says, consistently, that *while a worker holds a task, the task's outputs
  belong to that worker*.

- **`AddComment` is unchanged, and takes no token.** A person commenting on a
  task an agent currently holds is the normal case and the premise of the
  product. Comments are append-only discussion: nothing is overwritten, so there
  is nothing for a token to defend. It also stays the one channel a browser
  session, which holds no lease, can always use on a claimed task — which is
  what makes guarding the artifact list affordable.

- **`UpdateTask` is unchanged, and takes no token.** Editing title, body,
  priority, assignee or due date while somebody works the task is live triage,
  not a write to the worker's output, and the browser and terminal interfaces do
  it with no lease to hold. `UpdateTaskInput` already carries `Version`, which
  is the right control for two editors colliding on a field and is a better one
  than a lease: it catches a human racing a human, which a lease never would.

## Impact

- `internal/core/input.go`: `ArtifactInput` gains `LeaseToken`.
- `internal/service/artifact.go`: `PutArtifact` calls `requireLeaseToken`.
- `cmd/artifact.go`: `--lease-token` on `artifact put`.
- `docs/agents.md`, `skills/tix/SKILL.md`: the zombie section becomes true, and
  says which writes the lease covers and which it deliberately does not.

**What breaks.** Attaching an artifact to a task that holds a live lease now
needs that lease's token. That is a behaviour break for:

- `tix artifact put REF` with no `--lease-token`, against a claimed task. Exit
  `4`, `lease_expired`. Against an unclaimed task, unchanged.
- `PUT /tasks/{ref}/artifacts` with no `lease_token` in the body, same.
- The browser and terminal interfaces, which cannot mint a token, can no longer
  attach an artifact to a claimed task. They never could transition one either.
  Both still comment on it.

No signature changes: `core.Service` is untouched, the field is additive and
`omitempty`, and `internal/client` marshals the input whole, so the wire carries
it with no transport change. A Go caller constructing `ArtifactInput` with field
names still compiles.
