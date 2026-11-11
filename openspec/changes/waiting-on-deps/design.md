# Design

## Counting the whole selection

`core.TaskFilter.Tally` (in-process only, like `OmitBody`) asks `ListTasks` to fill `TaskPage.Tally`
in the same read transaction as the page: `store.TallyTasks` is one `GROUP BY project_id, status`
over the listing's own predicates and tenant scope, and two capped counts give the waiting and held
numbers. Open and done follow from each project's done state, and the project count from the distinct
project ids. No new service operation: nothing travels on the wire, so no surface changes beyond the
web screen that asks.

## Board columns

An agreed board issues one `ListTasks` per state (the filter plus `status:STATE`), capped at 50 a step;
`?more=STATE` raises one column's cap by a step, up to the 500-row page limit. The column total is that
listing's `Total`. A last read excludes every column state to find stray tasks. Lanes read one listing
capped at 200 a step (`?more=lanes`); grouped lanes are built from arbitrary keys, so per-lane reads
would need one query shape per grouping and were left out.

## Naming

"Waiting on other tasks" separates the dependency flag from the blocked workflow category. `waiting` is
already a state category, so the filter alias is `is:waiting-on`, not `is:waiting`. The JSON flag
`blocked` is kept for compatibility; `waiting_on` is added beside it.
