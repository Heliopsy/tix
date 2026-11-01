## ADDED Requirements

### Requirement: Inbox changes emit inbox.updated

A change that creates inbox items SHALL emit one `inbox.updated` event in its transaction whose
payload `recipients` lists the actor identifiers that received an item. Marking items read or
unread, and changing mutes, SHALL emit `inbox.updated` naming the actor. `inbox.updated` SHALL be
nameable in a subscription filter.

#### Scenario: A comment that notifies two people

- **WHEN** a comment creates items for bob and carol
- **THEN** the stream carries `comment.added` and one `inbox.updated` whose recipients are bob and carol
