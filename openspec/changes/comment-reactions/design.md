# Design

## A fixed set, named

Reactions are a closed set of eight, named in ASCII (`thumbs_up`, ..., `eyes`) with the emoji as a property
of the name. A name is what goes in a URL path, a CLI argument and the database, where an emoji would need
escaping and, for the heart, carries an invisible variation selector that some keyboards omit. Input still
accepts the emoji itself, with or without that selector, and any case of the name. GitHub's `+1` and `-1`
were not used: a CLI argument starting with `-` reads as a flag.

A closed set rather than any emoji keeps the summary short, the picker one row of eight, and validation a
table lookup.

## Storage

`comment_reactions(tenant_id, comment_id, actor_id, reaction, created_at)` with the key on all four of the
first, so a reaction is one row per actor per comment and the key's prefix serves the read by comment. Every
foreign key cascades. A soft-deleted comment keeps its rows, which no read reaches while it stays deleted.
Adding reads before it writes, as a task star does, because on PostgreSQL a failed insert aborts the
transaction the audit entry is in.

The read takes a list of comment ids, so a thread of any length is one query. It is not paginated: it is
bounded by the comments asked for, eight reactions and the tenant's actors.

## Service

`AddCommentReaction` and `RemoveCommentReaction` authorise `comment:write` on the comment's task, then
change one row. Only a change writes the audit entry and the event, so a repeat is silent. Both answer the
comment with its reactions summed up for the caller, which is what a client needs to reconcile an
optimistic toggle.

`ListComments` and `EditComment` add the summary for the reader. Exports, snapshots and task copies read the
store directly and carry no reactions: a reaction is a reader's acknowledgement, not part of the record.

## Notification

A reaction notifies nobody: no inbox item, so no notification channel either. Reacting is the cheap way to
say "seen", and a notification for every one would make it as loud as the comment it replaces. Webhooks and
the event stream see `comment.reacted` and `comment.unreacted` like any other event, with the comment as the
subject and `ref`, `task_id`, `comment_id` and `reaction` in the payload.

## Browser

Under each comment, one pill per reaction given (emoji and count, the reader's own outlined in the accent with
`aria-pressed`) and a button opening a popover of all eight. Every control is a plain form posting to
`/tasks/{ref}/comments/react` or `/unreact` that redirects back to the comment, so it works without a script;
with htmx the form names its comment's reactions region as both target and selection, so only that region
is swapped. The task screen subscribes to both event types, so a reaction given elsewhere redraws the thread.
A reader without `comment:write` sees the pills as text.

## Terminal

The thread draws a line of reactions under each comment, the reader's own in brackets. `)` (the mouth of a
`:)`) opens the numbered picker the priority and share pickers use, labelled with each reaction's count and
whether it is yours; picking one of yours takes it back.
