# Tasks

## 1. Widen the guards before moving anything

- [x] 1.1 `cmd/skill_test.go`: `skillSources` walks `skills/tix` and returns each
  Markdown file separately; `readSkill` joins them for the guards that ask
  whether the skill says a thing anywhere, `readSkillFile` serves the one that
  must read SKILL.md itself.
- [x] 1.2 `TestSkillNamesOnlyRealCommandsAndFlags` and
  `TestSkillQuotedFailuresAreOnesTheBinaryCanPrint` iterate per file, so a
  fence in one cannot absorb the next, and name the file in every failure.
- [x] 1.3 `TestSkillRecordsOneVersionMarker` counts one marker across the
  directory and asserts it is in SKILL.md.
- [x] 1.4 New `TestSkillReferencesEveryFileItShips`: SKILL.md links every
  Markdown file the skill ships.
- [x] 1.5 Commit, then watch each widened guard fail from a sibling file, and
  restore against git with `git diff` empty.

## 2. Close the invocation guard's blind spot

- [x] 2.1 `resolveInvocation` returns the leftover positional words instead of
  discarding them.
- [x] 2.2 `checkInvocation` asks the command's own `Args` validator about those
  words; watched failing on `tix claim ls`.
- [x] 2.3 Rejected the stronger form that also asserts on an empty argument
  list: it failed eight legitimate prose lines, and the gap is recorded in a
  comment instead.

## 3. Rewrite SKILL.md as the hot path

- [x] 3.1 Lead with recording the plan: `--parent` for containment,
  `--depends-on` for ordering, `task tree` to read it back, and the unit of
  loss stated.
- [x] 3.2 The claim/transition/release loop and `claim exec`, including the
  illegal `--on-start` self-transition found by running it.
- [x] 3.3 A "What the lease does not protect" section with the non-holder
  comment and artifact at exit `0`, and the two defences.
- [x] 3.4 A "Crash recovery" section: lazy expiry, the `-s todo` trap, the
  sweep, and `tix status` as the way to tell whether anything sweeps.
- [x] 3.5 Statuses and the six categories including `waiting`, terminal states,
  `cancelled` as the way to abandon work, and `--hops`.
- [x] 3.6 Exit codes, output formats, comments and artifacts, dependencies,
  `--db` discipline, "Do not", worked example.

## 4. Reference files

- [x] 4.1 `reference/queries.md`: filter flags and expression, the two meanings
  of exit `3`, deadlines, sorting and paging, the actor directory, `tix stats`
  with `by_category`, `tix watch` and the retention caveat on `--since`.
- [x] 4.2 `reference/setup.md`: targets and contexts, `tix status` versus
  `tix doctor`, tokens and scopes, workflows and custom fields, the audit log,
  `tix serve`, `tix tui`, retention and `prune`, import/export, bundles, sync,
  webhooks.
- [x] 4.3 `reference/ssh.md`: the SSH listener, moved whole.
- [x] 4.4 `reference/recovery.md`: the crash transcript.
- [x] 4.5 Front matter: `version: 9`, `verified-against` the build driven.

## 5. Run every command the skill teaches

- [x] 5.1 Build the binary and drive every invocation against scratch SQLite
  databases under a directory named for this agent. Never the zero-config
  store.
- [x] 5.2 Fix what that found: `tix claim ls` does not exist;
  `tix workflow put` takes no name; `field ls`/`field put` take `PROJECT`
  positionally; `webhook put` takes the URL positionally; `import` requires
  `--mode`; `bundle import` requires `--on-collision`; `sync run` requires a
  source id.
- [x] 5.3 Walk the plan-first loop end to end: decompose, claim, `kill -9`
  mid-work, show the task returning and another claim taking it.

## 6. Correct what disagreed with the binary

- [x] 6.1 `docs/agents.md`: "Why a zombie cannot write" said the lease is
  required on every write and that the new holder need not defend itself.
  Retitled and corrected, with the non-holder writes shown.
- [x] 6.2 `docs/agents.md`: the lease section now says expiry is lazy and that
  the status is not, with the status-scoped claim trap.
- [x] 6.3 `.gitleaks.toml`: lease-token allowlist widened to the skill directory.

## 7. Specs

- [x] 7.1 `specs/agent-skill/spec.md`: ADDED requirements for the plan-first
  instruction, the lease boundary, lazy expiry's two halves, hot-path
  separability, and directory-wide guard coverage.
- [x] 7.2 `specs/claim-lease/spec.md`: MODIFIED "Lease token required for
  lease-bound operations" to state the boundary explicitly, and MODIFIED "Lazy
  expiry is authoritative" to add the half it did not say -- that freeing the
  task and moving its status are separate events -- rather than adding a
  near-duplicate requirement beside it.
- [x] 7.3 `just check`.
