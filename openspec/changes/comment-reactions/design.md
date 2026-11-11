## Storage

`comment_reactions(tenant_id, comment_id, actor_id, emoji, created_at)` with the primary key on all four
identifying columns, so "each emoji once per actor" is the key rather than a check. Adding reads before it
writes, as starring does, because a failed insert on PostgreSQL aborts the transaction the audit entry is in.

Comments are soft deleted, so the cascade from `comments` never fires for an ordinary delete. The service
clears the rows itself in the same transaction as the delete.

## Shape on the wire

A comment gains `reactions: [{emoji, count, actor_ids}]`, grouped by emoji in the order each emoji was first
used, and omitted when empty. Actor IDs rather than handles, because every surface already resolves IDs to
names for comment authors and a reader can tell whether it reacted by finding its own ID.

The routes put the emoji in the path, `PUT` and `DELETE /comments/{id}/reactions/{emoji}`, the same idempotent
shape as a task's star. Both answer the comment with its reactions so a client redraws from the answer
without a second read.

## Validation

There is no grapheme segmentation in the standard library and the project avoids new dependencies, so the
service accepts a deliberately small grammar: an emoji base with an optional variation selector and skin
tone, joined by zero-width joiners; a regional indicator pair; a keycap; or a black flag with a tag
sequence. That covers the quick picker and what people type, and refuses words, several emoji and bare
modifiers. The bound is 64 bytes.

## Events

Two new types, `comment.reacted` and `comment.unreacted`, rather than overloading `task.updated`: a webhook
subscriber that cares about reactions can select them, and one that does not is not woken by them. The web
task screen's live subscription includes both, so chips update on every open screen.

## Terminal key

`&` on the detail view opens a one-line prompt for an emoji. An emoji the reader already left on the
selected comment is taken back; anything else is added. Every letter near `m` was taken.

## Not done

No notifications, by request: a reaction is the quiet alternative to a comment.
