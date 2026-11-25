## ADDED Requirements

### Requirement: Emoji reactions on comments

An actor with `comment:write` SHALL be able to add an emoji reaction to a live comment and remove it again.
An actor MAY leave several different emoji on one comment but each only once: adding one already there and
removing one not there SHALL change nothing and record nothing.

#### Scenario: Two emoji, then one taken back

- **GIVEN** a comment
- **WHEN** an actor reacts with 👍, again with 👍, then with 🎉, and removes 🎉
- **THEN** the comment carries one 👍 by that actor and no 🎉

#### Scenario: A deleted comment

- **WHEN** an actor reacts to a deleted or unknown comment
- **THEN** the call is refused as not_found

### Requirement: A reaction is a single emoji

The service SHALL accept as a reaction only one emoji grapheme, after trimming surrounding space, and SHALL
refuse anything else as invalid, including words, several emoji and a bare modifier.

#### Scenario: A word

- **WHEN** an actor reacts with `ok`
- **THEN** the call is refused as invalid and nothing is written

#### Scenario: A joined emoji

- **WHEN** an actor reacts with 👩‍💻 or 👍🏽 or 🇧🇬
- **THEN** the reaction is recorded

### Requirement: Comments carry their reactions

A comment read SHALL carry its reactions grouped by emoji, in the order each emoji was first used, each
with its count and the actors who reacted, earliest first. A comment nobody reacted to SHALL omit them.

#### Scenario: Listing a thread

- **GIVEN** two actors reacted 👍 to a comment
- **WHEN** the task's comments are listed
- **THEN** that comment carries 👍 with a count of 2 and both actors

### Requirement: Reactions are recorded and quiet

Each reaction added or removed SHALL write an audit entry and its event in the same transaction, and SHALL
create no inbox item or notification for anyone.

#### Scenario: Reacting to a comment on an assigned task

- **WHEN** someone reacts to a comment on a task assigned to somebody else
- **THEN** one audit entry and one event are written and the assignee's inbox is unchanged

### Requirement: Deleting a comment removes its reactions

Deleting a comment SHALL remove every reaction on it in the same transaction.

#### Scenario: Delete

- **GIVEN** a comment with reactions
- **WHEN** the comment is deleted
- **THEN** no reaction on it remains
