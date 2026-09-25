# Filtering

One expression language. What you type into the terminal interface's filter bar is what
`tix task ls --filter` takes -- both go through the same parser -- and the HTTP API carries the same terms as
query parameters. A filter that selects a set of tasks in one place selects the same set in the others.

The browser's filter box ran a second parser of its own once, which is why this paragraph used to warn you off
it. The two disagreed quietly: when the language gained negation and weak matching, `-tag:ops` failed there
with a clear error, which is survivable, but `title~api` was swallowed as free text and matched nothing, which
is an empty board, no error, and a reader who reasonably concludes their tasks are gone. The browser bar, the
terminal interface's bar and `tix task ls --filter` now run the same parser and cannot drift again.

## The shape of an expression

Space-separated terms, combined with AND. A term is `key:value`, and a bare word is free text.

```sh
tix task ls --filter 'project:infra status:todo priority:high'
```

Quote a value that holds spaces. A quote that *opens* the token makes the whole thing free text instead, which
is how you search for a literal colon:

```sh
tix task ls --filter 'title:"rotate the API keys"'     # a term, with a multi-word value
tix task ls --filter '"status:todo"'                   # free text, matching the characters "status:todo"
```

## Negation: a leading `-`

Prefix the whole term with `-` to exclude instead of select.

```sh
tix task ls --filter '-tag:ops'                        # everything that is not tagged ops
tix task ls --filter 'status:todo -tag:chore'          # to do, excluding chores
tix task ls --filter '-assignee:01M3... -priority:low' # not theirs, not low priority
```

Negatable: `project`, `status`, `tag`, `assignee`, `creator`, `claimed-by`, `priority`, `is:`, `title`, `body`,
`text`, and a bare word.

Not negatable: `sort`, `limit`, `parent`, `due-before`, `due-after`. Those name a shape of the listing rather
than a set of tasks, so "not sorted by title" is not a question with an answer; asking is a usage error, not a
silently ignored term.

Rules worth knowing:

- **An exclusion beats an inclusion of the same value.** `tag:ops -tag:ops` returns nothing. The terms are
  ANDed, and that is the honest result rather than a guess at what you meant.
- **A missing value is not an excluded value.** `-assignee:alice` returns unassigned tasks. Not being assigned
  to Alice is not the same as being assigned to somebody else, and SQL's `NOT IN` would have dropped them.
- **`-is:` inverts the sense.** `-is:claimed` means the same as `is:unclaimed`.

### Why `-tag:ops` and not `tag:!ops`

The negation belongs to the term, not to the value. `status:!done` would put a sigil inside a value that could
legitimately start with `!`, and it gives no spelling at all for negating a bare word. A `NOT` keyword would
need operator precedence and parentheses, which this flat conjunction deliberately does not have. A leading `-`
is one character, applies uniformly to every negatable term, and is what people already type into search boxes.

## Weak matching: `~` in place of `:`

`:` is exact, `~` is weak. A weak match succeeds when the value appears anywhere inside the field; an exact
match succeeds only when the value is the whole field. Both ignore case.

```sh
tix task ls --filter 'title~api'                # any title containing "api", anywhere
tix task ls --filter 'title:"deploy the api"'   # only that exact title
tix task ls --filter 'body~rollback'            # the body
tix task ls --filter 'text~gateway'             # either field
tix task ls --filter '-title~wip'               # every title that does not contain "wip"
tix task ls --filter '-spam'                    # a negated bare word is a negated weak match
```

`~` applies only to `title`, `body` and `text`. `status~todo` is a usage error: a status is a value from a
fixed set, and a substring of one is not a meaningful question.

Wildcard characters in the value are literal. `title~100%` finds the task whose title contains `100%`, and does
not match `1000 things`.

## Weak match and the storage engine

**A weak match behaves identically on SQLite and on PostgreSQL.** It is a case-insensitive substring test
(`LOWER(col) LIKE LOWER(?)`) built once and run unchanged on both, and a shared conformance corpus is asserted
against both engines in CI. That is the whole reason it exists as its own term rather than being folded into
the free-text query.

The one residue is case folding outside ASCII: SQLite's `LOWER` is ASCII-only, PostgreSQL's is locale-aware, so
`title~ÄPFEL` may match `Äpfel` on PostgreSQL and not on SQLite. Every ASCII value -- which is every status,
every tag and most titles -- folds identically.

