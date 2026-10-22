# Design

## One header, at the one writer

`renderStatus` is the single function that writes a screen. Every other render
path, including `render` and the error screen, routes through it. Setting the
header anywhere else would mean finding each caller and keeping the set
complete as screens are added, which is the shape of the defect being fixed:
a rule that holds everywhere it was remembered and nowhere it was not.

## no-store, not no-cache

`no-cache` permits storage and requires revalidation before reuse. That answers
the staleness half and leaves the disclosure half untouched: the response is
still written to disk, still readable by the next user of the machine, still
holdable by an intermediary.

`no-store` answers both. The cost is the back-forward cache, which browsers
decline to use for a `no-store` response, so a back-button restore re-fetches
instead of painting from memory. For a page listing another tenant's work that
is the correct trade, and it is written into the constant's comment so the next
person weighing it has the reasoning rather than the conclusion alone.

## Why the listing in the test is hand-written

`TestEveryScreenForbidsStorage` names the screens instead of enumerating the
route table. Deriving the list from the same structure the production code
walks would pass whether or not the header is set, because both sides would
move together. A hand-written list fails when somebody adds a screen, which is
the failure worth catching.
