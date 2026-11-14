# Design

## Match the claim on the actor, not the token

A lease in tix belongs to an actor. `tasks.claimed_by_actor_id` names the holder and
`tasks.lease_token` holds a secret minted at claim time; nothing on the task records which API
token made the claim. Matching on the specific token would need a new column, and every place that
writes a lease (claim, claim next, renew, reclaim, the sweeper, sync and snapshot import) to fill it
or clear it.

The gate therefore asks whether the token's actor holds a live claim on the task. That is the same
question the rest of the system already answers about a claim, and it is what history records: an
audit entry names the actor and the token. The consequence is that two tokens acting as the same
actor share claims. The recommendation for agents follows from that: one token per agent, each
acting as its own agent actor (`tix token create NAME --actor HANDLE`), so a claim, the audit trail
and the activity feed name the agent that did the work. Tokens of one person that all act as that
person cannot be told apart by a claim, and the docs say so.

The lease token stays the guard against a zombie worker. A move of a claimed task still needs the
lease token, as before; `require_claim` adds that the task must be claimed by the caller's actor at
all. A field edit needs the claim but not the lease token, because a field edit was never
lease-bound and the surfaces that edit fields do not carry one.

## Where the rule is read

The service reads the token when it judges a gated change, from `Actor.TokenID`, inside the same
transaction as the change. It skips the read when the caller is not a token or already holds the
claim. Reading it there rather than copying the flag onto the actor at sign-in keeps every
authenticator and every transport right without each having to carry a new field, and a token the
store no longer holds simply cannot have signed in.

## What is gated

Gated: `TransitionTask` and so `TransitionRoute` and every hop of it, `UpdateTask`, `CheckTaskItem`,
`AddTag`, `RemoveTag`, `MoveTask` and `DeleteTask`. A cascading delete is judged per task, so a
token cannot delete an unclaimed subtask by deleting its parent. `BulkTasks` already runs each task
through the single-task method, so every task is judged on its own and a refusal is that task's
outcome; the rest of the batch stands.

A claim that starts (`--start`) moves the task inside the claim's own transaction, after the claim
row is written, so the gate already sees the claim and the one step works.

Not gated: claiming, renewing and releasing a claim (the claim is the point), creating and copying
tasks, comments, reactions, artifacts (already lease-bound), stars and snoozes, dependencies, and
restoring or purging a task from the trash, which no claim can be taken on.

## Dry runs

`tix task mv --dry-run` and `dry_run` on a transition or route run every check of the real move in
a rolled-back transaction, so the gate refuses them with the same error. The client-side plans of
`tix task edit --dry-run` and `tix task rm --dry-run` judge only that the task exists, as they do for
every other permission today, and are left as they are.

## Rotation and updates

There is no token update path. The flag is chosen when a token is minted and `RotateToken` copies
it to the replacement. Turning it on for an existing token means issuing a new one and revoking the
old, which is what changing a token's scopes already means.