**Free text is different, and always has been.** A bare word, or `text:value`, feeds each engine's *native*
search: a `tsvector` match on PostgreSQL, which is word-based and stemmed, and a `LIKE` on SQLite, which is a
substring. `tix task ls --query deploy` therefore matches `deployment` on SQLite and (depending on the
dictionary) may not on PostgreSQL. That divergence predates this document; it is kept because the native search
is what makes a large PostgreSQL install fast. If you need an answer that does not depend on the engine, use
`text~deploy` rather than a bare `deploy`.

## Every term

| Term | Meaning | Negatable | Weak |
| --- | --- | --- | --- |
| `project:KEY`, `p:KEY` | project key or id, in any case | yes | no |
| `status:NAME`, `s:NAME` | workflow state, in any case | yes | no |
| `tag:NAME` | carries the tag | yes | no |
| `assignee:REF`, `a:REF` | assigned actor, by handle or id | yes | no |
| `creator:REF` | who created it, by handle or id | yes | no |
| `claimed-by:REF` | who holds the lease, by handle or id | yes | no |
| `priority:NAME` | `highest`…`lowest`, or 1–5 | yes | no |
| `title:VALUE` | whole title | yes | `title~` |
| `body:VALUE` | whole body | yes | `body~` |
| `text:VALUE`, `q:VALUE` | title or body | yes | `text~` |
| `due-before:DATE`, `due-after:DATE` | due bounds | no | no |
| `parent:REF`, `parent:none` | subtasks of, or roots | no | no |
| `is:claimed`, `is:unclaimed`, `is:blocked`, `is:unblocked`, `is:deleted`, `is:root` | state predicates | yes (except `is:root`) | no |
| `sort:FIELD`, `limit:N` | listing shape | no | no |
| bare word | free text, engine-native | yes (becomes a weak match) | n/a |

Dates take `YYYY-MM-DD`, `YYYY-MM-DD HH:MM`, or RFC 3339.

