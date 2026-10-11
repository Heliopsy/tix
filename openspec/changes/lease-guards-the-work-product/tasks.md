# Tasks

## 1. The artifact write reaches the lease guard

- [ ] 1.1 `internal/core/input.go`: `ArtifactInput.LeaseToken`, additive and `omitempty`, so the HTTP
      body carries it with no transport change
- [ ] 1.2 `internal/service/artifact.go`: `PutArtifact` calls the existing `requireLeaseToken` inside
      the write transaction, after the task is loaded and authorized and before the row is inserted
- [ ] 1.3 `internal/service/artifact_test.go`: the holder writes under its token; a live lease refuses
      a tokenless write and a non-matching one; a lapsed lease accepts no token and refuses an offered
      one; the zombie-after-re-claim case leaves the artifact list unchanged

## 2. The token reaches the artifact write from the command line

- [ ] 2.1 `cmd/artifact.go`: `--lease-token` on `artifact put`, named as `task mv`'s is
- [ ] 2.2 `cmd/artifact_test.go`: the flag carries the token through to the service

## 3. The documentation stops overclaiming

- [ ] 3.1 `docs/agents.md`: "Why a zombie cannot write" lists the artifact write among the writes the
      token covers, and says plainly that comments and field edits are deliberately not covered and why
- [ ] 3.2 `skills/tix/SKILL.md`: the artifact section shows `--lease-token` on a claimed task, and the
      stale-token section names the artifact write
- [ ] 3.3 `skills/tix/SKILL.md`: refresh `verified-against` to the commit this was checked against

## 4. Gates

- [ ] 4.1 Watch each new guard fail against a mutation of the code it protects, restore, confirm
      `git diff` is empty
- [ ] 4.2 Demonstrate end to end against a scratch database: the zombie's artifact write is refused
      after re-claim, and a session-authenticated comment with no token still lands
- [ ] 4.3 `just check`, `just test-postgres`, `just ci`
