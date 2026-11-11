# Reactions on comments

## Why

A comment that only needs an acknowledgement ("+1", "seen", "nice") costs a whole new comment today, which
pads the thread and notifies everybody following the task. A small fixed set of emoji reactions, the way
GitHub does it, lets a reader acknowledge a comment without writing one.

## What Changes

- A comment can carry reactions from a fixed set of eight: `thumbs_up` 👍, `thumbs_down` 👎, `laugh` 😄,
  `hooray` 🎉, `confused` 😕, `heart` ❤️, `rocket` 🚀, `eyes` 👀. Each actor gives each reaction at most
  once and can take it back; a repeat of either writes nothing.
- New service operations `AddCommentReaction` and `RemoveCommentReaction`, authorised as `comment:write` on
  the comment's task, the same as commenting. Both answer the comment with its reactions summed up.
- Every comment read (`ListComments`, an edit, a reaction) carries `reactions`: per reaction given, its
  emoji, count, whether the caller gave it and who did, in the fixed order.
- New event types `comment.reacted` and `comment.unreacted`, with audit actions `comment.react` and
  `comment.unreact`. A reaction creates no inbox item.
- HTTP `PUT` and `DELETE /api/v1/comments/{id}/reactions/{reaction}`; MCP derives its tools from them.
- CLI `tix comment react ID REACTION` and `tix comment unreact ID REACTION`; `comment ls` gains a
  REACTIONS column.
- The browser draws reactions under each comment as toggles, with a picker of all eight; htmx swaps only that
  comment's reactions and the task screen's live connection redraws them.
- The terminal's task view shows reactions under each comment and `)` opens a numbered reaction picker.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `data-model`: the comment reactions table and the reactions on a comment.
- `http-api`: the reaction routes and the `reactions` field on a comment.
- `event-stream`: the two new event types.
- `cli`: `tix comment react` and `unreact`.
- `tui`: reactions in the thread and the reaction picker.
- `web-tasks`: reactions under each comment on the task screen.

## Impact

- `internal/core` (`Reaction`, `ReactionSummary`, `CommentReaction`, `Comment.Reactions`, two `Service`
  methods, two event types), `internal/store` (migration, both engines), `internal/service`
  (`reaction.go`, comment reads), `internal/wire`, `internal/httpapi`, `internal/client`,
  `internal/capability`, `api/openapi.yaml`, `cmd/relations.go`, `internal/output`, `internal/tui`,
  `internal/web`, `internal/i18n`, docs and the skill.
- One migration, `comment_reactions`, portable to both engines. No data changes.
