## 1. Model and storage

- [x] 1.1 `comment_reactions` migration
- [x] 1.2 `core.Reaction`, `core.CommentReaction`, `Comment.Reactions`, `comment.reacted` and `comment.unreacted` events
- [x] 1.3 Store methods on both engines: add, remove, clear, list

## 2. Service

- [x] 2.1 `AddReaction` and `RemoveReaction` with single-emoji validation, audit and event
- [x] 2.2 Reactions on listed and edited comments
- [x] 2.3 Deleting a comment clears its reactions
- [x] 2.4 Unit tests, including the emoji grammar

## 3. Surfaces

- [x] 3.1 Registry entries `comment.react` and `comment.unreact`
- [x] 3.2 HTTP routes, client, `just openapi`
- [x] 3.3 `tix comment react ID EMOJI [--remove]`, REACTIONS column in the comment table
- [x] 3.4 Web chips, picker, live update, Bulgarian strings
- [x] 3.5 TUI reactions line and `&` key
- [x] 3.6 Transport matrix scenarios

## 4. Docs

- [x] 4.1 features.md, api.md, web-ui.md, tui.md, skill
