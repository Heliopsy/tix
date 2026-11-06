# Calendar feed of due dates

## Why

Due dates live in tix and nowhere else, so a person planning their week in Google Calendar, Apple
Calendar or Outlook does not see what falls due. Every one of those apps subscribes to an iCalendar
URL and polls it, which needs no integration on either side, only a URL the app can fetch without
signing in.

## What Changes

- Each person can hold one **calendar feed**: a secret URL answering an RFC 5545 `VCALENDAR` with
  one all-day `VEVENT` per task with a due date. The URL carries a dedicated read-only secret, so a
  calendar app fetches it without a session. The secret is shown when the feed is created or
  rotated and never again; rotating issues a new URL and stops the old one, revoking stops it.
- A feed selects with a filter expression in the ordinary filter language, stored with the feed.
  Without one it lists the tasks assigned to its owner that are not finished.
- The feed reads as its owner, with no more than task, project and workflow read authority, and
  re-resolves the owner's membership on every fetch: a removed member, a disabled account or a
  revoked feed answers 404, and a feed never lists a task its owner could not list.
- The answer carries a strong `ETag` and honours `If-None-Match`, so a client polling an unchanged
  feed gets `304 Not Modified`.
- Surfaces: `tix calendar show|create|rotate|filter|revoke`; `GET|POST|PATCH|DELETE /api/v1/calendar`,
  `POST /api/v1/calendar/rotate` and the public `GET /api/v1/calendar/feeds/{token}.ics`; a Calendar
  feed section on the browser's Settings screen.

## Impact

- New table `calendar_feeds` (migration 0022), store methods on both engines.
- `core.CalendarFeed`, `core.IssuedCalendarFeed`, `core.CalendarFeedInput`, `core.CalendarEntries`,
  `core.CalendarService` with six methods, one authz action (`calendar.feed`, needs `task:read`),
  six registry entries, a new `internal/ical` renderer.
- Nothing changes for anybody who never creates a feed.
