# Tasks

## 1. Keep what the editor does not render

- [x] 1.1 `internal/web/workflows.go`: `mergeDefinition`, taking each stored state and transition
      whole and overwriting only the fields the form edits, plus `DefaultLease`.
- [x] 1.2 `internal/web/workflows.go`: `renamedFrom` and `formerKey`, reading the migration lines as
      the renames they are, and following `RevertTo` through them.
- [x] 1.3 `internal/web/workflows.go`: `storedDefinition`, reading the workflow under the posted key
      and treating not-found as "this is a new workflow" rather than an error.
- [x] 1.4 `internal/web/workflows.go`: `putWorkflow` merges before calling `PutWorkflow`; the service
      keeps its replace-wholesale contract for the API and the CLI.

## 2. The state line says what it means

- [x] 2.1 `internal/web/workflows.go`: `parseTerminal` accepting `true`/`false`, `terminal`/`open`,
      `1`/`0`, `yes`/`no`, `on`/`off` and an empty field, refusing anything else.
- [x] 2.2 `internal/web/workflows.go`: `parseStates` returns the refusal as `core.Invalid`, so the
      save answers 400 instead of storing a guess.
- [x] 2.3 `internal/web/templates/workflow.html`: the help text names the accepted values and says
      the unshown fields are kept.

## 3. Guards

- [x] 3.1 `internal/web/workfloweditor_test.go`: a no-op save keeps every state's category and
      lease-expiry revert, and the definition's default lease.
- [x] 3.2 `internal/web/workfloweditor_test.go`: a no-op save keeps a transition's required scope
      and required comment.
- [x] 3.3 `internal/web/workfloweditor_test.go`: after a no-op save the restricted move is still
      refused for an actor without the scope and still permitted for one who has it.
- [x] 3.4 `internal/web/workfloweditor_test.go`: reflection over `core.State` and `core.Transition`
      fails on any field left at its zero value, then the whole struct is compared after a no-op save.
- [x] 3.5 `internal/web/workfloweditor_test.go`: a declared rename carries the unshown fields across.
- [x] 3.6 `internal/web/workfloweditor_test.go`: a state and a transition removed in the form stay
      removed, so the merge cannot be read as "nothing is ever removed".
- [x] 3.7 `internal/web/workfloweditor_test.go`: each documented terminal word, and a refusal for a
      word that is neither, with a terminal state present so the refusal is about the word.
