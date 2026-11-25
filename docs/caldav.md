# Tasks over CalDAV

Your tasks, as to-do lists in the apps people already keep them in: Apple Reminders on an iPhone or a Mac,
Thunderbird's Tasks, tasks.org, jtx Board or OpenTasks through DAVx5 on Android, and Evolution. `tix serve`
answers CalDAV (RFC 4791) for them under `/dav/`, and each person signs their app in with an app password of
their own.

A client shows the lists, keeps them current, and writes back: ticking a reminder off finishes the task,
editing it changes the task, adding one creates a task, and deleting one cancels it. Every change goes
through the same rules, audit entries and events as a change made in the browser, as you, within your
current role. [Writing back](#writing-back) says exactly what each change does. A password created
read-only, or one whose owner is a viewer, shows the lists without a way to change them.

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
- `--read-only` keeps the password from changing tasks, whatever your role allows. A tablet on the wall
  that should only show the list is the usual case.
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
(`write`, true when your role holds task write and task transition) and the `collections` choices.
[api.md](api.md#caldav) has the routes.

The name you give is yours: it must differ from your other live app passwords, and two people may both call
theirs `iPhone`. The token behind it is named `dav-HANDLE-NAME`, with a short suffix when that is taken,
which is what an administrator sees in `tix token ls`. An administrator revokes anybody's with `tix token rm
ID`; `tix token create --scope caldav` is refused, so there is one way to make one.

### What a password may do

The password is not its own authority. On every request it acts as you, with your current role in the
tenant, cut down to reading tasks, projects and workflows, and changing and moving tasks (`task:write` and
`task:transition`) unless it was created read-only. Deleting tasks is never in it: a `DELETE` cancels.
A demotion to viewer turns the client read-only on its next request, and its lists then advertise read
access alone. Removal from the tenant or a disabled account stops it at once, and so does revoking it. It
can never do more than you could.

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
| `UID` | `task-<id>@tix`, the same as the calendar feed's, stable across renames, new dates and moves; a task a client created keeps the client's UID and resource name |
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

## Writing back

A client writes with `PUT` and `DELETE`. Each list advertises `DAV:write-content`, `DAV:bind` and
`DAV:unbind` in `current-user-privilege-set` when the password may write, and `DAV:read` alone otherwise,
which is what makes a client draw it as editable or locked.

### Changing a task

A `PUT` to an object that exists needs `If-Match` with the ETag the client last fetched. Without it the
answer is `428`; with an older one it is `412`, and the task is unchanged. A change made in the browser
since the client's last fetch therefore wins, and the client offers to fetch it again (Thunderbird asks
whether to overwrite or discard). A successful update answers `204` with no `ETag`, because what tix stores
is not byte for byte what was sent; the client fetches the object again, and all of them do.

tix compares the object with what it would write for the task and changes only what differs:

| The client sends | tix changes |
| --- | --- |
| `SUMMARY` | the title. An empty one is ignored |
| `DESCRIPTION` | the body |
| `PRIORITY` | the priority: 1 and 2 highest, 3 and 4 high, 0 and 5 normal, 6 and 7 low, 8 and 9 lowest |
| `CATEGORIES` | the tags, the whole set. An empty `CATEGORIES` removes them |
| `DUE` | the due date. A date is midnight UTC, as on every surface |
| no `DUE` | clears the due date, because every client removes the property to remove a date |
| `DTSTART` in the future | snoozes the task until then. A past one changes nothing: Thunderbird writes the creation time there |
| `STATUS`, `COMPLETED` | moves the task, below |

A property the client leaves out leaves its field alone, apart from `DUE` and the status. Apple Reminders
rewrites the whole object on every change and leaves out `CATEGORIES`, so reading a missing property as
"clear it" would strip the tags from every task touched on an iPhone. Sending back an object exactly as it
was fetched changes nothing and does not move the task's version.

A date-time with `TZID` is read in that zone from the zone database built into tix. A zone the database does
not know is read from the `VTIMEZONE` the object carries, and failing that in your time zone preference. A
floating time, one with neither `Z` nor `TZID`, is read in your time zone preference, then the tenant's,
then UTC.

### Ticking off and reopening

| The client sends | The task moves to its workflow's state of the category |
| --- | --- |
| `STATUS:COMPLETED`, or `COMPLETED` or `PERCENT-COMPLETE:100` without a `STATUS` | done |
| `STATUS:IN-PROCESS` | in progress |
| `STATUS:NEEDS-ACTION`, or no status at all, on a done task | todo |
| `STATUS:CANCELLED` | cancelled, which takes it out of the list |

A blocked or waiting task reads as `NEEDS-ACTION`, so a client sending that back leaves it where it is. The
move takes the shortest route through the workflow, one ordinary transition per step, so a workflow that
needs two steps to reach done works and records both. A move the workflow refuses is `403`, with a plain
text body saying why, and nothing changes: an edge that needs a comment, an approval or a scope, a task
claimed by somebody else, no state of that category within reach, or two equally short routes (tix will not
guess which states to record in the task's history). Move such a task in tix instead.

The move is tried first without writing anything. Only when it would succeed are the other changes and then
the move made, so a refused tick leaves a renamed title unchanged too.

### Adding a task

A `PUT` to a name the list does not hold creates a task and answers `201`. In a project's list the task is
created in that project. In Assigned to me it is created in the tenant's `default` project and assigned to
you; in a tenant with several projects and none keyed `default` that is refused with `403`, and the task
has to be added to a project's list. `If-None-Match: *` asks for a create only; Evolution sends none, and a
`PUT` to a new name without it is a create too.

The client's `UID` and resource name are kept, so the client finds the task where it put it, with the UID it
gave it, in every list that holds it. A UID or a name another task in the tenant already has is `403` with
`CALDAV:no-uid-conflict`. A new task needs a `SUMMARY`.

### Deleting

`DELETE` never removes a task and never moves one to the trash. Trashing needs `task:delete`, which members do
not hold, and one tap on a phone should not be how work is lost.

- An open task is moved to its workflow's cancelled state, by the same route as `STATUS:CANCELLED`, and leaves
  the list. With no cancelled state within reach the answer is `403` and the task stays.
- A done task is left alone and the answer is `204`. Apple Reminders' "Clear completed" sends a `DELETE` for
  every finished reminder, and refusing them would show an error per task. The task stays in the list until
  `caldav.completed_window` passes: a client syncing by token does not see it again unless it changes, and
  a client that relists by ETag, such as Thunderbird, may show it again until it ages out.

### What is kept and what is dropped

Alarms (`VALARM`), `LOCATION`, `GEO`, `ATTACH`, `ATTENDEE`, `ORGANIZER`, `CLASS`, a `PERCENT-COMPLETE` other
than 100, and client extensions such as `X-APPLE-SORT-ORDER`, `X-MOZ-GENERATION` and `X-MOZ-LASTACK` have no
tix field. tix keeps them as the client sent them, for you and that task, and writes them back into your own
copy of the object, so an alarm set on your phone is still there on its next sync. They are never shown in
tix, never read by it, never written into anybody else's object, and are removed with the task, your account
or the tenant. At most 8 KiB is kept per person and task; past that the object is still accepted, the extras
are dropped, and the server logs a warning.

Dropped and not kept:

- `RRULE`, `RDATE`, `EXDATE`, `EXRULE` and `RECURRENCE-ID`. A repeat rule set on a phone does not make the task
  repeat; set the repeat in tix, which makes the next occurrence itself.
- `RELATED-TO`. Making a reminder a subtask on the phone does not reparent the task.
- `URL`, which tix writes as the task's page, and `DURATION`.

A `PUT` body is at most 256 KiB of `text/calendar` holding one `VCALENDAR` with exactly one `VTODO`, and any
`VTIMEZONE` it names. Another media type is `415`; anything else, an event or two to-dos among them, is `403`
with `CALDAV:valid-calendar-object-resource`.

### In the audit log

A change from a client is recorded like any other, as you, on the `caldav` channel with the app password's
token name, so `tix audit ls --via caldav` lists them and the task's history says `via CalDAV`. Its events
reach webhooks, the event stream and the browser's live updates as every change's do. What tix keeps for a
client, above, is the client's own copy and writes no audit entry and no event.

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
  refused, and a multiget names at most 1000 objects. A `PUT` body is at most 256 KiB.
- **Cross-site requests.** `/dav/` accepts no `POST` and sends no CORS headers, and `PUT` and `DELETE` are
  methods a browser preflights, so another site cannot make your browser write with credentials it once
  typed in.
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
- Ticking, editing, adding and deleting reminders all reach tix, as [Writing back](#writing-back) says.
  "Clear completed" leaves the finished tasks alone.
- Reminders' High is written as priority 1 and reads as highest, and its Low as 9, lowest.
- Moving a reminder to another list is refused, because the UID is already in use; move the task in tix.
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
the new account's CalDAV tab, tick the lists to sync. A list shows a lock when the password is read-only or
your role cannot change tasks.

What to expect:

- tasks.org and jtx Board show tags as tags and subtasks under their parent, from `CATEGORIES` and
  `RELATED-TO`.
- tasks.org hides a task until its `DTSTART`, so a snoozed task stays hidden until the snooze ends, as it
  does in tix.
- A tick in tasks.org sends `STATUS`, `COMPLETED` and `PERCENT-COMPLETE` together, which finishes the task.
  A subtask made in tasks.org is created as a task of its own; `RELATED-TO` is not read.
- The sync interval is set per account in DAVx5.

### Thunderbird

Thunderbird 102 and later discovers task lists from the server's address. In the Calendar or Tasks tab:
**New Calendar**, **On the Network**. Username is your username and Location is `https://tix.example.com/dav/`;
**Find Calendars**, then enter the app password when asked. Choose the lists to subscribe to. They appear
under Tasks, and Thunderbird marks them read-only when the password or your role cannot change tasks.

What to expect:

- Thunderbird polls `getctag` and then compares ETags, and uses sync tokens when the server offers them,
  which tix does. Refresh interval is in each list's properties.
- Categories show as Thunderbird categories; `X-TIX-REF` and `X-TIX-STATE` are not shown.
- Thunderbird sends the whole task back on every change; only what differs is applied. When the task changed
  in tix since Thunderbird fetched it, the save is refused with `412` and Thunderbird offers to overwrite or
  discard.

### Evolution

**File**, **New**, **Task List**. Type **CalDAV**, URL `https://tix.example.com/dav/`, User your username,
then **Find Task Lists** and the app password when asked. Pick a list; add the account once per list you want.

What to expect:

- Evolution polls `getctag` and fetches changed objects in one multiget.
- Evolution may save a new task without `If-None-Match`; tix creates it all the same.

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

- **Under `/dav/`, with `DAV:need-privileges`.** The password is read-only, or your role does not change
  tasks. `MOVE`, `COPY`, `MKCOL`, `MKCALENDAR` and `LOCK` are always refused. Undo the change in the client.
- **Under `/dav/`, with a sentence.** The workflow refused the move, and the sentence says why: a comment or
  approval is needed, somebody else holds a claim on the task, no such state can be reached, or two routes
  are equally short. Nothing changed. Move the task in tix.
- **`CALDAV:no-uid-conflict`.** The client created a task with a UID or a name another task already has,
  most often by moving a reminder between lists.
- **`CALDAV:valid-calendar-object-resource`.** The object was not one to-do tix can read: an event, two
  to-dos, no `UID`, no `SUMMARY` on a new task, a date it cannot read, or a body over 256 KiB.
- **On `/api/v1`, reason `caldav_only`.** An app password was used as an API token: in `tix ctx add`, a
  script, or the tix app's token field. It works only under `/dav/`. Use `tix token create` or a session for
  the API.
- **`DAV:valid-sync-token`.** The client's sync token is older than the server keeps. The client does a full
  sync by itself.

### 412, 428 and 415

- **`412 Precondition Failed`.** The task changed since the client fetched it. The client fetches it again,
  or asks you what to keep.
- **`428 Precondition Required`.** The client changed an object without `If-Match`, which tix requires so a
  phone cannot overwrite an edit it never saw. A client that does this is not one listed here.
- **`415 Unsupported Media Type`.** The body was not `text/calendar`.

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