A custom field is `field.NAME:VALUE`, which the whole expression language carries, so `--filter
'field.severity:sev1'` works on the command line as well as over HTTP. The value is read as JSON when
it parses as one, so `field.count:3` filters on the number and `field.paged:true` on the boolean,
falling back to the literal text. A custom field term cannot be negated or weakly matched yet, and
asking is a usage error rather than a term quietly dropped. See
[workflows.md](workflows.md#indexed-versus-scanned-field-filters) for what indexing one costs.

## A term that names nothing

A filter term naming something this tenant does not have fails the listing as `not_found`, exit 3. It
used to return an empty page and exit 0, which is the one answer that cannot be told apart from a
correct one: `tix task ls -p nosuchproject` printing `[]` reads as "that project has no work" rather
than "there is no such project", and a person, or an agent, acts on it.

```console
$ tix task ls -p nosuchproject
error: not_found: no project with key or id "nosuchproject"

$ tix task ls --status nosuchstatus
error: not_found: no workflow of this tenant defines a status "nosuchstatus"; the defined statuses are blocked, cancelled, doing, done, todo
```

This applies to `project`, `status`, `parent`, `assignee`, `creator` and `claimed-by`, on the
inclusion side and the exclusion side alike, and to the flags that carry the same terms. A `priority`
outside the range was already a usage error and needs no lookup.

**Case.** A project key and a status are matched without regard to case, because the store matches
both exactly, and `-p INFRA` or `--status TODO` selecting nothing is the same silent empty answer by
another route.

**Handles.** An actor reference is classified by shape: a value of exactly the length and alphabet of
a generated identifier is an identifier and is passed through without a lookup, so an actor of
another tenant stays assignable and filterable. Everything else is a handle, resolved against this
tenant's directory. Each distinct reference is resolved once per listing, and one cache is shared
between the inclusion and exclusion lists, so the cost is proportional to the references the filter
names rather than to the rows it returns.

**A status is checked against the tenant, not the listing.** The vocabulary is the union of the
states of every workflow the tenant defines. Workflows need not agree, so a status valid in one
project may be undefined in another, and checking a term against only the workflow of the project in
scope would refuse a query somebody legitimately meant: a listing spanning several projects, or the
whole tenant, may reasonably name a status only one workflow defines and should answer with that
workflow's tasks. A status some workflow declares is a real thing this listing may simply not reach,
and an empty page there is the truthful answer. A status no workflow declares can describe no task
anywhere and is a typo.

**A tag is the deliberate exception**, and it is recorded here so nobody later tidies it away. A tag
is free-form and comes into being by being applied, so there is no declaration against which one
could be called unknown: the only evidence a tag exists is that some task carries it, which makes "no
such tag" and "a tag with no tasks" the same state. Refusing the first would refuse the second, so
removing the last task from a tag would turn a working filter into a failure, a filter written before
the tag is first applied would fail rather than wait, and `-tag:ops`, whose whole purpose is that the
result carries none of it, would fail for naming a tag that is absent. `tix task ls -l nosuchtag`
returns an empty page and exit 0.

**A custom field key is the one remaining member of this family that still answers empty.** `--filter
'field.nosuchfield:high'` returns `[]` with exit 0 today. A field key is declared, unlike a tag, so
this is a gap rather than a decision; it is written down here so the difference is not mistaken for
one.

**A failed listing writes no document.** The error is the whole of the answer, so standard output
stays empty rather than carrying `[]` beside an error on standard error, which a pipeline reading
only stdout would read as a valid empty answer. Where records were already streamed before the
failure, a bracketed format is left unterminated on purpose, so a consumer parsing stdout gets a
syntax error rather than a short listing it would believe; NDJSON and YAML have no terminator to
withhold, and the lines already written stand, each true on its own.

In the browser, the same refusals are reported on the filter bar with the expression still in the
box, rather than replacing the listing with an error page. See
[web-ui.md](web-ui.md#the-task-list).

## Flags still work

`--filter` adds to the other flags rather than replacing them, so an alias carrying a house filter still
composes:

```sh
alias mine="tix task ls --filter '-tag:archived is:unblocked'"
mine --status todo --tag ops
```

## Over the HTTP API

Negated terms are `not_`-prefixed repeatable parameters; text terms are a repeatable `text` parameter carrying
the term in the same spelling the expression uses.

```console
$ curl -s -H "Authorization: Bearer $TIX_TOKEN" \
    'https://tix.example.com/api/v1/tasks?status=todo&not_tag=ops&text=title~api&text=-body~legacy'
```

| Parameter | Meaning |
| --- | --- |
| `not_project`, `not_status`, `not_tag`, `not_assignee`, `not_creator`, `not_claimed_by` | repeatable exclusions |
| `not_priority` | repeatable, numeric 1–5 |
| `text` | repeatable; `title~api`, `body:exact`, `any~value`, each optionally `-` prefixed |

Everything is keyset-paginated as usual: there is no `OFFSET`, and the `cursor` a page returns is what fetches
the next one. See [api.md](api.md).

## Filtering activity

The activity feed is a different set of rows, so it takes a different set of terms: an audit entry has a kind
and a source, and no status, tag or due date to ask about. The spelling is shared -- the same quoting, the same
leading `-` for negation -- so only the vocabulary changes.

| Term | Meaning |
| --- | --- |
| `actor:ID` | who made the change |
| `kind:TYPE` | what kind of record it happened to: `task`, `comment`, `project`, `workflow`, `user`, … |
| `action:NAME`, `type:NAME` | the recorded action, such as `task.created` |
| `source:NAME` | the surface it arrived through: `cli`, `web`, `api`, `tui`, `system` |
| `text:VALUE`, `q:VALUE`, bare word | free text over the action, the kind, the source and the before and after snapshots |

Every term negates with a leading `-`. Values of one term are alternatives, different terms are conjunctions,
and every free-text word must appear somewhere in the row.

```sh
tix audit ls --filter "kind:task source:web -action:task.deleted"
tix audit ls --filter "actor:01J0 certificates" -o ndjson
```

`--filter` and the `tix audit ls` flags both apply, so a filter never widens what `--subject-type`, `--actor`
or `--since` selected. The structured terms are answered by the store; the free text is applied over the rows
it returns, so a listing keeps reading pages until it has a screenful or has read eight of them, and says on
stderr how much it discarded.

In the terminal interface, `v` opens the activity view and `/` filters it with the same expression, `C` clears
it. One term is missing there: an event carries no source, because the event log records what happened rather
than which surface asked for it, so `source:` is refused on the live tail by name and belongs to
`tix audit ls`.

## Related

- [scripting.md](scripting.md) for NDJSON output and piping listings
- [workflows.md](workflows.md) for custom field filters and which of them are indexed
- [api.md](api.md) for the rest of the query parameters
