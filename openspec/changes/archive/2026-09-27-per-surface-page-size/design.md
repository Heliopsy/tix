# Design

## Why two keys rather than one

A single key would have to be read by the CLI, the browser and the API, and the one number it holds cannot be
right for all of them: the reason fifty rows is too many in a browser window is physical, and it does not
transfer to a 200-column terminal or to a pipe. Two keys make the surface the unit of configuration, and each
key's name says which surface it governs. They both ship as 25, so an install that configures nothing gets
one number everywhere and can then differ where it wants to, rather than having to learn three numbers
before it can predict anything.

`cli.` and `web.` are new namespaces rather than a row under `output.`, because `output.` is already
cross-surface: `output.time_format` and `output.timezone` are read by the browser as well as the command
line. A per-surface number under a shared namespace would be the one thing a reader could not deduce from
where it lives.

## Where the contract default stays

`core.DefaultPageLimit` is unchanged at 50. It answers a different question: what a caller who named no limit
gets. Every surface a person looks at now names a number, so the only remaining callers of that fallback are
the HTTP API and internal listings that page to `core.MaxPageLimit` anyway. `core.DefaultDisplayLimit`, 25, is
added beside it as the shipped value both keys default to and the browser handler's own fallback, so the
number lives in one place.

## Refused rather than clamped

A page size below 1 or above `core.MaxPageLimit` fails `config.Validate` with the key and the layer named.
Clamping would accept a zero, which reads as "show nothing", and silently turn it into a screenful: the
operator who typed it would keep believing the key means something it does not. `core.Page.Normalize` still
clamps at the contract boundary, where the caller is a client rather than a person who can be told.

## How the CLI applies it

`globals.pageSize` returns the typed `--limit` when `Flags().Changed` reports it and the configured value
otherwise. `Changed` is the discriminator because the flag declares a default equal to the configuration
default, and emptiness cannot separate "nobody typed it" from "somebody typed the default".

`tix task ls` also accepts `limit:` inside `--filter`. It cannot read the parsed filter to find out whether
the expression named one: `query.Parse` ends in `Validate`, which has already replaced an unset limit with
the contract default, so "asked for nothing" and "asked for fifty" arrive identical. This is not theoretical
-- it is why the branch that was meant to apply the flag was dead code, unreachable and unnoticed because the
value it would have written matched what `Normalize` had already written. The configured size is therefore
prepended to the expression as its first term, where a `limit:` the operator wrote later overrides it, and a
typed `--limit` overrides both.

## The browser's request-time size

`limit` is a query parameter, bounded to 1..`core.MaxPageLimit`, and the pager carries it in its own links
the way it carries the filter and the sort. A value that is not a usable number falls back to the configured
size rather than failing the screen, for the same reason an unwalkable cursor trail is dropped rather than
repaired: the parameter arrives from the address bar, so the worst an unreadable one may cost is the size
that was asked for.
