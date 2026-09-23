# Filtering

One expression language. What you type into the terminal interface's filter bar is what
`tix task ls --filter` takes -- both go through the same parser -- and the HTTP API carries the same terms as
query parameters. A filter that selects a set of tasks in one place selects the same set in the others.

The browser's filter box is the exception for now: it has its own parser, which does not yet understand `-` or
`~`. A negated term there is refused with an error naming the unknown key; a `~` term is silently read as free
text and matches nothing. Use the CLI or the terminal interface until that parser catches up.

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
| `project:KEY`, `p:KEY` | project key | yes | no |
| `status:NAME`, `s:NAME` | workflow state | yes | no |
| `tag:NAME` | carries the tag | yes | no |
| `assignee:ID`, `a:ID` | assigned actor | yes | no |
| `creator:ID` | who created it | yes | no |
| `claimed-by:ID` | who holds the lease | yes | no |
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

## Related

- [scripting.md](scripting.md) for NDJSON output and piping listings
- [workflows.md](workflows.md) for custom field filters and which of them are indexed
- [api.md](api.md) for the rest of the query parameters
