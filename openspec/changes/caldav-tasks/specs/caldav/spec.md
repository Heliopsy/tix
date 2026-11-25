## ADDED Requirements

### Requirement: CalDAV is served under /dav/

When `caldav.enabled` is true, `tix serve` SHALL answer CalDAV (RFC 4791) under `/dav/` on the same listener as the API, and `/.well-known/caldav` SHALL answer `301` to `/dav/` without a credential. When it is false, both SHALL answer `404`.

#### Scenario: The well-known address redirects

- **WHEN** a client sends `PROPFIND /.well-known/caldav` with no credential
- **THEN** the answer is `301` with `Location` ending `/dav/`

#### Scenario: Disabled CalDAV is not there

- **GIVEN** `caldav.enabled` is false
- **WHEN** a client with a valid app password sends `PROPFIND /dav/`
- **THEN** the answer is `404`

### Requirement: An unauthenticated DAV request is challenged for Basic

A request under `/dav/` without valid credentials SHALL be answered `401` with `WWW-Authenticate: Basic realm="tix", charset="UTF-8"`, and with no body that names a tenant, a person or a project.

#### Scenario: The first request of a client is challenged

- **WHEN** a client sends `PROPFIND /dav/` with no Authorization header
- **THEN** the answer is `401` and carries a Basic challenge

### Requirement: Discovery names the person's principal and calendar home

`PROPFIND` on `/dav/` SHALL answer `current-user-principal` as `/dav/TENANT/principals/ACTOR/`, where TENANT is the tenant key and ACTOR the lowercase actor identifier of the app password's owner. The principal SHALL answer `calendar-home-set` as `/dav/TENANT/calendars/ACTOR/`, `calendar-user-address-set` as the owner's `mailto:` address, and `displayname`.

#### Scenario: A client finds its calendar home

- **GIVEN** a person in tenant `acme` with an app password
- **WHEN** their client sends `PROPFIND /dav/` asking for `current-user-principal`, then `PROPFIND` on that principal asking for `calendar-home-set`
- **THEN** the answers name `/dav/acme/principals/ACTOR/` and `/dav/acme/calendars/ACTOR/`

### Requirement: A DAV request never reaches another tenant or another person

A DAV request SHALL read only the tenant its app password belongs to. A path naming another tenant key, another actor, a project the owner cannot read, or a task outside the collection SHALL answer `404`, the same answer as a path that never existed.

#### Scenario: Another tenant's key is not found

- **GIVEN** tenants `acme` and `globex` with the same project keys, and a person in `acme` with an app password
- **WHEN** the client sends `PROPFIND /dav/globex/calendars/ACTOR/`
- **THEN** the answer is `404` and no `globex` data is read

#### Scenario: Another person's home is not found

- **WHEN** a client authenticated as one person sends `PROPFIND` on another person's calendar home in the same tenant
- **THEN** the answer is `404`

### Requirement: The calendar home lists the collections its app password chose

The calendar home SHALL list `assigned`, named Assigned to me, when the app password's collections are `assigned` or `all`, and one collection `p-PROJECT` per project the owner can read, named after the project, when they are `projects` or `all`. Every collection's `supported-calendar-component-set` SHALL be exactly `VTODO`.

#### Scenario: The default password shows only Assigned to me

- **GIVEN** an app password created with no collections choice in a tenant with three projects
- **WHEN** the client sends `PROPFIND` with `Depth: 1` on the calendar home
- **THEN** it lists one calendar collection, Assigned to me, whose component set is `VTODO`

#### Scenario: All shows the projects too

- **GIVEN** an app password whose collections are `all`, in a tenant with three readable projects
- **WHEN** the client lists the calendar home
- **THEN** it lists four calendar collections

### Requirement: A collection holds open tasks and recently finished ones

A collection SHALL hold the tasks of its project, or for Assigned to me the tasks assigned to the owner, that are not in the trash, whose state is not in the cancelled category, and that are open or were completed within `caldav.completed_window`. It SHALL hold at most `caldav.max_objects`, open before done, then by due date, then by most recent update.

#### Scenario: Cancelled and old finished tasks are absent

