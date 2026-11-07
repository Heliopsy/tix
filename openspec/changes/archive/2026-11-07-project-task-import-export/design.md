# Design

## One project, not the tenant

The snapshot export already walks the tenant, and is tenant-wide authority that a pinned token is refused.
A project's task export is a separate operation with its own action, `task.export`, on the existing
`export` scope. It is confinable: everything it reaches belongs to the one project. The scope is reused
rather than a new one minted because members already hold `export`, and a second scope for the same
authority would be one more thing to grant.

## Streaming

The writer is handed one task at a time, in sequence order, from keyset pages of 200. Nothing is buffered
beyond a page and the per-run lookup caches (actor handles and task references), so memory is bounded by
the distinct actors and linked tasks, not by the project. The HTTP handler and the browser route send their
headers with the first byte, which is the header row: anything refused before it (an unknown column, a
filter that does not parse, a query naming another project) still answers as an error envelope or as the
dialog drawn again with the reason.

## Columns

Columns are named once in `core` and are the same names in every format and on every surface, so a file
written by one surface can be named by another and read back by the import. Custom fields are `custom:KEY`.
JSON keeps the requested order by writing each record as an ordered object. Lists are arrays in JSON and
joined with a comma and a space in CSV and Markdown.

## Dates

ISO by default because every importer reads it. The display style is resolved by the caller, which knows
the reader (configuration on the command line, the settings cookies in the browser), and passed to the
service as a named format and a zone, so the service renders it without knowing where the request came
from. The relative format cannot be read back and is refused, or replaced with ISO by the surfaces.

## Formula guard

CSV is opened by spreadsheets, so every cell goes through the guard the task listing's CSV already uses.
JSON, NDJSON and Markdown are not, and carry text unchanged; quoting them would corrupt a round trip.

## MCP

The export answers CSV by default and JSON on request. The OpenAPI generator now describes a JSON
alternative as an array of records and an NDJSON one as a record per line, so the MCP planner offers the
operation as a tool. It is a `POST` because the choices are a body, so the overlay's new `read_only` key
marks it as reading, with a reason.
