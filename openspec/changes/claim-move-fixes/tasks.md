# Tasks

## Service

- [x] 1.1 `WorkflowDefinition.StateKeys`, `UnknownState` and `StartState`; every unknown-state refusal lists the states
- [x] 1.2 `TransitionInput.DryRun` and `RouteInput.DryRun`: run the write and roll it back
- [x] 1.3 `ClaimInput` and `ClaimNextInput` take `start` and `start_status`; the claim moves the task in its transaction
- [x] 1.4 A terminal transition under the caller's live token releases the lease and records it
- [x] 1.5 A token-free move by the holder names `--lease-token` and `lease_token`
- [x] 1.6 A release after a finishing move, with nothing to write, succeeds without writing

## Surfaces

- [x] 2.1 `tix task mv --dry-run` and `--hops --dry-run` ask the service
- [x] 2.2 `tix claim task` and `tix claim next` take `--start` and `--start-status`
- [x] 2.3 `just openapi`, so the API and MCP carry the new fields
- [x] 2.4 Terminal: a lease released by a finishing move is forgotten
- [x] 2.5 Transport matrix scenarios for the new fields

## Docs

- [x] 3.1 docs/agents.md and the skill show the shorter loop
