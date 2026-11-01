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
| `ReleaseLease` with a status | `status` to the assignee; its comment as `AddComment` |

Editing only notifies handles that were not already in the old text, so fixing a typo in a comment
does not mention everybody again.

## Mentions

`@handle` where the handle is the actor handle grammar (letters, digits, `.`, `_`, `-`, trailing
punctuation dropped),
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
in `live.js`, subscribed to `inbox.updated` only. Pages carry no actor identifier (guards elsewhere
assert that a raw identifier never reaches a screen), so the badge cannot compare recipients itself;
every `inbox.updated` asks `/inbox/badge`, coalesced, and the server answers for whoever is reading.
Without scripting the count is as current as the page.

## Terminal

An `inbox` view on `I`: `enter` marks read and opens the task, `space` toggles read state, `a` marks
everything read. Mutes have no terminal screen; the registry records that as an exemption, since
mutes are a setting and the terminal has no settings for the account.

## Personal channels

A channel is a row in `inbox_channels` owned by one actor. `deliverInbox` queues one row in
`inbox_deliveries` per created item and matching enabled channel of its recipient, in the same
transaction, and adds to the mutation's queued count so the inline drain runs after commit. The queue is a
copy of `webhook_deliveries` with `channel_id` and `item_id` instead of endpoint and event, because the
existing table's foreign key and uniqueness are on webhook endpoints.

The webhook `Dispatcher` claims tenant deliveries first and channel deliveries in the room left, so the
server ticker, the inline drain, the retry schedule, the SSRF guard and the HMAC signature are the same
code. `ShapeChannelRequest` renders the message per format: Slack escapes its reserved characters, Discord
disables mentions, ntfy sends text with `Title` and `Click`, and generic sends signed JSON with the item.
A link to the task is built from the tenant's first domain; with none, messages carry the reference only.

The secret is generated at creation and returned once, like a token's. Audit snapshots cut the URL to its
scheme and host, because a Slack or Discord webhook URL is a credential.

## Live list

The inbox page renders the shared `live-screen` binding with `inbox.updated` as its filter and the bar and
list as its regions, so `live.js` refetches and swaps them with the focus and scroll handling every live
screen has. `live.js` now announces a redraw with `tix:regions`, and `inbox.js` highlights the rows that
were not there before.

## Alternatives

- Deriving items from the outbox after commit: rejected, the brief requires the items to be written
  in the triggering transaction, and a crash between commit and derivation would lose them.
- One item per reason rather than per task: noisier, and the strongest reason already says the most.
