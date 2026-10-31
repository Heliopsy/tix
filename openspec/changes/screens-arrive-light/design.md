# Design

## Indexes

The primary keys are `(task_id, tag_id)` and `(task_id, depends_on)`, without the tenant. The
tenant-scoped builder always adds `tenant_id = ?`, and with no statistics SQLite preferred the
older indexes whose first column matched it. Adding the tenant in front of the task gives the
planner an index that satisfies both equalities, and it takes it. The test asks SQLite for the plan
of each relation query rather than timing it, so it fails on the cause and not on a slow machine.

The dependency fill used `ORDER BY depends_on`; with that the planner still chose the depends_on
index to skip a sort. Ordering by `task_id, depends_on` gives the same order inside each task, which
is all the caller reads, and the new index supplies it.

## Compression

A wrapper decides at `WriteHeader` from the content type: `text/html` is compressed at
`gzip.BestSpeed` through a pooled writer, anything else passes through. Assets are compressed at
`gzip.BestCompression` once, at startup, and served from memory; each encoding gets the digest of its
own bytes as its ETag, because they are different representations. Every response says
`Vary: Accept-Encoding`.

## BREACH

The filter box reflects `q` into the page, and every form carries the CSRF value. With
compression, an attacker who can steer requests and observe sizes could recover that value one
character at a time. Each render now carries `base64(pad || pad XOR token)` with a fresh pad, the
pattern Django and Rails use. Nothing constant is left in the body. The session cookie never was.
