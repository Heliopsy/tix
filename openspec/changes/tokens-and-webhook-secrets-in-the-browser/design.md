# Design

## Which scope gates the wider listing

`ListTokens` already authorizes `authz.ActionTokenAdmin`, so `token:admin` is required to reach
`/admin/tokens` at all. Gating the wider listing on `token:admin` would therefore gate it on nothing: every
reader who can see the screen holds it, and the requirement that a reader without the gate keeps the
narrow view would be unsatisfiable.

The gate is `tenant:admin`. It is the scope that already means "administers this installation" in this
codebase — `authz.ActionServerRead` and `ActionRetentionWrite` map onto it for the same reason, and
`render.go`'s `advancedMode` already reads it off the actor to decide what the navigation offers. A reader
holding `token:admin` alone keeps exactly the screen they had.

The check is `actor.HasScope(core.ScopeTenantAdmin)` in the handler, not a call into `internal/authz`:
`internal/service` is the policy's only caller by architecture invariant, and this is not an authorization
decision. It decides what to *offer*. The authorization decision stays where it was, in
`(*Local).ListTokens` and `(*Local).RevokeToken`, both of which already permit a `token:admin` holder to
reach the tenant's tokens. That ordering matters: the browser is strictly narrower than the service, so a
mistake here can only ever hide something, never grant it.

This leaves one real shortfall, deliberately. A reader with `token:admin` and without `tenant:admin` can
list and revoke another actor's token from the command line and not from the browser. That is the
`Limitation` the capability registry now records — the same slot, a different claim.

## Reaching the tenant's tokens without a new service method

`core.Service.ListTokens` takes one actor identifier, and `internal/core` is the frozen contract. A
tenant-wide listing would be a new `Service` method, and by invariant every new `Service` method needs a
`capability.Registry` entry with CLI, HTTP and Web bindings — a new CLI command and a new HTTP route for a
listing the CLI can already assemble.

So the handler lists the directory and then lists each actor's tokens. `ListActors` needs only an
authenticated caller, the directory is the size of a tenant's staff and agents, and this screen is read by
an administrator during an incident rather than in a loop. The reader's own identifier is appended when the
directory does not contain it, so a reader whose actor row is not in the listing still sees their own
tokens rather than silently none.

Rows are ordered by owner and then by creation time, so one actor's credentials sit together. Interleaved
by age, a listing spanning five actors is a wall nobody can scan.

## Telling the two revocations apart

An audit entry already records the acting actor, so "who performed it" was never the missing half. What was
missing is whether it was theirs to perform.

Two candidate shapes: a field in the payload, or a distinct action. The action wins, because the audit trail
is filtered by action — `core.AuditFilter.Actions`, `tix audit --action`, the activity screen's own filter —
and a boolean buried in a payload is not reachable by any of them. `token.revoke_other` is a new action
value, not a new subject type, so nothing that reads the trail needs to learn a new shape;
`web.actionVerb` already reduces it to "delete" by its `revok` prefix.

Recording whose it was needs the token before it is revoked, which is `Tx.GetToken` — the same query as
`GetTokenByHash` with a different predicate, in both dialects. The read is inside the mutation's
transaction, so the owner recorded is the owner at the instant of revocation.

A guard for "the other case is recorded as such" proves nothing without a guard for the ordinary case
beside it: a trail that recorded `token.revoke_other` for every revocation would satisfy the first and be
useless. Both are pinned.

## The CLI default, and why `never` is a value rather than an empty string

`--expires` now carries the literal default `"90d"`, so `--help` prints `(default "90d")` and the skill's
stated default is checkable against the flag rather than against prose. An absent flag and an explicitly
empty one both resolve to that default; only the word `never` produces a token with no expiry. Leaving the
empty string meaning "never" would have made the breaking change invisible to exactly the caller it breaks,
whose script passes `--expires ""` from an unset shell variable.

The grammar grew a duration because the default has to be expressible in the flag it defaults, and `90d` is
how a person says it. `core.ParseDuration` already reads `30d`, so a date is distinguished from a duration
by trying the duration first and falling through — `2027-01-01` is not a duration and reaches `parseTime`
unchanged.

The expiry is resolved in `cmd` against `time.Now()` rather than in the service against its clock. It is
the caller's "ninety days from now", the same way the browser form's choices are the reader's, and the
service's contract stays an absolute instant that both surfaces compose. The cost is that the value is not
reproducible under a fake clock, so the CLI guards assert the day rather than the instant.

The HTTP API is left with no default. It takes `expires_at` as sent and mints a non-expiring token when the
field is absent. An API client composes that field rather than typing it, and a transport that invents a
value nobody set would make the same request mean different things on different releases.

## One secret region, two screens

The token screen's one-time secret already had everything the webhook screen needs: a `section.secret`
rather than a `.flash`, because a value nobody can fetch again must not be the most transient element on
the page; a `.copyline` that wraps instead of overflowing; and the one generic `data-copy-target` delegate
in `assets/copy.js`, so the value is still selectable in full with scripting off.

It is now a `secret` partial in `partials.html`, parameterised by a `oneTimeSecret` carrying the element
id, the heading, the note and the value. The stylesheet is untouched: `.secret` and `.copyline` were
already written against this markup. Both screens carry their value to the render through a one-time cookie
on their own path, so the value never reaches a URL or the audit trail, and one screen's cookie cannot be
read by the other.

### What happens when the operator supplies their own secret

Nothing is shown. `PutWebhook` returns `Secret` only when it generated one, which is the correct signal:
tix has learned nothing the operator does not already hold, and echoing a value they chose back into a
response would put a credential on a page for no gain. `TestWebhookSecretIsNeverDisplayed` already pins
that a *stored* secret never appears, and is unaffected.

Saving an existing endpoint with the field blank keeps the secret it has rather than rotating it — that is
`PutWebhook`'s existing behaviour, and nothing is shown then either, because nothing was generated. The
help text now states all three cases, because the whole defect here was help text that was a promise
nothing kept.
