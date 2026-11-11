## Why

Acknowledging a comment today means writing another one. "👍" as a comment notifies the assignee and
everyone who starred the task, and buries the thread in noise. People want a cheap, quiet way to say
"seen" or "agreed" on a comment, and the mobile app wants the same.

## What Changes

- A reaction is one actor's one emoji on one comment. An actor can leave several different emoji on a
  comment, each once; reacting again changes nothing, and removing it takes it back.
- The server accepts only a single emoji grapheme.
- New service operations `AddReaction` and `RemoveReaction` (`comment.react`, `comment.unreact`), bound on
  every surface: `tix comment react ID EMOJI [--remove]`, `PUT` and `DELETE /comments/{id}/reactions/{emoji}`,
  the task screen's reaction chips and picker, the terminal's `&` key, MCP through the generated tools, and
  SSH through the CLI.
- A comment read carries its reactions grouped by emoji, with a count and the actors who reacted.
- Each change writes an audit entry and a `comment.reacted` or `comment.unreacted` event, and notifies nobody.
- Deleting a comment removes its reactions.
- A new `comment_reactions` table.

## Impact

- Specs: task-management, event-stream, web-tasks, tui.
- Code: core, store (both engines), service, capability registry, httpapi, client, cmd, web, tui, output.
- API: two new routes and an optional `reactions` field on a comment. Existing clients are unaffected.
