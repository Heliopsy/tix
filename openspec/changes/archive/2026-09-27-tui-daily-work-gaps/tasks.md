# Tasks

## The history view

- [x] `tui/history.go`: `HistorySubject`, `Filter`, `Title`, `HistoryRow`, `HistoryEmptyState`, `HistoryCount`
- [x] `tui/history.go`: `openHistory` refusing the read as well as the view, `onHistory`, `handleHistoryKey`
- [x] `tui/commands.go`: `loadHistory` resolving the actors a page names to handles
- [x] `tui/keys.go`: the `History` binding, and the global key paired with the view it opens
- [x] `tui/scheme.go`: the history view in every collision set and in `viewName`
- [x] `tui/view.go`: the body, the title bar name and the status bar count

## Recording an artifact

- [x] `tui/form.go`: `ArtifactKindOptions` from `core.ArtifactKinds`, `ArtifactForm`, `ArtifactNote`
- [x] `tui/prompt.go`: the artifact name prompt
- [x] `tui/model.go`: the prompt handing over to the form, and the form making the call
- [x] `tui/commands.go`: `putArtifact`

## Restoring a deleted task

- [x] `tui/view.go`: `DeletedMarker` on a deleted card, and the legend entry
- [x] `tui/keys.go`: the `Restore` binding, `ActionContext.IsDeleted`, and a footer that offers one action
- [x] `tui/model.go`: `restoreTask` refusing a task that was never deleted
- [x] `tui/commands.go`: `restore`

## The assignee picker

- [x] `tui/form.go`: `ActorLabel`, `ActorOptions`, `ActorIDFor`, `AssigneeForm`
- [x] `tui/model.go`: `openAssigneeForm`, `onActors`, and the form resolving a handle to an identifier
- [x] `tui/commands.go`: `loadActors`
- [x] `tui/prompt.go`: the assignee prompt removed

## The registry

- [x] `capability/registry.go`: `audit.list`, `artifact.put`, `task.restore` and `actor.list` bound
- [x] `capability/registry.go`: the shortfall recorded against the terminal's `artifact.put`
- [x] `capability/parity_test.go`: the history view declared, the four added to the daily set, 44 to 40
