# Tasks

## 1. Blank clears the value

- [x] 1.1 `internal/web/form.go`: `sent`, which reports the value and whether the submission carried the
      key, so an absent key and an empty one stop reading alike
- [x] 1.2 `internal/web/taskdetail.go`: carry a blank custom field through as nil instead of dropping it,
      and say in the doc comment why an absent key still changes nothing
- [x] 1.3 `internal/web/admin.go`: send `DisplayName` on the key's presence rather than on its value
- [x] 1.4 `internal/web/blankclears_test.go`: a save naming one field of two, a save naming none, a
      required field refused rather than emptied, and an account update naming no name

## 2. Hiding a project hides that project

- [x] 2.1 `internal/web/projects.go`: `allProjects`, which walks the listing's cursor and reports whether
      it finished, with `projectScanPages` bounding it
- [x] 2.2 `internal/web/visibility.go`: `hiddenKeys`, read from the reader's own preference; drop
      `shownKeys`, which existed only to invert it
- [x] 2.3 `internal/web/tasks.go`: exclude the hidden keys instead of naming the shown ones, and drop the
      guard that existed because the inversion could empty itself
- [x] 2.4 `internal/web/tasks.go`, `internal/web/visibility.go`, `internal/web/activity.go`: the project
      walk at every caller that needs the whole set
- [x] 2.5 `internal/web/admin.go`: the tenant diagram's project count from the walk, with the `Uncounted`
      fallback every other row already had
- [x] 2.6 `internal/web/projectcap_test.go`: the control's boxes, the diagram's count, and finishing a
      task in a project past the first page

## 3. The activity feed's search is a filter

- [x] 3.1 `internal/core/filter.go`: `AuditFilter.Text`
- [x] 3.2 `internal/store/sql/text.go`: `ApplyContains`, one predicate for both engines
- [x] 3.3 `internal/store/sqlite/event.go`, `internal/store/postgres/event.go`: answer it, each naming its
      own columns, because the snapshots are TEXT on one and JSONB on the other
- [x] 3.4 `internal/httpapi/handlers_history.go`, `internal/client/history.go`: `text` on the wire
- [x] 3.5 `internal/web/activity.go`: hand the box to the store, drop the scan loop, drop
      `activityScanPages` and the `Scanned` figure with the window it described
- [x] 3.6 `internal/web/templates/partials.html`: an empty result that no longer claims a window
- [x] 3.7 `internal/service/history_test.go`: walking a filtered listing to its end, and the haystacks the
      term reads

## 4. The delivery log's pager

- [x] 4.1 `internal/web/webhooks.go`: the pager the handler's cursor was already waiting for
- [x] 4.2 `internal/web/templates/webhooks.html`: render it
- [x] 4.3 `internal/web/webhookspager_test.go`: the next link, and that page two holds different rows
