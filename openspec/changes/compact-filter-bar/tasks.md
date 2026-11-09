# Tasks

## Spec

- [x] Proposal, design and delta specs

## Filter language

- [ ] `@handle` and `-@handle` as assignee terms in `internal/query`, with tests
- [ ] `query.Terms`: the expression's raw terms with their canonical keys, with tests

## Web

- [ ] Toolbar partial on the task list and the project page: box, Filters, Sort, view switch, Select, More
- [ ] Filters popover pickers; the fold and its redirect on the task list and the project page
- [ ] Sort popover with `order=desc`, the list grouping and Group lanes by; pager and Clear keep `order`
- [ ] More popover: columns, Export CSV, page size, saved filters, hidden groups, project visibility
- [ ] Chips row with Clear all; chip links remove one term
- [ ] Suggestion data in the page; filterbar.js listbox; decisions in decide.js; feel.js leaves out untouched pickers and redraws the chips
- [ ] Stylesheet part for the toolbar; obsolete filter popover rules removed
- [ ] Bulgarian strings
- [ ] Web tests: one form, controls, chips, chip links, Clear all, popovers, project page, fold, order
- [ ] jstest for the suggestions, the insert and the keyboard decisions
- [ ] Screenshot states: no filter, many filters, Filters open, Bulgarian; toolbar height check

## Docs

- [ ] `docs/web-ui.md` filter toolbar section and the tips
- [ ] `docs/filtering.md` `@handle`
