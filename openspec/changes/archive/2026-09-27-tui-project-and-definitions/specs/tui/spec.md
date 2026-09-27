## ADDED Requirements

### Requirement: The terminal interface states how a project is configured

The terminal interface SHALL offer a screen that states, for one project, the project's own attributes, the
workflow its tasks move through and the custom field definitions those tasks carry.

The screen SHALL read the project it shows rather than reusing a row from a listing, so that what it states is
true after an action it performed.

The workflow SHALL be shown as its state machine: the initial state, every state with the category it reports
under and whether it is terminal, every permitted edge with what that edge requires, and the lease a claim
takes by default.

An attribute that is not set SHALL be shown as unset rather than as an empty row, so a blank cannot be
mistaken for a row that failed to draw.

A part of the screen the reader's authority does not reach SHALL say why it is not shown, and SHALL NOT cost
the reader the rest of the screen.

#### Scenario: The screen names the project it read

- **WHEN** a reader opens the project screen
- **THEN** it states that project's key, name, description, colour, icon and whether it is archived
- **AND** the project was read rather than taken from the listing

#### Scenario: The workflow is shown as a state machine

- **WHEN** the project screen is open
- **THEN** it names the workflow, its initial state, each of its states and each permitted edge
- **AND** marks a state a task stops in as terminal

#### Scenario: An archived project says when it was archived

- **WHEN** the project screen shows an archived project
- **THEN** it states that it is archived and when

#### Scenario: A refused workflow costs nothing else

- **WHEN** the reader may read the project but not its workflow
- **THEN** the screen says why the workflow is not shown
- **AND** still states the project's own attributes

#### Scenario: The screen opens on the project the reader chose

- **WHEN** a reader presses the key from the project listing
- **THEN** the screen opens on the row under the cursor
- **AND** pressing it from a board opens the project the board has open

### Requirement: A workflow is read in the terminal and changed elsewhere

The terminal interface SHALL NOT offer an editor for a workflow definition, and the screen that renders a
workflow SHALL name where a workflow is changed instead.

#### Scenario: The screen points at the command that edits a workflow

- **WHEN** the project screen renders a workflow
- **THEN** it names the command that changes one

### Requirement: The terminal interface edits every attribute a project accepts

The terminal interface SHALL offer, from the project screen, a change to each attribute the project update
accepts. An attribute whose values are a fixed set SHALL be answered from that set; an attribute that is free
text SHALL be answered by the single-line prompt, seeded with the value it would replace.

An attribute with no alternative to offer SHALL NOT be offered, and a change SHALL send only the attribute it
was asked for.

#### Scenario: A colour is chosen from the palette

- **WHEN** a reader chooses the colour attribute and steps its value
- **THEN** the value on screen is the value sent
- **AND** no other attribute is sent with it

#### Scenario: A name is typed into the prompt it opens

- **WHEN** a reader chooses the name attribute
- **THEN** the prompt opens seeded with the project's current name
- **AND** accepting it sends only the name

#### Scenario: One workflow is no choice

- **WHEN** the tenant has a single workflow
- **THEN** the project screen offers no workflow to move to

### Requirement: Archiving and deleting a project are confirmed against the named project

The terminal interface SHALL ask, before hiding or destroying a project, which of the two is meant, and SHALL
then confirm it against the project named as the reader sees it.

The confirmation for a deletion SHALL state how far the deletion reaches. The confirmation for an archive
SHALL NOT claim a deletion's reach.

An archive SHALL NOT be offered for a project that is already archived, because the service refuses it.

Once a project is deleted the interface SHALL leave the screen that showed it rather than reading a project
the service has removed.

#### Scenario: An archive names the project and nothing more

- **WHEN** a reader asks to archive the open project
- **THEN** the confirmation names that project and says it will archive it
- **AND** agreeing archives it and deletes nothing

#### Scenario: A deletion states its reach

- **WHEN** a reader asks to delete the open project
- **THEN** the confirmation names the project and states what goes with it
- **AND** agreeing deletes it and returns the reader to the project listing

#### Scenario: An archived project is not offered archiving

- **WHEN** the project screen shows an archived project
- **THEN** archiving is not among the actions offered

#### Scenario: Cancelling leaves the project alone

- **WHEN** a confirmation about a project is cancelled
- **THEN** the project is neither archived nor deleted

### Requirement: The terminal interface defines and removes a project's custom fields

The terminal interface SHALL define a custom field from the project screen, gathering its key and label as
text and the answers drawn from fixed sets in a form.

A redefinition SHALL be seeded from the definition it replaces, and SHALL carry through every part of that
definition the form did not ask about, because the operation replaces the whole definition.

A field type whose values the form cannot gather SHALL be offered only to a definition that already holds it.

Removing a definition SHALL be confirmed against the field named as the screen shows it.

A picker SHALL NOT be offered where there is nothing to pick.

#### Scenario: A new field is named and then shaped

- **WHEN** a reader defines a new custom field
- **THEN** the key and the label are taken as text
- **AND** the type and whether it is required are answered from their own lists
- **AND** the definition sent is one the operation accepts

#### Scenario: A redefinition starts from the field that was picked

- **WHEN** a reader picks the second of two definitions to redefine
- **THEN** the form opens on that definition's own type and requirement
- **AND** the parts the form did not ask about are sent unchanged

#### Scenario: An enumerated type is not offered to a field that cannot carry it

- **WHEN** a reader defines a new field
- **THEN** the type list omits the type whose values the form cannot gather
- **AND** a definition that already holds that type keeps it

#### Scenario: Removing a definition names it

- **WHEN** a reader asks to remove a custom field definition
- **THEN** the confirmation names that field
- **AND** agreeing removes that definition and no other

#### Scenario: A project with no definitions offers no picker

- **WHEN** the project defines no custom fields
- **THEN** the picker is not advertised
- **AND** the key that defines a new one still is

### Requirement: Every project affordance is gated by the authority it needs

The terminal interface SHALL offer each action on the project screen only to a reader who holds the authority
the operation it calls requires, and SHALL refuse the keystroke on the same question the footer and the help
overlay are filtered by.

An action the screen borrows a key for SHALL be described by what it does on that screen.

#### Scenario: A reader who may only read is offered nothing to change

- **WHEN** a reader who may read a project but not write one opens the screen
- **THEN** no action key is advertised
- **AND** pressing one opens no input and reaches no service call

#### Scenario: The overlay narrows to the reader

- **WHEN** a reader may edit a project and nothing else on the screen
- **THEN** the overlay names the edit and none of the others

#### Scenario: A borrowed key is described by what it does here

- **WHEN** the overlay describes the project screen
- **THEN** the edit key is described as editing a project
