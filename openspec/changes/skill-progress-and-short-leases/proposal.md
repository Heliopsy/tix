# Skill: progress comments on bigger tasks, short leases

## Why

Agents working a long task through the skill go quiet until they release it, so a person watching the board
cannot tell whether the work is moving, and a replacement after a crash starts with nothing. Agents also ask
for far longer leases than the work needs, partly because the skill's own planning example gave a step 2h:
a long lease is how long a task stays locked after its holder dies.

## What Changes

- The skill defines a bigger task (subtasks, more than about three steps, past one default lease, more than a
  couple of files or systems) and requires a comment with the plan at the start, one at each milestone and
  when blocked, and a closing summary with links. It names `tix comment add`, `claim release --comment` and
  the `comment_add` MCP tool, and says a quick task needs none.
- The skill tells an agent to claim with the default lease or a ttl close to the next stretch of work, renew
  as it goes (`tix claim renew` with or without `--ttl`), and release as soon as it stops. The planning
  example's leases shrink from 30m, 2h and 15m to 15m, the default and 10m.
- The skill's version goes to 26.
