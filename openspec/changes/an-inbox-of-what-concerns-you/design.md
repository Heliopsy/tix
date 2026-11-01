# Design: an inbox of what concerns you

## Where items are made

One notifier hangs off the mutation (`internal/service/inbox.go`). A write that can concern somebody
calls `m.notify(task, kind, recipients...)` while it holds its transaction; `flush` resolves the
queued candidates before the audit entries and events are written. Resolution drops the acting actor,
drops kinds each recipient muted, keeps the strongest reason per recipient and task, writes the rows,
and queues one `inbox.updated` event naming every recipient. Doing it at flush rather than at each
call site means a comment that mentions the assignee of a task becomes one `mention` item, not a
mention and a comment.

Call sites:

| Change | Kinds |
| --- | --- |
| `CreateTask` | `assigned` to the assignee, `mention` for handles in the description |
| `UpdateTask` | `assigned` when the assignee changes to someone, `mention` for handles new to the description |
| `AddComment` | `mention` for handles in the body, `comment` to the assignee and everyone who starred the task |
| `EditComment` | `mention` for handles new to the body |
| `TransitionTask` | `status` to the assignee; its comment as `AddComment` |
| `ReleaseLease` with a status | `status` to the assignee |

Editing only notifies handles that were not already in the old text, so fixing a typo in a comment
does not mention everybody again.

## Mentions

`@handle` where the handle is the actor handle grammar (lowercase letters, digits, `.`, `_`, `-`),
matched case-insensitively and not preceded by a word character, so `me@example.com` is not a
mention. A handle that names no actor is ignored rather than refused: a comment is prose.

## Storage

`inbox_items(tenant_id, id, recipient_id, kind, task_id, actor_id, comment_id, from_status,
to_status, excerpt, created_at, read_at)`, cascading from tenant, recipient and task. The listing is
keyset paginated on `(created_at, id)` newest first; a partial index on unread rows serves the badge
count. `inbox_mutes(tenant_id, actor_id, kind)` holds one row per muted kind.

The listing joins nothing in SQL. The service names the task (reference, title) and the actor
(handle) for the page it returns, at most one lookup per distinct task and actor on a page.

## Authority

`inbox.read` needs `task:read` and is not project-confinable: an inbox spans projects. Every method
works on the caller's own items only; an identifier of somebody else's item is not found.

## Live badge

`inbox.updated` carries `recipients` (actor identifiers). The sidebar's badge is a third live stream
in `live.js`, subscribed to `inbox.updated` only, refreshing when its own actor is named. A reader on
a page without the badge's script still sees the count on the next navigation.

## Outbound delivery

Left out. Webhook endpoints are tenant configuration filtered by event type; there is no per-user
endpoint and no per-recipient filter, and adding one is a webhook feature, not an inbox feature. A
tenant endpoint can subscribe to `inbox.updated`, whose payload names the recipients, which is enough
for a bridge to fan out.

## Alternatives

- Deriving items from the outbox after commit: rejected, the brief requires the items to be written
  in the triggering transaction, and a crash between commit and derivation would lose them.
- One item per reason rather than per task: noisier, and the strongest reason already says the most.
