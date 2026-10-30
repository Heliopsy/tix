# Design

## The decision in one line

A screen may show part of a listing. It may not show part of a listing without saying so.

Truncation is not the defect; silence is. Every fix here either reaches the end of the listing or
prints a sentence saying it did not, and the choice between those two is made on cost.

## Why the token figure counts the listing rather than the rows

Three options were on the table for the tenant diagram's "API tokens" row.

1. **Label it as the caller's own.** Free, honest, and wrong for the row's position: it sits between
   Members, Domains and Workflows, all tenant-wide, under a heading reading "What sits under this
   tenant". A reader counting their own credentials there learns nothing about the tenant.
2. **Add a tenant-wide count to `internal/service`.** One query instead of N. It also creates a
   second answer to the same question, maintained separately from the listing the screen renders,
   which is exactly the divergence this change exists to remove. `internal/core` is the frozen
   contract and a new operation needs a capability registry entry with three bindings, for a figure
   on an administration diagram.
3. **Count the listing the token screen renders.** `tokenShapeRow` calls `tokenListing`, the function
   `/admin/tokens` builds its table from. The figure cannot mean something the screen it links to does
   not, because it is that screen's rows counted.

The reader's-own branch of `tokenListing` cannot reach this row: `GetTenant` needs
`authz.ActionTenantAdmin`, which resolves to the same scope the listing branches on, so a reader who
can see the row holds it. The row therefore carries no note about whose figure it is, because there
is no case in which it is not the tenant's. A reader holding that scope without `token:admin` cannot
list tokens at all, and the row prints `unavailable`.

Three was chosen. It costs an actor directory walk plus one `ListTokens` per actor on the tenant
screen, which is what the token screen already pays, on a screen read by an administrator rather than
in a loop. The scale that matters here is actors, not tasks: a tenant's staff and agents, not its
1M+ rows.

## Why the actor walk is bounded, and what the bound means

`allActors` is `allProjects` with a different listing. Walk the cursor at `core.MaxPageLimit` a page,
stop at `actorScanPages`, return whether the walk finished.

The bound is not a cap on what the screen may show. It is the thing that stops a request handler
walking an unbounded table, and reaching it is a fact about the request, so it is returned rather
than swallowed: `tokenList.Whole` is false, the table says it is short, and the tenant row prints
`incomplete` instead of a number that would be a floor.

`projectScanPages` stays at 40. The alternative to a bound in a request handler is no bound, which is
how a request stops returning; a reviewer who wants 20,000 projects covered should ask for a count
query or a background figure, not for an unbounded loop. What was wrong was not the number but that
three of the six consumers of `allProjects` dropped the flag that says the walk stopped.

## Where a caveat lives

On the control, not the screen. `visibilityFormView.Whole` carries the project walk's completeness,
because the task screen and the settings screen render the same partial from the same constructor; a
caveat held by one screen's view is a caveat the other silently drops, which is the same class of bug
one level up. The settings screen's own `ProjectScan` field is therefore gone.

For a figure, incompleteness takes the figure's place rather than sitting beside it, which is what
the diagram already does for rows that have no count: `countShort` is `incomplete`, next to
`countFailed`, which is `unavailable`. A short walk is not a failed read and the row now distinguishes
them.

## Errors versus shortness on the statistics screen

The picker still swallows its error: a failed project listing leaves the screen without a picker
rather than without numbers, which is the right trade for a control beside figures that do not depend
on it. What it no longer does is read one page. Both outcomes -- a failed read and a walk that
stopped at its bound -- set one flag, and the screen says the picker does not list every project.
Distinguishing them on screen would be a distinction without a decision behind it: either way the
reader cannot choose a project that is missing, and either way the figures are unaffected.

## Testing

The short case cannot be seeded: `actorScanPages` is 20,000 actors. The fixture's service stand-in
answers every page with a cursor for another one, which is what a directory longer than any bounded
walk looks like to a handler, and the guard asserts what each screen renders in that state and that
it says nothing when the walk reached the end. The tail case is seeded for real, because 505 actors
is affordable and the bug there was the dropped cursor rather than the bound.

Every assertion reads the control or the row it names -- the visibility form, the statistics
`select`, one `<li>` of the diagram -- and not the page. Four guards in this repository have passed
over broken behaviour because the token they matched was somewhere else on the page.
