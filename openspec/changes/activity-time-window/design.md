# Design

- The bounds are `datetime-local` inputs, so the browser offers its own picker and posts `YYYY-MM-DDTHH:MM`,
  the layout the snooze picker already parses (`SnoozeAtLayout`). They are read with the reader's zone from
  Settings, like every other time the web shows or takes.
- `to` is inclusive of the unit it names: a minute for a date and time, a day for a bare date. The store
  compares `occurred_at <= until`, so the bound is the end of that unit minus a nanosecond.
- A bare date in the address is drawn in the input as `T00:00` (From) or `T23:59` (To), which means the same
  window, so the next submit of the form keeps it instead of dropping a value the input cannot hold.
- An unreadable bound is refused rather than ignored, since ignoring it shows the whole log under a filter
  the reader thinks is applied.
