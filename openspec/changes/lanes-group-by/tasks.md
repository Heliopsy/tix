# Tasks

## Spec

- [x] Proposal, design and delta specs

## Service

- [x] `core.LanesView`, the lanes groupings and `GroupNone`; `SplitView` accepts `lanes:BY`
- [x] `resolveGroup` and `shownGroup` for assignee, priority, tag and due lanes
- [x] Service tests for each new view's groups

## Web

- [x] `lanes` parameter, defaults on the task list and the project page, remembered on the task list
- [x] Group lanes by control in the view switch, a GET form that works without scripting
- [x] Lanes built by assignee, priority, tag and due, with the empty-lane rules
- [x] Arrangement keyed per grouping; group menu filter link for the new kinds
- [x] Filter form, Clear and pager keep `lanes`
- [x] Bulgarian strings and the (?) tip
- [x] Web tests: each grouping, defaults, address kept, no-JS, arrangement per grouping
- [x] jstest for the auto-submit
- [x] Screenshot of lanes by assignee on a project page

## Docs

- [x] `docs/web-ui.md` Lanes, The project page and Arranging columns and lanes
- [x] `cmd/view.go` help, `docs/api.md`, the tix skill