- **GIVEN** a project with an open task, a task finished yesterday, a task finished sixty days ago and a cancelled task, and the default window
- **WHEN** a client lists the project's collection
- **THEN** it holds exactly the open task and the task finished yesterday

#### Scenario: A collection past its bound says so

- **GIVEN** `caldav.max_objects` is 10 and a project holds 12 open tasks
- **WHEN** a client lists the collection
- **THEN** it holds 10 objects and its `calendar-description` says 2 were left out

### Requirement: Each task is one VTODO with a stable UID

An object SHALL be a `VCALENDAR` holding one `VTODO` whose `UID` is `task-<id>@tix`, or the client's UID for a task a client created, and SHALL keep that UID and resource name across renames, new due dates, project key changes and moves between collections.

#### Scenario: A renamed task keeps its UID and address

- **WHEN** a task's title and due date change between two fetches
- **THEN** both fetches return the object at the same address with the same `UID`, and the later one a higher `SEQUENCE`

### Requirement: Task fields map onto VTODO properties

A VTODO SHALL carry `SUMMARY` as the title alone, `DESCRIPTION` as the body as written, `CATEGORIES` as the tags, `URL` as the task's absolute page address, `X-TIX-REF` as its reference, `X-TIX-STATE` as its state key, `RELATED-TO;RELTYPE=PARENT` with the parent's UID when the owner can read the parent, and `CREATED`, `LAST-MODIFIED`, `DTSTAMP` and `SEQUENCE` from the task.

#### Scenario: A task's fields are all present

- **GIVEN** a task `INFRA-12` titled `Rotate keys` with tags `ops` and `security`, a body, and a readable parent
- **WHEN** a client fetches its object
- **THEN** `SUMMARY` is `Rotate keys`, `CATEGORIES` holds both tags, `X-TIX-REF` is `INFRA-12`, `URL` ends with the task's page and `RELATED-TO` is the parent's UID

### Requirement: Status maps from the state's category

`STATUS` SHALL be `NEEDS-ACTION` for a state in the todo, blocked or waiting category, `IN-PROCESS` for in progress, and `COMPLETED` for done, with `COMPLETED` set to the completion time in UTC and `PERCENT-COMPLETE:100`. `STATUS:CANCELLED` SHALL never be written, because a cancelled task is not a member.

#### Scenario: A blocked task reads as needing action

- **WHEN** a client fetches a task in a blocked state named `waiting-on-vendor`
- **THEN** `STATUS` is `NEEDS-ACTION` and `X-TIX-STATE` is `waiting-on-vendor`

#### Scenario: A finished task carries its completion

- **WHEN** a client fetches a task completed at 2026-11-20T09:15:00Z
- **THEN** `STATUS` is `COMPLETED` and `COMPLETED` is `20261120T091500Z`

### Requirement: Due dates follow the bare-date rule

A due instant at exactly midnight UTC SHALL be written `DUE;VALUE=DATE` with its UTC date, and any other due instant as a UTC date-time. While a task is snoozed to a moment before its due date, `DTSTART` SHALL be that moment, as a date when `DUE` is a date. A task without a due date SHALL carry no `DUE`.

#### Scenario: A bare due date is an all-day to-do

- **GIVEN** a task due on 2026-11-05 set from any surface as a bare date
- **WHEN** a client fetches it
- **THEN** it carries `DUE;VALUE=DATE:20261105`

#### Scenario: A due time is kept

- **GIVEN** a task due at 2026-11-05T14:30:00Z
- **WHEN** a client fetches it
- **THEN** it carries `DUE:20261105T143000Z`

### Requirement: Priority maps so that tix values round-trip

`PRIORITY` SHALL be written as 1 for highest, 3 for high, 0 for normal, 7 for low and 9 for lowest, and read as normal for 0 and 5, highest for 1 and 2, high for 3 and 4, low for 6 and 7, and lowest for 8 and 9.

#### Scenario: A normal task shows no priority

- **WHEN** a client fetches a task of normal priority
- **THEN** its `PRIORITY` is 0

### Requirement: Repeat rules are never written

A VTODO SHALL carry no `RRULE`, `RDATE` or `EXDATE`, whether or not its task repeats.

