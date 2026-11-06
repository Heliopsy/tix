## ADDED Requirements

### Requirement: Inbox view in the terminal interface

The terminal interface SHALL offer an Inbox view listing the reader's items newest first, unread
ones marked, from which Enter opens the task and marks the item read, a key toggles read state, and
a key marks everything read.

#### Scenario: Opening an item from the terminal

- **WHEN** a reader selects an unread item in the Inbox view and presses Enter
- **THEN** the task opens and the item is read

### Requirement: Channels are not managed in the terminal

The terminal interface SHALL NOT offer notification channels, and the registry SHALL record that as an
exemption naming the browser and `tix inbox channel`.

#### Scenario: The registry says why

- **WHEN** the capability registry is read for `inbox.channel.add`
- **THEN** its terminal binding is an exemption, not a gap
