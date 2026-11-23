# Design

## Where the reason comes from

The verifiers already tell every case apart; they collapse them into one `invalid credentials` error so a
client learns nothing. The reason is kept beside the error rather than inside it. `logRequest` puts an
`auth.RefusalNote` in the request context, the verifiers and the service write the reason into it, and
`logRequest` reads it once the request is answered. This is the pattern `noteTenant` already uses for the
request line's tenant. Nothing changes in the error a handler writes, so the client's answer cannot change,
and a verifier with no note in its context (the SSH listener's recheck, the TUI, a test) writes nothing.

The chain tries the bearer token before the cookie, so a later note replaces an earlier one, and a request
that goes on to authenticate clears the note. `no_credentials` is only noted when nothing more specific was.

A disabled actor used to be reported by the store lookup as "no such token". It now returns
`auth.Withdrawn`, which the verifiers turn into the same `invalid credentials` error and the
`actor_disabled` reason. Without it, a disabled actor's token is indistinguishable from a stranger's.

## Reason codes

Only states the code tells apart are named:

- An ended session is deleted, so it is `unknown_session`, the same as a forged cookie. There is no
  `session_revoked`.
- Token lookups are scoped to the host's tenant, so another tenant's token is `unknown_token` on a host bound
  to one tenant. `wrong_tenant` is what the router itself refuses on a shared host.
- `bad_password` is a usable account whose password did not match; `unknown_account` covers no account, a
  disabled one, a non-member and an account without a password. These are one code because the service
  tests them as one condition, `candidate.usable()`, before comparing the password at all.
- `scope_denied` and `project_denied` are told apart from the actor: a project-pinned token that holds the
  scope the action needs was refused for its project. Every other policy refusal is `scope_denied`.

## Naming a credential without logging it

- A token that was found is named by its id and name, which every listing already shows.
- No token prefix or hint is stored, and every token begins `tix_pat_`, so there is nothing more to show.
- A well formed token that matched nothing is named by `t-` and 16 hex of
  `SHA-256("token-ref:" + stored hash)`. This follows `auth.SessionRef`, the repo's existing convention for
  naming a credential in a record: a digest of the stored hash under its own label, so it is neither the
  value nor the hash a lookup accepts, and it cannot be matched against the `api_tokens` table. The value
  carries 256 random bits, so the digest cannot be used to guess it. 16 hex rather than 8 keeps it the same
  length as the session reference; it is a grouping key, not a secret.
- A malformed value gets no reference. It may be a password pasted into the wrong field, and a digest of a
  guessable value lets anyone with the log test guesses offline. The client address groups those attempts.
- A session cookie is named by `auth.SessionRef`, so a refused session can be matched to the audit entries
  it wrote while it worked. A cookie that is not the shape of a minted secret gets no reference, for the same
  reason as a malformed token.

## Repeats

`internal/logging.Dedupe` lets one record per key through per window and counts the rest, reported as
`suppressed` on the next record for that key. The key is client address, reason and token id, with an IPv6
address cut to its /64, since one host is usually handed a whole /64 and would otherwise get a key, and a line,
per address. An unmatched token's reference and a session reference are left out of the key on purpose: a
client sending random values would otherwise get a fresh key, and a line, per request.

A count nothing later carried is handed to a callback, which the router writes as an `auth refusals held back`
record through the same logger. `Allow` sweeps expired keys at most once a window, so an insert does not scan
the map, and `Run`, a server worker, sweeps once a window when no refusal arrives and reports every count left
at shutdown. The map holds 4096 keys. Once it is full, a new key shares one overflow bucket rather than evicting
a live one: evicting would reset a real noisy poller's count every time an attacker cycled addresses.

The repo's existing rate-limited record, the second-factor gate's audit entry, writes once an hour per actor.
That one is an audit row because the gate refuses a known person. A failed credential has no actor to audit,
and writing a row per failed request is what a misbehaving client must not be able to cause, so refusals go
to the log only. tix does not audit authentication failures elsewhere either.

## The request line

`client_ip` is `ClientIPFrom`, which `withForwarded` already resolved through the trusted proxy policy, and
`user_agent` is `core.TruncateUserAgent`, the 160 byte bound the activity log already uses. They are on every
request line rather than only refused ones, because a `404` or a `429` needs tracing too and the cost is two
short fields.

## Authorization refusals

A page can ask the service for something optional and carry on without it. A policy refusal is therefore
logged only when the request was answered `403`, and `no_credentials` only when it was answered `401`, so a
SCIM discovery document served to anybody is not logged as a refusal.

## SSH

The SSH listener already logs every refusal with the key fingerprint and source address, which honours the
PROXY protocol. Those lines gain `reason`. They are not deduplicated: the listener's per-source rate limit
already bounds how many connections, and so how many lines, one source can cause.
