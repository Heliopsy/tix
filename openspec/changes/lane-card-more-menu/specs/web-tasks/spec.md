## ADDED Requirements

### Requirement: Every lane card has a more actions menu the View settings cannot hide

Every card in the lanes view SHALL carry a "more actions" button, drawn as three dots, whatever
fields the reader has hidden through the View menu or the column picker. The button SHALL open a
popover menu inside the card with the same behaviour as the card's other menus: Escape SHALL close
the menu and return focus to the button without leaving the page, no keyboard shortcut SHALL act
while it is open, a live refresh SHALL wait while it is open, and on a coarse pointer the button and
every item SHALL answer a press at least 40px tall. While the menu is open the Down and Up arrow keys
SHALL move focus between its items.

#### Scenario: The dots survive every hidden field

- **GIVEN** a reader who has hidden every optional task field
- **WHEN** the lanes view is drawn
- **THEN** every card still has its more actions button and menu

#### Scenario: Escape closes only the menu

- **GIVEN** the more actions menu of a card is open
- **WHEN** the reader presses Escape
- **THEN** the menu closes, focus is on the card's more actions button, and the page stays

### Requirement: The more actions menu gives full control of the task in place

The menu SHALL offer, to a reader allowed each one: opening the task, honouring the reader's choice
of page or dialog; copying the task's link and reference; completing or reopening it along the
routes the tick offers, or the reason the tick gives when it offers none; every move the status
menu offers; a link to the task for each move that needs a comment; every priority; starring or
unstarring; renaming the title in place; setting the assignee; adding and removing tags; and a soft
delete. A reader without the scope for an action SHALL NOT be offered it: moves need the transition
scope, priority, rename, assignee and tags need task write, delete needs task delete, and starring
needs task read.

Each action SHALL be a form post carrying the CSRF token and the listing to return to, or a link,
and SHALL work without scripting. Under htmx a form SHALL swap only its own card, mark the card busy
while the request is in flight, and show a refusal inside the card. A delete SHALL bring the toast
with its Undo, and the Undo SHALL return the reader to the listing with the card restored.

#### Scenario: Changing the priority from the menu

- **GIVEN** a reader who may write tasks, with Priority hidden
- **WHEN** they choose High in a card's more actions menu
- **THEN** the task's priority is high and the reader returns to the same listing at that card

#### Scenario: Setting the assignee from the menu

- **GIVEN** a reader who may write tasks
- **WHEN** they submit an assignee in a card's more actions menu
- **THEN** the task is assigned to that actor and the reader returns to the listing at that card

#### Scenario: Adding and removing a tag from the menu

- **GIVEN** a reader who may write tasks, with Tags hidden
- **WHEN** they add a tag in the menu and then remove it
- **THEN** each change is saved and each returns to the listing at that card

#### Scenario: Deleting from the menu offers Undo

- **GIVEN** a reader who may delete tasks
- **WHEN** they delete a task from its card's menu
- **THEN** the task is soft deleted, the listing says so with an Undo
- **AND** the Undo restores the task and returns to the listing

#### Scenario: A read-only reader is offered only what they may do

- **GIVEN** a reader who may read tasks but not write, move or delete them
- **WHEN** the lanes view is drawn
- **THEN** each card's menu offers opening, copying and starring, and no form that changes the task

#### Scenario: A move that needs a comment opens the task

- **GIVEN** a workflow whose route out of the card's state requires a comment
- **WHEN** the card's menu is drawn
- **THEN** that move is offered as a link to the task's screen and not as a form

### Requirement: The task edit, tag and restore routes return to a posted listing

The task edit route, the tag and untag routes, and the restore route SHALL, when the form posts a
`next` listing, redirect to that listing at the task's card or row with the outcome in a toast.
Without `next` each SHALL redirect to the task's own screen as before.

#### Scenario: Editing without next stays on the task screen

- **GIVEN** the task screen's edit form, which posts no next
- **WHEN** it is saved
- **THEN** the reader is returned to the task's screen

#### Scenario: Restoring with next returns to the listing

- **GIVEN** a soft deleted task and a restore form posting a listing as next
- **WHEN** it is submitted
- **THEN** the task is restored and the reader is returned to that listing at the task
