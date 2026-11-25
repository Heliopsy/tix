# Tasks over CalDAV

Your tasks, as to-do lists in the apps people already keep them in: Apple Reminders on an iPhone or a Mac,
Thunderbird's Tasks, tasks.org, jtx Board or OpenTasks through DAVx5 on Android, and Evolution. `tix serve`
answers CalDAV (RFC 4791) for them under `/dav/`, and each person signs their app in with an app password of
their own.

**For now it is read-only.** A client shows the lists and keeps them current, and shows them as lists it
cannot change: a task cannot be ticked off, edited, added or removed from the client yet. Writing back is a
later phase. Until then a task is changed in tix, and the client picks the change up on its next sync.

CalDAV here is about to-dos. Due dates as events in Google Calendar, Apple Calendar or Outlook are the
calendar feed ([calendar.md](calendar.md)), which needs no app password and works where CalDAV does not.

Not served: events (`VEVENT`) over CalDAV, CardDAV, scheduling and invitations (RFC 6638), creating
calendars from a client (`MKCALENDAR`), sharing, and push. A client polls on its own schedule.

## App passwords

A client signs in with HTTP Basic. The username is your sign-in email, or your handle in the tenant, in any
case. The password is an **app password**, never your account password.

An app password is an API token (`tix_pat_...`) carrying the `caldav` scope and nothing else. It works only
under `/dav/`, and only for you: it is minted for your own person actor. Any person who can read tasks may
create one, including a member without token administration and somebody who signs in only through single
sign-on, who has no account password to type anyway. An agent cannot have one; it reads tasks with its own
token.

From the command line, connected to a server:

```sh
tix dav password create iPhone
tix dav password create desktop --collections projects
tix dav password list
tix dav password revoke ID
```

`create` prints the password once, with the address and the username to type. Only its hash is kept, as for
every token, so a lost one is revoked and replaced, not recovered. `tix dav show` prints whether this server
answers CalDAV, the address and your username, without creating anything.

The flags of `create`:

