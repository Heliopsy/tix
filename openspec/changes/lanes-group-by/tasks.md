# Tasks

## Spec

- [ ] Proposal, design and delta specs

## Service

- [ ] `core.LanesView`, the lanes groupings and `GroupNone`; `SplitView` accepts `lanes:BY`
- [ ] `resolveGroup` and `shownGroup` for assignee, priority, tag and due lanes
- [ ] Service tests for each new view's groups

## Web

- [ ] `lanes` parameter, defaults on the task list and the project page, remembered on the task list
- [ ] Group lanes by control in the view switch, a GET form that works without scripting
- [ ] Lanes built by assignee, priority, tag and due, with the empty-lane rules
- [ ] Arrangement keyed per grouping; group menu filter link for the new kinds
- [ ] Filter form, Clear and pager keep `lanes`
- [ ] Bulgarian strings and the (?) tip
- [ ] Web tests: each grouping, defaults, address kept, no-JS, arrangement per grouping
- [ ] jstest for the auto-submit
- [ ] Screenshot of lanes by assignee on a project page

## Docs

- [ ] `docs/web-ui.md` Lanes, The project page and Arranging columns and lanes
- [ ] `cmd/view.go` help, `docs/api.md`, the tix skill
