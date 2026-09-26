# Tasks

- [x] Split the gate into `reserve`, `cancel` and `acquire`, with the reservation counting against the
      per-key cap only and `acquire` re-reading both caps.
- [x] Resolve the presented key in `handle`, between the reservation and the slot, and carry the actor to
      the program handler on the session context.
- [x] Keep the capacity refusal ahead of the lookup so a full listener answers every key alike.
- [x] Add `TestAnUnenrolledKeyHoldsNoCapacityAnEnrolledKeyNeeds`, parking a stranger inside its own lookup
      against a real listener with one slot and requiring the enrolled key to be served.
