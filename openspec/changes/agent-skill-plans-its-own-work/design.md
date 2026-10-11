# Design

## The tension

Two requirements pull against each other. The skill should cover most of the
product, and it should cost little context, because SKILL.md loads in full
into every agent that uses it. Resolving that by cutting content arbitrarily
loses coverage; resolving it by padding loses the point.

The split is by **how often an agent needs the material**, not by subject.
Something an agent runs on every task is on the hot path. Something it needs
once per session, or once per installation, is behind a pointer. Something an
agent never needs is not in the skill at all.

## Why split, given the guards

Progressive disclosure was the obvious shape and the dangerous one. Every
guard in `cmd/skill_test.go` read one path, so moving content into sibling
files would have moved it out of verification: the guards would have gone on
passing while naming a narrower thing than they claimed. That is the exact
failure mode `AGENTS.md` records fourteen instances of.

So the guards were widened first, committed, and watched failing from a
sibling file before any content moved. `skillSources` walks the directory and
returns each file separately; the guards that ask "does the skill say X
anywhere" read the join, and the two that parse fenced blocks iterate per
file. Joining for the parsers would have been wrong: a fence opened in one
file and closed in the next swallows everything between, hiding every command
in it.

## What is on the hot path

- Recording the plan: `task add --parent` / `--depends-on`, `task tree`.
- The claim/transition/release loop and `claim exec`.
- Lease rules, stale-token handling, and the boundary of what the lease guards.
- Crash recovery and the `-s todo` trap, with `tix status` as the way to tell
  whether anything sweeps.
- Exit codes, including the two meanings of `3`.
- Statuses, the six categories, terminal states, `--hops`.
- Output formats, `task show`/`tree`/`ls` basics, paging defaults.
- Comments, artifacts, `--dry-run`, dependencies.
- `--db` discipline.

## What is behind a pointer, and why

| Feature | Disposition | Why |
| --- | --- | --- |
| `tix status` | hot path | answers whether a sweeper exists, which the crash trap makes load-bearing |
| `waiting` category | hot path | an agent must not assume the five built-in states; a `waiting` state is not work it can push |
| `cancelled` | hot path | the correct way to abandon work, so dependants unblock |
| `--hops` | hot path | the way out of a status with no direct edge to the target |
| filter language, `due:`, sorting, paging | queries.md | needed when a plain `-s`/`-p` will not do, not per task |
| `tix watch` | queries.md, rule on hot path | "do not poll" belongs on the hot path; the flags do not |
| `tix stats` | queries.md | reporting, not working |
| `tix actor` | queries.md | needed to resolve a handle a human gave you |
| tokens, scopes, contexts, targets | setup.md | once per installation; `--db` discipline stays on the hot path |
| workflows, custom fields | setup.md | `workflow get` is named on the hot path because reading states is per-tenant; defining them is not |
| `tix audit` | setup.md | explaining what an agent did, after the fact |
| `tix serve` | setup.md | operator work, but named because it is what runs the sweeper |
| `retention`, `prune` | setup.md | an agent never prunes, but it explains why an old `--since` cursor cannot resume |
| `import` / `export` | setup.md | whole-tenant snapshots, not agent work |
| `bundle` | setup.md | sharing configuration between installations |
| `sync` | setup.md | the only source of exit `7`, which an agent will otherwise never have seen |
| `webhook` | setup.md | an agent may register one; it will not usually |
| `tix ssh` | ssh.md | nothing about serving a terminal is needed to work a queue |
| `tix tui` | setup.md, one line | worth knowing a human is watching through it |
| the crash transcript | recovery.md | evidence for the hot path's central claim, read once |
| `tix tag add/rm/ls` | not in the skill | already a recorded omission: agents tag with `task add -l` |
| `ssh.trusted_proxies`, deployment flags | not in the skill | `docs/deployment.md` owns them |

## The lease boundary

`core.TransitionInput` is the only input type carrying `LeaseToken`, and
`requireLeaseToken` has one caller. So "lease-bound operation" means renew,
transition and release, and nothing else. `AddComment` and `PutArtifact` take
a ref and a body.

This is documented rather than changed. Changing it would mean a token on
comment and artifact inputs, which is a contract change to `internal/core`
and a behaviour change for every caller including the browser, where a person
commenting on a task holds no lease and should not need one. The honest
answer for now is to say so plainly and give the two defences that already
exist — renew as a cheap holder check, and close through `claim release`,
whose comment and results ride the guarded transition.

The spec delta states the boundary explicitly because the existing wording
reads as stronger than the code: "A stale token SHALL NOT be able to renew,
transition, release, or write a result" is true of a *release* carrying a
result, and was easy to read as true of any result write.

## Verification

Everything the skill teaches was run against scratch SQLite databases under a
directory named for this agent, never the zero-config store. That found six
errors in prose written from `--help` output alone:

- `tix claim ls` does not exist.
- `tix workflow put` takes no name argument; the key comes from the document.
- `tix field ls` / `field put` take `PROJECT` positionally, not `-p`.
- `tix webhook put` takes the URL positionally, not `--url`.
- `tix import` requires `--mode`; `tix bundle import` requires `--on-collision`.
- `tix sync run` requires a source id.

Two of those are now caught by a guard. The rest are argument semantics that
only running the command reveals, which is the argument for running every
command the skill teaches rather than trusting a flag dump.

## The guard blind spots

`resolveInvocation` stopped at the last word the command tree recognised and
discarded the rest, so a misspelled subcommand read as an argument to its
parent and nothing objected. It now returns the leftover positional words and
the guard asks the command's own `Args` validator about them.

A stronger version — always calling `Args`, so a *missing* required argument
fails too — was written, run, and rejected. It failed eight legitimate lines:
prose naming a command beside a flag to talk about the flag (`tix task add
--due`), a bare `tix claim exec` in a sentence, and every `claim exec`
invocation, because words after `--` belong to the child process and so
exec's required argument is never in the list. A guard that forces prose to
be rewritten around it is worse than the gap it closes, and the gap is
recorded in a comment instead.
