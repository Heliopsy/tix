# Calendar feed

Your due dates, in the calendar you already plan your week in. Each person can hold one calendar feed: a
private address that Google Calendar, Apple Calendar or Outlook subscribes to and polls. Every task the feed
selects that has a due date becomes an all-day event on that date, linking back to the task.

## Getting the address

From the browser: **Settings**, then **Calendar feed**, then **Create feed**. The next screen shows the
address with a Copy button, and a link that asks this device's calendar to subscribe.

From the command line, connected to a server:

```console
$ tix calendar create
the feed address is shown once and cannot be retrieved again
...
url: https://tix.example.com/api/v1/calendar/feeds/tix_cal_....ics
```

Over the API: `POST /api/v1/calendar`; see [api.md](api.md#calendar-feed).

Then paste the address into your calendar:

- **Google Calendar**: Other calendars, then From URL.
- **Apple Calendar**: File, then New Calendar Subscription.
- **Outlook**: Add calendar, then Subscribe from web.

The address is shown once, when the feed is created or rotated, because only its hash is stored, the same
as an API token. If you lose it, rotate the feed for a new one.

## What it lists

Without a filter, the feed lists the tasks **assigned to you that are not finished**: any state that is not
terminal in the task's own workflow. With a filter, it lists whatever the [filter expression](filtering.md)
selects, finished tasks included. Either way only tasks with a due date appear, and never more than 2000,
the latest due first.

```sh
tix calendar create --filter 'project:infra -status:done'
tix calendar filter 'tag:ops'      # change it; the address stays the same
tix calendar filter 'assignee:me'  # "me" is the feed's owner, finished tasks included
tix calendar filter                # back to "assigned to me, not finished"
```

A filter is checked when it is saved: a project or status that does not exist is refused there rather
than answered with an empty calendar forever after.

## Who can read it

Anybody holding the address reads what it lists, without signing in, because a calendar app cannot sign
in. Treat it like a password. The address can do nothing else: its secret starts `tix_cal_` and is not an
API token, so it authenticates no other route, and an API token in a feed address reads nothing.

The feed reads **as you**, with no more than reading tasks, projects and workflows, and it asks again on
every fetch. A feed never lists a task you could not list yourself. When you are removed from the tenant,
your account is disabled, or the feed is rotated or revoked, its address answers `404` from the next fetch.

```sh
tix calendar rotate    # new address, same filter; the old one stops at once
tix calendar revoke    # stops it; subscribed calendars keep what they last fetched
tix calendar show      # whether you have one, since when, and its filter
```

The server log records a feed request with the secret replaced by `[redacted]`.

## What an event carries

| Property | Value |
| --- | --- |
| `UID` | `task-<id>@tix`, stable for the life of the task, across renames, new dates and moves |
| `SUMMARY` | `INFRA-12: the title`, with a check mark (✓) in front once the task is finished |
| `DTSTART`, `DTEND` | the due date as an all-day `VALUE=DATE`, ending the next day |
| `DESCRIPTION` | the reference, state and priority, and the task's address |
| `URL` | the task's page |
| `STATUS` | `CANCELLED` for a task in a cancelled state, `CONFIRMED` otherwise |
| `SEQUENCE`, `LAST-MODIFIED`, `DTSTAMP` | the task's version and its last update |
| `CATEGORIES` | the task's tags |
| `TRANSP` | `TRANSPARENT`, so a due date never marks your day as busy |

### Why events and not to-dos

iCalendar has a `VTODO` component made for tasks, and none of the three calendars people actually use shows
it from a subscription: Google Calendar ignores it, Outlook drops it, and Apple Calendar only reads tasks
over CalDAV. An all-day `VEVENT` is drawn by all three. `VEVENT` has no `COMPLETED` property, so a finished
task is marked in its summary instead, and `CANCELLED` is kept for work that was abandoned, since calendars
draw a cancelled event struck through or hide it.

### Dates and time zones

A bare due date, from any surface, is stored as midnight UTC, and the feed writes that UTC date as a
floating all-day date. A floating date is the same day in every zone, so a task due on the 5th sits on the
5th whether you open the calendar in Sofia or in San Francisco. A due time other than midnight still lands
on its UTC date: the feed is about days.

## Polling and caching

Calendar apps decide how often to fetch; Google takes hours, Apple and Outlook let you choose. The feed
suggests an hour with `REFRESH-INTERVAL` and `X-PUBLISHED-TTL`. Each answer carries a strong `ETag` and
`Cache-Control: private, max-age=300`, and a request repeating the `ETag` in `If-None-Match` gets
`304 Not Modified` with no body. The document is a function of the tasks alone, so the tag only changes
when a listed task does.
