# Tasks

## 1. Rendering

- [x] 1.1 `internal/markdown`: goldmark with the GFM extensions, safe nodes, reference and handle links
- [x] 1.2 `internal/markdown`: checklist count, item states and single-item rewrite
- [x] 1.3 XSS corpus and checklist tests

## 2. Service and contract

- [x] 2.1 `internal/core`: `Checklist`, `CheckItemInput`, `CheckTaskItem`, filter fields and markers
- [x] 2.2 `internal/service`: `CheckTaskItem` with audit and event; counts on read
- [x] 2.3 Store predicates on both engines, the shared corpus, the in-memory matcher, the filter language

## 3. Surfaces

- [x] 3.1 HTTP route, list parameters, client
- [x] 3.2 `tix task check`, `tix task uncheck`, the checklist line of `task show`
- [x] 3.3 Web: rendered description and comments, tickable items, Preview, badges and progress bars
- [x] 3.4 TUI: the `[` key, the count in the detail view
- [x] 3.5 Capability registry, input and filter parity
- [x] 3.6 Bulgarian strings, docs, skill
