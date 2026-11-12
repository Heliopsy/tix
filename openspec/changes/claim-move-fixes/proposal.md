# Claim and move fixes from the agent experiment

## Why

Fresh agents driving the CLI hit three things in a row. `tix task mv REF STATE --dry-run` approved states the
workflow does not have (`review`, `in-progress`, `bogus`) while the real move refused them with exit 2, so a
dry run told them nothing. Every agent claimed a task and then had to move it to `doing` in a second call,
and their first try at that call left out the lease token and failed with exit 4. And moving a task to
`done` under the lease token left the lease held, so each agent needed a third call to release it.

## What Changes

- A dry run of a transition is the transition, rolled back. `TransitionInput` and `RouteInput` take
  `dry_run`; the service runs every check the real move runs, inside the same transaction, and rolls it back.
  `tix task mv --dry-run` (with and without `--hops`) asks the service instead of only resolving the
  reference, so it refuses what the move refuses with the same error and exit code.
- A status that is not a state of the task's workflow is refused with the workflow's states listed in the
  message and in `details.states`, on a move, a route, a bulk move and a release with a status.
- `ClaimInput` and `ClaimNextInput` take `start` and `start_status`. A claim that starts moves the task, in
  the claim's own transaction and under its new lease, to the named state or to the first in-progress state
  the workflow allows from where the task is. `tix claim task` and `tix claim next` take `--start` and
  `--start-status STATE`; the API and MCP take the fields. It stays opt-in.
- A transition into a terminal state made under the caller's live lease token releases that lease in the same
  write, with a `task.release` audit entry and a `task.released` event. A release that comes afterwards with
  that token and nothing to write succeeds without writing.
- The refusal of a move without a token, when the caller is the holder, says so and names `--lease-token`
  and the `lease_token` field of the claim output.