- `--collections assigned|projects|all` chooses which lists the client sees (see [Collections](#collections)).
  `assigned` is the default.
- `--read-only` keeps the password from changing tasks once CalDAV accepts writes. Today every password reads
  only, so it changes nothing yet.
- `--expires DURATION` stops it after that long; `never` is the default. This differs from `tix token
  create`, which defaults to ninety days, because a phone account that stops syncing one day with no prompt
  looks broken rather than expired.

From the browser: **Settings**, then **Tasks in other apps**. The section shows the address and your
username, lists your app passwords with their collections, when each was created and last used, and has a
Revoke button on each behind a confirm dialog. Its create button asks for a name, the collections and
read-only, and the next screen shows the password once beside a Copy button.

Over the API: `POST /api/v1/dav/passwords` with `{"name": "iPhone"}` and optionally `collections`,
`read_only` and `expires_at`, answering `201` with the `password` once, the `username` and the `url`.
`GET /api/v1/dav/passwords` lists them without secrets, revoked ones included, and
`DELETE /api/v1/dav/passwords/ID` revokes one. `GET /api/v1/dav` answers whether CalDAV is on (`enabled`, and
a `reason` when it is not), the `url`, the exact `username`, whether your DAV requests may change tasks
(`write`, false for now) and the `collections` choices. [api.md](api.md#caldav) has the routes.

The name you give is yours: it must differ from your other live app passwords, and two people may both call
theirs `iPhone`. The token behind it is named `dav-HANDLE-NAME`, with a short suffix when that is taken,
which is what an administrator sees in `tix token ls`. An administrator revokes anybody's with `tix token rm
ID`; `tix token create --scope caldav` is refused, so there is one way to make one.

### What a password may read

The password is not its own authority. On every request it reads as you, with your current role in the
tenant, cut down to reading tasks, projects and workflows. A demotion lands on the next request. Removal from
the tenant or a disabled account stops it at once, and so does revoking it. It can never read more than you
could.

Creating and revoking one write the ordinary `token.create` and `token.revoke` audit entries and
`token.created` and `token.revoked` events, with `caldav` in the scopes, so a webhook sees them too.

## Addresses

Give a client the server's address, `https://tix.example.com`. Clients that support RFC 6764 ask
`/.well-known/caldav` first, which answers `301` to `/dav/`. A client that wants a full URL is given
`https://tix.example.com/dav/`, which is what `tix dav show` and Settings print. From there the client finds
the rest on its own:

```text
/dav/                                   who you are signed in as
/dav/TENANT/principals/ACTOR/           you, in that tenant
/dav/TENANT/calendars/ACTOR/            your lists
/dav/TENANT/calendars/ACTOR/assigned/   Assigned to me
/dav/TENANT/calendars/ACTOR/p-PROJECT/  one per project
```

`TENANT` is the tenant key. `ACTOR` and `PROJECT` are lowercase identifiers rather than your handle or a
project key, so renaming a project, changing its key or changing your handle does not look to a client like
one list deleted and another created. Changing the **tenant key** does change every address: the client's
stored URLs then answer `404`, and the account has to be removed from the client and added again.

On a host serving several tenants, the tenant comes from the key in the path, and for `/dav/` itself from
the tenant the app password belongs to. A path naming another tenant, another person, a project you cannot
read or a task that is not in the list answers `404`, the same answer as a path that never existed.

`caldav.enabled: false` turns the whole surface off: `/dav/` and `/.well-known/caldav` answer `404`, and the
existing app passwords can still be listed and revoked. See [configuration.md](configuration.md).

## Collections

Each list a client shows is a calendar collection holding only to-dos (its component set is exactly
`VTODO`). There are two kinds:

- **Assigned to me** (`assigned`): the tasks assigned to you, in every project you can read.
- **One per project** (`p-PROJECT`): that project's tasks, named after the project and coloured with its
  colour. An archived project has none.

Each app password chooses, when it is created:

| `--collections` | The client shows |
| --- | --- |
| `assigned` (default) | Assigned to me alone |
| `projects` | one list per project you can read |
| `all` | both. A task assigned to you is then in two lists, Assigned to me and its project's |

A phone usually wants `assigned`, so it does not show every project as a list full of other people's and
agents' work. A desktop client wants `projects`. Changing the choice means a new app password, since clients
cache the list of lists.

A list holds a task when it is not in the trash, its state is not in the cancelled category, and it is open
or was completed within `caldav.completed_window` (14 days by default). A finished task stays long enough to
show as ticked, then leaves. A cancelled task leaves at once. Snoozed tasks are listed.

A list holds at most `caldav.max_objects` tasks (2000 by default). Past that it keeps open work before
finished work, then the soonest due date, undated last, then the most recently updated, and its description
says how many were left out, for example `Tasks of Infrastructure (INFRA); 14 more left out past the limit of
2000`.

## What each field maps to

Each task is one object holding one `VCALENDAR` with one `VTODO`.

| VTODO property | From the task |
| --- | --- |
| `UID` | `task-<id>@tix`, the same as the calendar feed's, stable across renames, new dates and moves |
| `SUMMARY` | the title alone, with no reference in front of it |
| `DESCRIPTION` | the body as written; Markdown stays text. Absent when the body is empty |
| `STATUS` | `NEEDS-ACTION` for a todo, blocked or waiting state, `IN-PROCESS` in progress, `COMPLETED` done |
| `COMPLETED`, `PERCENT-COMPLETE` | for a finished task, when it was completed in UTC, and `100` |
| `DUE` | the due date, see below. Absent when the task has none |
| `DTSTART` | the snooze, while it is in the future and before the due date |
| `PRIORITY` | highest `1`, high `3`, normal `0`, low `7`, lowest `9` |
| `CATEGORIES` | the task's tags |
| `RELATED-TO;RELTYPE=PARENT` | the parent task's UID, for a subtask |
| `URL` | the task's page in the browser, as an absolute address |
| `X-TIX-REF` | the reference, `INFRA-12` |
| `X-TIX-STATE` | the state's key, such as `waiting-on-vendor` |
| `CREATED` | when the task was created |
| `LAST-MODIFIED`, `DTSTAMP` | its last update |
| `SEQUENCE` | the task's version |

`STATUS:CANCELLED` is never written, because a cancelled task is not in any list. Blocked and waiting have
no VTODO counterpart and read as `NEEDS-ACTION`; `X-TIX-STATE` names the real state for a client that shows
extra properties.

A repeating task is one to-do with no `RRULE`, `RDATE` or `EXDATE`. tix makes the next occurrence itself when
one is finished; a client given a repeat rule would roll the to-do forward on its own and the two would
disagree. The next occurrence appears on the next sync as a new to-do.

**Dates.** A bare due date is stored as midnight UTC on every surface, and is written as a date,
`DUE;VALUE=DATE:20261105`, which every client shows as that day in every zone. Any other due time is a UTC
date-time, `DUE:20261105T143000Z`, which clients show at the right local time. The calendar feed always writes
a date, because an all-day event cannot carry a time usefully; a reminder can. When `DUE` is a date,
`DTSTART` is a date too, since RFC 5545 asks for the pair to match. A snooze on or after the due date, or one
that has passed, writes no `DTSTART`.

**Priority.** iCalendar runs from 1, the highest, to 9, with 0 for none. Normal is written as 0, so a normal
task carries no priority marker in Reminders instead of one on every row. Clients with three levels show
highest and high as High, low and lowest as Low.

## Sync

Every object carries a strong ETag that changes when anything in what it renders changes, and only then.
`GET` answers `304` to an `If-None-Match` that matches. Every list carries a sync token (RFC 6578) and a
`getctag` with the same value, for clients that poll the tag and then compare ETags.

A sync token stands for a snapshot of the list: which tasks it held, with their ETags. Syncing with a token
answers the difference from that snapshot to the list as it is now, tasks added or changed and tasks that
left. That covers every way a task leaves without anything happening to it: its completion ages out of the
window, another task pushes it past the bound, somebody edits the workflow so its state is now cancelled, or
your role changes. When nothing in the tenant changed since the token, the answer is "no changes" without
listing anything.

Renaming a project or changing its key changes every object in it (`X-TIX-REF` and `URL`), so the client
downloads the list again once.

The server keeps the last three snapshots per app password and list, plus any younger than a day. A client
holding an older token is told it is no longer valid (`403` with `DAV:valid-sync-token`) and does a full
sync, which clients do without asking. Nothing is lost by it; it costs one full download of the list.

There is no push. How often a client polls is its own setting, so a change in tix reaches a phone on the
client's next refresh rather than at once.

## Security

- **TLS.** Basic authentication sends the app password on every request, so CalDAV must be reached over
  HTTPS, from `tix serve` with a certificate or a proxy in front of it. `tix serve` already refuses a
  non-loopback bind without TLS, and iOS refuses a plain HTTP account of its own accord.
  [deployment.md](deployment.md) covers both.
- **Only app passwords.** Under `/dav/` an account password, a session cookie, a bearer header and every
  other API token are refused, and an app password is refused everywhere else, on `/api/v1` included. An
  account password typed into a client is never checked against the stored hash: a value that is not shaped
  like a token is refused before any lookup.
- **No lockout.** A failed DAV sign-in is written to the refusal log and does nothing else. A phone retrying
  a revoked password does not lock your sign-in, your single sign-on, your second factor or your other
  tokens. An app password carries 256 random bits, so there is nothing on this surface worth guessing.
- **Second factor.** An app password is an API token, which the second-factor policy exempts. You proved your
  factor when you signed in to create it.
- **One tenant per request.** The tenant and the person are settled before any task is read, and anything
  else answers `404`, as in [Addresses](#addresses).
- **Request bodies.** A `PROPFIND` or `REPORT` body is at most 64 KiB, 32 elements deep and 2000 elements, with
  no document type or entity declarations, or it is `400`. `Depth: infinity`, or no `Depth` header at all, is
  refused, and a multiget names at most 1000 objects.
- **Logs.** The Authorization header is never logged, and no address carries a secret.

## Setting up a client

Every client needs the same three things: the server's address, your username, and an app password. `tix dav
show` or Settings has the first two.

### Apple Reminders

On an iPhone or iPad: **Settings**, **Apps**, **Reminders**, **Reminders Accounts**, **Add Account**,
**Other**, **Add CalDAV Account**. On older iOS the path starts **Settings**, **Reminders**, **Accounts**.
Server is the server's host name, `tix.example.com`; User Name is your username; Password is the app
password. iOS finds the rest through `/.well-known/caldav`.

On a Mac: **System Settings**, **Internet Accounts**, **Add Other Account**, **CalDAV account**. Choose
**Advanced** when the automatic setup cannot find the server, and give Server Address `tix.example.com`,
Server Path `/dav/`, port 443 with SSL.

What to expect:

- The lists appear in Reminders, not in Calendar, because they hold only to-dos.
- They are read-only for now: Reminders shows them without a way to tick or add.
- Reminders shows no tags, subtasks or sections for a CalDAV list, so `CATEGORIES` and `RELATED-TO` are not
  drawn. The task's page is the link to open.
- Reminders has three priorities. tix's highest and high show as High, low and lowest as Low, and normal shows
  no marker. Medium never appears.
- A task due at a time shows that time; a bare due date shows as a day.
- iOS will not add an account over plain HTTP.

### DAVx5 and tasks.org

DAVx5 syncs CalDAV into Android's task providers, which tasks.org, jtx Board and OpenTasks read. Install the
tasks app first, so DAVx5 offers its task lists.

In DAVx5: **+**, **Login with URL and user name**. Base URL is `https://tix.example.com` (DAVx5 follows the
well-known address; `https://tix.example.com/dav/` works too), then your username and the app password. In
the new account's CalDAV tab, tick the lists to sync. Each one shows a lock, since it is read-only.

What to expect:

- tasks.org and jtx Board show tags as tags and subtasks under their parent, from `CATEGORIES` and
  `RELATED-TO`.
- tasks.org hides a task until its `DTSTART`, so a snoozed task stays hidden until the snooze ends, as it
  does in tix.
- The sync interval is set per account in DAVx5.

### Thunderbird

Thunderbird 102 and later discovers task lists from the server's address. In the Calendar or Tasks tab:
**New Calendar**, **On the Network**. Username is your username and Location is `https://tix.example.com/dav/`;
**Find Calendars**, then enter the app password when asked. Choose the lists to subscribe to. They appear
under Tasks, and Thunderbird marks them read-only on its own.

What to expect:

- Thunderbird polls `getctag` and then compares ETags, and uses sync tokens when the server offers them,
  which tix does. Refresh interval is in each list's properties.
- Categories show as Thunderbird categories; `X-TIX-REF` and `X-TIX-STATE` are not shown.

### Evolution

**File**, **New**, **Task List**. Type **CalDAV**, URL `https://tix.example.com/dav/`, User your username,
then **Find Task Lists** and the app password when asked. Pick a list; add the account once per list you want.

What to expect:

- Evolution polls `getctag` and fetches changed objects in one multiget.
- The list is read-only, so the editor opens tasks without saving changes back.

## Troubleshooting

An administrator finds every refused DAV sign-in in the server log, with its reason; the refusal log is
described in [deployment.md](deployment.md). The first request of every client is sent with no credential
and answered `401` with a Basic challenge: that is how a client learns to send them, not a failure.

### 401 Unauthorized

| Reason in the log | What happened | What to do |
| --- | --- | --- |
| `dav_password_required` | The client sent your account password, another API token, a bearer header or a cookie | Create an app password and put it in the password field |
| `dav_user_mismatch` | The app password is valid and the username is somebody else's | Use your sign-in email or handle, as `tix dav show` prints it |
| `unknown_token` | No app password in this tenant matches: mistyped, from another server, or another tenant's | Paste it again, or create a new one |
| `token_revoked` | The app password was revoked, by you or an administrator | Create a new one and replace it in the client |
| `token_expired` | It was created with `--expires` and that time has passed | Create a new one |
| `actor_disabled` | Your account or actor is disabled | Ask an administrator |
| `wrong_tenant` | On a shared host, the password belongs to another tenant than the one the address resolved | Use the address of your own tenant, or the plain `/dav/` |

A person removed from the tenant is refused with `401` from the next request too, whatever the password.

### 403 Forbidden

- **Under `/dav/`, on a change.** CalDAV is read-only for now, so `PUT`, `DELETE`, `MOVE` and the other write
  methods answer `403` and nothing changes. A client that tries anyway has ignored the read-only privileges;
  undo the change in the client.
- **On `/api/v1`, reason `caldav_only`.** An app password was used as an API token: in `tix ctx add`, a
  script, or the tix app's token field. It works only under `/dav/`. Use `tix token create` or a session for
  the API.
- **`DAV:valid-sync-token`.** The client's sync token is older than the server keeps. The client does a full
  sync by itself.

### 404 Not Found

- **Every request, after it used to work.** The tenant key changed, and every stored address with it.
  Remove the account from the client and add it again.
- **`/dav/` and `/.well-known/caldav` both.** CalDAV is turned off on this server (`caldav.enabled: false`).
  `tix dav show` says so, with the reason.
- **One project's list.** You can no longer read the project, or it was archived. The client removes the
  list on its next refresh.

### A task is missing

In order of likelihood:

- It is assigned to somebody else, and the password shows only Assigned to me. Use a password created with
  `--collections projects` or `all` to see whole projects.
- It is finished, and was completed longer ago than `caldav.completed_window` (14 days by default).
- It is cancelled, or in the trash. Neither is listed.
- The list is past `caldav.max_objects`. Its description, shown in most clients' list properties, says how
  many were left out. Open work with the nearest due dates is kept first.
- The client has not synced yet. Refresh it, or shorten its interval.
- tasks.org hides a task until its snooze ends.

### A task is there twice

The password was created with `--collections all`, so a task assigned to you is in Assigned to me and in its
project's list. Create a password with `assigned` or `projects` instead and revoke the old one.

### The priority looks wrong in Reminders

Reminders has High, Medium and Low. tix's five priorities are written so they read back to themselves, which
puts highest and high both under High, low and lowest both under Low, and normal under none. The task itself
is unchanged; its page shows the real priority.

## Related

- [calendar.md](calendar.md) for due dates as events in a subscribed calendar
- [configuration.md](configuration.md) for `caldav.enabled`, `caldav.completed_window` and `caldav.max_objects`
- [deployment.md](deployment.md) for TLS, proxies and the refusal log
- [api.md](api.md#caldav) for the app password routes
