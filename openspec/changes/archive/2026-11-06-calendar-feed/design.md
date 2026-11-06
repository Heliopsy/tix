# Design

## A feed table rather than an API token with a new scope

The obvious reuse is an API token carrying a `calendar:read` scope. It does not fit, for four
reasons:

- An API token authenticates the HTTP API from a header. A feed secret travels in a URL, which a
  calendar provider stores, fetches from its own servers and may log, so it must not be able to
  authenticate anything except the feed. A separate table and a separate prefix (`tix_cal_`) make
  that structural rather than a scope check somebody has to remember.
- A token's scopes are frozen when it is minted. A feed has to follow its owner's membership, so a
  demotion or removal lands on the next fetch.
- A feed carries a filter, and there is one per person. Token names are unique per tenant, so two
  people calling theirs `calendar` would collide, and token administration would fill with feeds.
- Rotation keeps the filter and replaces the secret, which is a different lifecycle from revoke and
  mint.

The secret is stored as a SHA-256 hash, like every other credential, so it is shown once at
creation and rotation. A partial unique index keeps one live feed per actor.

## VEVENT, not VTODO

Google Calendar ignores `VTODO` in a subscribed feed entirely, Outlook drops it on import, and Apple
Calendar shows subscribed `VTODO` nowhere (Reminders reads tasks only over CalDAV). An all-day
`VEVENT` is drawn by all three, so each task is one event. RFC 5545 gives `VEVENT` no `COMPLETED`,
so the state maps as:

| Task | `STATUS` | `SUMMARY` |
| --- | --- | --- |
| open (any non-terminal category) | `CONFIRMED` | `REF: title` |
| finished (terminal, category done) | `CONFIRMED` | `✓ REF: title` |
| abandoned (terminal, category cancelled) | `CANCELLED` | `REF: title` |

`TRANSP:TRANSPARENT` keeps a due date from marking the day busy. `SEQUENCE` is the task's version
and `LAST-MODIFIED` and `DTSTAMP` its update time, so the document is a pure function of the tasks
and its ETag only changes when they do.

## Dates

A due date is an instant, and a bare date given on any surface is stored as midnight UTC. The feed
writes the UTC calendar date as a floating `VALUE=DATE` with `DTEND` the next day, which every
client draws on that day whatever zone it is in. A due time other than midnight still lands on its
UTC date; the feed is about days.

## The read

`ReadCalendarFeed(token)` runs with no actor. It checks the token's shape, looks the hash up in the
tenant the request's Host resolved to, loads the owner and their membership, and builds a reading
actor holding the intersection of the owner's role with `task:read`, `project:read` and
`workflow:read`. The listing then goes through `ListTasks` like any other read, so the policy that
decides what the owner may list decides what the feed lists. Every failure, malformed, unknown,
revoked, removed or disabled, is the same not found.

The listing pages by keyset over `due_at` descending and stops at 2000 entries. The default filter
excludes in SQL the states that are terminal in every workflow defining them and drops the rest of
the finished tasks by category in memory.

## Caching

The handler hashes the rendered body for a strong ETag, answers `304` to a matching
`If-None-Match`, and sends `Cache-Control: private, max-age=300`. The calendar carries
`REFRESH-INTERVAL` and `X-PUBLISHED-TTL` of one hour as a hint to clients that honour it.
