# Make the number of rows a listing shows configurable, per surface

## Why

Nothing configured how many rows a listing carries. Every surface fell through to
`core.DefaultPageLimit`, fifty, which is the contract's fallback for a caller that names no limit rather than
a judgement about what is worth looking at. A screenshot of the browser's task list came out 5630 pixels
tall, more than three screens, because fifty rows is more than anybody reads before paging.

The browser had no way to ask for any other number at all: no flag, no key and no query parameter. The CLI
had `--limit` and the API takes a limit parameter, so the browser was the one surface where the row count was
whatever the contract happened to say.

One number for the whole install would be the wrong shape. How many rows are worth showing is a property of
the thing showing them: a browser window and a terminal do not hold the same number of lines, and a command
writing into a pipe is a third case again.

## What Changes

- **`cli.page_size` and `web.page_size`**, both defaulting to 25, resolved through the documented layers:
  flags beat environment beats `.env` beats file beats defaults.
- **`cli.page_size` is the fallback of `--limit`** on every listing command. The flag is gated on
  `Flags().Changed`, so a flag nobody typed is not a layer.
- **`web.page_size` is what a browser listing shows**, read by `tix serve` and passed to the browser handler.
- **The browser can ask for a page size at request time** with a `limit` query parameter, which the pager's
  links carry forward for the rest of that walk.
- **An unusable page size is refused, not clamped.** Below 1 or above `core.MaxPageLimit` fails at startup
  with the key and the layer named.

## Impact

- `core.DefaultPageLimit` stays 50. It is the fallback for a caller that names no limit, which after this
  change means an HTTP API client rather than a surface a person is looking at.
- There is no `tui.page_size`. The terminal interface does not page: it reads the listing up to
  `core.MaxPageLimit` and scrolls it, so it has no page size to configure. A key nothing reads is the defect
  the shadowing guard exists to catch.
- `tix task ls` and the other listing commands print 25 rows rather than 50 where nothing asks otherwise. A
  script that depends on a count already has `--limit`.