#### Scenario: A repeating task is one to-do

- **WHEN** a client fetches a task that repeats every week
- **THEN** the object holds one `VTODO` and no recurrence property

### Requirement: Objects carry ETags and collections carry sync tokens

Every object SHALL carry a strong `getetag` that changes when anything in its rendering changes and only then. Every collection SHALL carry `DAV:sync-token` and `CS:getctag` with the same value, a URI that changes when a member is added, removed or changed. `GET` SHALL honour `If-None-Match`.

#### Scenario: An unchanged object is not sent twice

- **WHEN** a client fetches an object, then fetches it again with `If-None-Match` set to its ETag
- **THEN** the second answer is `304`

#### Scenario: A project rename changes every ETag in its collection

- **GIVEN** a project whose key changes from `infra` to `ops`
- **WHEN** a client lists its collection's ETags before and after
- **THEN** every ETag differs, because `X-TIX-REF` and `URL` changed

### Requirement: sync-collection answers what changed since a token

A `sync-collection` report (RFC 6578) at sync level 1 SHALL answer every member added or changed since its token with its ETag, and every departed member as `404`. An empty token SHALL answer every member. A token the server no longer holds SHALL answer `403` with `DAV:valid-sync-token`.

#### Scenario: A finished task that ages out is reported as removed

- **GIVEN** a client holding a sync token for a collection that held a task completed thirteen days ago, and the default window
- **WHEN** two days pass with no change to the task and the client syncs with its token
- **THEN** the answer reports that task's address as `404`

#### Scenario: Nothing changed is an empty answer

- **WHEN** a client syncs twice with no change in the tenant between
- **THEN** the second answer lists no members and returns the same token

#### Scenario: A forgotten token asks for a full sync

- **WHEN** a client syncs with a token the server has pruned
- **THEN** the answer is `403` with `DAV:valid-sync-token`

### Requirement: sync-collection honours a limit

A `sync-collection` report with a `limit` smaller than the number of changes SHALL answer that many changes, a token from which the rest can be asked for, and a `507` response for the request URI.

#### Scenario: A limited sync resumes

- **GIVEN** 5 changes since a client's token
- **WHEN** the client syncs with `limit` 2, then with the token it was given, with no limit
- **THEN** the first answer lists 2 changes and a `507`, and the second the other 3

### Requirement: calendar-query filters over VTODO

A `calendar-query` report SHALL evaluate comp, prop and param filters, `is-not-defined`, `text-match` with `i;ascii-casemap` and `i;octet`, and `time-range` on `VTODO` as RFC 4791 section 9.9 defines, over the collection's members. An unsupported collation SHALL answer `403` with `CALDAV:supported-collation`.

#### Scenario: Incomplete to-dos only

- **GIVEN** a collection with two open tasks and one finished task
- **WHEN** a client sends a `calendar-query` with a `VTODO` comp filter and `COMPLETED` `is-not-defined`
- **THEN** the answer lists the two open tasks

### Requirement: calendar-multiget returns the named objects

A `calendar-multiget` report SHALL answer each named object of the collection with the requested properties and `calendar-data`, and `404` for each name it does not hold. More than 1000 names SHALL answer `403`.

#### Scenario: A deleted name in a multiget

- **WHEN** a client multigets two names, one of which left the collection
- **THEN** the answer holds the one object and a `404` for the other name

### Requirement: Unknown properties are answered as not found

A `PROPFIND` asking for a property tix does not serve SHALL answer it as `404` in its own `propstat`, beside the properties it does serve.

#### Scenario: Apple's push property

- **WHEN** a client asks a collection for `displayname` and `CS:push-transports`
- **THEN** `displayname` is in a `200` propstat and `push-transports` in a `404` propstat

### Requirement: DAV request bodies are bounded and never expand entities

A `PROPFIND` or `REPORT` body SHALL be at most 64 KiB, nest at most 32 elements deep and hold at most 2000 elements, and SHALL hold no document type declaration or entity declaration. A body breaking any rule SHALL answer `400` without being evaluated.

#### Scenario: An entity declaration is refused

