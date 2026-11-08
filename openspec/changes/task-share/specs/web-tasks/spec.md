## ADDED Requirements

### Requirement: The task screen has a Share menu

The task screen's header, as a page and in the task dialog, SHALL carry a Share button named for the
task that opens a menu offering Copy link, Copy as Markdown and Copy key and title, and Share… where
the browser offers a system share sheet. The menu SHALL close on Escape and SHALL be usable from
the keyboard.

#### Scenario: The menu

- **GIVEN** task INFRA-12 titled "Fix the build"
- **WHEN** a reader presses Share on its screen
- **THEN** a menu offers Copy link, Copy as Markdown and Copy key and title

#### Scenario: A phone offers its share sheet

- **GIVEN** a browser with `navigator.share`
- **WHEN** the reader chooses Share…
- **THEN** the system share sheet opens with the task's title and address

### Requirement: Each copy says what it copied

Copy link SHALL copy the task's absolute address, Copy as Markdown SHALL copy
`[INFRA-12 Fix the build](URL)` with `\`, `[` and `]` in the title escaped and the title on one
line, and Copy key and title SHALL copy `INFRA-12 Fix the build`. Each SHALL report its outcome in
a toast that assistive technology announces, and SHALL return focus to the Share button.

#### Scenario: Link copied

- **WHEN** the reader chooses Copy link
- **THEN** the clipboard holds the task's absolute address
- **AND** a toast says "Link copied"

### Requirement: A shared link opens the task for a member of the same tenant

The shared address SHALL be the task's screen, `/tasks/REF`, on the scheme and host the reader is
on, and SHALL name no tenant. On a shared host it SHALL open the task for another member of the
same tenant whose session is in that tenant.

#### Scenario: A member of the tenant opens the link

- **GIVEN** a shared host serving tenants acme and beta, and a link copied by a member of acme
- **WHEN** another member of acme, signed in to acme there, opens it
- **THEN** they see the task

### Requirement: The Share menu works without scripting

Without scripting, the Share button SHALL still open the menu, and the menu SHALL show the task's
absolute address as a plain link the reader can select. Controls that need a script SHALL NOT be
drawn.

#### Scenario: No script

- **GIVEN** a browser with scripting off
- **WHEN** the reader opens the Share menu
- **THEN** it shows the address as a link and no copy buttons

### Requirement: A lane card's menu shares the task

A lane card's more actions menu SHALL offer Copy as Markdown and Copy key and title beside Copy
link and Copy reference, and Share… where the browser offers a system share sheet.

#### Scenario: Markdown from a card

- **WHEN** the reader chooses Copy as Markdown in a card's more actions menu
- **THEN** the clipboard holds the card's task as a Markdown link

## MODIFIED Requirements

### Requirement: One actions menu holds every action on the task as a whole

The header SHALL open one actions menu and the Share menu. The actions menu SHALL hold Copy the
task id, and for a writer Copy to, Save as template and Snooze (or Unsnooze for a snoozed task),
Reclaim the lease where the reader may reclaim a live lease, and Delete with its permanent and
subtask options where the reader may delete. Copy link SHALL be in the Share menu. None of these
SHALL be drawn anywhere else on the screen.

#### Scenario: The menu

- **WHEN** a writer opens the actions menu
- **THEN** it offers Copy the task id, Copy to, Save as template, Snooze and Delete

#### Scenario: No second copy

- **WHEN** the task screen is drawn
- **THEN** Copy link, Copy to, Save as template and Delete each appear once, in their menu
