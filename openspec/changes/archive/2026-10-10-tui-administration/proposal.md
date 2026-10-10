# A tenant administration screen, for the thing the boards live in

## Why

Forty operations were recorded as terminal gaps, and thirty-two of them said the same thing: there is no
administration view. The command line, the HTTP API and the browser all expose them; only the terminal does
not, which makes the terminal the outlier rather than the operations exotic.

Eleven of the forty are one subject. A tenant is its own attributes, the hostnames that resolve to it and the
actors who belong to it, and a reader asking "what is this tenant and who is in it" asks one question rather
than three. The interface already has a screen called `tenant`, and until now it stated only the key the
session was pinned to and offered a switch. It read nothing, so it could answer nothing.

The primitives this needs already exist: the confirmation that names its subject, the form whose every field
picks from a fixed list, and the single-line prompt for the answers a fixed list cannot hold. Nothing new is
invented here; the screen is assembled from what the project screen already proved.

## What Changes

- **The tenant view reads the tenant.** `T` now fetches the tenant in force, the tenants this session can
  see, its domains and its memberships, and states them. What a reader's authority does not reach says so in
  place rather than costing the reader the rest of the screen.
- **A cursor over the rows that can be removed.** The domains and the members are one selectable list, so a
  removal has a named subject rather than a typed identifier.
- **Editing the tenant.** `e` opens a form whose first field is the attribute. The theme is answered from its
  own list of the palettes the build carries; the name hands over to the single-line prompt, seeded with the
  value it would replace.
- **Adding a domain or a member.** `n` asks which. A domain hands over to the prompt for its hostname and to a
  form for its certificate mode. A member is gathered entirely in a form: the actor is picked from the
  tenant's own directory, because a reader knows a colleague by handle and has never seen the identifier the
  service stores, and the role is one of three.
- **Removing either, behind a confirmation that names it.** `X` removes the selected row. The question names
  the hostname or the handle, never "the selected row".
- **The switch stays.** Everything the view already did, it goes on doing: the key it takes, the refusal of
  the tenant it is already on, and the read-only statement a session with no dialer gets.

## Impact

- `internal/tui`: `tenant.go` grows the screen, its rows and its three forms; the view gains a cursor, a
  read, four actions and two confirmations. No new key is bound: the screen borrows `e`, `n` and `X` from the
  board, which is what keeps every keybinding scheme working here without rebinding anything.
- `internal/capability/registry.go`: nine terminal gaps become bindings, so the recorded terminal gap count
  falls from 40 to 31. `tenant.create` and `tenant.delete` stay recorded gaps, because a session pinned to one
  tenant is the wrong place to make another and the wrong place to destroy the one it is using.
- `docs/tui.md`: the tenant row of the views table, the `Where` column of the bindings it borrows, and the
  count and shapes in "What the terminal does not do".
- No change to `internal/service`, `internal/store`, `internal/core`, `internal/web` or `cmd`. Every operation
  already existed; what is new is that the terminal can reach it.
