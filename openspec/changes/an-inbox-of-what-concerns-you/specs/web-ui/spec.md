## ADDED Requirements

### Requirement: Inbox in the web interface

The sidebar SHALL carry an Inbox entry with a badge counting unread items, hidden at zero, refreshed
live over the event stream when an `inbox.updated` event names the reader. The inbox page SHALL list
the reader's items newest first, each naming the task, the actor, what happened and when, unread
items marked, with a filter for unread only, per-item mark read and mark unread, mark all read, and
keyset paging. Opening an item SHALL mark it read and open the task the way the reader's dialog
setting says. Every action SHALL work without scripting and swap in place with it. Settings SHALL
offer one switch per kind to mute it.

#### Scenario: The badge counts unread items

- **GIVEN** a reader with two unread items
- **WHEN** they open any page
- **THEN** the sidebar's Inbox entry shows 2

#### Scenario: Opening an item

- **WHEN** a reader opens an unread item
- **THEN** the item is marked read and the task opens

#### Scenario: Muting from settings

- **WHEN** a reader turns off status changes in Settings
- **THEN** `status` is muted for them

### Requirement: Mention completion

Comment and description boxes SHALL offer the tenant's handles when the reader types `@` followed by
the start of a handle, filtered as they type, chosen with the arrow keys and Enter or Tab, dismissed
with Escape without closing anything else. Without scripting the box SHALL stay a plain text box.

#### Scenario: Completing a handle

- **WHEN** a reader types "@bo" in a comment box and presses Enter on "bob"
- **THEN** the box reads "@bob " and the comment is not submitted