- **WHEN** a client sends a `PROPFIND` whose body declares an entity
- **THEN** the answer is `400`

#### Scenario: An infinite depth is refused

- **WHEN** a client sends `PROPFIND` with `Depth: infinity`
- **THEN** the answer is `403` with `DAV:propfind-finite-depth`

### Requirement: Only PUT and DELETE change anything

`MKCOL`, `MKCALENDAR`, `MOVE`, `COPY`, `ACL`, `LOCK`, `UNLOCK` and `POST` SHALL answer `403`, and `PROPPATCH` SHALL answer `207` with `403` for each property. While the app password is read-only or its owner may not write, `current-user-privilege-set` SHALL be `DAV:read` alone and `PUT` and `DELETE` SHALL answer `403` with `DAV:need-privileges` and change nothing.

#### Scenario: A read-only password cannot write

- **GIVEN** an app password created read-only
- **WHEN** a client sends `PUT` to an existing object
- **THEN** the answer is `403` with `DAV:need-privileges` and the task is unchanged

#### Scenario: Moving an object is refused

- **WHEN** a client sends `MOVE` for an object
- **THEN** the answer is `403` and the task is unchanged

### Requirement: Write-back reads its privileges from the owner's role

With write-back, `current-user-privilege-set` SHALL add `DAV:write-content`, `DAV:bind` and `DAV:unbind` on a collection when the owner's current role holds `task:write` and `task:transition` and the app password is not read-only, and SHALL stay `DAV:read` otherwise.

#### Scenario: A viewer's phone stays read-only

- **GIVEN** a person whose role is viewer and an app password that is not read-only
- **WHEN** the client asks a collection for `current-user-privilege-set`
- **THEN** it holds `DAV:read` alone

### Requirement: An update needs the current ETag

A `PUT` to an existing object SHALL require `If-Match`. Without it the answer SHALL be `428`; with an ETag other than the current one, `412`, and the task SHALL be unchanged. A successful update SHALL go through the task update with the version the ETag names, and SHALL answer `204` with no `ETag`.

#### Scenario: A stale write is refused

- **GIVEN** a client holding an object's ETag, and the task's title changed in the browser since
- **WHEN** the client sends `PUT` with `If-Match` set to the old ETag
- **THEN** the answer is `412` and the title is the browser's

### Requirement: An update applies only what changed

A `PUT` SHALL change the title from `SUMMARY`, the body from `DESCRIPTION`, the priority from `PRIORITY`, the tags from `CATEGORIES` and the snooze from a future `DTSTART`, each only when it differs from what tix would write. An absent `SUMMARY`, `DESCRIPTION`, `PRIORITY` or `CATEGORIES` SHALL leave its field unchanged. An absent `DUE` SHALL clear the due date.

#### Scenario: Apple drops tags it does not show

- **GIVEN** a task with tags `ops` and `security`
- **WHEN** a client sends `PUT` changing `SUMMARY` and carrying no `CATEGORIES`
- **THEN** the title changes and both tags remain

#### Scenario: Removing a date in the client clears it

- **WHEN** a client sends `PUT` for a task with a due date, carrying no `DUE`
- **THEN** the task has no due date

### Requirement: Status from a client moves the task through its workflow

A `PUT` changing `STATUS` or `COMPLETED` SHALL move the task by the multi-hop route to its workflow's state of the matching category: done for `COMPLETED`, in progress for `IN-PROCESS`, todo for `NEEDS-ACTION` and cancelled for `CANCELLED`. A move the workflow refuses, needs a comment, approval or claim for, or reaches by two equally short routes SHALL answer `403` and change nothing.

#### Scenario: Ticking a reminder finishes the task

- **GIVEN** an open task whose workflow reaches done in two steps
- **WHEN** a client sends `PUT` with `STATUS:COMPLETED`
- **THEN** the task is in the done state, and two transitions are in its audit trail

#### Scenario: A guarded move is refused

- **GIVEN** a workflow whose only route to done requires a comment
- **WHEN** a client sends `PUT` with `STATUS:COMPLETED` and a new `SUMMARY`
- **THEN** the answer is `403` and the task is unchanged, its title included

