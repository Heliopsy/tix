# Design

## Where the settings live

`sync_sources.config` (JSON, JSONB on PostgreSQL) and `sync_sources.mapping_path` were created by 0001 and
never written. They are used as they are, so part one needs no migration. `config` holds a flat map of
strings; `page_size` is stored as its decimal text so the map has one value type.

## Allowed keys per system

| System | Keys |
| --- | --- |
| generic | `file`, `page_size` |
| jira | `url`, `project`, `query`, `page_size` |
| openproject | `url`, `project`, `query`, `page_size` |

`mapping_path` is a field of its own on every system. These are exactly the settings the adapters read from
`SourceConfig`. Credential keys (`token`, `user`, `password` and anything `isSecretKey` matches) are checked
first, so sending one gets the credential refusal rather than "unknown key". A source updated by `PUT`
replaces its settings whole, like its name and system.

## Precedence

The run builds the environment reader `LoadSourceConfig` already takes from the stored settings layered over
`os.Getenv`: a lookup of a variable whose setting is stored answers the stored value, any other lookup goes
to the environment. Credentials are never stored, so they always come from the environment. Nothing else in
the run changes, and a source with no stored settings reads exactly what it read before.

## The cursor write keeps the settings

The run records its cursor through the same store `PutSyncSource` as registration, which writes every
column. It carries the source's settings through, or a run would wipe them.

## Dropping `indexed`

`ALTER TABLE field_defs DROP COLUMN indexed` on both engines. SQLite has supported `DROP COLUMN` since 3.35
and the column is in no index, constraint, trigger or view, so it is the simple form of the statement rather
than a table rebuild. The PostgreSQL translation keeps `field_defs.indexed` in its boolean column list so
0001 still creates the column with the right type on a fresh database before 0029 removes it.

No decoder in the HTTP API, bundle import or snapshot import rejects unknown keys, so `indexed` in old input
is ignored without code. Tests hold each path to that, so a later switch to strict decoding has to decide
about the key on purpose.
