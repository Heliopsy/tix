# An inbox of what concerns you

## Why

tix tells a person nothing unless they go looking. A comment that names them, a task handed to
them, a reply on something they are watching and a task of theirs moving on all pass silently
through the activity feed, mixed in with everybody else's work. People asked for an inbox: one
place that lists what concerns *them*, with an unread count they can see from any screen.

Nothing handles mentions today. `@handle` in a comment is plain text.

## What Changes

- **Inbox items.** Each actor has an inbox of items, each naming a task, the actor who caused it,
  what happened and when, and whether it has been read. An item is created when:
  - someone mentions you as `@handle` in a comment or a task description (`mention`);
  - a task is assigned to you (`assigned`);
  - someone comments on a task you are assigned to or have starred (`comment`);
  - a task you are assigned to changes status (`status`).
  Never for your own actions. One change gives one recipient at most one item per task, the
  strongest reason winning (mention, then assigned, then status, then comment). Items are written in
  the same transaction as the change that caused them.
- **Read state.** Mark an item read or unread, or everything read. Each is a mutation with an audit
  entry and an `inbox.updated` event; a change that changes nothing writes nothing.
- **Mutes.** A reader can mute any of the four kinds; a muted kind creates no item for them.
- **Live count.** Creating items and changing read state emit `inbox.updated` naming the recipients,
  so the web sidebar's unread badge refreshes over the existing event stream.
- Surfaces: `tix inbox ls|read|unread|read-all|count|prefs`, `GET /v1/inbox` (keyset paginated),
  `GET /v1/inbox/count`, `PUT|DELETE /v1/inbox/{id}/read`, `POST /v1/inbox/read-all`,
  `GET|PUT /v1/inbox/prefs`; an Inbox entry in the web sidebar with an unread badge, an inbox page,
  per-kind mute switches in Settings, and `@` completion of handles in comment and description
  boxes; an Inbox view in the terminal interface.
- **Notification channels.** Each person can send their own inbox items to channels of their own:
  Slack, Discord, ntfy or signed JSON, per kind, with Send test, recent deliveries and redeliver. The
  webhook dispatcher drains their queue with its own retries, guard and signature. Surfaces: Settings,
  Notifications; `tix inbox channel ls|add|edit|rm|test|deliveries|redeliver`; `/v1/channels`.
- **Live inbox.** The inbox page redraws in place on `inbox.updated`, highlighting new rows.

## Impact

- New tables `inbox_items`, `inbox_mutes`, `inbox_channels` and `inbox_deliveries` (migration 0024), store methods on both engines.
- `core.InboxItem`, `core.InboxKind`, `core.InboxFilter`, `core.InboxPrefs`, `core.InboxService`
  with seven methods, one event type, one authz action, seven registry entries.
- Comment, task create, task update, transition and release paths call one notifier.
