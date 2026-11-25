# Tasks

## 1. Spec

- [x] 1.1 Delta spec for web-tasks

## 2. Server

- [x] 2.1 `view` query parameter read before the cookie; GET /tasks with a view remembers it
- [x] 2.2 Scope the task screen to a project: filter, routes, live subscription, quick add project
- [x] 2.3 `/projects/KEY` and `/projects/KEY/live` served by the scoped task screen
- [x] 2.4 View switch links carrying the query

## 3. Browser

- [x] 3.1 Project header on the scoped screen, project select hidden, Clear and pager scoped
- [x] 3.2 feel.js redraws the switch after an in-place filter; palette follows the links
- [x] 3.3 Link switch styled by the existing `.segmented a.segment` rule, no new part needed
- [x] 3.4 Bulgarian strings and (i) tips

## 4. Tests and docs

- [x] 4.1 Project list view with filters
- [x] 4.2 Every switch link keeps the query, main and project, every view
- [x] 4.3 No-JS switch round trip
- [x] 4.4 docs/web-ui.md, screenshot manifest, ui-check
