# Tasks

## 1. The artifact write reaches the lease guard

- [x] 1.1 `internal/core/input.go`: `ArtifactInput.LeaseToken`, additive and `omitempty`, so the HTTP
      body carries it with no transport change
- [x] 1.2 `internal/service/artifact.go`: `PutArtifact` calls the existing `requireLeaseToken` inside
      the write transaction, after the task is loaded and authorized and before the row is inserted
- [x] 1.3 `internal/service/artifact_test.go`: `TestArtifactWriteIsLeaseBound` covers the holder, a
      live lease refusing a tokenless and a non-matching write, and a lapsed lease accepting no token
      and refusing an offered one; `TestZombieCannotWriteOverTheNewHolder` is the reported defect;
      `TestCommentingAndEditingStayOpenOnAClaimedTask` is the half that must keep working

## 2. The token reaches the artifact write from the command line

- [x] 2.1 `cmd/artifact.go`: `--lease-token` on `artifact put`, named as `task mv`'s is, and the
      command's exit-code line now names lease expiry
- [x] 2.2 `cmd/artifact_test.go`: `TestArtifactPutOnAClaimedTaskNeedsTheLeaseToken` claims, is refused
      without the flag, and succeeds with it

## 3. The documentation stops overclaiming

- [x] 3.1 `docs/agents.md`: "Why a zombie cannot write" lists the artifact write among the writes the
      token covers, states that a zombie presenting no token is refused too, and carries a new
      subsection naming comments and field edits as deliberately uncovered, with the reason for each
- [x] 3.2 `skills/tix/SKILL.md`: the artifact write is listed among the token-bound writes, the two
      untokened writes are named, the stale-token block quotes both artifact refusals, and the
      worked example passes `--lease-token` because the task is claimed by then
- [x] 3.3 `skills/tix/SKILL.md`: `verified-against` refreshed to the commit this was checked against

## 4. Gates

- [x] 4.1 Watched three mutations fail: the service guard removed, the CLI token wiring removed, and
      the flag removed altogether. Each restored with `git checkout --`, `git diff` empty after each
- [x] 4.2 Demonstrated end to end against scratch databases under this agent's own directory: a
      SIGKILLed worker's artifact write refused after re-claim, with and without its token; a human's
      browser comment and field edit landing on the same held task; and the HTTP API refusing and
      accepting the same three ways
- [x] 4.3 `just check` exit 0 and `just test-postgres` exit 0 with both engines, against this code on
      the earlier branch point; `just check` re-run on the rebased tree. `just ci` was started and
      stopped when the branch was rebased, so the gosec and image gates have NOT been run here

## 5. Superseded spec text

- [x] 5.1 `openspec/changes/agent-skill-plans-its-own-work/specs/claim-lease/spec.md`: that change,
      still unarchived, required the artifact write to take no lease token and to be checked against
      none, which is the behaviour this change reverses. Its artifact clause and its "An artifact is
      not a lease-bound operation" scenario are withdrawn with the supersession noted; its comment
      clause stands, because this change keeps comments open deliberately
