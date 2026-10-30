# Design

## The property being delivered

An agent holding a lease can trust that its results will not be overwritten by a
dead predecessor. "Results" is the artifact list. That is the narrowest
statement that covers the reported defect, and narrowing it is the point: every
write this change does *not* guard is one a human does on a live task every day.

## Why not one rule for all three paths

The first design proposed for this was "refuse a write when a live lease belongs
to a different actor, needing no contract change". It is wrong, and the reason
is worth recording because it is attractive and cheap.

tix is one queue shared by humans and agents. A person commenting on a task an
agent currently holds is not an edge case; it is how the product is used. A
blanket "different actor holds the lease ⇒ refuse" refuses them. That is a worse
product than the defect, and it would have been introduced by a rule chosen for
costing nothing rather than for being right.

So each path is decided on what it is for.

| Path | Guard | What it is for |
| --- | --- | --- |
| `PutArtifact` | `requireLeaseToken` | the holder's work product |
| `AddComment` | none | discussion, often human-to-agent on a claimed task |
| `UpdateTask` | none | shared triage metadata, already guarded by `Version` |

## PutArtifact: the same rule transitions already use

`requireLeaseToken`'s semantics are reused unchanged, and reused rather than
reimplemented because the spec requires exactly one guard to make this
judgement — the two-implementation version of this is already recorded in
`claim-lease` as how the two came to disagree.

- No live lease and no token: allowed. This is what makes lazy expiry
  authoritative, and it means a worker writing to a task nobody holds is not
  obstructed.
- No live lease, token offered: refused as stale. The caller believes it holds a
  lease it does not, and silently accepting is how it goes on writing to work
  that is free again.
- Live lease, no token or a non-matching token: refused as lease expiry. The
  check is `RenewLease` against the offered token, so it proves the token holds
  the lease rather than comparing strings.

**The browser consequence is not new.** `internal/web` has recorded exemptions
for `ClaimTask`, `RenewLease` and `ReleaseLease` saying a lease token cannot be
minted for a browser session. `TransitionTask` has a web binding all the same,
and `internal/web/tasks.go` calls it with no `LeaseToken` — so the browser
already cannot move a claimed task, and gets `lease_expired` when it tries. The
artifact list now behaves the same way. This is the decisive argument for
guarding artifacts fully rather than looking for something weaker: the product
rule "a claimed task's state belongs to its holder" is already shipped and
already has this cost, and extending it to outputs makes the lease coherent
instead of making it stricter.

What keeps that affordable is that commenting is left open. A person who wants
to say something about a task an agent holds has a channel that always works; a
person who wants to attach a file to it has to wait, reclaim, or say it in a
comment. If that trade ever proves wrong, the repair is a `task:reclaim`-style
scope that permits an artifact write without a token, not a weakening of the
guard.

### Why not a weaker artifact guard

A stale-token-only rule — refuse an offered token that is not live, allow a
caller that offers none — was considered and rejected. It protects nothing: a
zombie running `tix artifact put` passes no token today, so it would still
write. `docs/agents.md` is right that the zombie must not need to cooperate, and
a guard that only catches a worker honest enough to present its credentials
catches no zombies at all.

## Comments: nothing to defend

Comments are append-only. `AddComment` inserts a row; no path replaces or
reorders one, so a zombie comment cannot displace the holder's. It is noise in a
discussion thread, attributed to its author and timestamped, which is what a
discussion thread is for.

Adding a token here would also cost more than anywhere else: `AddComment(ctx,
ref, body string)` carries no input struct, so it would mean changing a
`core.Service` signature — the frozen contract — plus `internal/client` and
every surface, for a guard that is cooperative-only on a path where nothing is
overwritten. `0.15.0` already shipped one breaking change this week. Declined.

## UpdateTask: Version is the right control, and a lease is the wrong one

Field edits are the arguable case, and the argument that settles it is that tix
already ships a concurrency control for them. `UpdateTaskInput.Version` refuses
an edit built on a stale read. That catches a human racing a human and a human
racing an agent, in both directions, which is the actual shape of a field
collision; a lease token catches only "not the holder", which is not the same
question and is wrong about the common case.

Gating field edits on the lease would mean the browser could not re-prioritise
or re-assign a claimed task — and re-prioritising the queue while agents work it
is the thing a human operator is *for*. Custom fields are the part of
`UpdateTask` closest to being work product, and the lease-bound way to write
them already exists: `TransitionInput.CustomFields`, under the guard, on the
transition the holder makes anyway.

## Where the token comes from

`tix artifact put --lease-token T`, matching `tix task mv --lease-token T`
rather than `claim renew --token T`, because this is a task write that happens
to need a lease, not a lease operation. `tix claim exec` and `claim release
--result` already carry results through the guarded path and need no change,
which is the existing safe route and the reason the flag is a fallback rather
than the primary interface: an agent that uses `claim exec` never types a token.
