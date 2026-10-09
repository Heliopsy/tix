# Tasks

## 1. The preference

- [x] 1.1 `internal/web/taskview.go`: `TaskViewCookie`, the two view names, `taskViewOf` reading
      anything unrecognised as the list, `taskViewChoices` for the switch, and `setTaskView` storing
      nothing for the default
- [x] 1.2 `internal/web/routes.go`, `internal/web/session.go`: `POST /taskview` beside the other
      per-browser display preferences

## 2. The merge rule

- [x] 2.1 `internal/web/taskboard.go`: `workflowShape`, the structural identity a board merges on,
      with the doc comment saying what is compared, what is excluded and why the asymmetry of cost
      decides the borderline fields
- [x] 2.2 `internal/web/taskboard.go`: `candidateProjects`, read off the assembled filter so
      visibility, an explicit `project:` term and a negated one are one rule
- [x] 2.3 `internal/web/taskboard.go`: `groupByWorkflow`, deterministically ordered, keeping a
      project whose workflow cannot be resolved visible in its own group rather than dropping it
- [x] 2.4 `internal/web/taskboard.go`: `buildBoard`, resolving each card's routes from the card's own
      project's workflow, and collecting any task no column could hold
- [x] 2.5 `internal/web/taskboard.go`: `narrowHref`, the one-click route to a drawable board, keeping
      the filter and sort and dropping the cursor and its trail
- [x] 2.6 `internal/web/tasks.go`: `workflowsByID` shared by `completeStates`, `workflowsByProject`
      and the board, so the screen lists workflows once rather than three times

## 3. The screen

- [x] 3.1 `internal/web/tasks.go`: the view, the switch and the board on `tasksView`, resolved from
      the same filter the list was answered with
- [x] 3.2 `internal/web/templates/tasks.html`: the view switch beside the filter bar
- [x] 3.3 `internal/web/templates/tasks.html`: the board, its per-card move control posting to the
      task's own transition route, the project mark on a merged card, the refusal naming each
      workflow group, and the unplaced-task notice
- [x] 3.4 `internal/web/render.go`: a `stateLabel` helper, so a column heading falls back to the
      state's key rather than rendering empty
- [x] 3.5 `internal/web/assets/app.css`: the switch, the refusal, the unplaced notice, and the
      merged board's column track
- [x] 3.6 `internal/web/assets/live.js`: refresh the board from `pathname + search`, so a move on a
      filtered board does not come back unfiltered

## 4. Guards

- [x] 4.1 `internal/web/taskview_test.go`: the preference persists; switching preserves the filter
      and the visibility choice; identical workflows merge; differing ones refuse and say which
      projects disagree; a card offers no control for a state its own workflow cannot reach
- [x] 4.2 `internal/web/taskboard_internal_test.go`: the shape, what it ignores and what it does
      not, and the grouping's determinism
- [x] 4.3 `internal/web/jstest/live.test.mjs`: the refresh carries the query string

## 5. Documentation

- [x] 5.1 `docs/web-ui.md`: the switch, what workflow equality means, what a refusal says, and the
      new row in the preferences table