### Requirement: A client unticks a task by leaving the status out

A `PUT` for a done task carrying no `STATUS` and no `COMPLETED` SHALL move it to its workflow's todo state, as `STATUS:NEEDS-ACTION` does. A `PUT` with no `STATUS` carrying `COMPLETED` or `PERCENT-COMPLETE:100` SHALL move an open task to done. For any other task an absent `STATUS` SHALL leave the state alone.

#### Scenario: Unticking in a client reopens the task

- **GIVEN** a task in its done state
- **WHEN** a client sends `PUT` without `STATUS` or `COMPLETED`
- **THEN** the task is in its todo state

#### Scenario: An in-progress task stays in progress

- **GIVEN** a task in progress
- **WHEN** a client that does not write `STATUS` sends `PUT` with a new `SUMMARY`
- **THEN** the title changes and the task is still in progress

### Requirement: A client can create a task

A `PUT` to a name the collection does not hold SHALL create a task: in the collection's project, or for Assigned to me in the tenant's default project assigned to the owner. The client's `UID` and resource name SHALL be kept, so the task is found at that name with that UID afterwards. A UID already in use in the tenant SHALL answer `403` with `CALDAV:no-uid-conflict`.

#### Scenario: A reminder added on a phone becomes a task

- **WHEN** a client sends `PUT /dav/acme/calendars/ACTOR/assigned/ABC.ics` with `If-None-Match: *` and a VTODO with `UID:ABC` and `SUMMARY:Call the bank`
- **THEN** the answer is `201`, a task titled `Call the bank` exists in the default project assigned to the owner, and `GET` of the same name returns `UID:ABC`

#### Scenario: No default project to create in

- **GIVEN** a tenant with several projects and none keyed `default`
- **WHEN** a client creates a task in Assigned to me
- **THEN** the answer is `403` naming a project's list as the place to add it, and no task is created

### Requirement: DELETE cancels and never removes

A `DELETE` of an open task SHALL move it to its workflow's cancelled state by the multi-hop route and answer `204`, or answer `403` and change nothing when no cancelled state is reachable. A `DELETE` of a done task SHALL answer `204` and change nothing. A `DELETE` SHALL never move a task to the trash or remove it.

#### Scenario: Deleting a reminder cancels the task

- **WHEN** a client deletes an open task's object
- **THEN** the task is in its cancelled state, it is not in the trash, and the next sync reports its name as `404`

#### Scenario: Clearing completed reminders is harmless

- **WHEN** a client deletes a done task's object
- **THEN** the answer is `204` and the task is unchanged

### Requirement: Properties tix cannot store are kept for their owner

Properties and components of a `PUT` with no tix field, other than recurrence, SHALL be kept verbatim for that person and task, up to 8 KiB, and written back only into that person's objects. Recurrence properties SHALL be dropped. Past 8 KiB, the object SHALL be accepted and the extras dropped.

#### Scenario: An alarm set on the phone survives a sync

- **WHEN** a client sends `PUT` with a `VALARM`, then fetches the object
- **THEN** the object carries the same `VALARM`

#### Scenario: Another person never sees it

- **WHEN** a second person's client fetches the same task
- **THEN** its object carries no `VALARM`

### Requirement: A PUT body is one bounded VTODO

A `PUT` body SHALL be at most 256 KiB of `text/calendar` holding one `VCALENDAR` with exactly one `VTODO`. Another media type SHALL answer `415`; anything else malformed, `403` with `CALDAV:valid-calendar-object-resource`.

#### Scenario: An event is refused

- **WHEN** a client sends `PUT` with a `VEVENT`
- **THEN** the answer is `403` with `CALDAV:valid-calendar-object-resource`

### Requirement: A DAV write is recorded on the caldav channel

Every audit entry and event a `PUT` or `DELETE` under `/dav/` writes SHALL carry the access channel `caldav` with the app password's token identifier and name, and the app password's owner as the actor.

#### Scenario: The audit log lists a phone's changes

- **WHEN** a client ticks a task off over CalDAV and an administrator lists the audit log with `--via caldav`
- **THEN** the task's transition entries are listed, each naming the owner and the app password's token
