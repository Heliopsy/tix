## ADDED Requirements

### Requirement: The terminal has an integrations view with a tab per scope

The terminal interface SHALL offer an integrations view, opened by one cross-view key, holding a webhooks tab,
a sync tab and a retention tab. A tab SHALL be offered only when the reader may perform its read, and the
arrow keys that move between columns on the board SHALL move between the offered tabs.

#### Scenario: A sync administrator sees only the sync tab

- **GIVEN** a reader holding `sync:admin` and neither `webhook:admin` nor `tenant:admin`
- **WHEN** the reader opens the integrations view
- **THEN** the sync tab is shown
- **AND** neither the webhooks tab nor the retention tab is named
- **AND** no webhook or retention read is sent

#### Scenario: A reader with none of the three scopes is not offered the view

- **WHEN** a reader holding none of the three scopes presses the key
- **THEN** the view does not open and the help overlay does not name the key

### Requirement: The webhooks tab lists endpoints and deliveries under one cursor

The webhooks tab SHALL list every endpoint with its URL, whether it is active and the event types it receives,
followed by the newest page of deliveries with status, attempts, last error and the endpoint delivered to. One
cursor SHALL run over both, and a log longer than the page SHALL say so and name `tix webhook deliveries`.

#### Scenario: Moving past the last endpoint selects the first delivery

- **WHEN** a reader moves down from the last endpoint
- **THEN** the first delivery is selected

### Requirement: The webhooks tab registers, edits, deletes and redelivers

The webhooks tab SHALL register an endpoint from a form of URL, event types, active and signing secret, edit
the selected endpoint from the same form seeded with its values, delete it behind a confirmation naming its
URL, and redeliver the selected delivery. A secret tix generates SHALL be shown once on the tab.

#### Scenario: Registering with no secret shows the generated one once

- **WHEN** a reader registers an endpoint leaving the secret blank
- **THEN** the endpoint is registered and the tab shows the secret tix returned
- **AND** leaving and reopening the view no longer shows it

#### Scenario: Deleting names the URL

- **WHEN** a reader presses the removal key on an endpoint
- **THEN** the confirmation names its URL and nothing is deleted until the reader agrees

#### Scenario: Redelivering the selected delivery

- **WHEN** a reader presses enter on a delivery
- **THEN** that delivery is queued again

### Requirement: Secrets in terminal forms are write-only

A signing secret or sync credential SHALL be gathered by a masked field, drawn without its characters, never
seeded with a stored value, and sent only when typed, so a blank field keeps what is stored.

#### Scenario: Editing an endpoint keeps its secret

- **WHEN** a reader edits an endpoint and leaves the secret blank
- **THEN** the update carries no secret and the stored one is kept

#### Scenario: A typed credential is masked

- **WHEN** a reader types a token into the sync source form
- **THEN** the form never draws the token's characters

### Requirement: The sync tab lists sources with what a run would use

The sync tab SHALL list every source with its name, system, last run and status, its stored settings, which
credentials are stored with their hints, where its mapping and credentials come from on the server and what a
run still lacks.

#### Scenario: A source missing a setting says so

- **WHEN** the server reports a source lacking its URL
- **THEN** the source's rows name what is missing

### Requirement: The sync tab configures, sets up, checks and deletes sources

The sync tab SHALL configure a source from a form of name, system, the settings that system takes, mapping path,
mapping document and credentials, set up the selected source from the same form seeded with its stored values,
check a source without saving it, and delete the selected source behind a confirmation naming it.

#### Scenario: Only the chosen system's settings are offered

- **WHEN** the reader picks the generic system
- **THEN** the form offers a file and no URL, project or query

#### Scenario: A check saves nothing

- **WHEN** a reader submits the form with check only set
- **THEN** the source is validated with nothing written and the tab draws what a run would use

### Requirement: The sync tab runs a source

Enter on a source SHALL open a form of dry run, on by default, and full, and SHALL run the source with those
answers. The run's counts, warnings, unmatched people and cursor, or the reason it was refused, SHALL be drawn
on the tab.

#### Scenario: A dry run is the default

- **WHEN** a reader presses enter on a source and accepts the form unchanged
- **THEN** the run is sent as a dry run and the tab says nothing was written

### Requirement: The retention tab edits the windows and prunes

The retention tab SHALL show the events, audit entries and webhook deliveries windows, edit them from a form
read by the duration grammar the CLI uses, a blank window keeping its value, and prune from a form of dry run,
on by default, and a limit. A prune that writes SHALL ask first, and its counts SHALL be drawn on the tab.

#### Scenario: A blank window is kept

- **WHEN** a reader edits the retention windows and blanks the audit window
- **THEN** the update leaves the audit window unset so the stored value is kept

#### Scenario: A real prune asks first

- **WHEN** a reader turns dry run off and submits the prune form
- **THEN** a confirmation opens and nothing is pruned until the reader agrees

### Requirement: The integrations view offers only what the reader's authority reaches

Each integrations action SHALL be gated on the operation it calls, in the footer, in the help overlay and at
the keystroke, so that none of the three offers what the others refuse.

#### Scenario: A reader who may list but not write is offered no writes

- **WHEN** a reader who may list webhooks but not change them opens the webhooks tab
- **THEN** the footer offers neither register, edit, delete nor redeliver
- **AND** pressing those keys opens nothing and sends nothing
