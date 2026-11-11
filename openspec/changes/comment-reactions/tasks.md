# Tasks

## 1. Model and storage

- [x] 1.1 `core.Reaction`, `ReactionSummary`, `CommentReaction`, `Comment.Reactions`, the two event types
- [x] 1.2 Migration `comment_reactions`, store methods on SQLite and PostgreSQL
- [x] 1.3 Store tests on both engines

## 2. Service

- [x] 2.1 `AddCommentReaction`, `RemoveCommentReaction`, reactions on `ListComments` and `EditComment`
- [x] 2.2 Service tests: per reader summary, idempotence, events and audit, no inbox, refusals

## 3. Transports

- [x] 3.1 `wire` route, `httpapi` handlers, `client` methods
- [x] 3.2 Registry entries, `just openapi`
- [x] 3.3 HTTP and client round trip tests

## 4. CLI and TUI

- [x] 4.1 `tix comment react` and `unreact`, the REACTIONS column
- [x] 4.2 TUI: reactions in the thread, `)` picker
- [x] 4.3 CLI and TUI tests

## 5. Web

- [x] 5.1 Reactions and picker under each comment, htmx and no-JS, live subscription
- [x] 5.2 Stylesheet part, Bulgarian strings
- [x] 5.3 Web tests

## 6. Docs

- [x] 6.1 `docs/api.md`, `docs/web-ui.md`, `docs/features.md`, `docs/tui.md`, README, the skill
