# An agent skill that records its own plan before working it

## Why

The skill taught an agent to drain a queue somebody else had filled. It said
nothing about writing a plan down, so an agent handed the skill and an
assignment kept the plan in its context and claimed nothing until it started
work. A crash then cost the whole plan, which is the one failure tix is built
to prevent: a task in the store survives the process that created it, and a
decomposition held in a context window does not.

Three further problems were found by driving the CLI rather than by reading:

- **The skill, and `docs/agents.md`, both overstated the lease.**
  `docs/agents.md` is titled "Why a zombie cannot write" and says the lease
  token "is required on every write that touches a claimed task", concluding
  that "the new holder does not need to defend itself". `LeaseToken` is a field
  on `core.TransitionInput` alone, and `requireLeaseToken` has exactly one
  caller, `internal/service/task.go`. `AddComment` and `PutArtifact` take no
  token. Verified: a caller holding no lease at all writes a comment and an
  artifact to a task somebody else holds, exit `0` both times. For a queue of
  agents that is the wrong way round, and the documentation said the opposite.

- **A crashed task is claimable again but has not moved, and nothing said so.**
  Expiry is lazy, so the lease lapses and the row reads as unclaimed with
  nothing running. The status stays where it was. A worker draining with
  `claim next -s todo` — the filter the skill's own example used — therefore
  never sees the crashed task again: `no_task_available`, exit `3`, while the
  task sits claimable in `doing`. Only `tix claim sweep` reverts the status,
  and the skill described the sweep as the thing that frees the task rather
  than the thing that moves it.

- **Five features had never reached the skill at all** — `tix status`, the
  `waiting` state category, `bundle`, `retention` and `prune`, by grep count
  zero — and six more were named exactly once. `tix status` is the command that
  answers whether anything is sweeping, which the trap above turns into
  load-bearing information.

Against that, the file was 661 lines and 32KB, loaded in full into every agent
that used it.

A fourth problem was in the guards rather than the prose. `cmd/skill_test.go`
read `const skillFile = "../skills/tix/SKILL.md"` — one path — so splitting
the file would have left everything that moved unverified: a flag that no
longer exists, a quoted error the binary cannot print, a stated default that
drifted. Two blind spots were also found in the one guard that reads
invocations, both by typing a command that does not exist and watching the
suite pass.

## What Changes

- **The skill leads with recording the plan.** `task add --parent` for
  containment and `--depends-on` for ordering, stated as deliberately
  different things, with `task tree` to read back what was committed to, and
  the durability argument spelled out: a plan in a context window is lost
  entirely, a plan in the store costs one task.

- **The skill is honest about the limit of the lease.** A section says the
  lease guards transitions and nothing else, shows a non-holder writing a
  comment and an artifact at exit `0`, and gives the two defences that exist:
  renew before recording a result, or record it through `claim release`, which
  is lease-guarded.

- **The crash is demonstrated rather than described.** A reference file carries
  the transcript of an agent being `kill -9`ed mid-task and another picking the
  task up, including the `-s todo` trap and the sweep that fixes it.

- **The skill is four files instead of one.** SKILL.md is the loop an agent
  runs constantly; `reference/queries.md`, `reference/setup.md`,
  `reference/ssh.md` and `reference/recovery.md` hold what it needs once in a
  while. Hot-path load falls from 661 lines and 32KB to 482 lines and 22KB — a
  31% cut — while coverage goes up: five features that had never been
  mentioned are now covered, four of them behind a pointer.

- **Every guard reads the whole skill directory.** `skillSources` walks
  `skills/tix` and the guards aggregate per file, so a reference file is
  checked exactly as SKILL.md was. Files are kept apart rather than joined,
  because a fence opened in one would otherwise swallow the next file's prose.

- **A new guard requires SKILL.md to link every file the skill ships.**
  Progressive disclosure only discloses what something points at: an unlinked
  reference is not deeper material but dead material, going stale unread while
  the other guards report the skill is sound.

- **The invocation guard now asks each command's own `Args` validator about
  the positional words left over.** `tix claim ls` does not exist and passed
  the old guard, because resolution stops at the last word it recognised and
  `ls` was simply dropped. It now fails.

- **`docs/agents.md` is corrected** on the zombie claim and on the status half
  of lazy expiry, which is where it disagreed with the binary rather than with
  the skill.

## Impact

- `skills/tix/SKILL.md` rewritten; `skills/tix/reference/{queries,setup,ssh,recovery}.md` added.
- `cmd/skill_test.go`: directory-wide reader, one new guard, two fixed blind spots.
- `docs/agents.md`: the zombie section and the lease section.
- `.gitleaks.toml`: the lease-token allowlist widened from `SKILL.md` to the skill directory.
- No production code changes. The lease boundary is documented, not altered.
