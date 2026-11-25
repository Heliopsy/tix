# Tasks

## Spec

- [x] Proposal, design and delta specs

## Core and store

- [x] `Task.WaitingOn` filled with open dependency references in both engines
- [x] `store.TallyTasks` grouped count, `TaskFilter.Tally` and `TaskPage.Tally`
- [x] `is:waiting-on` alias

## Web

- [x] Heading counts from the tally; "waiting on other tasks" links to `is:waiting-on`
- [x] Waiting mark on board cards, lane cards and list rows
- [x] Board columns read per state with Show N more; lanes unpaged with Show N more
- [x] Bulgarian strings

## TUI and CLI

- [x] TUI board read per column; `!` legend says waiting on another task
- [x] CLI table WAITING column names the references

## Docs and tests

- [x] web-ui.md, tui.md, features.md, filtering.md
- [x] Tests for the tally, the alias, the mark and the board columns
