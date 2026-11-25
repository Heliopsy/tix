# Tasks

## Spec

- [x] Proposal, design and delta specs

## Filter language

- [x] `@handle` and `-@handle` as assignee terms in `internal/query`, with tests
- [x] `query.Terms`: the expression's raw terms with their canonical keys, with tests

## Web

- [x] Toolbar partial on the task list and the project page: box, Filters, Sort, view switch, Select, More
- [x] Filters popover pickers; the fold and its redirect on the task list and the project page
- [x] Sort popover with `order=desc`, the list grouping and Group lanes by; pager and Clear keep `order`
- [x] More popover: columns, Export CSV, page size, saved filters, hidden groups, project visibility
- [x] Chips row with Clear all; chip links remove one term
- [x] Suggestion data in the page; filterbar.js listbox; decisions in decide.js; feel.js leaves out untouched pickers and redraws the chips
- [x] Stylesheet part for the toolbar; obsolete filter popover rules removed
- [x] Bulgarian strings
- [x] Web tests: one form, controls, chips, chip links, Clear all, popovers, project page, fold, order
- [x] jstest for the suggestions, the insert and the keyboard decisions
- [x] Screenshot states: no filter, many filters, Filters open, Bulgarian; toolbar height check

## Docs

- [x] `docs/web-ui.md` filter toolbar section and the tips
- [x] `docs/filtering.md` `@handle`
